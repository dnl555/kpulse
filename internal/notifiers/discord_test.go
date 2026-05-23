package notifiers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dnl555/kpulse/internal/alert"
)

func TestDiscordSend(t *testing.T) {
	var got map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.WriteHeader(204)
	}))
	defer srv.Close()
	n := NewDiscord(srv.URL, nil)
	if err := n.Send(context.Background(), alert.Alert{Severity: alert.Warning, Title: "T", Body: "B"}); err != nil {
		t.Fatal(err)
	}
	if got["content"] == "" {
		t.Errorf("missing content: %+v", got)
	}
}

func TestDiscordTruncatesLongBody(t *testing.T) {
	var got map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.WriteHeader(204)
	}))
	defer srv.Close()
	n := NewDiscord(srv.URL, nil)
	huge := strings.Repeat("x", 3000)
	if err := n.Send(context.Background(), alert.Alert{Body: huge}); err != nil {
		t.Fatal(err)
	}
	if len(got["content"]) > discordLimit+20 {
		t.Errorf("content too long: %d", len(got["content"]))
	}
}
