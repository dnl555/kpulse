package engine

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dnl555/kpulse/internal/alert"
	"github.com/dnl555/kpulse/internal/notifiers"
)

type capture struct {
	mu   sync.Mutex
	sent []alert.Alert
}

func (c *capture) Name() string { return "cap" }
func (c *capture) Send(_ context.Context, a alert.Alert) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sent = append(c.sent, a)
	return nil
}
func (c *capture) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.sent)
}

func TestEngineCriticalFiresImmediately(t *testing.T) {
	reg := notifiers.NewRegistry()
	cap := &capture{}
	reg.Register(cap)
	e := New(Options{
		Dedupe:           NewDeduper(time.Hour),
		Router:           NewRouter(nil, []string{"cap"}),
		Registry:         reg,
		DigestEnabled:    true,
		DigestInterval:   time.Minute,
		DigestSeverities: []alert.Severity{alert.Info, alert.Warning},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.Run(ctx)

	e.Submit(alert.Alert{Severity: alert.Critical, Monitor: "x", Title: "t"})
	deadline := time.After(2 * time.Second)
	for cap.Count() == 0 {
		select {
		case <-deadline:
			t.Fatal("critical alert not delivered")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestEngineDigestBatchesLowSeverity(t *testing.T) {
	reg := notifiers.NewRegistry()
	cap := &capture{}
	reg.Register(cap)
	e := New(Options{
		Dedupe:           NewDeduper(time.Hour),
		Router:           NewRouter(nil, []string{"cap"}),
		Registry:         reg,
		DigestEnabled:    true,
		DigestInterval:   100 * time.Millisecond,
		DigestSeverities: []alert.Severity{alert.Info, alert.Warning},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.Run(ctx)

	e.Submit(alert.Alert{Severity: alert.Warning, Monitor: "x", ObjectName: "a"})
	e.Submit(alert.Alert{Severity: alert.Warning, Monitor: "x", ObjectName: "b"})

	deadline := time.After(2 * time.Second)
	for cap.Count() == 0 {
		select {
		case <-deadline:
			t.Fatal("digest not delivered")
		case <-time.After(20 * time.Millisecond):
		}
	}
	cap.mu.Lock()
	defer cap.mu.Unlock()
	if !strings.Contains(cap.sent[0].Body, "2 alerts") {
		t.Errorf("digest body missing count: %s", cap.sent[0].Body)
	}
}
