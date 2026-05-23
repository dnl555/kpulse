package notifiers

import (
	"context"
	"strings"
	"testing"

	"github.com/dnl555/kpulse/internal/alert"
)

type fakeSMTP struct {
	from string
	to   []string
	msg  []byte
}

func (f *fakeSMTP) SendMail(_ string, _ smtpAuth, from string, to []string, msg []byte) error {
	f.from, f.to, f.msg = from, to, msg
	return nil
}

func TestEmailSendComposesMessage(t *testing.T) {
	f := &fakeSMTP{}
	e := &Email{from: "alerts@example.com", to: []string{"me@example.com"}, sender: f}
	a := alert.Alert{Severity: alert.Critical, Cluster: "c", Namespace: "ns", ObjectKind: "Pod", ObjectName: "p", Title: "T", Body: "B", Reason: "OOMKilled"}
	if err := e.Send(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	if f.from != "alerts@example.com" || len(f.to) != 1 {
		t.Errorf("from/to wrong: %s %v", f.from, f.to)
	}
	body := string(f.msg)
	for _, want := range []string{"Subject: ", "[critical]", "c", "OOMKilled", "T"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q\nbody:\n%s", want, body)
		}
	}
}
