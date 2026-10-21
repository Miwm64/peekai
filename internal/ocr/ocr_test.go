package ocr

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"strings"
	"testing"

	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"github.com/Miwm64/peekai/internal/ai"
	"github.com/Miwm64/peekai/internal/config"
)

// When FAKE_TESSERACT is set, the test binary acts as tesseract: it checks
// it got a PNG on stdin and prints FAKE_TESSERACT's value (or fails).
func TestMain(m *testing.M) {
	if out, ok := os.LookupEnv("FAKE_TESSERACT"); ok {
		if _, err := png.Decode(os.Stdin); err != nil {
			fmt.Fprintln(os.Stderr, "fake tesseract: stdin is not a PNG:", err)
			os.Exit(1)
		}
		if out == "FAIL" {
			fmt.Fprintln(os.Stderr, "Error opening data file eng.traineddata")
			os.Exit(1)
		}
		fmt.Print(out)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func fakeTesseract(t *testing.T, output string) Engine {
	t.Helper()
	t.Setenv("FAKE_TESSERACT", output)
	e, err := New(config.OCR{Engine: "tesseract_cli", Path: os.Args[0]}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func blank() image.Image { return image.NewRGBA(image.Rect(0, 0, 10, 10)) }

func TestTesseractCleansOutput(t *testing.T) {
	e := fakeTesseract(t, "  Hello  \n\n\n  world \n\f")
	got, err := e.Recognize(context.Background(), blank())
	if err != nil {
		t.Fatal(err)
	}
	if got != "Hello\n\nworld" {
		t.Errorf("text = %q", got)
	}
}

func TestTesseractReportsStderr(t *testing.T) {
	e := fakeTesseract(t, "FAIL")
	_, err := e.Recognize(context.Background(), blank())
	if err == nil || !strings.Contains(err.Error(), "eng.traineddata") {
		t.Errorf("err = %v", err)
	}
}

func TestTesseractCancelled(t *testing.T) {
	e := fakeTesseract(t, "x")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := e.Recognize(ctx, blank()); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v", err)
	}
}

func TestNewErrors(t *testing.T) {
	if _, err := New(config.OCR{Engine: "magic"}, nil); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Errorf("unknown engine: err = %v", err)
	}
	_, err := New(config.OCR{Engine: "tesseract_cli", Path: "/no/such/tesseract"}, nil)
	if err == nil || !strings.Contains(err.Error(), "tesseract not found") {
		t.Errorf("missing binary: err = %v", err)
	}
}

type fakeAI struct{ got ai.Request }

func (f *fakeAI) Analyze(_ context.Context, req ai.Request) (string, error) {
	f.got = req
	return " line one \n\n\n line two ", nil
}

func TestLLM(t *testing.T) {
	p := &fakeAI{}
	e, err := New(config.OCR{Engine: "llm"}, p)
	if err != nil {
		t.Fatal(err)
	}
	got, err := e.Recognize(context.Background(), blank())
	if err != nil {
		t.Fatal(err)
	}
	if got != "line one\n\nline two" {
		t.Errorf("text = %q", got)
	}
	if p.got.Image == nil || !strings.Contains(p.got.Prompt, "Transcribe") {
		t.Errorf("request = %+v", p.got)
	}
}

// TestRealTesseract runs the installed tesseract on rendered text.
func TestRealTesseract(t *testing.T) {
	if _, err := exec.LookPath("tesseract"); err != nil {
		t.Skip("tesseract not installed")
	}
	e, err := New(config.OCR{Engine: "tesseract_cli", Lang: "eng"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := e.Recognize(context.Background(), textImage("HELLO PEEKAI"))
	if err != nil {
		t.Fatal(err)
	}
	// The tiny bitmap font makes some letters ambiguous (I vs L); check words
	// that read reliably.
	if !strings.Contains(got, "HELLO") || !strings.Contains(got, "PEEKA") {
		t.Errorf("text = %q", got)
	}
}

// textImage renders s in black on white, scaled up so OCR can read it.
func textImage(s string) image.Image {
	small := image.NewRGBA(image.Rect(0, 0, 8*len(s)+16, 24))
	draw.Draw(small, small.Bounds(), image.White, image.Point{}, draw.Src)
	d := font.Drawer{Dst: small, Src: image.NewUniform(color.Black), Face: basicfont.Face7x13, Dot: fixed.P(8, 17)}
	d.DrawString(s)

	const scale = 4
	big := image.NewRGBA(image.Rect(0, 0, small.Bounds().Dx()*scale, small.Bounds().Dy()*scale))
	draw.NearestNeighbor.Scale(big, big.Bounds(), small, small.Bounds(), draw.Src, nil)
	return big
}
