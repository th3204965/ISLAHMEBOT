package groq

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/th3204965/islahmebot/httpclient"
)

// systemPrompt locks the Groq Llama 3 model into extremely fast Romanized Hinglish generation.
const systemPrompt = `You are a respectful, warm, and comforting Islamic voice assistant.
CRITICAL INSTRUCTION: You must respond ONLY in conversational spoken Hindustani.
However, you MUST write your response using the English/Latin alphabet (Roman Urdu / Hinglish). 
Example: "Aap kaise hain... Din mein, paanch farz namazein hoti hain."
Do NOT use Devanagari (अाप) or Arabic/Urdu scripts (ش). Do NOT provide English translations. Do NOT use prefixes like "Hindi:" or "Urdu:".
Output ONLY the raw conversational Roman text that should be immediately spoken aloud directly to the user.
Use accurate phonetic Arabic pronunciation for Islamic terms like Salah, Quran, Hadith, etc.
CRITICAL FOR VOICE NATURALNESS: To make the text-to-speech engine sound incredibly natural and human, use commas (,) and ellipses (...) frequently to insert natural speaking pauses and breaths.
Ground your answers in the Quran and Sahih Hadith. Keep responses incredibly concise (1-3 sentences maximum) for lower latency.
If asked about an uncertain or complex Fatwa, politely decline and advise consulting a qualified Islamic scholar.`

// GenerateTextStream uses Groq's Llama 3.3 70B to generate text and streams complete sentences back via a callback.
func GenerateTextStream(ctx context.Context, chatID int64, text string, onSentence func(string)) (string, error) {
	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("GROQ_API_KEY is not set")
	}

	apiURL := fmt.Sprintf("%s/chat/completions", groqBaseURL)

	type Request struct {
		Model    string    `json:"model"`
		Messages []Message `json:"messages"`
		Stream   bool      `json:"stream"`
	}

	h := getHistory(chatID)
	h.AddMessage("user", text)

	reqBody := Request{
		Model:  "llama-3.3-70b-versatile",
		Stream: true,
	}

	finalMessages := []Message{{Role: "system", Content: systemPrompt}}
	finalMessages = append(finalMessages, h.GetMessages()...)
	reqBody.Messages = finalMessages

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal groq llm request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create groq llm request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpclient.Shared.Do(req)
	if err != nil {
		return "", fmt.Errorf("groq llm request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("groq llm error %d: %s", resp.StatusCode, string(body))
	}

	reader := bufio.NewReader(resp.Body)
	var fullResponse strings.Builder
	var buffer strings.Builder

	flushBuffer := func(force bool) {
		str := strings.TrimSpace(buffer.String())
		if str != "" {
			if force || strings.ContainsAny(str, ".?!|\n") {
				onSentence(str)
				buffer.Reset()
			}
		}
	}

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", fmt.Errorf("error reading stream: %w", err)
		}

		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}

		data := bytes.TrimPrefix(line, []byte("data: "))
		if string(data) == "[DONE]" {
			break
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}

		if err := json.Unmarshal(data, &chunk); err != nil {
			continue // skip bad chunks
		}

		if len(chunk.Choices) > 0 {
			content := chunk.Choices[0].Delta.Content
			if content != "" {
				fullResponse.WriteString(content)
				buffer.WriteString(content)

				// If the incoming text contains a major sentence boundary, flush it.
				if strings.ContainsAny(content, ".?!\n") {
					flushBuffer(false)
				}
			}
		}
	}

	flushBuffer(true) // flush anything remaining

	answer := fullResponse.String()
	h.AddMessage("assistant", answer)

	return answer, nil
}
