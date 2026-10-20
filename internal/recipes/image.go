package recipes

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png" // register decoders for image.Decode
	"net/http"

	"github.com/disintegration/imaging"
	_ "golang.org/x/image/webp" // register the WebP decoder
)

const (
	fullMaxEdge  = 1600 // longest edge of the stored full-size image
	thumbMaxEdge = 640  // longest edge of the list-card thumbnail
	fullQuality  = 82
	thumbQuality = 78
	// maxPixels guards against decompression bombs: a tiny file that decodes
	// to a huge bitmap. 60 megapixels covers any phone camera.
	maxPixels = 60_000_000
)

// processImage validates an uploaded image and returns resized JPEG
// versions: a full-size one (longest edge capped at fullMaxEdge) and a
// thumbnail. EXIF orientation is applied, so phone photos come out upright,
// and metadata (including GPS location) is dropped by re-encoding.
// Transparent PNGs are flattened onto white.
func processImage(data []byte) (full, thumb []byte, err error) {
	// Sniff the content type from the bytes; never trust the client's header.
	switch http.DetectContentType(data) {
	case "image/jpeg", "image/png", "image/webp":
	default:
		return nil, nil, invalid("image must be JPEG, PNG or WebP")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, nil, invalid("could not read image: %v", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > maxPixels {
		return nil, nil, invalid("image is too large (limit %d megapixels)", maxPixels/1_000_000)
	}
	img, err := imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
	if err != nil {
		return nil, nil, invalid("could not read image: %v", err)
	}
	img = flatten(img)

	if full, err = encodeJPEG(fitWithin(img, fullMaxEdge), fullQuality); err != nil {
		return nil, nil, err
	}
	if thumb, err = encodeJPEG(fitWithin(img, thumbMaxEdge), thumbQuality); err != nil {
		return nil, nil, err
	}
	return full, thumb, nil
}

// fitWithin scales img down so neither side exceeds edge; smaller images are
// left alone (never upscaled).
func fitWithin(img image.Image, edge int) image.Image {
	b := img.Bounds()
	if b.Dx() <= edge && b.Dy() <= edge {
		return img
	}
	if b.Dx() >= b.Dy() {
		return imaging.Resize(img, edge, 0, imaging.Lanczos)
	}
	return imaging.Resize(img, 0, edge, imaging.Lanczos)
}

// flatten draws img over a white background so transparency does not turn
// black when encoded as JPEG.
func flatten(img image.Image) image.Image {
	dst := image.NewRGBA(img.Bounds())
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), img, img.Bounds().Min, draw.Over)
	return dst
}

func encodeJPEG(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
