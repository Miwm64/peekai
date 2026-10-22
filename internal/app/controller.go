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
	"strings"
	"time"

	"github.com/Miwm64/peekai/internal/ai"
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
	ActCycleModel   = "cycle_model"
	// ActSelectModel + a model name switches to that model (tray menu).
	ActSelectModel = "model:"
	ActToggleMode  = "toggle_mode"
	ActCopyResult  = "copy_result"
	ActClear       = "clear"
	ActReload      = "reload_config"
	ActQuit        = "quit"
)

// ErrReload is returned by Run when the user asked to reload the config.
// The caller loads it again and starts a new Controller with the
// Snapshot of this one.
var ErrReload = errors.New("reload config")

// Deps are the controller's collaborators.
type Deps struct {
	// Pipeline.AI is the client for the config's default model.
	Pipeline pipeline.Pipeline
	// AI lists the models cycle_model switches between, in config order.
	// Empty means the config's models, all served by Pipeline.AI (tests).
	AI []AIOption
	// NewOCR builds the OCR engine on first use of ocr mode, and again
	// after a provider switch (the llm engine reads with the provider).
	NewOCR    func(ai.Provider) (pipeline.Recognizer, error)
	UI        ui.Presenter
	Clipboard func(text string) error
	// Restore continues from a previous controller after a config reload.
	Restore *Snapshot
}

// Snapshot is what a controller hands to its successor on reload: the
// last answer, the AI context, what's open, and the runtime choices
// (kept when they still exist in the new config).
type Snapshot struct {
	content ui.Content
	last    string
	history []ai.Turn
	answer  settings
	visible map[ui.View]bool
	current settings
}

// Snapshot captures the state to carry across a config reload.
func (c *Controller) Snapshot() Snapshot {
	return Snapshot{
		content: c.content,
		last:    c.last,
		history: slices.Clone(c.history),
		answer:  c.answer,
		visible: maps.Clone(c.visible),
		current: c.current(),
	}
}

// restore takes over a previous controller's state.
func (c *Controller) restore(s Snapshot) {
	c.content, c.last, c.history, c.answer = s.content, s.last, s.history, s.answer
	if s.visible != nil {
		c.visible = s.visible
	}
	if c.content.State == ui.Thinking { // that run ended with the old controller
		c.content.State = ui.Idle
		if c.last != "" {
			c.content.State = ui.Ready
		}
	}
	if s.current.mode != "" {
		c.mode = s.current.mode
	}
	if _, ok := c.prompts[s.current.prompt]; ok {
		c.active = s.current.prompt
	}
	if i := c.modelIndex(s.current.model); i >= 0 && i != c.aiIdx {
		if p, err := c.client(i); err == nil {
			c.aiIdx = i
			c.deps.Pipeline.AI = p
		}
	}
}

// AIOption is one configured model to switch to; New is called on first
// use.
type AIOption struct {
	Name        string // the model's name in the config
	Provider    string
	Model       string // the provider's model ID
	Vision      bool
	Free        bool
	Description string
	New         func() (ai.Provider, error)
}

// OptionsFrom describes the config's models; newClient builds the client
// for one of them.
func OptionsFrom(cfg config.Config, newClient func(config.ResolvedModel) (ai.Provider, error)) []AIOption {
	var opts []AIOption
	for _, m := range cfg.ResolvedModels() {
		opts = append(opts, AIOption{
			Name: m.Name, Provider: m.Provider.Name, Model: m.Model.Model,
			Vision: m.Vision, Free: m.Free, Description: m.Description,
			New: func() (ai.Provider, error) { return newClient(m) },
		})
	}
	return opts
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

	aiOpts    []AIOption
	aiIdx     int
	aiBuilt   map[int]ai.Provider
	autoClear bool
	history   []ai.Turn // AI conversation so far, sent with each capture

	content ui.Content // what the windows display
	last    string     // last answer, for copy
	// answer holds the settings the shown answer was made with; when they
	// differ from the current ones it is marked stale.
	answer  settings
	visible map[ui.View]bool

	runID   int
	cancel  context.CancelFunc
	results chan result
}

