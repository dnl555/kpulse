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

type Slack struct {
	url    string
	client *http.Client
}

func NewSlack(url string, client *http.Client) *Slack {
	if client == nil {
		client = http.DefaultClient
	}
	return &Slack{url: url, client: client}
}

func (s *Slack) Name() string { return "slack" }

func (s *Slack) Send(ctx context.Context, a alert.Alert) error {
	emoji := map[alert.Severity]string{alert.Info: ":information_source:", alert.Warning: ":warning:", alert.Critical: ":rotating_light:"}[a.Severity]
	text := fmt.Sprintf("%s *[%s]* `%s/%s` %s\n%s", emoji, a.Cluster, a.Namespace, a.Object(), a.Title, a.Body)
	body, err := json.Marshal(map[string]any{"text": text})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		buf, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("slack http %d: %s", resp.StatusCode, string(buf))
	}
	return nil
}
