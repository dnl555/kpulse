package engine

import (
	"testing"
	"time"

	"github.com/dnl555/kpulse/internal/alert"
)

func TestStatsRecorderTracksFireAndResolveCounters(t *testing.T) {
	s := newStatsRecorder(10)
	a := alert.Alert{Monitor: "pod_crashes", Severity: alert.Critical, FiredAt: time.Unix(1700000000, 0).UTC()}
	s.record(a, []string{"email"})
	s.record(a, []string{"email"})
	resolved := a
	resolved.State = alert.StateResolved
	s.record(resolved, []string{"email"})

	mons := s.Monitors()
	if len(mons) != 1 {
		t.Fatalf("got %d monitors", len(mons))
	}
	if mons[0].Fires != 2 || mons[0].Resolves != 1 {
		t.Errorf("counters wrong: %+v", mons[0])
	}
	if mons[0].LastEvent != "resolved" {
		t.Errorf("last event = %q", mons[0].LastEvent)
	}
}

func TestStatsRecorderRingBufferNewestFirst(t *testing.T) {
	s := newStatsRecorder(3)
	for i := 0; i < 5; i++ {
		s.record(alert.Alert{
			Monitor:  "x",
			Severity: alert.Info,
			Title:    string(rune('A' + i)),
			FiredAt:  time.Unix(int64(1700000000+i), 0).UTC(),
		}, nil)
	}
	recent := s.Recent()
	if len(recent) != 3 {
		t.Fatalf("ring buffer kept %d, want 3", len(recent))
	}
	// newest first
	if recent[0].Title != "E" || recent[1].Title != "D" || recent[2].Title != "C" {
		t.Errorf("ordering wrong: %v %v %v", recent[0].Title, recent[1].Title, recent[2].Title)
	}
}

func TestStatsRecorderEmptyMonitorsOnStartup(t *testing.T) {
	s := newStatsRecorder(10)
	if len(s.Monitors()) != 0 || len(s.Recent()) != 0 {
		t.Error("fresh recorder should be empty")
	}
}
