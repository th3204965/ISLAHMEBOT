package telegram

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestSendMessage(t *testing.T) {
	// Setup mock server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/botTEST_TOKEN/sendMessage" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "chat_id=123") || !strings.Contains(string(body), "text=Hello") {
			t.Errorf("Unexpected body: %s", string(body))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// Override globals for test
	os.Setenv("TELEGRAM_BOT_TOKEN", "TEST_TOKEN")
	originalBaseURL := telegramBaseURL
	telegramBaseURL = ts.URL
	defer func() { telegramBaseURL = originalBaseURL }()

	err := SendMessage(context.Background(), 123, "Hello")
	if err != nil {
		t.Errorf("SendMessage failed: %v", err)
	}
}

func TestSendMessage_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"ok":false,"description":"Bad Request"}`))
	}))
	defer ts.Close()

	os.Setenv("TELEGRAM_BOT_TOKEN", "TEST_TOKEN")
	originalBaseURL := telegramBaseURL
	telegramBaseURL = ts.URL
	defer func() { telegramBaseURL = originalBaseURL }()

	err := SendMessage(context.Background(), 123, "Hello")
	if err == nil {
		t.Error("Expected error for 400 response, got nil")
	}
	if !strings.Contains(err.Error(), "telegram error 400") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestGetFileURL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/botTEST_TOKEN/getFile" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		if r.URL.RawQuery != "file_id=FILE123" {
			t.Errorf("Unexpected query: %s", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true,"result":{"file_path":"voice/file.ogg"}}`))
	}))
	defer ts.Close()

	os.Setenv("TELEGRAM_BOT_TOKEN", "TEST_TOKEN")
	originalBaseURL := telegramBaseURL
	telegramBaseURL = ts.URL
	defer func() { telegramBaseURL = originalBaseURL }()

	url, err := GetFileURL(context.Background(), "FILE123")
	if err != nil {
		t.Fatalf("GetFileURL failed: %v", err)
	}

	expectedURL := ts.URL + "/file/botTEST_TOKEN/voice/file.ogg"
	if url != expectedURL {
		t.Errorf("Expected URL %s, got %s", expectedURL, url)
	}
}
