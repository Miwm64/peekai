// Package overlay is the on-screen ui.Presenter: a popup, a running line
// and an optional status pill, drawn with go-gui and kept above other apps
// (also fullscreen ones) by ui/ontop. Windows start hidden and open only
// through Show; Update changes what they display.
package overlay

import (
	"context"
	"fmt"
	"image"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"

	"github.com/Miwm64/peekai/internal/capture"
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
	tickerInset = 16 // gap before the running line's text, px
	tickerLift  = 6  // moves the running line's text up, px
)

var (
	fg       = gui.RGB(238, 238, 242)
	dim      = gui.RGB(160, 160, 170)
	textSize = float32(16)
)

type popupState struct{ content ui.Content }

type textState struct{ text string }

type tickerState struct {
	text  string
	x     float32 // left edge of the text in the bar, px; shrinks as it moves left
	textW float32
}

// Overlay implements ui.Presenter. Create it with New and start it with Run
// on the main goroutine.
type Overlay struct {
	app    *gui.App
	popup  *gui.Window
	ticker *gui.Window
	status *gui.Window

	mu       sync.Mutex
	uiCfg    config.UI // read through ui(); replaced by SetUI on reload
	content  ui.Content
	visible  map[ui.View]bool
	stopTick chan struct{} // closes to stop the running line animation

	actions   chan string           // tray clicks, see Actions
	tray      *gui.SystemTrayHandle // UI thread only
	noticeGen int
}

var _ ui.Presenter = (*Overlay)(nil)

func New(cfg config.UI) *Overlay {
	o := &Overlay{uiCfg: cfg, app: gui.NewApp(), visible: map[ui.View]bool{}, actions: make(chan string, 4)}
	o.popup = newWindow(popupTitle, cfg.Popup.Width, cfg.Popup.MaxHeight, &popupState{}, o.popupView)
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
			o.startTray()
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

// ui returns the current UI settings.
func (o *Overlay) ui() config.UI {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.uiCfg
}

// SetUI applies new UI settings (after a config reload). Open windows pick
// them up when shown again; colors and opacity change at once.
func (o *Overlay) SetUI(cfg config.UI) {
	o.mu.Lock()
	o.uiCfg = cfg
	o.mu.Unlock()
	o.onMain(func() {
		for _, w := range []*gui.Window{o.popup, o.ticker, o.status} {
			w.InvalidateLayout()
		}
	})
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

// Update changes what the windows show. Open windows refresh; closed ones
// stay closed and show the new content when opened.
func (o *Overlay) Update(c ui.Content) {
	o.mu.Lock()
	o.content = c
	popupOpen, tickerOpen := o.visible[ui.Popup], o.visible[ui.Ticker]
	o.mu.Unlock()

	o.onMain(func() {
		gui.State[popupState](o.popup).content = c
		if popupOpen {
			o.popup.InvalidateLayout()
		}
		o.updateTray(c)
	})
	if tickerOpen {
		o.startTicker(tickerText(c), false)
	}
	if o.ui().Status.Enabled {
		switch c.State {
		case ui.Thinking:
			o.showStatus("● thinking…")
		case ui.Ready:
			o.Notify("✓ answer ready")
		}
	}
}

func (o *Overlay) Show(v ui.View) {
	o.mu.Lock()
	o.visible[v] = true
	c := o.content
	o.mu.Unlock()

	if v == ui.Ticker {
		o.startTicker(tickerText(c), true)
		return
	}
	o.onMain(func() {
		gui.State[popupState](o.popup).content = c
		show(o.popup, popupTitle, func(s ontop.Rect) ontop.Rect {
			return ontop.Rect{X: s.X + margin, Y: s.Y + margin, W: float64(o.ui().Popup.Width), H: float64(o.ui().Popup.MaxHeight)}
		}, ontop.Options{})
	})
}

func (o *Overlay) Hide(v ui.View) {
	o.mu.Lock()
	o.visible[v] = false
	o.mu.Unlock()
	if v == ui.Ticker {
		o.stopTicker()
		o.onMain(func() { ontop.Hide(tickerTitle) })
		return
	}
	o.onMain(func() { ontop.Hide(popupTitle) })
}

// Notify logs msg and, if the status pill is enabled, shows it briefly.
func (o *Overlay) Notify(msg string) {
	fmt.Println("peekai:", msg)
	if !o.ui().Status.Enabled {
		return
	}
	o.mu.Lock()
	o.noticeGen++
	gen := o.noticeGen
	o.mu.Unlock()
	o.showStatus(msg)
	time.AfterFunc(noticeAfter, func() {
		o.mu.Lock()
		stale := o.noticeGen != gen
		thinking := o.content.State == ui.Thinking
		o.mu.Unlock()
		switch {
		case stale:
		case thinking:
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

// tickerText is the single line the running line scrolls.
func tickerText(c ui.Content) string {
	var s string
	switch c.State {
	case ui.Thinking:
		s = "● thinking… (" + c.Prompt + ")"
	case ui.Failed:
		s = "✕ " + c.Text
	default:
		s = c.Body()
	}
	return strings.Join(strings.Fields(s), " ") // one line: newlines → spaces
}

// startTicker scrolls line in the running line. With place it also puts
// the window on screen (on Show); otherwise it only swaps the text of the
// already open window.
func (o *Overlay) startTicker(line string, place bool) {
	o.stopTicker()
	stop := make(chan struct{})
	o.mu.Lock()
	o.stopTick = stop
	o.mu.Unlock()

	o.onMain(func() {
		if place {
			show(o.ticker, tickerTitle, func(s ontop.Rect) ontop.Rect {
				h := float64(o.ui().Ticker.Height)
				y := s.Y + s.H - h
				if o.ui().Ticker.Position == "top" {
					y = s.Y
				}
				return ontop.Rect{X: s.X, Y: y, W: s.W, H: h}
			}, ontop.Options{ClickThrough: true})
		}
		st := gui.State[tickerState](o.ticker)
		st.text = line
		st.textW = o.ticker.TextWidth(line, tickerStyle())
		winW, _ := o.ticker.WindowSize()
		st.x = float32(winW) // enters right at the right edge
		if o.ui().Ticker.Start == "left" {
			st.x = tickerInset // already visible at the left
		}
		o.ticker.InvalidateLayout()
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
				st.x -= float32(o.ui().Ticker.Speed) * dt
				if st.x < -st.textW { // gone off the left: re-enter at the right edge
					winW, _ := w.WindowSize()
					st.x = float32(winW)
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
	c := gui.State[popupState](w).content
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFill,
		Color:      bg(o.ui().Popup.Opacity),
		Radius:     gui.RadiusPx(10),
		Padding:    gui.PadAll(16),
		Spacing:    gui.SpacingPx(10),
		Scrollable: true,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      c.Header(),
				Mode:      gui.TextModeSingleLine,
				TextStyle: gui.TextStyle{Size: textSize - 3, Color: dim},
			}),
			gui.Text(gui.TextCfg{
				Text:      c.Body(),
				Mode:      gui.TextModeWrap,
				TextStyle: gui.TextStyle{Size: textSize, Color: fg, LineSpacing: 4},
			}),
		},
	})
}

func (o *Overlay) tickerView(w *gui.Window) gui.View {
	st := gui.State[tickerState](w)
	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Color:   bg(o.ui().Ticker.Opacity),
		Padding: gui.PadVH(6, 0),
		Clip:    true,
		Content: []gui.View{gui.Row(gui.ContainerCfg{
			// go-gui draws the line's text low in its box; lift it to center it.
			TransformFn: func() gui.Transform { return gui.Translate(gui.State[tickerState](w).x, -tickerLift) },
			Content:     []gui.View{gui.Text(gui.TextCfg{Text: st.text, Mode: gui.TextModeSingleLine, TextStyle: tickerStyle()})},
		})},
	})
}

func (o *Overlay) statusView(w *gui.Window) gui.View {
	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Color:   bg(0.85),
		Radius:  gui.RadiusPx(17),
		Padding: gui.PadVH(0, 14),
		VAlign:  gui.VAlignMiddle,
		Content: []gui.View{gui.Text(gui.TextCfg{
			Text:      gui.State[textState](w).text,
			Mode:      gui.TextModeSingleLine,
			TextStyle: gui.TextStyle{Size: 14, Color: fg},
		})},
	})
}

