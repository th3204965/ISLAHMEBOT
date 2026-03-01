package gemini

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
)

// The SSE stream returns data prefixed with "data: " and then a JSON blob
type streamChunk struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				InlineData struct {
					MimeType string `json:"mimeType"`
					Data     string `json:"data"` // base64 encoded audio bytes
				} `json:"inlineData"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

// parseGeminiAudioStream streams SSE data chunks from the Gemini HTTP response,
// parses the JSON, extracts the base64 audio parts, decodes them, and writes
// the raw binary bytes directly to the provided io.WriteCloser (the pipe).
func parseGeminiAudioStream(body io.ReadCloser, pw io.WriteCloser) error {
	scanner := bufio.NewScanner(body)
	
	// Default buffer might be too small for large base64 chunks; increase it.
	const maxCapacity = 1024 * 1024 // 1MB buffer should be enough per SSE event
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	dataPrefix := []byte("data: ")

	for scanner.Scan() {
		line := scanner.Bytes()
		
		// Ignore empty lines or comments
		if len(line) == 0 || !bytes.HasPrefix(line, dataPrefix) {
			continue
		}

		// Extract the JSON payload after "data: "
		jsonPayload := bytes.TrimPrefix(line, dataPrefix)

		var chunk streamChunk
		if err := json.Unmarshal(jsonPayload, &chunk); err != nil {
			// It might be the "[DONE]" token at the end of the stream
			if string(jsonPayload) == "\"[DONE]\"" || string(jsonPayload) == "[DONE]" {
				break
			}
			continue // skip malformed chunks
		}

		// Extract base64 and write raw bytes
		if len(chunk.Candidates) > 0 && len(chunk.Candidates[0].Content.Parts) > 0 {
			for _, part := range chunk.Candidates[0].Content.Parts {
				if part.InlineData.Data != "" {
					b64Str := part.InlineData.Data
					
					// Decode the base64 inlineData directly
					decoder := base64.NewDecoder(base64.StdEncoding, bytes.NewBufferString(b64Str))
					
					// Stream copy the decoded bytes to the output pipe
					if _, err := io.Copy(pw, decoder); err != nil {
						return fmt.Errorf("failed writing decoded audio to pipe: %w", err)
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading gemini stream: %w", err)
	}

	return nil
}
