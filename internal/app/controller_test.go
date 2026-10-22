package app

import (
	"context"
	"errors"
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

// Update drops the provider/model part of the header to keep expectations
// short; TestHeaderShowsProviderAndModel checks it.
func (r *recorder) Update(c ui.Content) {
	r.calls <- "update " + strings.Replace(c.Header(), " · ollama/qwen2.5vl:7b", "", 1) + " | " + c.Text
}
func (r *recorder) Show(v ui.View)    { r.calls <- "show " + v.String() }
func (r *recorder) Hide(v ui.View)    { r.calls <- "hide " + v.String() }
func (r *recorder) Notify(msg string) { r.calls <- "notify " + msg }

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
	h.expect("update no result yet · " + cfg.Prompts.Active + " · image | ")
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

// ready matches the Update for a finished answer (its time varies).
func (h *harness) expectReady(text string) {
	h.t.Helper()
	select {
	case got := <-h.calls:
		if !strings.HasPrefix(got, "update ✓ answer") || !strings.HasSuffix(got, "| "+text) {
			h.t.Fatalf("ui call = %q, want ready %q", got, text)
		}
	case <-time.After(2 * time.Second):
		h.t.Fatalf("timed out waiting for answer %q", text)
	}
}

func TestCaptureNeverOpensWindows(t *testing.T) {
	h := start(t, nil)
	h.send(ActCapture)
	h.expect("update ● thinking… · a · image | ")
	h.expectReady("A/image")
	select {
	case got := <-h.calls:
		t.Fatalf("unexpected ui call after capture: %q", got)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestToggleOpensAndCloses(t *testing.T) {
	h := start(t, nil)
	h.send(ActTogglePopup) // opens even before any answer
	h.expect("show popup")
	h.send(ActCapture) // open window gets the new content
	h.expect("update ● thinking… · a · image | ")
	h.expectReady("A/image")
	h.send(ActTogglePopup)
	h.expect("hide popup")
	h.send(ActToggleTicker)
	h.expect("show ticker")
	h.send(ActHideAll)
	h.expect("hide ticker")
	h.send(ActHideAll) // nothing open: no calls
	h.send(ActCopyResult)
	h.expect("notify copied to clipboard")
}

func TestCopyBeforeAnyResult(t *testing.T) {
	h := start(t, nil)
	h.send(ActCopyResult)
	h.expect("notify nothing to copy yet")
}

func TestCyclePromptAndToggleMode(t *testing.T) {
	h := start(t, nil)
	h.send(ActCyclePrompt)
	h.expect("notify prompt: b", "update no result yet · b · image | ")
	h.send(ActToggleMode)
	h.expect("notify mode: ocr", "update no result yet · b · ocr | ")
	h.send(ActCapture)
	h.expect("update ● thinking… · b · ocr | ")
	h.expectReady("B/text:words")
	h.send(ActCyclePrompt) // after an answer: title updates, answer kept as previous
	h.expect("notify prompt: fail", "update previous answer · fail · ocr | B/text:words")
	h.send(ActCyclePrompt)
	h.expect("notify prompt: slow", "update previous answer · slow · ocr | B/text:words")
	h.send(ActCyclePrompt) // back to the answer's own settings: no longer stale
	h.send(ActToggleMode)
	h.expect("notify prompt: a", "update previous answer · a · ocr | B/text:words",
		"notify mode: image", "update previous answer · a · image | B/text:words")
}

func TestHeaderShowsProviderAndModel(t *testing.T) {
	c := ui.Content{State: ui.Ready, Text: "x", Prompt: "explain", Mode: "image", Provider: "routerai", Model: "gpt-x"}
	if h := c.Header(); !strings.Contains(h, "explain · image · routerai/gpt-x") {
		t.Errorf("header = %q", h)
	}
	c.Stale = true
	if h, b := c.Header(), c.Body(); !strings.HasPrefix(h, "previous answer") || b != "(previous answer)\nx" {
		t.Errorf("stale header/body = %q / %q", h, b)
	}
}

func TestHideAllKeepsRunning(t *testing.T) {
	h := start(t, nil)
	h.send(ActTogglePopup)
	h.expect("show popup")
	h.send(ActCapture)
	h.expect("update ● thinking… · a · image | ")
	h.send(ActHideAll)
	// Order of hide vs. answer depends on timing; both must arrive.
	got := map[string]bool{}
	for range 2 {
		select {
		case c := <-h.calls:
			got[strings.SplitN(c, " ·", 2)[0]] = true
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out, got %v", got)
		}
	}
	if !got["hide popup"] || !got["update ✓ answer"] {
		t.Errorf("calls = %v", got)
	}
}

func TestRetriggerReplacesRun(t *testing.T) {
	h := start(t, func(c *config.Config) { c.Prompts.Active = "slow" })
	h.send(ActCapture)
	h.expect("update ● thinking… · slow · image | ")
	h.send(ActCyclePrompt) // slow → a (sorted: a, b, fail, slow); title updates at once
	h.expect("notify prompt: a", "update ● thinking… · a · image | ")
	h.send(ActCapture)
	h.expect("update ● thinking… · a · image | ")
	h.expectReady("A/image")
}

func TestErrorIsShownAsContent(t *testing.T) {
	h := start(t, func(c *config.Config) { c.Prompts.Active = "fail" })
	h.send(ActCapture)
	h.expect("update ● thinking… · fail · image | ",
		"notify error: ai: boom",
		"update ✕ failed · fail · image | ai: boom")
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

func TestClear(t *testing.T) {
	h := start(t, nil)
	h.send(ActCapture)
	h.expect("update ● thinking… · a · image | ")
	h.expectReady("A/image")
	h.send(ActClear)
	h.expect("update no result yet · a · image | ", "notify cleared")
	h.send(ActCopyResult)
	h.expect("notify nothing to copy yet")
}

func TestClearCancelsRun(t *testing.T) {
	h := start(t, func(c *config.Config) { c.Prompts.Active = "slow" })
	h.send(ActCapture)
	h.expect("update ● thinking… · slow · image | ")
	h.send(ActClear)
	h.expect("update no result yet · slow · image | ", "notify cleared")
	select {
	case got := <-h.calls:
		t.Fatalf("cancelled run still reported: %q", got)
	case <-time.After(200 * time.Millisecond):
	}
}
