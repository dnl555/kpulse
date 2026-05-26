package engine

import (
	"context"
	"testing"
	"time"

	"github.com/dnl555/kpulse/internal/alert"
	"github.com/dnl555/kpulse/internal/notifiers"
)

func TestEngineResetClearsActiveAndDedupe(t *testing.T) {
	reg := notifiers.NewRegistry()
	reg.Register(&capture{})
	e := New(Options{
		Dedupe: NewDeduper(time.Hour), Router: NewRouter(nil, []string{"cap"}),
		Registry: reg, ResolutionEnabled: true,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.Run(ctx)

	a := alert.Alert{Severity: alert.Critical, Monitor: "pvc_usage", Namespace: "ns", ObjectKind: "PVC", ObjectName: "data", Reason: "Full"}
	e.Submit(a)
	waitFor(t, func() bool {
		_, present := e.opts.Dedupe.Snapshot()[a.Key()]
		return present
	})

	if got := len(e.ActiveSnapshot()); got != 1 {
		t.Fatalf("active before reset = %d, want 1", got)
	}
	activeN, dedupeN := e.Reset()
	if activeN != 1 || dedupeN != 1 {
		t.Errorf("Reset returned (%d, %d), want (1, 1)", activeN, dedupeN)
	}
	if got := len(e.ActiveSnapshot()); got != 0 {
		t.Errorf("active after reset = %d, want 0", got)
	}
	if got := len(e.opts.Dedupe.Snapshot()); got != 0 {
		t.Errorf("dedupe after reset = %d, want 0", got)
	}

	// After reset the same alert key should fire again, not be suppressed.
	if !e.opts.Dedupe.Allow(a) {
		t.Error("after reset, the same key should be allowed (not dedupe-suppressed)")
	}
}
