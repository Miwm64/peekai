package app

import (
	"context"
	"errors"
	"fmt"
	"image"
	"strings"
	"testing"
	"time"

	"github.com/Miwm64/peekai/internal/ai"
	"github.com/Miwm64/peekai/internal/config"
	"github.com/Miwm64/peekai/internal/pipeline"
	"github.com/Miwm64/peekai/internal/ui"
)

// recorder is a ui.Presenter that reports every call on a channel.
type recorder struct{ calls chan string }

func (r *recorder) Show(v ui.View, text string) { r.calls <- fmt.Sprintf("show %s: %s", v, text) }
func (r *recorder) Hide(v ui.View)              { r.calls <- "hide " + v.String() }
func (r *recorder) Busy(on bool)                { r.calls <- fmt.Sprint("busy ", on) }
func (r *recorder) Notify(msg string)           { r.calls <- "notify " + msg }

type screen struct{}

func (screen) Capture(context.Context) (image.Image, error) {
	return image.NewRGBA(image.Rect(0, 0, 2, 2)), nil
}

// fakeAI answers with the prompt and mode it got; "slow" blocks until cancelled.
type fakeAI struct{}

func (fakeAI) Analyze(ctx context.Context, req ai.Request) (string, error) {
	if req.Prompt == "slow" {
		<-ctx.Done()
		return "", ctx.Err()
	}
	if req.Prompt == "fail" {
		return "", errors.New("boom")
	}
	kind := "image"
	if req.Image == nil {
		kind = "text:" + req.Text
	}
	return req.Prompt + "/" + kind, nil
}

type fakeOCR struct{}

func (fakeOCR) Recognize(context.Context, image.Image) (string, error) { return "words", nil }

type harness struct {
	t       *testing.T
	actions chan string
	calls   chan string
	copied  string
	done    chan error
}

func start(t *testing.T, mutate func(*config.Config)) *harness {
	t.Helper()
	cfg := config.Default()
	cfg.Prompts.List = map[string]string{"a": "A", "b": "B", "slow": "slow", "fail": "fail"}
	cfg.Prompts.Active = "a"
	if mutate != nil {
		mutate(&cfg)
	}
	h := &harness{t: t, actions: make(chan string), calls: make(chan string, 32), done: make(chan error, 1)}
	c := New(cfg, Deps{
		Pipeline:  pipeline.Pipeline{Capturer: screen{}, AI: fakeAI{}},
		NewOCR:    func() (pipeline.Recognizer, error) { return fakeOCR{}, nil },
		UI:        &recorder{calls: h.calls},
		Clipboard: func(s string) error { h.copied = s; return nil },
	})
	go func() { h.done <- c.Run(context.Background(), h.actions) }()
	t.Cleanup(func() {
		select {
		case h.actions <- ActQuit:
			<-h.done
		case <-h.done:
		}
	})
	return h
}

func (h *harness) send(a string) { h.actions <- a }

// expect waits for the next UI calls, in order.
func (h *harness) expect(want ...string) {
	h.t.Helper()
	for _, w := range want {
		select {
		case got := <-h.calls:
			if got != w {
				h.t.Fatalf("ui call = %q, want %q", got, w)
			}
		case <-time.After(2 * time.Second):
			h.t.Fatalf("timed out waiting for %q", w)
		}
	}
}

func TestCaptureShowsInDefaultView(t *testing.T) {
	h := start(t, func(c *config.Config) { c.Output = config.OutputTicker })
	h.send(ActCapture)
	h.expect("busy true", "busy false", "show ticker: A/image")
}

func TestCaptureToSpecificViewsAndToggle(t *testing.T) {
	h := start(t, nil)
	h.send(ActCapturePopup)
	h.expect("busy true", "busy false", "show popup: A/image")
	h.send(ActTogglePopup)
	h.expect("hide popup")
	h.send(ActToggleTicker)
	h.expect("show ticker: A/image")
	h.send(ActHideAll)
	h.expect("hide ticker")
}

func TestToggleBeforeAnyResult(t *testing.T) {
	h := start(t, nil)
	h.send(ActTogglePopup)
	h.expect("notify nothing to show yet")
	h.send(ActCopyResult)
	h.expect("notify nothing to copy yet")
}

func TestCyclePromptAndToggleMode(t *testing.T) {
	h := start(t, nil)
	h.send(ActCyclePrompt)
	h.expect("notify prompt: b")
	h.send(ActToggleMode)
	h.expect("notify mode: ocr")
	h.send(ActCapturePopup)
	h.expect("busy true", "busy false", "show popup: B/text:words")
}

func TestHideAllCancelsRun(t *testing.T) {
	h := start(t, func(c *config.Config) { c.Prompts.Active = "slow" })
	h.send(ActCapture)
	h.expect("busy true")
	h.send(ActHideAll)
	h.expect("busy false")
	// The cancelled run must not show anything afterwards.
	h.send(ActTogglePopup)
	h.expect("notify nothing to show yet")
}

func TestRetriggerReplacesRun(t *testing.T) {
	h := start(t, func(c *config.Config) { c.Prompts.Active = "slow" })
	h.send(ActCapture)
	h.expect("busy true")
	h.send(ActCyclePrompt) // slow → a (sorted: a, b, fail, slow)
	h.expect("notify prompt: a")
	h.send(ActCapture)
	h.expect("busy false", "busy true", "busy false", "show popup: A/image")
}

func TestErrorIsNotified(t *testing.T) {
	h := start(t, func(c *config.Config) { c.Prompts.Active = "fail" })
	h.send(ActCapture)
	h.expect("busy true", "busy false")
	select {
	case got := <-h.calls:
		if !strings.HasPrefix(got, "notify error: ai: boom") {
			t.Errorf("got %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no error notification")
	}
}

func TestCopyResult(t *testing.T) {
	h := start(t, nil)
	h.send(ActCapture)
	h.expect("busy true", "busy false", "show popup: A/image")
	h.send(ActCopyResult)
	h.expect("notify copied to clipboard")
	if h.copied != "A/image" {
		t.Errorf("copied %q", h.copied)
	}
}

func TestQuitStopsRun(t *testing.T) {
	h := start(t, nil)
	h.send(ActQuit)
	select {
	case err := <-h.done:
		if err != nil {
			t.Errorf("Run = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return on quit")
	}
	h.done <- nil // let Cleanup finish
}
