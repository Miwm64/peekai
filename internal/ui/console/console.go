// Package console is a ui.Presenter that prints to a terminal, for
// debugging and machines without a display.
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
	mu      sync.Mutex
	out     io.Writer
	content ui.Content
	visible map[ui.View]bool
}

func New(out io.Writer) *Presenter {
	return &Presenter{out: out, visible: map[ui.View]bool{}}
}

func (p *Presenter) Update(c ui.Content) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.content = c
	p.printf("[%s]\n", c.Header())
	for v, on := range p.visible {
		if on {
			p.print(v)
		}
	}
}

func (p *Presenter) Show(v ui.View) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.visible[v] = true
	p.print(v)
}

func (p *Presenter) Hide(v ui.View) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.visible[v] = false
	p.printf("[%s hidden]\n", v)
}

func (p *Presenter) Notify(msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.printf("[%s] %s\n", time.Now().Format("15:04:05"), msg)
}

func (p *Presenter) print(v ui.View) {
	p.printf("── %s: %s ──\n%s\n%s\n", v, p.content.Header(), p.content.Body(), strings.Repeat("─", 40))
}

func (p *Presenter) printf(format string, args ...any) { fmt.Fprintf(p.out, format, args...) }
