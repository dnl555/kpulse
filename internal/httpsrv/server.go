// Package httpsrv exposes health, readiness, metrics, and ad-hoc channel test endpoints.
package httpsrv

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/dnl555/kpulse/internal/alert"
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

type Server struct {
	reg     *notifiers.Registry
	ready   func() bool
	engine  Resetter
	persist PersistedStore
}

func New(reg *notifiers.Registry, ready func() bool) *Server {
	return &Server{reg: reg, ready: ready}
}

// WithReset wires the reset endpoint. Both arguments may be nil if you only
// want partial behavior; nil engine disables the endpoint, nil persist means
// the next snapshot will re-write whatever was just cleared.
func (s *Server) WithReset(engine Resetter, persist PersistedStore) *Server {
	s.engine = engine
	s.persist = persist
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
	mux.HandleFunc("/test-channel", func(w http.ResponseWriter, r *http.Request) {
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
	})
	mux.HandleFunc("/reset-dedupe", s.handleReset)
	return mux
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
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"active_cleared":  activeN,
		"dedupe_cleared":  dedupeN,
		"state_persisted": persisted,
	})
}

func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	srv := &http.Server{Addr: addr, Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second}
	go func() { <-ctx.Done(); _ = srv.Shutdown(context.Background()) }()
	return srv.ListenAndServe()
}
