// Package console is a ui.Presenter that prints to a terminal. It stands in
// until the on-top windows exist and stays useful for debugging.
package console

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/Miwm64/peekai/internal/ui"
)

type Presenter struct {
	mu  sync.Mutex
	out io.Writer
}

func New(out io.Writer) *Presenter { return &Presenter{out: out} }

func (p *Presenter) Show(v ui.View, text string) {
	p.printf("── %s ──\n%s\n%s\n", v, text, strings.Repeat("─", 40))
}

func (p *Presenter) Hide(v ui.View) { p.printf("[%s hidden]\n", v) }

func (p *Presenter) Busy(on bool) {
	if on {
		p.printf("[working…]\n")
	}
}

func (p *Presenter) Notify(msg string) { p.printf("[%s] %s\n", time.Now().Format("15:04:05"), msg) }

func (p *Presenter) printf(format string, args ...any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintf(p.out, format, args...)
}
