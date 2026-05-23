package notifiers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dnl555/kpulse/internal/alert"
)

func TestSlackSend(t *testing.T) {
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	n := NewSlack(srv.URL, &http.Client{})
	err := n.Send(context.Background(), alert.Alert{
		Severity: alert.Critical, Cluster: "c1", Namespace: "ns", ObjectKind: "Pod",
		ObjectName: "p", Reason: "OOMKilled", Title: "Pod OOMKilled", Body: "msg",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := received["text"]; !ok {
		t.Errorf("missing text field: %+v", received)
	}
}

func TestSlackHTTPErrorIsReturned(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()
	n := NewSlack(srv.URL, &http.Client{})
	if err := n.Send(context.Background(), alert.Alert{}); err == nil {
		t.Fatal("expected error on 500")
	}
}
