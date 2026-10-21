package ai

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/jpeg"
)

// jpegQuality keeps screen text readable while staying much smaller than PNG.
const jpegQuality = 90

// dataURL encodes img as a base64 JPEG data URL.
func dataURL(img image.Image) (string, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return "", err
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
