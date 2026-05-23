package notifiers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/dnl555/kpulse/internal/alert"
)

type Webhook struct {
	url     string
	headers map[string]string
	client  *http.Client
}

func NewWebhook(url string, headers map[string]string, client *http.Client) *Webhook {
	if client == nil {
		client = http.DefaultClient
	}
	return &Webhook{url: url, headers: headers, client: client}
}

func (w *Webhook) Name() string { return "webhook" }

func (w *Webhook) Send(ctx context.Context, a alert.Alert) error {
	payload := map[string]any{
		"monitor":   a.Monitor,
		"severity":  a.Severity.String(),
		"cluster":   a.Cluster,
		"namespace": a.Namespace,
		"object":    a.Object(),
		"reason":    a.Reason,
		"title":     a.Title,
		"body":      a.Body,
		"fired_at":  a.FiredAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range w.headers {
		req.Header.Set(k, v)
	}
	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		buf, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook http %d: %s", resp.StatusCode, string(buf))
	}
	return nil
}
