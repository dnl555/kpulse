package notifiers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dnl555/kpulse/internal/alert"
)

func TestWebhookSendIncludesHeadersAndBody(t *testing.T) {
	var gotAuth string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.WriteHeader(204)
	}))
	defer srv.Close()
	n := NewWebhook(srv.URL, map[string]string{"Authorization": "Bearer xyz"}, nil)
	a := alert.Alert{Monitor: "pod_crashes", Severity: alert.Critical, Cluster: "c", Namespace: "ns",
		ObjectKind: "Pod", ObjectName: "p", Reason: "OOMKilled", Title: "T", Body: "B", FiredAt: time.Now().UTC()}
	if err := n.Send(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer xyz" {
		t.Errorf("auth header = %q", gotAuth)
	}
	if gotBody["monitor"] != "pod_crashes" || gotBody["severity"] != "critical" {
		t.Errorf("body = %+v", gotBody)
	}
}

func TestWebhookErrorOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	n := NewWebhook(srv.URL, nil, nil)
	if err := n.Send(context.Background(), alert.Alert{}); err == nil {
		t.Error("expected error on 500")
	}
}
