// Package ui defines how results are shown. Implementations live in
// sub-packages (console, and the on-screen overlay).
package ui

import (
	"fmt"
	"time"
)

// View is one of the two windows.
type View int

const (
	Popup View = iota
	Ticker
)

func (v View) String() string {
	if v == Ticker {
		return "ticker"
	}
	return "popup"
}

// State is where the current analysis stands.
type State int

const (
	Idle     State = iota // nothing captured yet
	Thinking              // waiting for the AI
	Ready                 // answer available
	Failed                // last analysis failed
)

// Content is what the windows display. Text is the answer (Ready) or the
// error message (Failed). Prompt, Mode and Model (the model's name in the
// config) are the current settings; Stale marks an answer made with
// different ones.
type Content struct {
	State   State
	Text    string
	Stale   bool
	Prompt  string
	Mode    string
	Model   string
	Free    bool        // the current model is free
	Models  []ModelInfo // all configured models, for the tray menu
	Context int         // earlier exchanges the AI will see with the next capture
	At      time.Time   // when the answer arrived
}

// ModelInfo describes a configured model.
type ModelInfo struct {
	Name        string
	Description string
	Free        bool
	Vision      bool
}

// Presenter shows results. Windows open and close only through Show and
// Hide, which the controller calls on toggle shortcuts; Update changes what
// they display, also while open, but never opens one. Methods may be called
// from any goroutine.
type Presenter interface {
	Update(c Content)
	Show(v View)
	Hide(v View)
	// Notify reports a short message, e.g. "mode: ocr".
	Notify(msg string)
}

// Header is the window's title line, e.g.
// "✓ answer · explain · image · qwen-local · free · 14:02".
func (c Content) Header() string {
	meta := c.Prompt + " · " + c.Mode + " · " + c.Model
	if c.Free {
		meta += " · free"
	}
	if c.Context > 0 {
		meta += fmt.Sprintf(" · ctx %d", c.Context)
	}
	switch {
	case c.State == Thinking:
		return "● thinking… · " + meta
	case c.Stale:
		return "previous answer · " + meta
	case c.State == Ready:
		return "✓ answer · " + meta + " · " + c.At.Format("15:04")
	case c.State == Failed:
		return "✕ failed · " + meta
	}
	return "no result yet · " + meta
}

// Body is the main text to display. While thinking, or after the settings
// changed, the last answer stays visible marked as previous.
func (c Content) Body() string {
	switch {
	case c.State == Idle:
		return "Press the capture shortcut to analyze the screen."
	case c.State == Thinking && c.Text == "":
		return "Working on it…"
	case c.State == Thinking, c.Stale:
		return "(previous answer)\n" + c.Text
	}
	return c.Text
}
