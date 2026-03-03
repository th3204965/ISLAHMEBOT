package groq

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestTranscribeAudio(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/audio/transcriptions" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"text":"hello world"}`))
	}))
	defer ts.Close()

	os.Setenv("GROQ_API_KEY", "TEST_TOKEN")
	originalBaseURL := groqBaseURL
	groqBaseURL = ts.URL
	defer func() { groqBaseURL = originalBaseURL }()

	audioData := strings.NewReader("dummy audio data")
	text, err := TranscribeAudio(context.Background(), audioData)
	if err != nil {
		t.Fatalf("TranscribeAudio failed: %v", err)
	}

	if text != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", text)
	}
}

func TestTranscribeAudio_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"Unauthorized"}`))
	}))
	defer ts.Close()

	os.Setenv("GROQ_API_KEY", "TEST_TOKEN")
	originalBaseURL := groqBaseURL
	groqBaseURL = ts.URL
	defer func() { groqBaseURL = originalBaseURL }()

	audioData := strings.NewReader("dummy audio data")
	_, err := TranscribeAudio(context.Background(), audioData)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "groq error 401") {
		t.Errorf("Unexpected error message: %v", err)
	}
}
