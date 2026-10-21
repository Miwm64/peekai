// Package app turns hotkey actions into behaviour. It owns the runtime
// state: current mode and prompt, the last answer, what is visible, and the
// in-flight analysis.
package app

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/Miwm64/peekai/internal/config"
	"github.com/Miwm64/peekai/internal/pipeline"
	"github.com/Miwm64/peekai/internal/ui"
)

// Action names, matching the keys of config.Hotkeys.Bindings.
const (
	ActCapture       = "capture"
	ActCapturePopup  = "capture_popup"
	ActCaptureTicker = "capture_ticker"
	ActTogglePopup   = "toggle_popup"
	ActToggleTicker  = "toggle_ticker"
	ActHideAll       = "hide_all"
	ActCyclePrompt   = "cycle_prompt"
	ActToggleMode    = "toggle_mode"
	ActCopyResult    = "copy_result"
	ActQuit          = "quit"
)

// Deps are the controller's collaborators.
type Deps struct {
	Pipeline pipeline.Pipeline
	// NewOCR builds the OCR engine on first use of ocr mode.
	NewOCR    func() (pipeline.Recognizer, error)
	UI        ui.Presenter
	Clipboard func(text string) error
}

type Controller struct {
	deps    Deps
	output  ui.View
	mode    string
	prompts map[string]string
	names   []string // sorted prompt names, for cycling
	active  string

	last    string
	visible map[ui.View]bool

	runID   int
	cancel  context.CancelFunc
	results chan result
}

type result struct {
	id   int
	view ui.View
	res  pipeline.Result
	err  error
}

func New(cfg config.Config, deps Deps) *Controller {
	return &Controller{
		deps:    deps,
		output:  ui.ParseView(cfg.Output),
		mode:    cfg.Mode,
		prompts: cfg.Prompts.List,
		names:   slices.Sorted(maps.Keys(cfg.Prompts.List)),
		active:  cfg.Prompts.Active,
		visible: map[ui.View]bool{},
		results: make(chan result),
	}
}

// Run handles actions until quit, ctx is done or actions is closed.
func (c *Controller) Run(ctx context.Context, actions <-chan string) error {
	defer c.stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case a, ok := <-actions:
			if !ok || a == ActQuit {
				return nil
			}
			c.handle(ctx, a)
		case r := <-c.results:
			c.finish(r)
		}
	}
}

func (c *Controller) handle(ctx context.Context, action string) {
	switch action {
	case ActCapture:
		c.start(ctx, c.output)
	case ActCapturePopup:
		c.start(ctx, ui.Popup)
	case ActCaptureTicker:
		c.start(ctx, ui.Ticker)
	case ActTogglePopup:
		c.toggle(ui.Popup)
	case ActToggleTicker:
		c.toggle(ui.Ticker)
	case ActHideAll:
		c.stop()
		for _, v := range []ui.View{ui.Popup, ui.Ticker} {
			if c.visible[v] {
				c.deps.UI.Hide(v)
				c.visible[v] = false
			}
		}
	case ActCyclePrompt:
		i := slices.Index(c.names, c.active)
		c.active = c.names[(i+1)%len(c.names)]
		c.deps.UI.Notify("prompt: " + c.active)
	case ActToggleMode:
		if c.mode == config.ModeOCR {
			c.mode = config.ModeImage
		} else {
			c.mode = config.ModeOCR
		}
		c.deps.UI.Notify("mode: " + c.mode)
	case ActCopyResult:
		c.copyLast()
	default:
		c.deps.UI.Notify("unknown action: " + action)
	}
}

// start cancels any running analysis and begins a new one shown in v.
func (c *Controller) start(ctx context.Context, v ui.View) {
	c.stop()
	if c.mode == config.ModeOCR && c.deps.Pipeline.OCR == nil {
		engine, err := c.deps.NewOCR()
		if err != nil {
			c.deps.UI.Notify("ocr: " + err.Error())
			return
		}
		c.deps.Pipeline.OCR = engine
	}

	c.runID++
	id := c.runID
	runCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	p := c.deps.Pipeline // copy: the goroutine never sees later changes
	opts := pipeline.Options{Mode: c.mode, Prompt: c.prompts[c.active]}

	c.deps.UI.Busy(true)
	go func() {
		res, err := p.Run(runCtx, opts)
		select {
		case c.results <- result{id: id, view: v, res: res, err: err}:
		case <-runCtx.Done(): // cancelled or controller stopped: nobody listens
		}
	}()
}

func (c *Controller) finish(r result) {
	if r.id != c.runID {
		return // superseded or cancelled run
	}
	c.cancel()
	c.cancel = nil
	c.deps.UI.Busy(false)
	switch {
	case errors.Is(r.err, context.Canceled):
	case r.err != nil:
		c.deps.UI.Notify("error: " + r.err.Error())
	default:
		c.last = r.res.Answer
		c.deps.UI.Show(r.view, c.last)
		c.visible[r.view] = true
	}
}

// stop cancels the in-flight analysis, if any.
func (c *Controller) stop() {
	if c.cancel == nil {
		return
	}
	c.cancel()
	c.cancel = nil
	c.runID++ // drop its result when it arrives
	c.deps.UI.Busy(false)
}

func (c *Controller) toggle(v ui.View) {
	switch {
	case c.visible[v]:
		c.deps.UI.Hide(v)
		c.visible[v] = false
	case c.last == "":
		c.deps.UI.Notify("nothing to show yet")
	default:
		c.deps.UI.Show(v, c.last)
		c.visible[v] = true
	}
}

func (c *Controller) copyLast() {
	if c.last == "" {
		c.deps.UI.Notify("nothing to copy yet")
		return
	}
	if err := c.deps.Clipboard(c.last); err != nil {
		c.deps.UI.Notify(fmt.Sprintf("copy failed: %v", err))
		return
	}
	c.deps.UI.Notify("copied to clipboard")
}