// settings are the choices an answer depends on.
type settings struct{ prompt, mode, model string }

type result struct {
	id  int
	set settings
	res pipeline.Result
	err error
}

func New(cfg config.Config, deps Deps) *Controller {
	opts := deps.AI
	if len(opts) == 0 {
		p := deps.Pipeline.AI
		opts = OptionsFrom(cfg, func(config.ResolvedModel) (ai.Provider, error) { return p, nil })
	}
	c := &Controller{
		deps:      deps,
		aiOpts:    opts,
		aiBuilt:   map[int]ai.Provider{},
		autoClear: cfg.AI.AutoClear,
		mode:      cfg.Capture.Mode,
		prompts:   cfg.Prompts.List,
		names:     slices.Sorted(maps.Keys(cfg.Prompts.List)),
		active:    cfg.Prompts.Default,
		visible:   map[ui.View]bool{},
		results:   make(chan result),
	}
	c.aiIdx = max(0, c.modelIndex(cfg.AI.DefaultModel))
	if deps.Pipeline.AI != nil { // nil: couldn't start; retried on capture
		c.aiBuilt[c.aiIdx] = deps.Pipeline.AI
	}
	return c
}

// Run handles actions until quit, ctx is done or actions is closed.
func (c *Controller) Run(ctx context.Context, actions <-chan string) error {
	defer c.stop()
	c.content = ui.Content{State: ui.Idle}
	if c.deps.Restore != nil {
		c.restore(*c.deps.Restore)
	}
	c.publish()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case a, ok := <-actions:
			if !ok || a == ActQuit {
				return nil
			}
			if a == ActReload {
				return ErrReload
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
		c.publish()
	case ActCycleModel:
		c.cycleModel()
	case ActToggleMode:
		if c.mode == config.ModeOCR {
			c.mode = config.ModeImage
		} else {
			c.mode = config.ModeOCR
		}
		c.deps.UI.Notify("mode: " + c.mode)
		c.publish()
	case ActCopyResult:
		c.copyLast()
	case ActClear:
		// Forget everything; open windows stay open and show the empty state.
		c.stop()
		c.last = ""
		c.history = nil
		c.content = ui.Content{State: ui.Idle}
		c.publish()
		c.deps.UI.Notify("cleared")
	default:
		if name, ok := strings.CutPrefix(action, ActSelectModel); ok {
			c.selectModel(name)
			return
		}
		c.deps.UI.Notify("unknown action: " + action)
	}
}

