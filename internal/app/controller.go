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
	"time"

	"github.com/Miwm64/peekai/internal/config"
	"github.com/Miwm64/peekai/internal/pipeline"
	"github.com/Miwm64/peekai/internal/ui"
)

// Action names, matching the keys of config.Hotkeys.Bindings.
const (
	ActCapture      = "capture"
	ActTogglePopup  = "toggle_popup"
	ActToggleTicker = "toggle_ticker"
	ActHideAll      = "hide_all"
	ActCyclePrompt  = "cycle_prompt"
	ActToggleMode   = "toggle_mode"
	ActCopyResult   = "copy_result"
	ActClear        = "clear"
	ActQuit         = "quit"
)

// Deps are the controller's collaborators.
type Deps struct {
	Pipeline pipeline.Pipeline
	// NewOCR builds the OCR engine on first use of ocr mode.
	NewOCR    func() (pipeline.Recognizer, error)
	UI        ui.Presenter
	Clipboard func(text string) error
}

// Controller runs analyses on the capture shortcut and opens or closes
// windows only on their toggle shortcuts; results update the windows'
// content without opening them.
type Controller struct {
	deps    Deps
	mode    string
	prompts map[string]string
	names   []string // sorted prompt names, for cycling
	active  string

	content ui.Content // what the windows display
	last    string     // last answer, for copy
	visible map[ui.View]bool

	runID   int
	cancel  context.CancelFunc
	results chan result
}

type result struct {
	id     int
	prompt string
	mode   string
	res    pipeline.Result
	err    error
}

func New(cfg config.Config, deps Deps) *Controller {
	return &Controller{
		deps:    deps,
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
	c.content = ui.Content{State: ui.Idle, Prompt: c.active, Mode: c.mode}
	c.deps.UI.Update(c.content)
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
		c.start(ctx)
	case ActTogglePopup:
		c.toggle(ui.Popup)
	case ActToggleTicker:
		c.toggle(ui.Ticker)
	case ActHideAll:
		// Only hides; a running analysis continues and its answer is
		// there next time a window is opened.
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
		c.refreshIdle()
	case ActToggleMode:
		if c.mode == config.ModeOCR {
			c.mode = config.ModeImage
		} else {
			c.mode = config.ModeOCR
		}
		c.deps.UI.Notify("mode: " + c.mode)
		c.refreshIdle()
	case ActCopyResult:
		c.copyLast()
	case ActClear:
		// Forget everything; open windows stay open and show the empty state.
		c.stop()
		c.last = ""
		c.content = ui.Content{State: ui.Idle, Prompt: c.active, Mode: c.mode}
		c.deps.UI.Update(c.content)
		c.deps.UI.Notify("cleared")
	default:
		c.deps.UI.Notify("unknown action: " + action)
	}
}

// start cancels any running analysis and begins a new one. It changes the
// windows' content to "thinking" but never opens a window.
func (c *Controller) start(ctx context.Context) {
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
	prompt, mode := c.active, c.mode
	opts := pipeline.Options{Mode: mode, Prompt: c.prompts[prompt]}

	c.content = ui.Content{State: ui.Thinking, Text: c.last, Prompt: prompt, Mode: mode}
	c.deps.UI.Update(c.content)
	go func() {
		res, err := p.Run(runCtx, opts)
		select {
		case c.results <- result{id: id, prompt: prompt, mode: mode, res: res, err: err}:
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
	switch {
	case errors.Is(r.err, context.Canceled):
		return
	case r.err != nil:
		c.content = ui.Content{State: ui.Failed, Text: r.err.Error(), Prompt: r.prompt, Mode: r.mode}
		c.deps.UI.Notify("error: " + r.err.Error())
	default:
		c.last = r.res.Answer
		c.content = ui.Content{State: ui.Ready, Text: c.last, Prompt: r.prompt, Mode: r.mode, At: time.Now()}
	}
	c.deps.UI.Update(c.content)
}

// refreshIdle shows the new prompt/mode while nothing has been captured.
func (c *Controller) refreshIdle() {
	if c.content.State == ui.Idle {
		c.content.Prompt, c.content.Mode = c.active, c.mode
		c.deps.UI.Update(c.content)
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
}

// toggle opens or closes a window. It always opens, even before the first
// answer: the window then shows "thinking" or "no result yet".
func (c *Controller) toggle(v ui.View) {
	c.visible[v] = !c.visible[v]
	if c.visible[v] {
		c.deps.UI.Show(v)
	} else {
		c.deps.UI.Hide(v)
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
