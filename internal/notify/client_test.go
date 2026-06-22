package notify

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mihirsn/planck/internal/config"
)

func TestSendNtfy_Success(t *testing.T) {
	var receivedTitle, receivedBody, receivedAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedTitle = r.Header.Get("Title")
		receivedAuth = r.Header.Get("Authorization")
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		receivedBody = string(buf[:n])
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(config.NotifyConfig{}, io.Discard)
	cfg := &config.NtfyConfig{
		Topic:  "test-topic",
		Server: srv.URL,
		Token:  "mytoken",
	}

	payload := AlertPayload{
		Title:   "My Title",
		Message: "My Message",
	}

	err := c.sendNtfy(cfg, payload)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if receivedTitle != "My Title" {
		t.Errorf("expected Title header 'My Title', got %q", receivedTitle)
	}
	if receivedBody != "My Message" {
		t.Errorf("expected body 'My Message', got %q", receivedBody)
	}
	if receivedAuth != "Bearer mytoken" {
		t.Errorf("expected 'Bearer mytoken', got %q", receivedAuth)
	}
}

func TestSendWebhook_Success(t *testing.T) {
	var receivedAuth string
	var receivedPayload AlertPayload

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		json.NewDecoder(r.Body).Decode(&receivedPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(config.NotifyConfig{}, io.Discard)
	cfg := &config.WebhookConfig{
		URL: srv.URL,
		Headers: map[string]string{
			"Authorization": "Bearer webhook-token",
		},
	}

	payload := AlertPayload{
		Container: "my-api",
		Type:      "cpu",
		Title:     "High CPU",
		Value:     95.5,
		Threshold: 80.0,
	}

	err := c.sendWebhook(cfg, payload)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if receivedAuth != "Bearer webhook-token" {
		t.Errorf("expected 'Bearer webhook-token', got %q", receivedAuth)
	}
	if receivedPayload.Container != "my-api" {
		t.Errorf("expected container 'my-api', got %q", receivedPayload.Container)
	}
	if receivedPayload.Value != 95.5 {
		t.Errorf("expected value 95.5, got %f", receivedPayload.Value)
	}
}
