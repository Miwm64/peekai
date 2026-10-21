package pipeline

import (
	"context"
	"errors"
	"image"
	"strings"
	"testing"

	"github.com/Miwm64/peekai/internal/ai"
	"github.com/Miwm64/peekai/internal/config"
)

// log records the order stages ran in.
type log []string

type fakeCapturer struct {
	log *log
	err error
}

func (f fakeCapturer) Capture(ctx context.Context) (image.Image, error) {
	*f.log = append(*f.log, "capture")
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return image.NewRGBA(image.Rect(0, 0, 3000, 1000)), f.err
}

type fakeOCR struct{ log *log }

func (f fakeOCR) Recognize(_ context.Context, img image.Image) (string, error) {
	*f.log = append(*f.log, "ocr")
	if img.Bounds().Dx() != 3000 {
		return "", errors.New("ocr got a downscaled image")
	}
	return "screen words", nil
}

type fakeAI struct {
	log *log
	got *ai.Request
}

func (f fakeAI) Analyze(_ context.Context, req ai.Request) (string, error) {
	*f.log = append(*f.log, "ai")
	*f.got = req
	return "answer", nil
}

func newTest() (*Pipeline, *log, *ai.Request) {
	var l log
	var got ai.Request
	return &Pipeline{
		Capturer:   fakeCapturer{log: &l},
		OCR:        fakeOCR{log: &l},
		AI:         fakeAI{log: &l, got: &got},
		MaxImagePx: 1500,
	}, &l, &got
}

func TestImageMode(t *testing.T) {
	p, l, got := newTest()
	res, err := p.Run(context.Background(), Options{Mode: config.ModeImage, Prompt: "Explain ({{.Mode}})"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(*l, ",") != "capture,ai" {
		t.Errorf("stages = %v", *l)
	}
	if got.Prompt != "Explain (image)" || got.Text != "" {
		t.Errorf("request = %+v", got)
	}
	if got.Image == nil || got.Image.Bounds().Dx() != 1500 {
		t.Error("AI did not get the downscaled image")
	}
	if res.Answer != "answer" || res.Mode != config.ModeImage {
		t.Errorf("result = %+v", res)
	}
}

func TestOCRMode(t *testing.T) {
	p, l, got := newTest()
	if _, err := p.Run(context.Background(), Options{Mode: config.ModeOCR, Prompt: "Explain"}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(*l, ",") != "capture,ocr,ai" {
		t.Errorf("stages = %v", *l)
	}
	if got.Image != nil || got.Text != "screen words" || got.Prompt != "Explain" {
		t.Errorf("request = %+v", got)
	}
}

func TestOCRTextInTemplateIsNotDuplicated(t *testing.T) {
	p, _, got := newTest()
	if _, err := p.Run(context.Background(), Options{Mode: config.ModeOCR, Prompt: "Fix: {{.Text}}"}); err != nil {
		t.Fatal(err)
	}
	if got.Prompt != "Fix: screen words" || got.Text != "" {
		t.Errorf("request = %+v", got)
	}
}

func TestOCRModeWithoutEngine(t *testing.T) {
	p, l, _ := newTest()
	p.OCR = nil
	_, err := p.Run(context.Background(), Options{Mode: config.ModeOCR, Prompt: "x"})
	if !errors.Is(err, ErrNoOCR) || len(*l) != 0 {
		t.Errorf("err = %v, stages = %v", err, *l)
	}
}

func TestErrorsNameTheStage(t *testing.T) {
	p, l, _ := newTest()
	p.Capturer = fakeCapturer{log: l, err: errors.New("denied")}
	_, err := p.Run(context.Background(), Options{Mode: config.ModeImage, Prompt: "x"})
	if err == nil || err.Error() != "capture: denied" {
		t.Errorf("err = %v", err)
	}

	p, _, _ = newTest()
	_, err = p.Run(context.Background(), Options{Mode: config.ModeImage, Prompt: "{{.Nope}}"})
	if err == nil || !strings.HasPrefix(err.Error(), "prompt:") {
		t.Errorf("err = %v", err)
	}
}

func TestCancelled(t *testing.T) {
	p, l, _ := newTest()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := p.Run(ctx, Options{Mode: config.ModeImage, Prompt: "x"})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v", err)
	}
	if strings.Join(*l, ",") != "capture" {
		t.Errorf("stages after cancel = %v", *l)
	}
}
