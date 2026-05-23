package alert

import (
	"testing"
	"time"
)

func TestSeverityString(t *testing.T) {
	cases := map[Severity]string{
		Info: "info", Warning: "warning", Critical: "critical",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("Severity(%d).String() = %q, want %q", s, got, want)
		}
	}
}

func TestParseSeverity(t *testing.T) {
	cases := map[string]Severity{
		"info": Info, "Warning": Warning, "CRITICAL": Critical,
	}
	for in, want := range cases {
		got, err := ParseSeverity(in)
		if err != nil || got != want {
			t.Errorf("ParseSeverity(%q) = (%v, %v), want (%v, nil)", in, got, err, want)
		}
	}
	if _, err := ParseSeverity("nope"); err == nil {
		t.Error("expected error for unknown severity")
	}
}

func TestAlertKey(t *testing.T) {
	a := Alert{Monitor: "pod_crashes", Namespace: "default", ObjectKind: "Pod", ObjectName: "foo", Reason: "OOMKilled"}
	if a.Key() == "" {
		t.Fatal("Key() returned empty")
	}
	b := a
	if a.Key() != b.Key() {
		t.Error("Key() not stable across identical alerts")
	}
	b.Namespace = "other"
	if a.Key() == b.Key() {
		t.Error("Key() should differ when namespace differs")
	}
}

func TestAlertFiredAtDefaults(t *testing.T) {
	a := Alert{}
	a.EnsureFiredAt()
	if a.FiredAt.IsZero() || time.Since(a.FiredAt) > time.Second {
		t.Errorf("EnsureFiredAt did not set time: %v", a.FiredAt)
	}
}
