package vips

/*
#cgo pkg-config: vips
#include "vips.h"
*/
import "C"

import (
	"bytes"
	"errors"
	"fmt"
	img "image"
	"image/jpeg"
	"image/png"

	"math"
	"os"
	"runtime"
	"unsafe"

	"github.com/chai2010/webp"
	"github.com/muesli/smartcrop"
	"github.com/muesli/smartcrop/nfnt"
)

const DEBUG = false

var (
	MARKER_JPEG = []byte{0xff, 0xd8}
	MARKER_PNG  = []byte{0x89, 0x50}
	MARKER_GIF  = []byte{0x47, 0x49, 0x46}
	MARKER_WEBP = []byte{0x57, 0x45, 0x42, 0x50}
)

type ImageType int

const (
	UNKNOWN ImageType = iota
	JPEG
	PNG
	GIF
	WEBP
)

type Interpolator int

const (
	BICUBIC Interpolator = iota
	BILINEAR
	NOHALO
)

type Extend int

const (
	EXTEND_BLACK Extend = C.VIPS_EXTEND_BLACK
	EXTEND_WHITE Extend = C.VIPS_EXTEND_WHITE
)

var interpolations = map[Interpolator]string{
	BICUBIC:  "bicubic",
	BILINEAR: "bilinear",
	NOHALO:   "nohalo",
}

func (i Interpolator) String() string { return interpolations[i] }

type Options struct {
	Height       int
	Width        int
	Crop         bool
	FeatureCrop  bool
	Enlarge      bool
	Extend       Extend
	Interlaced   bool
	Embed        bool
	Interpolator Interpolator
	BlurAmount   float32
	Gravity      Gravity
	Quality      int
	Format       ImageType
}

func init() {
	Initialize()
}

var initialized bool

