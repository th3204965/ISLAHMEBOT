package groq

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestTranscribeAudio_MissingAPIKey(t *testing.T) {
	os.Unsetenv("GROQ_API_KEY")
	_, err := TranscribeAudio(strings.NewReader("fake audio"))
	if err == nil || !strings.Contains(err.Error(), "GROQ_API_KEY") {
		t.Fatalf("expected API key error, got: %v", err)
	}
}

func TestTranscriptionResponse_Parsing(t *testing.T) {
	j := `{"text": "test transcription"}`
	var r TranscriptionResponse
	if err := json.Unmarshal([]byte(j), &r); err != nil {
		t.Fatal(err)
	}
	if r.Text != "test transcription" {
		t.Errorf("got %q", r.Text)
	}
}

func TestTranscriptionResponse_Empty(t *testing.T) {
	j := `{"text": ""}`
	var r TranscriptionResponse
	json.Unmarshal([]byte(j), &r)
	if r.Text != "" {
		t.Errorf("expected empty, got %q", r.Text)
	}
}
