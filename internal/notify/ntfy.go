// Package notify sends alert notifications to an ntfy server.
package notify

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Client sends notifications to an ntfy server.
type Client struct {
	server string
	topic  string
	token  string
	http   *http.Client
}

// NewClient creates a new ntfy notification client.
func NewClient(server, topic, token string) *Client {
	return &Client{
		server: strings.TrimRight(server, "/"),
		topic:  topic,
		token:  token,
		http:   &http.Client{Timeout: 10 * time.Second},
	}
}

// Send delivers a notification with the given title and message body.
func (c *Client) Send(title, message string) error {
	url := fmt.Sprintf("%s/%s", c.server, c.topic)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, strings.NewReader(message))
	if err != nil {
		return fmt.Errorf("failed to build ntfy request: %w", err)
	}

	req.Header.Set("Title", title)
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Priority", "high")
	req.Header.Set("Tags", "warning,chart_with_upwards_trend")

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send ntfy notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ntfy returned HTTP %d for topic %q", resp.StatusCode, c.topic)
	}

	return nil
}
