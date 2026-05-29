package notifiers

import (
	"net/http"

	"github.com/dnl555/kpulse/internal/config"
)

func Build(cfg *config.Config, sec config.SecretMap) (*Registry, error) {
	r := NewRegistry()
	httpClient := &http.Client{}
	if ref := cfg.Channels.Slack.WebhookURLFromSecret; ref != "" {
		url, err := sec.Get(ref)
		if err != nil {
			return nil, err
		}
		r.Register(NewSlack(url, httpClient))
	}
	if cfg.Channels.Email.SMTPHost != "" && len(cfg.Channels.Email.To) > 0 {
		u, _ := sec.Get(cfg.Channels.Email.UserFromSecret)
		p, _ := sec.Get(cfg.Channels.Email.PassFromSecret)
		email := NewEmail(cfg.Channels.Email.SMTPHost, cfg.Channels.Email.SMTPPort, u, p, cfg.Channels.Email.From, cfg.Channels.Email.To)
		if cfg.Channels.Email.ReplyTo != "" {
			email = email.WithReplyTo(cfg.Channels.Email.ReplyTo)
		}
		r.Register(email)
	}
	if cfg.Channels.Webhook.URL != "" {
		h, err := sec.ExpandMap(cfg.Channels.Webhook.Headers)
		if err != nil {
			return nil, err
		}
		r.Register(NewWebhook(cfg.Channels.Webhook.URL, h, httpClient))
	}
	if ref := cfg.Channels.Discord.WebhookURLFromSecret; ref != "" {
		url, err := sec.Get(ref)
		if err != nil {
			return nil, err
		}
		r.Register(NewDiscord(url, httpClient))
	}
	if ref := cfg.Channels.Teams.WebhookURLFromSecret; ref != "" {
		url, err := sec.Get(ref)
		if err != nil {
			return nil, err
		}
		r.Register(NewTeams(url, httpClient))
	}
	return r, nil
}
