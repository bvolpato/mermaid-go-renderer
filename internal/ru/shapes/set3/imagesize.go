// This file ports no upstream file. It stands for what the browser does
// for imageSquare.ts:
//
//	const img = new Image();
//	img.src = node?.img ?? '';
//	await img.decode();
//	... img.naturalWidth, img.naturalHeight
//
// The natural size of an image comes from the image data. internal/imagesize
// reads the data of a data URL: the layout engine uses the same package for
// the img elements of labels. For another URL the render needs a resolver:
// SetImageResolver for this shape, or the ImageSize option of the render,
// which the engine and this shape share. The rules for each image format
// are what Chromium does (tools/reference/gen-image-size.mjs records them).

package set3

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/imagesize"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// ImageSize is the natural size of an image: img.naturalWidth and
// img.naturalHeight.
type ImageSize = imagesize.Size

// ImageResolver gives the natural size of the image at a URL. It returns
// an error when the browser cannot load or decode the image.
type ImageResolver func(src string) (ImageSize, error)

// ErrImageDecode is the error of img.decode() for an image that the
// browser cannot load or cannot decode. Its text is the message of the
// EncodingError of Chromium.
var ErrImageDecode = imagesize.ErrDecode

// SetImageResolver sets the resolver of a render for the image sources
// that are not data URLs. Without it the shape asks the ImageSize option of
// the render. Without both such an image fails as it does in a browser that
// cannot fetch it.
func SetImageResolver(ctx *render.Context, resolver ImageResolver) {
	*imageResolverSlot(ctx) = resolver
}

func imageResolverSlot(ctx *render.Context) *ImageResolver {
	return ctx.Value("set3.imageResolver", func() any { return new(ImageResolver) }).(*ImageResolver)
}

// naturalImageSize is the natural size of the image at src after
// img.decode().
func naturalImageSize(ctx *render.Context, src string) (ImageSize, error) {
	src = imagesize.CleanURL(src)
	if imagesize.IsDataURL(src) {
		return imagesize.FromDataURL(src)
	}
	if src == "" {
		return ImageSize{}, ErrImageDecode
	}
	if resolver := *imageResolverSlot(ctx); resolver != nil {
		return resolver(src)
	}
	// The resolver of the render, which the layout engine also uses for
	// the img elements of labels.
	if ctx.Engine != nil && ctx.Engine.ImageSize != nil {
		if w, h, ok := ctx.Engine.ImageSize(src); ok {
			return ImageSize{Width: w, Height: h}, nil
		}
	}
	return ImageSize{}, ErrImageDecode
}

// percentDecode is the percent-decode of the URL standard.
func percentDecode(s string) []byte { return imagesize.PercentDecode(s) }

// forgivingBase64Decode is the forgiving-base64 decode of the Infra
// standard.
func forgivingBase64Decode(data []byte) ([]byte, bool) { return imagesize.ForgivingBase64Decode(data) }
