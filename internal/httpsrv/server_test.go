package httpsrv

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dnl555/kpulse/internal/alert"
	"github.com/dnl555/kpulse/internal/notifiers"
)

type sentinel struct{ called bool }

func (s *sentinel) Name() string                                 { return "stub" }
func (s *sentinel) Send(_ context.Context, _ alert.Alert) error { s.called = true; return nil }

func TestHealthz(t *testing.T) {
	reg := notifiers.NewRegistry()
	srv := New(reg, func() bool { return true })
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("got %d", rec.Code)
	}
}

func TestReadyzNotReady(t *testing.T) {
	reg := notifiers.NewRegistry()
	srv := New(reg, func() bool { return false })
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("got %d", rec.Code)
	}
}

func TestTestChannel(t *testing.T) {
	reg := notifiers.NewRegistry()
	st := &sentinel{}
	reg.Register(st)
	srv := New(reg, func() bool { return true })
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/test-channel?name=stub", nil))
	if rec.Code != http.StatusOK || !st.called {
		t.Errorf("code=%d called=%v", rec.Code, st.called)
	}
}

func TestTestChannelUnknown(t *testing.T) {
	reg := notifiers.NewRegistry()
	srv := New(reg, func() bool { return true })
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/test-channel?name=nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("got %d", rec.Code)
	}
}
