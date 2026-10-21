package hotkey

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"

	"golang.design/x/hotkey"
)

// Source delivers the action name of each pressed hotkey.
type Source interface {
	Events() <-chan string
	Close()
}

// Listener holds system-wide hotkey registrations.
type Listener struct {
	events chan string
	hks    []*hotkey.Hotkey
	done   chan struct{}
	wg     sync.WaitGroup
	once   sync.Once
}

// Listen registers bindings (action → shortcut) as global hotkeys.
// On macOS it must run after mainthread.Init has started the main loop.
func Listen(bindings map[string]string) (*Listener, error) {
	l := &Listener{events: make(chan string, 8), done: make(chan struct{})}
	var errs []error
	failed := map[string][]string{} // register error → actions, so a missing permission is reported once
	for _, action := range slices.Sorted(maps.Keys(bindings)) {
		s := bindings[action]
		sc, err := Parse(s)
		if err != nil {
			errs = append(errs, fmt.Errorf("hotkeys.%s: %w", action, err))
			continue
		}
		hk := hotkey.New(sysMods(sc.Mods), sysKeys[sc.Key])
		if err := hk.Register(); err != nil {
			failed[err.Error()] = append(failed[err.Error()], action)
			continue
		}
		l.hks = append(l.hks, hk)
		l.wg.Add(1)
		go l.forward(action, hk)
	}
	for msg, actions := range failed {
		errs = append(errs, fmt.Errorf("register %s: %s", strings.Join(actions, ", "), msg))
	}
	if err := errors.Join(errs...); err != nil {
		l.Close()
		return nil, err
	}
	return l, nil
}

func (l *Listener) forward(action string, hk *hotkey.Hotkey) {
	defer l.wg.Done()
	for {
		select {
		case _, ok := <-hk.Keydown():
			if !ok {
				return
			}
			select {
			case l.events <- action:
			default: // drop presses while the controller is busy
			}
		case <-l.done:
			return
		}
	}
}

func (l *Listener) Events() <-chan string { return l.events }

// Close unregisters every hotkey.
func (l *Listener) Close() {
	l.once.Do(func() {
		close(l.done)
		for _, hk := range l.hks {
			_ = hk.Unregister()
		}
		l.wg.Wait()
	})
}
