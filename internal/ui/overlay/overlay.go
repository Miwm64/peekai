// Package overlay is the on-screen ui.Presenter: a popup, a running line
// and a small status pill, drawn with go-gui and kept above other apps
// (also fullscreen ones) by ui/ontop. All windows start hidden.
package overlay

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"

	"github.com/Miwm64/peekai/internal/config"
	"github.com/Miwm64/peekai/internal/ui"
	"github.com/Miwm64/peekai/internal/ui/ontop"
)

// Window titles; ui/ontop finds the native windows by them.
const (
	popupTitle  = "peekai-popup"
	tickerTitle = "peekai-ticker"
	statusTitle = "peekai-status"
)

const (
	margin      = 24
	statusW     = 260
	statusH     = 34
	noticeAfter = 2 * time.Second
	frame       = time.Second / 60
)

var (
	fg       = gui.RGB(238, 238, 242)
	textSize = float32(16)
)

type textState struct{ text string }

type tickerState struct {
	text   string
	offset float32 // how far the text has moved left, in px
	textW  float32
}

// Overlay implements ui.Presenter. Create it with New and start it with Run
// on the main goroutine.
type Overlay struct {
	cfg config.UI

	app    *gui.App
	popup  *gui.Window
	ticker *gui.Window
	status *gui.Window

	mu        sync.Mutex
	stopTick  chan struct{} // closes to stop the running line animation
	busy      bool
	noticeGen int
	popupGen  int
}

var _ ui.Presenter = (*Overlay)(nil)

func New(cfg config.UI) *Overlay {
	o := &Overlay{cfg: cfg, app: gui.NewApp()}
	o.popup = newWindow(popupTitle, cfg.Popup.Width, cfg.Popup.MaxHeight, &textState{}, o.popupView)
	o.ticker = newWindow(tickerTitle, 800, cfg.Ticker.Height, &tickerState{}, o.tickerView)
	o.status = newWindow(statusTitle, statusW, statusH, &textState{}, o.statusView)
	return o
}

func newWindow(title string, w, h int, state any, view func(*gui.Window) gui.View) *gui.Window {
	return gui.NewWindow(gui.WindowCfg{
		Title: title, Width: w, Height: h,
		Decorations: gui.DecorationNone, Transparent: true, BgColor: gui.RGBA(0, 0, 0, 0),
		State:  state,
		OnInit: func(w *gui.Window) { w.SetView(view) },
	})
}

// Run starts the UI on the calling goroutine, which must be the main one,
// and runs work alongside it. When work returns the process exits: an
// overlay app's run loop doesn't end when its windows close.
func (o *Overlay) Run(work func() error) {
	go func() {
		o.onMain(func() {
			ontop.Background()
			for _, t := range []string{popupTitle, tickerTitle, statusTitle} {
				ontop.Hide(t)
			}
		})
		code := 0
		if err := work(); err != nil {
			fmt.Fprintln(os.Stderr, "peekai:", err)
			code = 1
		}
		os.Exit(code)
	}()
	backend.RunApp(o.app, o.popup, o.ticker, o.status)
}

// onMain runs fn on the UI thread.
func (o *Overlay) onMain(fn func()) {
	o.popup.QueueCommand(func(*gui.Window) { fn() })
}

// show places a window on the display under the mouse, puts it on top
// without stealing focus, and makes go-gui redraw it (it stops drawing
// hidden windows and doesn't see ontop showing them).
func show(w *gui.Window, title string, place func(screen ontop.Rect) ontop.Rect, opts ontop.Options) {
	ontop.Place(title, place(ontop.Screen()), opts)
	ontop.Show(title)
	w.InvalidateLayout()
}

func (o *Overlay) Show(v ui.View, text string) {
	if v == ui.Ticker {
		o.showTicker(text)
		return
	}
	o.mu.Lock()
	o.popupGen++
	gen := o.popupGen
	o.mu.Unlock()

	o.onMain(func() {
		gui.State[textState](o.popup).text = text
		show(o.popup, popupTitle, func(s ontop.Rect) ontop.Rect {
			return ontop.Rect{X: s.X + margin, Y: s.Y + margin, W: float64(o.cfg.Popup.Width), H: float64(o.cfg.Popup.MaxHeight)}
		}, ontop.Options{})
	})
	if d := time.Duration(o.cfg.Popup.AutoHide); d > 0 {
		time.AfterFunc(d, func() {
			o.mu.Lock()
			current := o.popupGen == gen
			o.mu.Unlock()
			if current {
				o.Hide(ui.Popup)
			}
		})
	}
}

func (o *Overlay) Hide(v ui.View) {
	if v == ui.Ticker {
		o.stopTicker()
		o.onMain(func() { ontop.Hide(tickerTitle) })
		return
	}
	o.mu.Lock()
	o.popupGen++ // cancel a pending autohide
	o.mu.Unlock()
	o.onMain(func() { ontop.Hide(popupTitle) })
}

