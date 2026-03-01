package groq

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
)

type TranslationResponse struct {
	Text string `json:"text"`
}

// TranscribeAudio streams the given io.Reader directly to Groq's STT engine
func TranscribeAudio(audioReader io.Reader) (string, error) {
	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("GROQ_API_KEY is not set")
	}

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	// Stream constructing in goroutine
	go func() {
		defer pw.Close()
		defer writer.Close()

		part, err := writer.CreateFormFile("file", "audio.ogg")
		if err != nil {
			pw.CloseWithError(fmt.Errorf("failed to create groq form file: %w", err))
			return
		}

		if _, err := io.Copy(part, audioReader); err != nil {
			pw.CloseWithError(fmt.Errorf("failed to copy audio to groq stream: %w", err))
			return
		}

		// Write model field
		if err := writer.WriteField("model", "whisper-large-v3"); err != nil {
			pw.CloseWithError(fmt.Errorf("failed to write model field: %w", err))
			return
		}
	}()

	url := "https://api.groq.com/openai/v1/audio/transcriptions"
	req, err := http.NewRequest(http.MethodPost, url, pr)
	if err != nil {
		return "", fmt.Errorf("failed to create groq request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call groq: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("groq api error %d: %s", resp.StatusCode, string(body))
	}

	var transResp TranslationResponse
	if err := json.NewDecoder(resp.Body).Decode(&transResp); err != nil {
		return "", fmt.Errorf("failed to decode groq response: %w", err)
	}

	return transResp.Text, nil
}