// GuardCapture keeps the overlay out of screenshots taken by c. macOS and
// Windows exclude our windows themselves; elsewhere (X11) the open windows
// are hidden for the moment of the capture.
func (o *Overlay) GuardCapture(c capture.Capturer) capture.Capturer {
	if ontop.ExcludesFromCapture {
		return c
	}
	return guarded{o: o, next: c}
}

type guarded struct {
	o    *Overlay
	next capture.Capturer
}

// compositorDelay lets the window manager remove hidden windows from the
// screen before the capture reads it.
const compositorDelay = 80 * time.Millisecond

func (g guarded) Capture(ctx context.Context) (image.Image, error) {
	titles := []string{popupTitle, tickerTitle, statusTitle}
	hidden := make(chan struct{})
	g.o.onMain(func() {
		for _, t := range titles {
			ontop.Hide(t)
		}
		close(hidden)
	})
	select {
	case <-hidden:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	time.Sleep(compositorDelay)
	defer g.o.restore()
	return g.next.Capture(ctx)
}

// restore shows again the windows that were open before a guarded capture.
func (o *Overlay) restore() {
	o.mu.Lock()
	popup, ticker := o.visible[ui.Popup], o.visible[ui.Ticker]
	status := o.uiCfg.Status.Enabled && o.content.State == ui.Thinking
	o.mu.Unlock()
	o.onMain(func() {
		if popup {
			ontop.Show(popupTitle)
		}
		if ticker {
			ontop.Show(tickerTitle)
		}
		if status {
			ontop.Show(statusTitle)
		}
	})
}
