package notifiers

import (
	"testing"

	"github.com/dnl555/kpulse/internal/config"
)

func TestBuildRegistry(t *testing.T) {
	cfg := &config.Config{
		Channels: config.Channels{
			Slack: config.SlackChannel{WebhookURLFromSecret: "SLACK_URL"},
			Email: config.EmailChannel{SMTPHost: "h", SMTPPort: 587, From: "a", To: []string{"b"}, UserFromSecret: "U", PassFromSecret: "P"},
		},
	}
	sec := config.SecretMap{"SLACK_URL": "https://hooks/x", "U": "u", "P": "p"}
	r, err := Build(cfg, sec)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Get("slack"); !ok {
		t.Error("slack not registered")
	}
	if _, ok := r.Get("email"); !ok {
		t.Error("email not registered")
	}
}

func TestBuildSkipsUnconfiguredChannels(t *testing.T) {
	r, err := Build(&config.Config{}, config.SecretMap{})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(r.Names()); got != 0 {
		t.Errorf("expected empty registry, got %d", got)
	}
}
