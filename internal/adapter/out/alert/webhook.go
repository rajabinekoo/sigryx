package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portout "github.com/rajabinekoo/sigryx/internal/core/port/out"
)

type Webhook struct {
	url    string
	client *http.Client
}

func NewWebhook(rawURL string, timeout time.Duration) (*Webhook, error) {
	rawURL = strings.TrimSpace(rawURL)
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	if rawURL == "" {
		return &Webhook{client: &http.Client{Timeout: timeout}}, nil
	}
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, fmt.Errorf("invalid alert webhook URL")
	}
	return &Webhook{url: rawURL, client: &http.Client{Timeout: timeout}}, nil
}

func (w *Webhook) Send(ctx context.Context, alert domain.Alert) error {
	if w == nil || w.url == "" {
		return nil
	}
	body, err := json.Marshal(alert)
	if err != nil {
		return fmt.Errorf("marshal alert: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create alert request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("send alert webhook: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("alert webhook returned status %d", resp.StatusCode)
	}
	return nil
}

var _ portout.AlertSink = (*Webhook)(nil)
