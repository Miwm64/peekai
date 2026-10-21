// Package ui defines how results are shown. Implementations live in
// sub-packages (console now, on-top windows later).
package ui

// View is one of the two ways to show a result.
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

// ParseView maps config.Output ("popup" | "ticker") to a View.
func ParseView(s string) View {
	if s == "ticker" {
		return Ticker
	}
	return Popup
}

// Presenter shows results. Methods may be called from any goroutine.
type Presenter interface {
	Show(v View, text string)
	Hide(v View)
	Busy(on bool)
	// Notify shows a short status line, e.g. "mode: ocr" or an error.
	Notify(msg string)
}
