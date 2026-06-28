package engine

import (
	"sync"
	"time"

	"github.com/dnl555/kpulse/internal/alert"
)

// MonitorStats summarises what one monitor has done since process start.
type MonitorStats struct {
	Name      string    `json:"name"`
	Fires     int       `json:"fires"`
	Resolves  int       `json:"resolves"`
	LastFired time.Time `json:"last_fired,omitzero"`
	LastEvent string    `json:"last_event,omitempty"` // "firing" | "resolved"
}

// RecentEntry is a single past alert kept for the UI activity feed.
type RecentEntry struct {
	At       time.Time `json:"at"`
	State    string    `json:"state"` // firing | resolved
	Severity string    `json:"severity"`
	Monitor  string    `json:"monitor"`
	NS       string    `json:"namespace,omitempty"`
	Object   string    `json:"object,omitempty"`
	Reason   string    `json:"reason,omitempty"`
	Title    string    `json:"title"`
	Body     string    `json:"body,omitempty"`
	Channels []string  `json:"channels,omitempty"`
}

// statsRecorder is shared mutable state for the UI-introspection surface.
// Kept off the hot path: every fire/resolve grabs the mutex once.
type statsRecorder struct {
	mu       sync.Mutex
	monitors map[string]*MonitorStats
	recent   []RecentEntry
	capacity int
}

func newStatsRecorder(capacity int) *statsRecorder {
	if capacity <= 0 {
		capacity = 100
	}
	return &statsRecorder{
		monitors: map[string]*MonitorStats{},
		capacity: capacity,
	}
}

func (s *statsRecorder) record(a alert.Alert, channels []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stat, ok := s.monitors[a.Monitor]
	if !ok {
		stat = &MonitorStats{Name: a.Monitor}
		s.monitors[a.Monitor] = stat
	}
	if a.State == alert.StateResolved {
		stat.Resolves++
		stat.LastEvent = "resolved"
	} else {
		stat.Fires++
		stat.LastEvent = "firing"
	}
	if !a.FiredAt.IsZero() {
		stat.LastFired = a.FiredAt
	}

	entry := RecentEntry{
		At:       a.FiredAt,
		State:    a.State.String(),
		Severity: a.Severity.String(),
		Monitor:  a.Monitor,
		NS:       a.Namespace,
		Object:   a.Object(),
		Reason:   a.Reason,
		Title:    a.Title,
		Body:     a.Body,
		Channels: append([]string(nil), channels...),
	}
	if entry.At.IsZero() {
		entry.At = time.Now().UTC()
	}
	s.recent = append(s.recent, entry)
	if len(s.recent) > s.capacity {
		// keep tail
		s.recent = s.recent[len(s.recent)-s.capacity:]
	}
}

// Monitors returns a snapshot, sorted alphabetically by monitor name.
func (s *statsRecorder) Monitors() []MonitorStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]MonitorStats, 0, len(s.monitors))
	for _, st := range s.monitors {
		out = append(out, *st)
	}
	// stable order: oldest-known monitor first by name
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[i].Name > out[j].Name {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// Recent returns a snapshot of the most recent N entries, newest first.
func (s *statsRecorder) Recent() []RecentEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]RecentEntry, len(s.recent))
	for i, e := range s.recent {
		out[len(s.recent)-1-i] = e
	}
	return out
}
