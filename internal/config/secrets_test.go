package config

import "testing"

func TestResolveSecretRef(t *testing.T) {
	s := SecretMap{"SLACK_WEBHOOK_URL": "https://hooks.slack.com/x", "SMTP_PASS": "p"}
	if got, _ := s.Get("SLACK_WEBHOOK_URL"); got != "https://hooks.slack.com/x" {
		t.Errorf("got %q", got)
	}
	if _, err := s.Get("MISSING"); err == nil {
		t.Error("expected error for missing key")
	}
}

func TestExpandHeaders(t *testing.T) {
	s := SecretMap{"TOKEN": "abc"}
	in := map[string]string{"Authorization": "Bearer $TOKEN", "X-Plain": "literal"}
	out, err := s.ExpandMap(in)
	if err != nil {
		t.Fatal(err)
	}
	if out["Authorization"] != "Bearer abc" || out["X-Plain"] != "literal" {
		t.Errorf("got %+v", out)
	}
	if _, err := s.ExpandMap(map[string]string{"X": "$NOPE"}); err == nil {
		t.Error("expected error for missing token")
	}
}
