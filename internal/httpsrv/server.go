// Package httpsrv exposes health, readiness, metrics, ad-hoc channel test
// endpoints, the read-only JSON API at /api/v1/*, and the embedded UI at /.
package httpsrv

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/dnl555/kpulse/internal/alert"
	"github.com/dnl555/kpulse/internal/engine"
	"github.com/dnl555/kpulse/internal/notifiers"
)

// Resetter clears in-memory state. Returns counts of (active, dedupe) cleared.
type Resetter interface {
	Reset() (int, int)
}

// PersistedStore clears the persisted-state ConfigMap so the next snapshot
// does not re-rehydrate. May be nil; the endpoint will skip persistence then.
type PersistedStore interface {
	Clear(ctx context.Context) error
}

// Introspector exposes the engine state the UI consumes. Concrete implementor
// is *engine.Engine; the interface is here to keep the http package import
// graph one-way (httpsrv -> engine, not the other way).
type Introspector interface {
	ActiveSnapshot() map[string]alert.Alert
	MonitorStats() []engine.MonitorStats
	Recent() []engine.RecentEntry
}

// ClusterInfo is what the UI shows in the header / About panel. The httpsrv
// caller assembles this from the runtime config so we do not import the
// config package here.
type ClusterInfo struct {
	Name              string    `json:"name"`
	Version           string    `json:"version"`
	StartedAt         time.Time `json:"started_at"`
	NamespacesInclude []string  `json:"namespaces_include"`
	NamespacesExclude []string  `json:"namespaces_exclude"`
	DedupeWindow      string    `json:"dedupe_window"`
	DigestEnabled     bool      `json:"digest_enabled"`
	DigestInterval    string    `json:"digest_interval"`
	ResolutionEnabled bool      `json:"resolution_enabled"`
}

// MonitorView is the per-monitor summary the UI lists on the Monitors page.
type MonitorView struct {
	Name      string         `json:"name"`
	Enabled   bool           `json:"enabled"`
	Knobs     map[string]any `json:"knobs,omitempty"`
	Fires     int            `json:"fires"`
	Resolves  int            `json:"resolves"`
	LastFired time.Time      `json:"last_fired,omitzero"`
	LastEvent string         `json:"last_event,omitempty"`
}

// MonitorsProvider returns the static monitor list (one entry per built-in
// monitor, with its current Enabled + per-monitor knobs). The httpsrv
// package does not import config; the caller assembles this slice.
type MonitorsProvider func() []MonitorView

type Server struct {
	reg      *notifiers.Registry
	ready    func() bool
	engine   Resetter
	persist  PersistedStore
	intro    Introspector
	cluster  func() ClusterInfo
	monitors MonitorsProvider
	uiFS     fs.FS
}

func New(reg *notifiers.Registry, ready func() bool) *Server {
	return &Server{reg: reg, ready: ready}
}

// WithReset wires the reset endpoint.
func (s *Server) WithReset(engine Resetter, persist PersistedStore) *Server {
	s.engine = engine
	s.persist = persist
	return s
}

// WithUI wires the JSON API + the embedded SPA at /. All four arguments must
// be non-nil to enable the UI; if any is nil the / and /api/* routes return
// 404 and the rest of the server keeps working.
func (s *Server) WithUI(intro Introspector, cluster func() ClusterInfo, monitors MonitorsProvider, uiFS fs.FS) *Server {
	s.intro = intro
	s.cluster = cluster
	s.monitors = monitors
	s.uiFS = uiFS
	return s
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if !s.ready() {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, "# kpulse self metrics placeholder\nkpulse_up 1\n")
	})
	mux.HandleFunc("/test-channel", s.handleTestChannel)
	mux.HandleFunc("/reset-dedupe", s.handleReset)

	// JSON API + embedded UI.
	mux.HandleFunc("/api/v1/cluster", s.handleAPICluster)
	mux.HandleFunc("/api/v1/alerts/active", s.handleAPIActive)
	mux.HandleFunc("/api/v1/alerts/recent", s.handleAPIRecent)
	mux.HandleFunc("/api/v1/monitors", s.handleAPIMonitors)
	mux.HandleFunc("/api/v1/channels", s.handleAPIChannels)
	mux.HandleFunc("/", s.handleUI)

	return mux
}