func Initialize() {
	if initialized {
		return
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := C.vips_initialize(); err != 0 {
		C.vips_shutdown()
		panic("unable to start vips!")
	}

	C.vips_concurrency_set(8)
	C.vips_cache_set_max_mem(100 * 1048576) // 100Mb
	C.vips_cache_set_max(500)

	initialized = true
}

func Shutdown() {
	if !initialized {
		return
	}

	C.vips_shutdown()

	initialized = false
}

func Debug() {
	C.im__print_all()
}

func Resize(buf []byte, o Options) ([]byte, error) {
	debug("%#+v", o)

	// Empty buffers panic, return to sender
	if len(buf) == 0 {
		return buf, nil
	}

	// detect (if possible) the file type
	typ := UNKNOWN
	switch {
	case bytes.Equal(buf[:2], MARKER_JPEG):
		typ = JPEG
	case bytes.Equal(buf[:2], MARKER_PNG):
		typ = PNG
	case bytes.Equal(buf[:3], MARKER_GIF):
		typ = GIF
	case bytes.Equal(buf[8:12], MARKER_WEBP):
		typ = WEBP
	default:
		return nil, errors.New("unknown image format")
	}

	// create an image instance
	var image, tmpImage *C.struct__VipsImage

	// feed it
	switch typ {
	case JPEG:
		C.vips_jpegload_buffer_seq(unsafe.Pointer(&buf[0]), C.size_t(len(buf)), &image)
	case PNG:
		C.vips_pngload_buffer_seq(unsafe.Pointer(&buf[0]), C.size_t(len(buf)), &image)
	case GIF:
		C.vips_gifload_buffer_seq(unsafe.Pointer(&buf[0]), C.size_t(len(buf)), &image)
	case WEBP:
		C.vips_webpload_buffer_seq(unsafe.Pointer(&buf[0]), C.size_t(len(buf)), &image)
	}

	// cleanup
	defer func() {
		C.vips_thread_shutdown()
		C.vips_error_clear()
	}()

	// defaults
	if o.Quality == 0 {
		o.Quality = 100
	}

	// get WxH
	inWidth := int(image.Xsize)
	inHeight := int(image.Ysize)

	// prepare for factor
	factor := 0.0

	// Do not enlarge the output if the input width *or* height are already less than the required dimensions
	if !o.Enlarge {
		if inWidth < o.Width {
			o.Width = inWidth
		}

		if inHeight < o.Height {
			o.Height = inHeight
		}
	}

	// image calculations
	switch {
	// Fixed width and height
	case o.Width > 0 && o.Height > 0:
		xf := float64(inWidth) / float64(o.Width)
		yf := float64(inHeight) / float64(o.Height)
		if o.Crop {
			factor = math.Min(xf, yf)
		} else {
			factor = math.Max(xf, yf)
		}
	// Fixed width, auto height
	case o.Width > 0:
		factor = float64(inWidth) / float64(o.Width)
		o.Height = int(math.Floor(float64(inHeight) / factor))
	// Fixed height, auto width
	case o.Height > 0:
		factor = float64(inHeight) / float64(o.Height)
		o.Width = int(math.Floor(float64(inWidth) / factor))
	// Identity transform
	default:
		factor = 1
		o.Width = inWidth
		o.Height = inHeight
	}

	debug("transform from %dx%d to %dx%d", inWidth, inHeight, o.Width, o.Height)

	// shrink
	shrink := int(math.Floor(factor))
	if shrink < 1 {
		shrink = 1
	}

	// residual
	residual := float64(shrink) / factor

	debug("factor: %v, shrink: %v, residual: %v", factor, shrink, residual)

	// Try to use libjpeg shrink-on-load
	shrinkOnLoad := 1
	if typ == JPEG && shrink >= 2 {
		switch {
		case shrink >= 8:
			factor = factor / 8
			shrinkOnLoad = 8
		case shrink >= 4:
			factor = factor / 4
			shrinkOnLoad = 4
		case shrink >= 2:
			factor = factor / 2
			shrinkOnLoad = 2
		}
	}

	if shrinkOnLoad > 1 {
		debug("shrink on load %d", shrinkOnLoad)
		// Recalculate integral shrink and double residual
		factor = math.Max(factor, 1.0)
		shrink = int(math.Floor(factor))
		residual = float64(shrink) / factor
		// Reload input using shrink-on-load
		err := C.vips_jpegload_buffer_shrink(unsafe.Pointer(&buf[0]), C.size_t(len(buf)), &tmpImage, C.int(shrinkOnLoad))
		C.g_object_unref(C.gpointer(image))
		image = tmpImage
		if err != 0 {
			return nil, resizeError()
		}
	}

	if shrink > 1 {
		debug("shrink %d", shrink)
		// Use vips_shrink with the integral reduction
		err := C.vips_shrink_0(image, &tmpImage, C.double(float64(shrink)), C.double(float64(shrink)))
		C.g_object_unref(C.gpointer(image))
		image = tmpImage
		if err != 0 {
			return nil, resizeError()
		}

		// Recalculate residual float based on dimensions of required vs shrunk images
		shrunkWidth := int(image.Xsize)
		shrunkHeight := int(image.Ysize)

		residualx := float64(o.Width) / float64(shrunkWidth)
		residualy := float64(o.Height) / float64(shrunkHeight)
		if o.Crop {
			residual = math.Max(residualx, residualy)
		} else {
			residual = math.Min(residualx, residualy)
		}
	}

	// Use vips_affine with the remaining float part
	debug("residual: %v", residual)
	if residual != 0 {
		debug("residual %.2f", residual)
		// Create interpolator - "bilinear" (default), "bicubic" or "nohalo"
		is := C.CString(o.Interpolator.String())
		interpolator := C.vips_interpolate_new(is)

		// Perform affine transformation
		err := C.vips_affine_interpolator(image, &tmpImage, C.double(residual), 0, 0, C.double(residual), interpolator)
		C.g_object_unref(C.gpointer(image))

		image = tmpImage

		C.free(unsafe.Pointer(is))
		C.g_object_unref(C.gpointer(interpolator))

		if err != 0 {
			return nil, resizeError()
		}
	}

	// Crop/embed
	affinedWidth := int(image.Xsize)
	affinedHeight := int(image.Ysize)

	if affinedWidth != o.Width || affinedHeight != o.Height {
		if o.Crop && !o.FeatureCrop {
			// Crop
			debug("cropping")
			left, top := sharpCalcCrop(affinedWidth, affinedHeight, o.Width, o.Height, o.Gravity)
			o.Width = int(math.Min(float64(affinedWidth), float64(o.Width)))
			o.Height = int(math.Min(float64(affinedHeight), float64(o.Height)))
			err := C.vips_extract_area_0(image, &tmpImage, C.int(left), C.int(top), C.int(o.Width), C.int(o.Height))
			C.g_object_unref(C.gpointer(image))
			image = tmpImage
			if err != 0 {
				return nil, resizeError()
			}
		} else if o.Embed {
			debug("embedding with extend %d", o.Extend)
			left := (o.Width - affinedWidth) / 2
			top := (o.Height - affinedHeight) / 2
			err := C.vips_embed_extend(image, &tmpImage, C.int(left), C.int(top), C.int(o.Width), C.int(o.Height), C.int(o.Extend))
			C.g_object_unref(C.gpointer(image))
			image = tmpImage
			if err != 0 {
				return nil, resizeError()
			}
		}
	} else {
		debug("canvased same as affined")
	}

	// Always convert to sRGB colour space
	C.vips_colourspace_0(image, &tmpImage, C.VIPS_INTERPRETATION_sRGB)
	C.g_object_unref(C.gpointer(image))
	image = tmpImage

	// Apply blur if needed
	if o.BlurAmount > 0 {
		if -1 != C.vips_gaussian_blur(image, &tmpImage, C.double(o.BlurAmount)) {
			C.g_object_unref(C.gpointer(image))
			image = tmpImage
		}
	}

	// Finally save
	length := C.size_t(0)
	var ptr unsafe.Pointer
	switch o.Format {
	case PNG:
		C.vips_pngsave_custom(image, &ptr, &length, 1, C.int(o.Quality), C.int(Btoi(o.Interlaced)))
	case JPEG, UNKNOWN:
		C.vips_jpegsave_custom(image, &ptr, &length, 1, C.int(o.Quality), C.int(Btoi(o.Interlaced)))
	case WEBP:
		C.vips_webpsave_custom(image, &ptr, &length, 1, C.int(o.Quality))
	default:
		C.vips_jpegsave_custom(image, &ptr, &length, 1, C.int(o.Quality), C.int(Btoi(o.Interlaced)))
	}
	C.g_object_unref(C.gpointer(image))

	// get back the buffer
	buf = C.GoBytes(ptr, C.int(length))
	C.g_free(C.gpointer(ptr))

	if o.FeatureCrop {
		tmpImg, _, err := img.Decode(bytes.NewReader(buf))
		if err == nil {
			o.Width = int(math.Min(float64(affinedWidth), float64(o.Width)))
			o.Height = int(math.Min(float64(affinedHeight), float64(o.Height)))
			analyzer := smartcrop.NewAnalyzer(nfnt.NewDefaultResizer())
			imageRect, _ := analyzer.FindBestCrop(tmpImg, o.Width, o.Height)

			// The crop will have the requested aspect ratio, but you need to copy/scale it yourself
			debug("Smart crop: %+v\n", imageRect)

			type SubImager interface {
				SubImage(r img.Rectangle) img.Image
			}
			featuredImg := tmpImg.(SubImager).SubImage(imageRect)
			debug("Smart crop featued image generated")

			buffer := new(bytes.Buffer)
			switch o.Format {
			case PNG:
				err = png.Encode(buffer, featuredImg)
			case JPEG, UNKNOWN:
				err = jpeg.Encode(buffer, featuredImg, &jpeg.Options{Quality: o.Quality})
			case WEBP:
				err = webp.Encode(buffer, featuredImg, &webp.Options{Lossless: true})
			default:
				err = jpeg.Encode(buffer, featuredImg, &jpeg.Options{Quality: o.Quality})
			}
			debug("Smart crop featued image encoded. %v", err)
			if err == nil {
				buf = buffer.Bytes()
				debug("Smart crop featued image buffered. %d", len(buf))
			} else {
				debug("Error finishing featured crop: %v", err)
			}
		} else {
			debug("Error featured cropping: %v", err)
		}
	}

	return buf, nil
}

func Btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

func resizeError() error {
	s := C.GoString(C.vips_error_buffer())
	C.vips_error_clear()
	return errors.New(s)
}

type Gravity int

const (
	CENTRE Gravity = iota
	NORTH
	EAST
	SOUTH
	WEST
)

func sharpCalcCrop(inWidth, inHeight, outWidth, outHeight int, gravity Gravity) (int, int) {
	left, top := 0, 0
	switch gravity {
	case NORTH:
		left = (inWidth - outWidth + 1) / 2
	case EAST:
		left = inWidth - outWidth
		top = (inHeight - outHeight + 1) / 2
	case SOUTH:
		left = (inWidth - outWidth + 1) / 2
		top = inHeight - outHeight
	case WEST:
		top = (inHeight - outHeight + 1) / 2
	default:
		left = (inWidth - outWidth + 1) / 2
		top = (inHeight - outHeight + 1) / 2
	}
	return left, top
}

func debug(format string, args ...interface{}) {
	if !DEBUG {
		return
	}
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}
