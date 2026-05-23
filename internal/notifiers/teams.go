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

type Teams struct {
	url    string
	client *http.Client
}

func NewTeams(url string, c *http.Client) *Teams {
	if c == nil {
		c = http.DefaultClient
	}
	return &Teams{url: url, client: c}
}

func (t *Teams) Name() string { return "teams" }

func (t *Teams) Send(ctx context.Context, a alert.Alert) error {
	color := map[alert.Severity]string{alert.Info: "0078D7", alert.Warning: "FFA500", alert.Critical: "D13438"}[a.Severity]
	payload := map[string]any{
		"@type":      "MessageCard",
		"@context":   "https://schema.org/extensions",
		"themeColor": color,
		"summary":    a.Title,
		"title":      fmt.Sprintf("[%s] %s", a.Cluster, a.Title),
		"sections": []map[string]any{
			{"text": a.Body, "facts": []map[string]string{
				{"name": "Namespace", "value": a.Namespace},
				{"name": "Object", "value": a.Object()},
				{"name": "Reason", "value": a.Reason},
				{"name": "Severity", "value": a.Severity.String()},
			}},
		},
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		buf, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("teams http %d: %s", resp.StatusCode, string(buf))
	}
	return nil
}
