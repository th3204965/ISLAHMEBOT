package groq

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestGenerateTextResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"choices": [
				{
					"message": {
						"content": "Assalamu alaikum. Aap kaise hain?"
					}
				}
			]
		}`))
	}))
	defer ts.Close()

	os.Setenv("GROQ_API_KEY", "TEST_TOKEN")
	originalBaseURL := groqBaseURL
	groqBaseURL = ts.URL
	defer func() { groqBaseURL = originalBaseURL }()

	text, err := GenerateTextResponse(context.Background(), "Salam")
	if err != nil {
		t.Fatalf("GenerateTextResponse failed: %v", err)
	}

	expected := "Assalamu alaikum. Aap kaise hain?"
	if text != expected {
		t.Errorf("Expected '%s', got '%s'", expected, text)
	}
}

func TestGenerateTextResponse_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"Unauthorized"}`))
	}))
	defer ts.Close()

	os.Setenv("GROQ_API_KEY", "TEST_TOKEN")
	originalBaseURL := groqBaseURL
	groqBaseURL = ts.URL
	defer func() { groqBaseURL = originalBaseURL }()

	_, err := GenerateTextResponse(context.Background(), "Salam")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "groq llm error 401") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestGenerateTextResponse_EmptyChoices(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices": []}`))
	}))
	defer ts.Close()

	os.Setenv("GROQ_API_KEY", "TEST_TOKEN")
	originalBaseURL := groqBaseURL
	groqBaseURL = ts.URL
	defer func() { groqBaseURL = originalBaseURL }()

	_, err := GenerateTextResponse(context.Background(), "Salam")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "empty groq llm response") {
		t.Errorf("Unexpected error message: %v", err)
	}
}
