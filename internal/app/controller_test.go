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

// Update drops the default model's part of the header to keep expectations
// short; TestHeaderShowsProviderAndModel checks it.
func (r *recorder) Update(c ui.Content) {
	r.calls <- "update " + strings.Replace(c.Header(), " · qwen-local · free", "", 1) + " | " + c.Text
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
	return startWith(t, mutate, nil)
}

func startWith(t *testing.T, mutate func(*config.Config), deps func(*Deps)) *harness {
	t.Helper()
	cfg := config.Default()
	cfg.Prompts.List = map[string]string{"a": "A", "b": "B", "slow": "slow", "fail": "fail"}
	cfg.Prompts.Default = "a"
	cfg.AI.AutoClear = true           // most tests check one exchange; TestContext covers history
	cfg.AI.Models = cfg.AI.Models[:1] // just qwen-local; model tests set Deps.AI
	if mutate != nil {
		mutate(&cfg)
	}
	h := &harness{t: t, actions: make(chan string), calls: make(chan string, 32), done: make(chan error, 1)}
	d := Deps{
		Pipeline:  pipeline.Pipeline{Capturer: screen{}, AI: fakeAI{}},
		NewOCR:    func(ai.Provider) (pipeline.Recognizer, error) { return fakeOCR{}, nil },
		UI:        &recorder{calls: h.calls},
		Clipboard: func(s string) error { h.copied = s; return nil },
	}
	if deps != nil {
		deps(&d)
	}
	c := New(cfg, d)
	go func() { h.done <- c.Run(context.Background(), h.actions) }()
	h.expect("update no result yet · " + cfg.Prompts.Default + " · image | ")
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
	c := ui.Content{State: ui.Ready, Text: "x", Prompt: "explain", Mode: "image", Model: "gpt-x", Free: true}
	if h := c.Header(); !strings.Contains(h, "explain · image · gpt-x · free") {
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
	h := start(t, func(c *config.Config) { c.Prompts.Default = "slow" })
	h.send(ActCapture)
	h.expect("update ● thinking… · slow · image | ")
	h.send(ActCyclePrompt) // slow → a (sorted: a, b, fail, slow); title updates at once
	h.expect("notify prompt: a", "update ● thinking… · a · image | ")
	h.send(ActCapture)
	h.expect("update ● thinking… · a · image | ")
	h.expectReady("A/image")
}

func TestErrorIsShownAsContent(t *testing.T) {
	h := start(t, func(c *config.Config) { c.Prompts.Default = "fail" })
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
	h := start(t, func(c *config.Config) { c.Prompts.Default = "slow" })
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

// prefixAI answers like fakeAI with a prefix, to tell providers apart.
type prefixAI struct{ prefix string }

func (p prefixAI) Analyze(ctx context.Context, req ai.Request) (string, error) {
	a, err := fakeAI{}.Analyze(ctx, req)
	return p.prefix + a, err
}

func TestCycleModel(t *testing.T) {
	h := startWith(t, nil, func(d *Deps) {
		d.AI = []AIOption{
			{Name: "qwen-local", Vision: true, Free: true, New: func() (ai.Provider, error) { return fakeAI{}, nil }},
			{Name: "g1", Vision: true, New: func() (ai.Provider, error) { return prefixAI{"G:"}, nil }},
			{Name: "r1", Vision: true, New: func() (ai.Provider, error) { return nil, errors.New("no key") }},
		}
	})
	h.send(ActCapture)
	h.expect("update ● thinking… · a · image | ")
	h.expectReady("A/image")

	h.send(ActCycleModel)
	h.expect("notify model: g1", "update previous answer · a · image · g1 | A/image")
	h.send(ActCapture)
	h.expect("update ● thinking… · a · image · g1 | A/image")
	select {
	case got := <-h.calls:
		if got != "update ✓ answer · a · image · g1 · "+time.Now().Format("15:04")+" | G:A/image" {
			t.Fatalf("got %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no answer from grok")
	}

	h.send(ActCycleModel) // routerai can't start: skipped, back to ollama
	h.expect("notify skipping r1: no key", "notify model: qwen-local",
		"update previous answer · a · image | G:A/image")
}

func TestCycleModelWithOnlyOne(t *testing.T) {
	h := start(t, nil)
	h.send(ActCycleModel)
	h.expect("notify only one model configured (add more to ai.models)")
}

// historyAI answers with how many earlier exchanges it was sent.
type historyAI struct{}

func (historyAI) Analyze(_ context.Context, req ai.Request) (string, error) {
	return fmt.Sprintf("seen %d", len(req.History)), nil
}

func TestContext(t *testing.T) {
	h := startWith(t, func(c *config.Config) { c.AI.AutoClear = false }, func(d *Deps) {
		d.Pipeline.AI = historyAI{}
	})
	h.send(ActCapture)
	h.expect("update ● thinking… · a · image | ")
	h.expectReady("seen 0")
	h.send(ActCapture) // the first exchange is sent along
	h.expect("update ● thinking… · a · image · ctx 1 | seen 0")
	h.expectReady("seen 1")
	h.send(ActClear) // forgets the context
	h.expect("update no result yet · a · image | ", "notify cleared")
	h.send(ActCapture)
	h.expect("update ● thinking… · a · image | ")
	h.expectReady("seen 0")
}

func TestAutoClearKeepsWindowsButNotContext(t *testing.T) {
	h := startWith(t, nil, func(d *Deps) { d.Pipeline.AI = historyAI{} })
	h.send(ActCapture)
	h.expect("update ● thinking… · a · image | ")
	h.expectReady("seen 0")
	h.send(ActCapture) // window keeps the previous answer; AI gets no history
	h.expect("update ● thinking… · a · image | seen 0")
	h.expectReady("seen 0")
}

// runUntilReload starts a controller, feeds it actions and returns what Run gave back.
func runUntilReload(t *testing.T, c *Controller, actions ...string) error {
	t.Helper()
	ch := make(chan string)
	done := make(chan error, 1)
	go func() { done <- c.Run(context.Background(), ch) }()
	for _, a := range actions {
		ch <- a
	}
	select {
	case err := <-done:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return")
		return nil
	}
}

func TestReloadHandsOverState(t *testing.T) {
	calls := make(chan string, 64)
	cfg := config.Default()
	cfg.Prompts.List = map[string]string{"a": "A", "b": "B"}
	cfg.Prompts.Default = "a"
	cfg.AI.AutoClear = false
	deps := Deps{
		Pipeline:  pipeline.Pipeline{Capturer: screen{}, AI: fakeAI{}},
		NewOCR:    func(ai.Provider) (pipeline.Recognizer, error) { return fakeOCR{}, nil },
		UI:        &recorder{calls: calls},
		Clipboard: func(string) error { return nil },
	}

	first := New(cfg, deps)
	ch := make(chan string)
	done := make(chan error, 1)
	go func() { done <- first.Run(context.Background(), ch) }()
	ch <- ActCyclePrompt // a → b
	ch <- ActToggleMode  // image → ocr
	ch <- ActTogglePopup
	ch <- ActCapture
	// Wait for the answer before reloading.
	deadline := time.After(2 * time.Second)
	for answered := false; !answered; {
		select {
		case c := <-calls:
			answered = strings.HasPrefix(c, "update ✓ answer")
		case <-deadline:
			t.Fatal("no answer before reload")
		}
	}
	ch <- ActReload
	if err := <-done; !errors.Is(err, ErrReload) {
		t.Fatalf("Run = %v, want ErrReload", err)
	}
	snap := first.Snapshot()

	deps.Restore = &snap
	second := New(cfg, deps) // config still says prompt a, mode image
	if err := runUntilReload(t, second, ActQuit); err != nil {
		t.Fatal(err)
	}
	if second.active != "b" || second.mode != config.ModeOCR {
		t.Errorf("runtime choices lost: prompt=%s mode=%s", second.active, second.mode)
	}
	if second.last != "B/text:words" || len(second.history) != 1 || !second.visible[ui.Popup] {
		t.Errorf("state lost: last=%q history=%d visible=%v", second.last, len(second.history), second.visible)
	}
	if second.content.State != ui.Ready || second.content.Stale {
		t.Errorf("content = %+v", second.content)
	}
}

func TestReloadDropsPromptThatNoLongerExists(t *testing.T) {
	cfg := config.Default()
	cfg.Prompts.List = map[string]string{"a": "A", "b": "B"}
	cfg.Prompts.Default = "a"
	c := New(cfg, Deps{UI: &recorder{calls: make(chan string, 16)}, Pipeline: pipeline.Pipeline{AI: fakeAI{}}})
	c.active = "b"
	snap := c.Snapshot()

	cfg.Prompts.List = map[string]string{"a": "A"} // b removed from config.json
	next := New(cfg, Deps{UI: &recorder{calls: make(chan string, 16)}, Pipeline: pipeline.Pipeline{AI: fakeAI{}}, Restore: &snap})
	if err := runUntilReload(t, next, ActQuit); err != nil {
		t.Fatal(err)
	}
	if next.active != "a" {
		t.Errorf("active = %q, want config's a", next.active)
	}
}

func TestImageModeRefusesModelWithoutVision(t *testing.T) {
	called := false
	h := startWith(t, nil, func(d *Deps) {
		d.AI = []AIOption{
			{Name: "qwen-local", Vision: true, Free: true, New: func() (ai.Provider, error) { return fakeAI{}, nil }},
			{Name: "llama", Vision: false, New: func() (ai.Provider, error) { return recordAI{called: &called}, nil }},
		}
	})
	h.send(ActCycleModel)
	h.expect("notify model: llama", "update no result yet · a · image · llama | ")
	h.send(ActCapture)
	msg := `model "llama" can't read images: switch to OCR mode or pick a vision model`
	h.expect("notify "+msg, "update ✕ failed · a · image · llama | "+msg)
	h.send(ActToggleMode) // OCR mode: the same model is fine
	h.expect("notify mode: ocr", "update previous answer · a · ocr · llama | "+msg)
	h.send(ActCapture)
	h.expect("update ● thinking… · a · ocr · llama | ")
	select {
	case got := <-h.calls:
		if !strings.HasPrefix(got, "update ✓ answer · a · ocr · llama") {
			t.Fatalf("got %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no answer in ocr mode")
	}
	if !called {
		t.Error("ocr capture did not reach the model")
	}
}

// recordAI notes that it was called; it is only ever sent text here.
type recordAI struct{ called *bool }

func (r recordAI) Analyze(_ context.Context, req ai.Request) (string, error) {
	*r.called = true
	if req.Image != nil {
		return "", errors.New("got an image")
	}
	return "ok", nil
}

func TestSelectModelFromTray(t *testing.T) {
	h := startWith(t, nil, func(d *Deps) {
		d.AI = []AIOption{
			{Name: "qwen-local", Vision: true, Free: true, New: func() (ai.Provider, error) { return fakeAI{}, nil }},
			{Name: "g1", Vision: true, New: func() (ai.Provider, error) { return prefixAI{"G:"}, nil }},
		}
	})
	h.send(ActSelectModel + "g1")
	h.expect("notify model: g1", "update no result yet · a · image · g1 | ")
	h.send(ActSelectModel + "nope")
	h.expect("notify no model named nope")
}
