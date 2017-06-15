#include <stdlib.h>
#include <vips/vips.h>
#include <vips/basic.h>
#include <vips/vips7compat.h>

int
vips_initialize()
{
    return vips_init("govips");
}

int
vips_gaussian_blur(VipsImage *in, VipsImage **out, double sigma)
{
  // return vips_gaussblur(in, out, sigma, "precision", VipsPrecision precision, "min_ampl", double min_amp, NULL);
  return vips_gaussblur(in, out, sigma, NULL);
}

int
vips_affine_interpolator(VipsImage *in, VipsImage **out, double a, double b, double c, double d, VipsInterpolate *interpolator)
{
    return vips_affine(in, out, a, b, c, d, "interpolate", interpolator, NULL);
};

int
vips_jpegload_buffer_seq(void *buf, size_t len, VipsImage **out)
{
    return vips_jpegload_buffer(buf, len, out, "access", VIPS_ACCESS_SEQUENTIAL, NULL);
};

int
vips_magickload_(const char *filename, VipsImage **out)
{
    return vips_magickload(filename, out, NULL);
};

int
vips_magickload_buffer_(void *buf, size_t len, VipsImage **out)
{
    return vips_magickload_buffer(buf, len, out, NULL);
};

int
vips_crop_(VipsImage *in, VipsImage **out, int left, int top, int width, int height)
{
  return vips_crop(in, out, left, top, width, height, NULL);
}


int
vips_jpegload_buffer_shrink(void *buf, size_t len, VipsImage **out, int shrink)
{
    return vips_jpegload_buffer(buf, len, out, "shrink", shrink, NULL);
};

int
vips_pngload_buffer_seq(void *buf, size_t len, VipsImage **out)
{
    return vips_pngload_buffer(buf, len, out, "access", VIPS_ACCESS_SEQUENTIAL, NULL);
};

int
vips_webpload_buffer_seq(void *buf, size_t len, VipsImage **out)
{
    return vips_webpload_buffer(buf, len, out, "access", VIPS_ACCESS_SEQUENTIAL, NULL);
};

int
vips_shrink_0(VipsImage *in, VipsImage **out, double xshrink, double yshrink)
{
    return vips_shrink(in, out, xshrink, yshrink, NULL);
};

int
vips_copy_0(VipsImage *in, VipsImage **out)
{
    return vips_copy(in, out, NULL);
}

int
vips_flatten_0(VipsImage *in, VipsImage **out)
{
    VipsArrayDouble *white = vips_array_double_newv(1.0, 255.0);

    int flatten_value = vips_flatten(in, out, "background", white, NULL);
    vips_area_unref((VipsArea *)white);

    return flatten_value;
}

int
vips_embed_extend(VipsImage *in, VipsImage **out, int left, int top, int width, int height, int extend)
{
    return vips_embed(in, out, left, top, width, height, "extend", extend, NULL);
}

int
vips_colourspace_0(VipsImage *in, VipsImage **out, VipsInterpretation space)
{
    return vips_colourspace(in, out, space, NULL);
};

int
vips_extract_area_0(VipsImage *in, VipsImage **out, int left, int top, int width, int height)
{
    return vips_extract_area(in, out, left, top, width, height, NULL);
}

int
vips_pngsave_custom(VipsImage *in, void **buf, size_t *len, int strip, int quality, int interlace)
{
    return vips_pngsave_buffer(in, buf, len, "interlace", interlace, NULL);
}

int
vips_jpegsave_custom(VipsImage *in, void **buf, size_t *len, int strip, int quality, int interlace)
{
    return vips_jpegsave_buffer(in, buf, len, "strip", strip, "Q", quality, "optimize_coding", TRUE, "interlace", interlace, NULL);
}

int
vips_webpsave_custom(VipsImage *in, void **buf, size_t *len, int strip, int quality)
{
    return vips_webpsave_buffer(in, buf, len, "strip", strip, "Q", quality, NULL);
}

int
vips_gifload_buffer_seq(void *buf, size_t len, VipsImage **out)
{
    return vips_gifload_buffer(buf, len, out, "access", VIPS_ACCESS_SEQUENTIAL, NULL);
};