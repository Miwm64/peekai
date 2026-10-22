// Package ui defines how results are shown. Implementations live in
// sub-packages (console, and the on-screen overlay).
package ui

import "time"

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
// error message (Failed).
type Content struct {
	State  State
	Text   string
	Prompt string
	Mode   string
	At     time.Time // when the answer arrived
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

// Header is a one-line summary, e.g. "● thinking… · explain · image".
func (c Content) Header() string {
	meta := c.Prompt + " · " + c.Mode
	switch c.State {
	case Thinking:
		return "● thinking… · " + meta
	case Ready:
		return "✓ answer · " + meta + " · " + c.At.Format("15:04")
	case Failed:
		return "✕ failed · " + meta
	}
	return "no result yet · " + meta
}

// Body is the main text to display.
func (c Content) Body() string {
	switch c.State {
	case Thinking:
		if c.Text != "" {
			return "(previous answer)\n" + c.Text
		}
		return "Working on it…"
	case Idle:
		return "Press the capture shortcut to analyze the screen."
	}
	return c.Text
}
