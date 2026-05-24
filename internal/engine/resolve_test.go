package engine

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/dnl555/kpulse/internal/alert"
	"github.com/dnl555/kpulse/internal/notifiers"
)

func TestResolveEmitsWhenPreviouslyFiring(t *testing.T) {
	reg := notifiers.NewRegistry()
	cap := &capture{}
	reg.Register(cap)
	e := New(Options{
		Dedupe: NewDeduper(time.Hour), Router: NewRouter(nil, []string{"cap"}),
		Registry: reg, ResolutionEnabled: true,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.Run(ctx)

	a := alert.Alert{Severity: alert.Critical, Monitor: "pvc_usage", Namespace: "ns", ObjectKind: "PersistentVolumeClaim", ObjectName: "data", Reason: "PVCHighUsage", Title: "PVC at 95%"}
	e.Submit(a)
	waitFor(t, func() bool { return cap.Count() == 1 })

	e.Resolve(a)
	waitFor(t, func() bool { return cap.Count() == 2 })

	cap.mu.Lock()
	defer cap.mu.Unlock()
	if cap.sent[1].State != alert.StateResolved {
		t.Errorf("second alert state = %v want resolved", cap.sent[1].State)
	}
	if !strings.Contains(cap.sent[1].Title, "Resolved") && !strings.Contains(cap.sent[1].Title, "PVC") {
		t.Errorf("resolved title odd: %q", cap.sent[1].Title)
	}
}

func TestResolveNoopWhenNeverFired(t *testing.T) {
	reg := notifiers.NewRegistry()
	cap := &capture{}
	reg.Register(cap)
	e := New(Options{
		Dedupe: NewDeduper(time.Hour), Router: NewRouter(nil, []string{"cap"}),
		Registry: reg, ResolutionEnabled: true,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.Run(ctx)

	e.Resolve(alert.Alert{Monitor: "pvc_usage", Namespace: "ns", ObjectKind: "PersistentVolumeClaim", ObjectName: "data", Reason: "PVCHighUsage"})
	// Wait briefly; ensure nothing was sent.
	time.Sleep(100 * time.Millisecond)
	if cap.Count() != 0 {
		t.Errorf("expected no resolved alert sent; got %d", cap.Count())
	}
}

func TestResolveDisabledByFlag(t *testing.T) {
	reg := notifiers.NewRegistry()
	cap := &capture{}
	reg.Register(cap)
	e := New(Options{
		Dedupe: NewDeduper(time.Hour), Router: NewRouter(nil, []string{"cap"}),
		Registry: reg, ResolutionEnabled: false,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.Run(ctx)

	a := alert.Alert{Severity: alert.Critical, Monitor: "x", Namespace: "ns", ObjectKind: "Pod", ObjectName: "p", Reason: "OOM", Title: "T"}
	e.Submit(a)
	waitFor(t, func() bool { return cap.Count() == 1 })

	e.Resolve(a)
	time.Sleep(100 * time.Millisecond)
	if cap.Count() != 1 {
		t.Errorf("resolve should be no-op when disabled; got %d sends", cap.Count())
	}
}

func TestReconcileResolvesMissingKeys(t *testing.T) {
	reg := notifiers.NewRegistry()
	cap := &capture{}
	reg.Register(cap)
	e := New(Options{
		Dedupe: NewDeduper(time.Hour), Router: NewRouter(nil, []string{"cap"}),
		Registry: reg, ResolutionEnabled: true,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.Run(ctx)

	a1 := alert.Alert{Severity: alert.Warning, Monitor: "pvc_usage", Namespace: "ns", ObjectKind: "PersistentVolumeClaim", ObjectName: "data1", Reason: "PVCHighUsage", Title: "data1 high"}
	a2 := alert.Alert{Severity: alert.Warning, Monitor: "pvc_usage", Namespace: "ns", ObjectKind: "PersistentVolumeClaim", ObjectName: "data2", Reason: "PVCHighUsage", Title: "data2 high"}
	e.Reconcile("pvc_usage", []alert.Alert{a1, a2})
	waitFor(t, func() bool { return cap.Count() == 2 })

	// Now only data2 is firing; data1 should resolve.
	e.Reconcile("pvc_usage", []alert.Alert{a2})
	waitFor(t, func() bool { return cap.Count() == 3 })

	cap.mu.Lock()
	defer cap.mu.Unlock()
	last := cap.sent[2]
	if last.State != alert.StateResolved {
		t.Errorf("last sent should be resolved; got state=%v title=%q", last.State, last.Title)
	}
	if last.ObjectName != "data1" {
		t.Errorf("resolved should be for data1; got %q", last.ObjectName)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for !cond() {
		select {
		case <-deadline:
			t.Fatal("condition not met within 2s")
		case <-time.After(10 * time.Millisecond):
		}
	}
}
