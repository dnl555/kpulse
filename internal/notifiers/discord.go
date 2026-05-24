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

const discordLimit = 1900

type Discord struct {
	url    string
	client *http.Client
}

func NewDiscord(url string, client *http.Client) *Discord {
	if client == nil {
		client = http.DefaultClient
	}
	return &Discord{url: url, client: client}
}

func (d *Discord) Name() string { return "discord" }

func (d *Discord) Send(ctx context.Context, a alert.Alert) error {
	var emoji, prefix string
	if a.State == alert.StateResolved {
		emoji = "OK"
		prefix = "**[RESOLVED]** "
	} else {
		emoji = map[alert.Severity]string{alert.Info: "i", alert.Warning: "!", alert.Critical: "!!"}[a.Severity]
	}
	content := fmt.Sprintf("[%s] %s**[%s]** `%s/%s` %s\n%s", emoji, prefix, a.Cluster, a.Namespace, a.Object(), a.Title, a.Body)
	if len(content) > discordLimit {
		content = content[:discordLimit] + " (truncated)"
	}
	body, _ := json.Marshal(map[string]string{"content": content})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		buf, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("discord http %d: %s", resp.StatusCode, string(buf))
	}
	return nil
}
