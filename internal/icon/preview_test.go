package icon

import (
	"image/png"
	"os"
	"testing"
)

func TestWritePreview(t *testing.T) {
	out := os.Getenv("ICON_PREVIEW_DIR")
	if out == "" {
		t.Skip("set ICON_PREVIEW_DIR to write previews")
	}
	for name, img := range map[string]bool{"app": true, "tray": false} {
		f, err := os.Create(out + "/icon-" + name + ".png")
		if err != nil {
			t.Fatal(err)
		}
		size := 512
		if !img {
			size = 128
		}
		if err := png.Encode(f, Render(size, img)); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
}