func (s *Server) handleTestChannel(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	n, ok := s.reg.Get(name)
	if !ok {
		http.Error(w, "unknown channel", http.StatusNotFound)
		return
	}
	err := n.Send(r.Context(), alert.Alert{
		Monitor: "test", Severity: alert.Info, Cluster: "test", Namespace: "kpulse",
		Title: "kpulse test alert", Body: "If you can read this, the channel works.", FiredAt: time.Now().UTC(),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = w.Write([]byte("sent"))
}

func (s *Server) handleReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	if s.engine == nil {
		http.Error(w, "reset endpoint not wired", http.StatusServiceUnavailable)
		return
	}
	activeN, dedupeN := s.engine.Reset()
	persisted := false
	if s.persist != nil {
		if err := s.persist.Clear(r.Context()); err != nil {
			http.Error(w, fmt.Sprintf("cleared in-memory but failed to clear persisted state: %v", err), http.StatusInternalServerError)
			return
		}
		persisted = true
	}
	writeJSON(w, map[string]any{
		"active_cleared":  activeN,
		"dedupe_cleared":  dedupeN,
		"state_persisted": persisted,
	})
}

func (s *Server) handleAPICluster(w http.ResponseWriter, _ *http.Request) {
	if s.cluster == nil {
		http.NotFound(w, nil)
		return
	}
	writeJSON(w, s.cluster())
}

type apiActiveAlert struct {
	Key       string    `json:"key"`
	Monitor   string    `json:"monitor"`
	Severity  string    `json:"severity"`
	Cluster   string    `json:"cluster,omitempty"`
	Namespace string    `json:"namespace,omitempty"`
	Object    string    `json:"object"`
	Reason    string    `json:"reason,omitempty"`
	Title     string    `json:"title"`
	Body      string    `json:"body,omitempty"`
	FiredAt   time.Time `json:"fired_at"`
}

func (s *Server) handleAPIActive(w http.ResponseWriter, _ *http.Request) {
	if s.intro == nil {
		http.NotFound(w, nil)
		return
	}
	snap := s.intro.ActiveSnapshot()
	out := make([]apiActiveAlert, 0, len(snap))
	for key, a := range snap {
		out = append(out, apiActiveAlert{
			Key: key, Monitor: a.Monitor, Severity: a.Severity.String(),
			Cluster: a.Cluster, Namespace: a.Namespace, Object: a.Object(),
			Reason: a.Reason, Title: a.Title, Body: a.Body, FiredAt: a.FiredAt,
		})
	}
	// newest first by FiredAt
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[i].FiredAt.Before(out[j].FiredAt) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	writeJSON(w, map[string]any{"count": len(out), "alerts": out})
}

func (s *Server) handleAPIRecent(w http.ResponseWriter, _ *http.Request) {
	if s.intro == nil {
		http.NotFound(w, nil)
		return
	}
	r := s.intro.Recent()
	writeJSON(w, map[string]any{"count": len(r), "alerts": r})
}

func (s *Server) handleAPIMonitors(w http.ResponseWriter, _ *http.Request) {
	if s.monitors == nil {
		http.NotFound(w, nil)
		return
	}
	views := s.monitors()
	// merge stats from engine
	if s.intro != nil {
		stats := s.intro.MonitorStats()
		byName := make(map[string]engine.MonitorStats, len(stats))
		for _, st := range stats {
			byName[st.Name] = st
		}
		for i := range views {
			if st, ok := byName[views[i].Name]; ok {
				views[i].Fires = st.Fires
				views[i].Resolves = st.Resolves
				views[i].LastFired = st.LastFired
				views[i].LastEvent = st.LastEvent
			}
		}
	}
	writeJSON(w, map[string]any{"count": len(views), "monitors": views})
}

type apiChannel struct {
	Name string `json:"name"`
}

func (s *Server) handleAPIChannels(w http.ResponseWriter, _ *http.Request) {
	names := s.reg.Names()
	out := make([]apiChannel, 0, len(names))
	for _, n := range names {
		out = append(out, apiChannel{Name: n})
	}
	writeJSON(w, map[string]any{"count": len(out), "channels": out})
}

// handleUI serves the embedded SPA. Any non-API, non-asset request that
// looks like a route falls back to index.html so client-side routing works.
func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	if s.uiFS == nil {
		http.NotFound(w, r)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	f, err := s.uiFS.Open(path)
	if err == nil {
		defer f.Close()
		http.ServeContent(w, r, path, time.Time{}, f.(readSeeker))
		return
	}
	// fall back to index.html for client-side routes
	f, err = s.uiFS.Open("index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeContent(w, r, "index.html", time.Time{}, f.(readSeeker))
}

type readSeeker interface {
	Read(p []byte) (n int, err error)
	Seek(offset int64, whence int) (int64, error)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	srv := &http.Server{Addr: addr, Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second}
	go func() { <-ctx.Done(); _ = srv.Shutdown(context.Background()) }()
	return srv.ListenAndServe()
}
