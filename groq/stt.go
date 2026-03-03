package groq

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"

	"github.com/th3204965/islahmebot/httpclient"
)

var groqBaseURL = "https://api.groq.com/openai/v1"

// TranscriptionResponse holds the STT result from Groq.
type TranscriptionResponse struct {
	Text string `json:"text"`
}

// TranscribeAudio streams audio to Groq's Whisper STT and returns the transcribed text.
// Uses io.Pipe for zero-copy streaming — the audio bytes flow directly from the
// HTTP download into the Groq upload without buffering the entire file in memory.
func TranscribeAudio(ctx context.Context, audioReader io.Reader) (string, error) {
	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("GROQ_API_KEY is not set")
	}

	// Build multipart form body via pipe (zero-copy)
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		defer writer.Close()

		part, err := writer.CreateFormFile("file", "audio.ogg")
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		if _, err := io.Copy(part, audioReader); err != nil {
			pw.CloseWithError(err)
			return
		}
		writer.WriteField("model", "whisper-large-v3")
		writer.WriteField("language", "hi")
	}()

	apiURL := fmt.Sprintf("%s/audio/transcriptions", groqBaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, pr)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := httpclient.Shared.Do(req)
	if err != nil {
		return "", fmt.Errorf("groq request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("groq error %d: %s", resp.StatusCode, string(body))
	}

	var result TranscriptionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Text, nil
}
