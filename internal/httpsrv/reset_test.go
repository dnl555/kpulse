package httpsrv

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dnl555/kpulse/internal/notifiers"
)

type fakeResetter struct{ active, dedupe int }

func (f *fakeResetter) Reset() (int, int) { return f.active, f.dedupe }

type fakePersist struct {
	cleared bool
	err     error
}

func (f *fakePersist) Clear(_ context.Context) error {
	f.cleared = true
	return f.err
}

func TestResetDedupePOST(t *testing.T) {
	reg := notifiers.NewRegistry()
	r := &fakeResetter{active: 3, dedupe: 7}
	p := &fakePersist{}
	srv := New(reg, func() bool { return true }).WithReset(r, p)

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest("POST", "/reset-dedupe", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["active_cleared"].(float64) != 3 || got["dedupe_cleared"].(float64) != 7 {
		t.Errorf("got %+v", got)
	}
	if got["state_persisted"] != true {
		t.Error("state_persisted should be true")
	}
	if !p.cleared {
		t.Error("persist Clear was not called")
	}
}

func TestResetRejectsGET(t *testing.T) {
	srv := New(notifiers.NewRegistry(), func() bool { return true }).
		WithReset(&fakeResetter{}, nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/reset-dedupe", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("got %d", rec.Code)
	}
}

func TestResetWhenNotWired(t *testing.T) {
	srv := New(notifiers.NewRegistry(), func() bool { return true })
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest("POST", "/reset-dedupe", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("got %d", rec.Code)
	}
}
