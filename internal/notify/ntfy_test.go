package notify_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mihirsn/planck/internal/notify"
)

func TestSend_Success(t *testing.T) {
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

	c := notify.NewClient(srv.URL, "test-topic", "")
	err := c.Send("My Title", "My Message")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if receivedTitle != "My Title" {
		t.Errorf("expected Title header 'My Title', got %q", receivedTitle)
	}
	if receivedBody != "My Message" {
		t.Errorf("expected body 'My Message', got %q", receivedBody)
	}
	if receivedAuth != "" {
		t.Errorf("expected no auth header, got %q", receivedAuth)
	}
}

func TestSend_WithToken(t *testing.T) {
	var receivedAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := notify.NewClient(srv.URL, "test-topic", "mytoken")
	err := c.Send("Title", "Body")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if receivedAuth != "Bearer mytoken" {
		t.Errorf("expected 'Bearer mytoken', got %q", receivedAuth)
	}
}

func TestSend_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := notify.NewClient(srv.URL, "test-topic", "")
	err := c.Send("Title", "Body")
	if err == nil {
		t.Error("expected error for HTTP 500 response")
	}
}

func TestSend_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := notify.NewClient(srv.URL, "test-topic", "wrong-token")
	err := c.Send("Title", "Body")
	if err == nil {
		t.Error("expected error for HTTP 401 response")
	}
}

func TestSend_UnreachableServer(t *testing.T) {
	// Use a port that's not listening.
	c := notify.NewClient("http://127.0.0.1:19999", "test-topic", "")
	err := c.Send("Title", "Body")
	if err == nil {
		t.Error("expected error for unreachable server")
	}
}

func TestSend_TrailingSlash(t *testing.T) {
	// Verify that a trailing slash in the server URL doesn't cause double-slashes.
	var requestPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := notify.NewClient(srv.URL+"/", "my-topic", "")
	err := c.Send("Title", "Body")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if requestPath != "/my-topic" {
		t.Errorf("expected path /my-topic, got %q", requestPath)
	}
}