func (o *Overlay) Busy(on bool) {
	o.mu.Lock()
	o.busy = on
	o.noticeGen++
	o.mu.Unlock()
	if on {
		o.showStatus("● thinking…")
	} else {
		o.onMain(func() { ontop.Hide(statusTitle) })
	}
}

// Notify shows msg in the status pill for a moment (errors and switches
// like "mode: ocr"), and also logs it.
func (o *Overlay) Notify(msg string) {
	fmt.Println("peekai:", msg)
	o.mu.Lock()
	o.noticeGen++
	gen := o.noticeGen
	o.mu.Unlock()
	o.showStatus(msg)
	time.AfterFunc(noticeAfter, func() {
		o.mu.Lock()
		stale := o.noticeGen != gen
		busy := o.busy
		o.mu.Unlock()
		switch {
		case stale:
		case busy:
			o.showStatus("● thinking…")
		default:
			o.onMain(func() { ontop.Hide(statusTitle) })
		}
	})
}

func (o *Overlay) showStatus(text string) {
	o.onMain(func() {
		gui.State[textState](o.status).text = text
		show(o.status, statusTitle, func(s ontop.Rect) ontop.Rect {
			return ontop.Rect{X: s.X + s.W - statusW - margin, Y: s.Y + margin, W: statusW, H: statusH}
		}, ontop.Options{ClickThrough: true})
	})
}

func (o *Overlay) showTicker(text string) {
	o.stopTicker()
	line := strings.Join(strings.Fields(text), " ") // one line: newlines → spaces
	stop := make(chan struct{})
	o.mu.Lock()
	o.stopTick = stop
	o.mu.Unlock()

	o.onMain(func() {
		var screenW float64
		show(o.ticker, tickerTitle, func(s ontop.Rect) ontop.Rect {
			screenW = s.W
			h := float64(o.cfg.Ticker.Height)
			y := s.Y + s.H - h
			if o.cfg.Ticker.Position == "top" {
				y = s.Y
			}
			return ontop.Rect{X: s.X, Y: y, W: s.W, H: h}
		}, ontop.Options{ClickThrough: true})
		st := gui.State[tickerState](o.ticker)
		st.text = line
		st.textW = o.ticker.TextWidth(line, tickerStyle())
		st.offset = -float32(screenW) // start just off the right edge
	})

	go o.animate(stop)
}

// animate moves the running line left at the configured speed, looping.
func (o *Overlay) animate(stop chan struct{}) {
	t := time.NewTicker(frame)
	defer t.Stop()
	last := time.Now()
	for {
		select {
		case <-stop:
			return
		case now := <-t.C:
			dt := float32(now.Sub(last).Seconds())
			last = now
			o.ticker.QueueCommand(func(w *gui.Window) {
				st := gui.State[tickerState](w)
				st.offset += float32(o.cfg.Ticker.Speed) * dt
				if st.offset > st.textW {
					winW, _ := w.WindowSize()
					st.offset = -float32(winW)
				}
				w.InvalidateRender()
			})
		}
	}
}

func (o *Overlay) stopTicker() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.stopTick != nil {
		close(o.stopTick)
		o.stopTick = nil
	}
}

// ── views ──

func bg(opacity float64) gui.Color { return gui.RGBA(18, 18, 22, uint8(opacity*255)) }

func tickerStyle() gui.TextStyle { return gui.TextStyle{Size: textSize + 2, Color: fg} }

func (o *Overlay) popupView(w *gui.Window) gui.View {
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFill,
		Color:      bg(o.cfg.Popup.Opacity),
		Radius:     gui.RadiusPx(10),
		Padding:    gui.PadAll(16),
		Scrollable: true,
		Content: []gui.View{gui.Text(gui.TextCfg{
			Text:      gui.State[textState](w).text,
			Mode:      gui.TextModeWrap,
			TextStyle: gui.TextStyle{Size: textSize, Color: fg, LineSpacing: 4},
		})},
	})
}

func (o *Overlay) tickerView(w *gui.Window) gui.View {
	st := gui.State[tickerState](w)
	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Color:   bg(o.cfg.Ticker.Opacity),
		Padding: gui.PadVH(6, 0),
		Clip:    true,
		Content: []gui.View{gui.Row(gui.ContainerCfg{
			TransformFn: func() gui.Transform { return gui.Translate(-gui.State[tickerState](w).offset, 0) },
			Content:     []gui.View{gui.Text(gui.TextCfg{Text: st.text, Mode: gui.TextModeSingleLine, TextStyle: tickerStyle()})},
		})},
	})
}

func (o *Overlay) statusView(w *gui.Window) gui.View {
	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Color:   bg(0.85),
		Radius:  gui.RadiusPx(17),
		Padding: gui.PadVH(7, 14),
		Content: []gui.View{gui.Text(gui.TextCfg{
			Text:      gui.State[textState](w).text,
			Mode:      gui.TextModeSingleLine,
			TextStyle: gui.TextStyle{Size: 14, Color: fg},
		})},
	})
}
