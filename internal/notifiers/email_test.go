package notifiers

import (
	"bytes"
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/textproto"
	"strings"
	"testing"
	"time"

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

func newTestEmail(f *fakeSMTP) *Email {
	return &Email{
		from: "alerts@example.com", to: []string{"me@example.com"},
		sender: f,
		now:    func() time.Time { return time.Date(2026, 5, 24, 9, 0, 0, 0, time.UTC) },
	}
}

func sampleAlert() alert.Alert {
	return alert.Alert{
		Severity: alert.Critical, Cluster: "prod-eks-1", Namespace: "checkout",
		ObjectKind: "Pod", ObjectName: "api-7d9f", Reason: "OOMKilled",
		Monitor: "pod_crashes", Title: "OOMKilled on api-7d9f/server",
		Body:    "Container server in pod checkout/api-7d9f is in state OOMKilled",
		FiredAt: time.Date(2026, 5, 24, 8, 59, 0, 0, time.UTC),
	}
}

func headers(msg []byte) map[string]string {
	idx := bytes.Index(msg, []byte("\r\n\r\n"))
	hdrs := map[string]string{}
	for _, line := range strings.Split(string(msg[:idx]), "\r\n") {
		if i := strings.Index(line, ":"); i > 0 {
			hdrs[strings.ToLower(strings.TrimSpace(line[:i]))] = strings.TrimSpace(line[i+1:])
		}
	}
	return hdrs
}

type readPart struct {
	Header textproto.MIMEHeader
	Body   []byte
}

func parseParts(t *testing.T, msg []byte) []readPart {
	t.Helper()
	idx := bytes.Index(msg, []byte("\r\n\r\n"))
	hdrs := headers(msg)
	mt, params, err := mime.ParseMediaType(hdrs["content-type"])
	if err != nil {
		t.Fatalf("parse content-type: %v", err)
	}
	if !strings.HasPrefix(mt, "multipart/") {
		t.Fatalf("not multipart: %s", mt)
	}
	mr := multipart.NewReader(bytes.NewReader(msg[idx+4:]), params["boundary"])
	var parts []readPart
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("next part: %v", err)
		}
		body, err := io.ReadAll(p)
		if err != nil {
			t.Fatalf("read part: %v", err)
		}
		parts = append(parts, readPart{Header: p.Header, Body: body})
	}
	return parts
}

func TestEmailHeaders(t *testing.T) {
	f := &fakeSMTP{}
	e := newTestEmail(f)
	if err := e.Send(context.Background(), sampleAlert()); err != nil {
		t.Fatal(err)
	}
	h := headers(f.msg)
	wants := map[string]string{
		"from":              "alerts@example.com",
		"to":                "me@example.com",
		"x-kpulse-cluster":  "prod-eks-1",
		"x-kpulse-monitor":  "pod_crashes",
		"x-kpulse-severity": "critical",
		"auto-submitted":    "auto-generated",
		"mime-version":      "1.0",
	}
	for k, want := range wants {
		if got := h[k]; got != want {
			t.Errorf("%s = %q want %q", k, got, want)
		}
	}
	for _, must := range []string{"date", "message-id", "list-id", "subject", "content-type"} {
		if h[must] == "" {
			t.Errorf("missing header %s", must)
		}
	}
	if !strings.HasPrefix(h["content-type"], "multipart/alternative") {
		t.Errorf("expected multipart/alternative outer; got %q", h["content-type"])
	}
	if !strings.Contains(h["subject"], "[prod-eks-1]") || !strings.Contains(h["subject"], "OOMKilled") {
		t.Errorf("subject malformed: %q", h["subject"])
	}
}

func TestEmailMultipartAlternativeBodies(t *testing.T) {
	f := &fakeSMTP{}
	e := newTestEmail(f)
	if err := e.Send(context.Background(), sampleAlert()); err != nil {
		t.Fatal(err)
	}
	parts := parseParts(t, f.msg)
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts (plain+html); got %d", len(parts))
	}
	gotTypes := []string{parts[0].Header.Get("Content-Type"), parts[1].Header.Get("Content-Type")}
	if !strings.HasPrefix(gotTypes[0], "text/plain") || !strings.HasPrefix(gotTypes[1], "text/html") {
		t.Errorf("part types wrong: %v", gotTypes)
	}
	plain := string(parts[0].Body)
	htmlBody := string(parts[1].Body)
	for _, want := range []string{"OOMKilled", "prod-eks-1", "checkout", "kpulse"} {
		if !strings.Contains(plain, want) {
			t.Errorf("plain body missing %q", want)
		}
		if !strings.Contains(htmlBody, want) {
			t.Errorf("html body missing %q", want)
		}
	}
	if !strings.Contains(htmlBody, "#dc2626") {
		t.Errorf("html body missing critical color")
	}
	if !strings.Contains(htmlBody, "<!doctype html>") {
		t.Errorf("html body missing doctype")
	}
}

func TestEmailWithAttachments(t *testing.T) {
	f := &fakeSMTP{}
	e := newTestEmail(f)
	a := sampleAlert()
	a.Attachments = []alert.Attachment{{Name: "pod.log", ContentType: "text/plain", Body: []byte("line1\nline2\n")}}
	if err := e.Send(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	h := headers(f.msg)
	if !strings.HasPrefix(h["content-type"], "multipart/mixed") {
		t.Errorf("expected multipart/mixed when attachments present; got %q", h["content-type"])
	}
	parts := parseParts(t, f.msg)
	if len(parts) != 2 {
		t.Fatalf("expected 2 top-level parts (alt+attachment); got %d", len(parts))
	}
	if !strings.Contains(parts[1].Header.Get("Content-Disposition"), "pod.log") {
		t.Errorf("attachment filename missing: %q", parts[1].Header.Get("Content-Disposition"))
	}
}

func TestEmailFromWithDisplayNameAndReplyTo(t *testing.T) {
	f := &fakeSMTP{}
	e := newTestEmail(f)
	e.from = `"Kpulse Alerts" <alerts@updates.example.com>`
	e.replyTo = "team@example.com"
	if err := e.Send(context.Background(), sampleAlert()); err != nil {
		t.Fatal(err)
	}
	// SMTP envelope (MAIL FROM) must be the bare address, not the display name.
	if f.from != "alerts@updates.example.com" {
		t.Errorf("envelope from = %q, want bare address", f.from)
	}
	h := headers(f.msg)
	if h["from"] != `"Kpulse Alerts" <alerts@updates.example.com>` {
		t.Errorf("From header = %q", h["from"])
	}
	if h["reply-to"] != "team@example.com" {
		t.Errorf("Reply-To header = %q", h["reply-to"])
	}
}

func TestEmailReplyToOmittedByDefault(t *testing.T) {
	f := &fakeSMTP{}
	e := newTestEmail(f) // no WithReplyTo
	if err := e.Send(context.Background(), sampleAlert()); err != nil {
		t.Fatal(err)
	}
	if _, ok := headers(f.msg)["reply-to"]; ok {
		t.Error("Reply-To should not be emitted when not configured")
	}
}

func TestSubjectTruncation(t *testing.T) {
	got := truncateSubject(strings.Repeat("x", 200))
	if len(got) != subjectMaxLen {
		t.Errorf("got len %d want %d", len(got), subjectMaxLen)
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("expected ... suffix: %q", got)
	}
}
