package groq

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

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

// GenerateTextResponse uses Groq's Llama 3.3 70B to generate an almost instantaneous text response.
// It retrieves the conversational context based on the chatID before firing the completion request.
func GenerateTextResponse(ctx context.Context, chatID int64, text string) (string, error) {
	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("GROQ_API_KEY is not set")
	}

	apiURL := fmt.Sprintf("%s/chat/completions", groqBaseURL)

	type Request struct {
		Model    string    `json:"model"`
		Messages []Message `json:"messages"`
	}

	h := getHistory(chatID)
	h.AddMessage("user", text)

	reqBody := Request{
		Model: "llama-3.3-70b-versatile",
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

	var res struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", fmt.Errorf("failed to decode groq llm response: %w", err)
	}

	if len(res.Choices) == 0 {
		return "", fmt.Errorf("empty groq llm response")
	}

	answer := res.Choices[0].Message.Content
	h.AddMessage("assistant", answer)

	return answer, nil
}
