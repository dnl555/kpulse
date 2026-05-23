package engine

import (
	"testing"
	"time"

	"github.com/dnl555/kpulse/internal/alert"
)

func TestDedupeAllowsFirstThenSuppresses(t *testing.T) {
	now := time.Now()
	d := NewDeduper(10 * time.Minute)
	d.clock = func() time.Time { return now }
	a := alert.Alert{Monitor: "x", Namespace: "ns", ObjectKind: "Pod", ObjectName: "p", Reason: "r"}

	if !d.Allow(a) {
		t.Error("first should be allowed")
	}
	if d.Allow(a) {
		t.Error("immediate repeat should be suppressed")
	}
	d.clock = func() time.Time { return now.Add(11 * time.Minute) }
	if !d.Allow(a) {
		t.Error("after window should fire again")
	}
}

func TestSnapshotRestore(t *testing.T) {
	d := NewDeduper(time.Hour)
	a := alert.Alert{Monitor: "x"}
	d.Allow(a)
	snap := d.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("snapshot len %d", len(snap))
	}
	d2 := NewDeduper(time.Hour)
	d2.Restore(snap)
	if d2.Allow(a) {
		t.Error("restored deduper should suppress")
	}
}
