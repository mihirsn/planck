// Package notify sends alert notifications to configured destinations (ntfy, webhooks).
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mihirsn/planck/internal/config"
)

// AlertPayload contains the structured data for an alert.
type AlertPayload struct {
	Container string  `json:"container"`
	Type      string  `json:"type"` // e.g. "error_rate", "p95_latency", "rps", "cpu", "memory"
	Title     string  `json:"title"`
	Endpoint  string  `json:"endpoint,omitempty"`
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`
	Unit      string  `json:"unit,omitempty"` // e.g. "%", "ms", "MB"
	Message   string  `json:"message"`        // Pre-formatted markdown string (used by ntfy)
	Timestamp string  `json:"timestamp"`
}

// Client dispatches notifications to all configured destinations.
type Client struct {
	cfg  config.NotifyConfig
	http *http.Client
	out  io.Writer
}

// NewClient creates a new notification client.
func NewClient(cfg config.NotifyConfig, out io.Writer) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: 10 * time.Second},
		out:  out,
	}
}

// Send dispatches the AlertPayload to all configured destinations in the background.
// It returns immediately without blocking. Errors are printed to the client's io.Writer.
func (c *Client) Send(payload AlertPayload) {
	if c.cfg.Ntfy != nil {
		go func() {
			if err := c.sendNtfy(c.cfg.Ntfy, payload); err != nil {
				fmt.Fprintf(c.out, "     ⚠  Failed to send ntfy alert: %v\n", err)
			}
		}()
	}
	if c.cfg.Webhook != nil {
		go func() {
			if err := c.sendWebhook(c.cfg.Webhook, payload); err != nil {
				fmt.Fprintf(c.out, "     ⚠  Failed to send webhook alert: %v\n", err)
			}
		}()
	}
}

func (c *Client) sendNtfy(cfg *config.NtfyConfig, payload AlertPayload) error {
	server := strings.TrimRight(cfg.Server, "/")
	url := fmt.Sprintf("%s/%s", server, cfg.Topic)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, strings.NewReader(payload.Message))
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}

	req.Header.Set("Title", payload.Title)
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Priority", "high")
	req.Header.Set("Tags", "exclamation")
	req.Header.Set("Markdown", "yes")

	if cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("returned HTTP %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) sendWebhook(cfg *config.WebhookConfig, payload AlertPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON payload: %w", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, cfg.URL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("returned HTTP %d", resp.StatusCode)
	}

	return nil
}
