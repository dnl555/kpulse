// Package httpsrv exposes health, readiness, metrics, and ad-hoc channel test endpoints.
package httpsrv

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/dnl555/kpulse/internal/alert"
	"github.com/dnl555/kpulse/internal/notifiers"
)

type Server struct {
	reg   *notifiers.Registry
	ready func() bool
}

func New(reg *notifiers.Registry, ready func() bool) *Server {
	return &Server{reg: reg, ready: ready}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if !s.ready() {
			http.Error(w, "not ready", 503)
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
			http.Error(w, "unknown channel", 404)
			return
		}
		err := n.Send(r.Context(), alert.Alert{
			Monitor: "test", Severity: alert.Info, Cluster: "test", Namespace: "kpulse",
			Title: "kpulse test alert", Body: "If you can read this, the channel works.", FiredAt: time.Now().UTC(),
		})
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		_, _ = w.Write([]byte("sent"))
	})
	return mux
}

func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	srv := &http.Server{Addr: addr, Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second}
	go func() { <-ctx.Done(); _ = srv.Shutdown(context.Background()) }()
	return srv.ListenAndServe()
}
