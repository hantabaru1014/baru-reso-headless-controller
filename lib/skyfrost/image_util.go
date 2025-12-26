package skyfrost

import (
	"bytes"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"

	"github.com/go-errors/errors"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // WebP decoder
)

const (
	IconSize         = 256
	MaxImageFileSize = 10 * 1024 * 1024 // 10MB
)

// ProcessIconImage validates and processes an image for use as a user icon.
// If the image is already 256x256 PNG (as processed by the frontend), it passes through.
// Otherwise, it crops to square from center, resizes to 256x256, and encodes as PNG.
func ProcessIconImage(data []byte) ([]byte, error) {
	if len(data) > MaxImageFileSize {
		return nil, errors.Errorf("image file too large: %d bytes (max %d)", len(data), MaxImageFileSize)
	}

	// Decode image
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, errors.Errorf("failed to decode image: %w", err)
	}

	// Validate format
	switch format {
	case "png", "jpeg", "gif", "webp":
		// OK
	default:
		return nil, errors.Errorf("unsupported image format: %s", format)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// If already 256x256 PNG (frontend-processed), pass through as-is
	if format == "png" && width == IconSize && height == IconSize {
		return data, nil
	}

	// Otherwise, process the image (for backwards compatibility or direct API calls)
	var cropRect image.Rectangle
	if width > height {
		// Landscape: crop from center horizontally
		offset := (width - height) / 2
		cropRect = image.Rect(offset, 0, offset+height, height)
	} else if height > width {
		// Portrait: crop from center vertically
		offset := (height - width) / 2
		cropRect = image.Rect(0, offset, width, offset+width)
	} else {
		// Already square
		cropRect = bounds
	}

	// Create cropped image
	croppedSize := cropRect.Dx()
	cropped := image.NewRGBA(image.Rect(0, 0, croppedSize, croppedSize))
	draw.Draw(cropped, cropped.Bounds(), img, cropRect.Min, draw.Src)

	// Resize to 256x256
	resized := image.NewRGBA(image.Rect(0, 0, IconSize, IconSize))
	draw.CatmullRom.Scale(resized, resized.Bounds(), cropped, cropped.Bounds(), draw.Over, nil)

	// Encode as PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, resized); err != nil {
		return nil, errors.Errorf("failed to encode image as PNG: %w", err)
	}

	return buf.Bytes(), nil
}
