package engine

import (
	"testing"

	"github.com/dnl555/kpulse/internal/alert"
	"github.com/dnl555/kpulse/internal/config"
)

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestRouterCriticalMatch(t *testing.T) {
	rules := []config.RoutingRule{
		{Match: config.RoutingMatch{Severity: "critical"}, Channels: []string{"slack", "email"}},
		{Match: config.RoutingMatch{Monitor: "tls_cert_expiry"}, Channels: []string{"email"}},
	}
	r := NewRouter(rules, []string{"slack"})
	got := r.Channels(alert.Alert{Severity: alert.Critical, Monitor: "pod_crashes"})
	if want := []string{"slack", "email"}; !equal(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
	got = r.Channels(alert.Alert{Severity: alert.Info, Monitor: "tls_cert_expiry"})
	if want := []string{"email"}; !equal(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
	got = r.Channels(alert.Alert{Severity: alert.Info, Monitor: "warning_events"})
	if want := []string{"slack"}; !equal(got, want) {
		t.Errorf("fallback to defaults; got %v want %v", got, want)
	}
}
