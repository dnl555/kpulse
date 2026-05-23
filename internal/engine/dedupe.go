// Package engine handles dedupe, routing, and digest batching for alerts.
package engine

import (
	"sync"
	"time"

	"github.com/dnl555/kpulse/internal/alert"
)

type Deduper struct {
	mu     sync.Mutex
	window time.Duration
	last   map[string]time.Time
	clock  func() time.Time
}

func NewDeduper(window time.Duration) *Deduper {
	return &Deduper{window: window, last: map[string]time.Time{}, clock: time.Now}
}

func (d *Deduper) Allow(a alert.Alert) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := d.clock()
	if prev, ok := d.last[a.Key()]; ok && now.Sub(prev) < d.window {
		return false
	}
	d.last[a.Key()] = now
	return true
}

func (d *Deduper) Snapshot() map[string]time.Time {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make(map[string]time.Time, len(d.last))
	for k, v := range d.last {
		out[k] = v
	}
	return out
}

func (d *Deduper) Restore(m map[string]time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for k, v := range m {
		d.last[k] = v
	}
}
