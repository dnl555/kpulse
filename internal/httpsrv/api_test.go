package httpsrv

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dnl555/kpulse/internal/alert"
	"github.com/dnl555/kpulse/internal/engine"
	"github.com/dnl555/kpulse/internal/notifiers"
)

type fakeIntro struct {
	active  map[string]alert.Alert
	stats   []engine.MonitorStats
	recents []engine.RecentEntry
}

func (f *fakeIntro) ActiveSnapshot() map[string]alert.Alert { return f.active }
func (f *fakeIntro) MonitorStats() []engine.MonitorStats    { return f.stats }
func (f *fakeIntro) Recent() []engine.RecentEntry           { return f.recents }

func newUIServer(intro *fakeIntro) *Server {
	reg := notifiers.NewRegistry()
	srv := New(reg, func() bool { return true })
	srv = srv.WithUI(
		intro,
		func() ClusterInfo {
			return ClusterInfo{Name: "test-cluster", Version: "v0.3.0", StartedAt: time.Unix(1700000000, 0).UTC(), DedupeWindow: "30m"}
		},
		func() []MonitorView {
			return []MonitorView{
				{Name: "pod_crashes", Enabled: true, Knobs: map[string]any{"threshold": 5}},
				{Name: "warning_events", Enabled: false},
			}
		},
		UIFS(),
	)
	return srv
}

func TestAPICluster(t *testing.T) {
	srv := newUIServer(&fakeIntro{})
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/cluster", nil))
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	var got ClusterInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "test-cluster" || got.Version != "v0.3.0" {
		t.Errorf("got %+v", got)
	}
}

func TestAPIActiveAlertsNewestFirst(t *testing.T) {
	intro := &fakeIntro{active: map[string]alert.Alert{
		"k-old": {Monitor: "a", Title: "older", FiredAt: time.Unix(1700000000, 0).UTC(), Severity: alert.Warning},
		"k-new": {Monitor: "b", Title: "newer", FiredAt: time.Unix(1700000100, 0).UTC(), Severity: alert.Critical},
	}}
	srv := newUIServer(intro)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/alerts/active", nil))
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	var resp struct {
		Count  int              `json:"count"`
		Alerts []apiActiveAlert `json:"alerts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 2 {
		t.Fatalf("count = %d", resp.Count)
	}
	if resp.Alerts[0].Title != "newer" {
		t.Errorf("expected newer first; got %s", resp.Alerts[0].Title)
	}
}

func TestAPIMonitorsMergesEngineStats(t *testing.T) {
	intro := &fakeIntro{stats: []engine.MonitorStats{{Name: "pod_crashes", Fires: 4, Resolves: 2, LastEvent: "firing"}}}
	srv := newUIServer(intro)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/monitors", nil))
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	var resp struct {
		Count    int           `json:"count"`
		Monitors []MonitorView `json:"monitors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	var pc *MonitorView
	for i := range resp.Monitors {
		if resp.Monitors[i].Name == "pod_crashes" {
			pc = &resp.Monitors[i]
		}
	}
	if pc == nil || pc.Fires != 4 || pc.Resolves != 2 {
		t.Errorf("pod_crashes stats not merged: %+v", pc)
	}
}

func TestAPIChannels(t *testing.T) {
	srv := newUIServer(&fakeIntro{})
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/channels", nil))
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestUIServesIndexAtRoot(t *testing.T) {
	srv := newUIServer(&fakeIntro{})
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 200 {
		t.Fatalf("status %d body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct == "" {
		t.Errorf("missing content-type")
	}
	body := rec.Body.String()
	if len(body) == 0 || !contains(body, "kpulse") {
		t.Errorf("index.html doesn't look right: %q", body[:min(200, len(body))])
	}
}

func TestUIServesAssetAtPath(t *testing.T) {
	srv := newUIServer(&fakeIntro{})
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/app.js", nil))
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	if !contains(rec.Body.String(), "fetchJSON") {
		t.Errorf("app.js doesn't look like our app: first 200 chars: %q", rec.Body.String()[:min(200, rec.Body.Len())])
	}
}

func TestUIDisabledWhenNotWired(t *testing.T) {
	reg := notifiers.NewRegistry()
	srv := New(reg, func() bool { return true })
	for _, path := range []string{"/", "/api/v1/cluster", "/api/v1/alerts/active", "/api/v1/monitors", "/api/v1/alerts/recent"} {
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s without WithUI should be 404; got %d", path, rec.Code)
		}
	}
}

func TestAPIChannelsAlwaysAvailable(t *testing.T) {
	// channels endpoint only needs the registry, not the introspector.
	reg := notifiers.NewRegistry()
	srv := New(reg, func() bool { return true })
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/channels", nil))
	if rec.Code != 200 {
		t.Errorf("/api/v1/channels should always work; got %d", rec.Code)
	}
}

// helpers
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// keep imports happy
var _ = context.Background