// start cancels any running analysis and begins a new one. It changes the
// windows' content to "thinking" but never opens a window.
func (c *Controller) start(ctx context.Context) {
	c.stop()
	if len(c.aiOpts) == 0 {
		c.refuse("no model yet: open Settings from the peekai tray icon and add a provider and a model")
		return
	}
	if c.deps.Pipeline.AI == nil {
		p, err := c.client(c.aiIdx)
		if err != nil {
			c.refuse(fmt.Sprintf("%s: %v (add the key in Settings)", c.aiOpts[c.aiIdx].Name, err))
			return
		}
		c.deps.Pipeline.AI = p
	}
	// Refuse before sending anything: an image to a model that can't read
	// it only wastes a request.
	if o := c.aiOpts[c.aiIdx]; c.mode == config.ModeImage && !o.Vision {
		c.refuse(fmt.Sprintf("model %q can't read images: switch to OCR mode or pick a vision model", o.Name))
		return
	}
	if c.mode == config.ModeOCR && c.deps.Pipeline.OCR == nil {
		engine, err := c.deps.NewOCR(c.deps.Pipeline.AI)
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
	set := c.current()
	opts := pipeline.Options{Mode: set.mode, Prompt: c.prompts[set.prompt]}

	if c.autoClear {
		c.history = nil // fresh AI conversation; the windows keep the last answer
	}
	opts.History = slices.Clone(c.history)
	c.content = ui.Content{State: ui.Thinking, Text: c.last}
	c.publish()
	go func() {
		res, err := p.Run(runCtx, opts)
		select {
		case c.results <- result{id: id, set: set, res: res, err: err}:
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
		c.content = ui.Content{State: ui.Failed, Text: r.err.Error()}
		c.deps.UI.Notify("error: " + r.err.Error())
	default:
		c.last = r.res.Answer
		c.history = append(c.history, r.res.Turn)
		c.content = ui.Content{State: ui.Ready, Text: c.last, At: time.Now()}
	}
	c.answer = r.set
	c.publish()
}

// publish sends the content with the current settings, marking a finished
// answer stale when it was made with other ones.
func (c *Controller) publish() {
	cur := c.current()
	c.content.Prompt, c.content.Mode, c.content.Model = cur.prompt, cur.mode, cur.model
	c.content.Free = len(c.aiOpts) > 0 && c.aiOpts[c.aiIdx].Free
	c.content.Models = c.models()
	done := c.content.State == ui.Ready || c.content.State == ui.Failed
	c.content.Stale = done && c.answer != cur
	c.content.Context = len(c.history)
	if c.autoClear {
		c.content.Context = 0 // the next capture starts fresh anyway
	}
	c.deps.UI.Update(c.content)
}

func (c *Controller) current() settings {
	s := settings{prompt: c.active, mode: c.mode, model: "no model"}
	if len(c.aiOpts) > 0 {
		s.model = c.aiOpts[c.aiIdx].Name
	}
	return s
}

// refuse fails a capture before anything is sent, explaining why.
func (c *Controller) refuse(msg string) {
	c.content = ui.Content{State: ui.Failed, Text: msg}
	c.answer = c.current()
	c.deps.UI.Notify(msg)
	c.publish()
}

// models describes the configured models for the windows and tray.
func (c *Controller) models() []ui.ModelInfo {
	out := make([]ui.ModelInfo, len(c.aiOpts))
	for i, o := range c.aiOpts {
		out[i] = ui.ModelInfo{Name: o.Name, Description: o.Description, Free: o.Free, Vision: o.Vision}
	}
	return out
}

func (c *Controller) modelIndex(name string) int {
	return slices.IndexFunc(c.aiOpts, func(o AIOption) bool { return o.Name == name })
}

// client returns the AI client for model i, building it on first use.
func (c *Controller) client(i int) (ai.Provider, error) {
	if p, ok := c.aiBuilt[i]; ok {
		return p, nil
	}
	p, err := c.aiOpts[i].New()
	if err != nil {
		return nil, err
	}
	c.aiBuilt[i] = p
	return p, nil
}

// use switches to model i. A running analysis keeps its model; the next
// capture uses the new one.
func (c *Controller) use(i int, p ai.Provider) {
	c.aiIdx = i
	c.deps.Pipeline.AI = p
	c.deps.Pipeline.OCR = nil // the llm engine reads with the model; rebuilt on next use
	c.deps.UI.Notify("model: " + c.aiOpts[i].Name)
	c.publish()
}

// selectModel switches to the model with this name (tray menu).
func (c *Controller) selectModel(name string) {
	i := c.modelIndex(name)
	if i < 0 {
		c.deps.UI.Notify("no model named " + name)
		return
	}
	p, err := c.client(i)
	if err != nil {
		c.deps.UI.Notify(fmt.Sprintf("can't use %s: %v", name, err))
		return
	}
	c.use(i, p)
}

// cycleModel switches to the next model whose client starts, skipping
// (and reporting) ones that fail, e.g. for a missing API key.
func (c *Controller) cycleModel() {
	if len(c.aiOpts) < 2 {
		c.deps.UI.Notify("only one model configured (add more to ai.models)")
		return
	}
	for step := 1; step < len(c.aiOpts); step++ {
		next := (c.aiIdx + step) % len(c.aiOpts)
		p, err := c.client(next)
		if err != nil {
			c.deps.UI.Notify(fmt.Sprintf("skipping %s: %v", c.aiOpts[next].Name, err))
			continue
		}
		c.use(next, p)
		return
	}
	c.deps.UI.Notify("no other model could start")
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
