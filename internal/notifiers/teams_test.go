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

func TestTeamsSend(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.WriteHeader(200)
	}))
	defer srv.Close()
	n := NewTeams(srv.URL, nil)
	if err := n.Send(context.Background(), alert.Alert{Severity: alert.Critical, Title: "T", Namespace: "ns", ObjectKind: "Pod", ObjectName: "p", Reason: "OOMKilled"}); err != nil {
		t.Fatal(err)
	}
	if got["@type"] != "MessageCard" {
		t.Errorf("@type = %v", got["@type"])
	}
	if got["themeColor"] != "D13438" {
		t.Errorf("themeColor = %v", got["themeColor"])
	}
}
