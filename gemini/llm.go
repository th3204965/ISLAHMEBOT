package gemini

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

const systemInstruction = `You are a respectful, warm, and comforting Islamic voice assistant.
You support multilingual conversations (Hindi, Urdu, English) with accurate Arabic pronunciation.
You must ground your answers in the Quran and Sahih Hadith.
CRITICAL: If asked about an uncertain or complex Fatwa, politely decline to answer and advise the user to consult a qualified Islamic scholar.`

// Structure matching the Gemini GenerateContent request for Audio
type GenerateContentRequest struct {
	SystemInstruction Content          `json:"systemInstruction"`
	Contents          []Content        `json:"contents"`
	GenerationConfig  GenerationConfig `json:"generationConfig"`
}

type Content struct {
	Parts []Part `json:"parts"`
}

type Part struct {
	Text string `json:"text"`
}

type GenerationConfig struct {
	ResponseModalities []string     `json:"responseModalities"`
	SpeechConfig       SpeechConfig `json:"speechConfig"`
}

type SpeechConfig struct {
	VoiceConfig VoiceConfig `json:"voiceConfig"`
}

type VoiceConfig struct {
	PrebuiltVoiceConfig PrebuiltVoiceConfig `json:"prebuiltVoiceConfig"`
}

type PrebuiltVoiceConfig struct {
	VoiceName string `json:"voiceName"`
}

// GenerateAudioStream takes text input and returns an io.ReadCloser containing the OGG audio stream.
// The caller is responsible for closing the returned stream.
func GenerateAudioStream(text string) (io.ReadCloser, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY is not set")
	}

	reqBody := GenerateContentRequest{
		SystemInstruction: Content{
			Parts: []Part{{Text: systemInstruction}},
		},
		Contents: []Content{
			{Parts: []Part{{Text: text}}},
		},
		GenerationConfig: GenerationConfig{
			ResponseModalities: []string{"AUDIO"},
			SpeechConfig: SpeechConfig{
				VoiceConfig: VoiceConfig{
					PrebuiltVoiceConfig: PrebuiltVoiceConfig{
						VoiceName: "Kore", // Standard voice config
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal gemini request: %w", err)
	}

	// We are using the newer Gemini API v1beta / v1 interface that supports multimodal
	// Using the standard REST endpoint
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?alt=sse&key=%s", apiKey)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create gemini request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call gemini: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		errorBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gemini api error %d: %s", resp.StatusCode, string(errorBody))
	}

	// Because Gemini standard endpoint returns JSON with inline Base64 or we need to decode it.
	// Oh! Wait, the exact Gemini REST API behavior for AUDIO modality returns JSON with inline base64 data under `inlineData`. 
	// To maintain extreme low latency without a memory spike, we must parse the JSON stream to extract the base64 chunks and pipe them...
	// Note: We'll implement a stream decoder to parse the Server-Sent Events (alt=sse) chunks 
	// and decode the inlineData base64 -> raw bytes pipe.

	pr, pw := io.Pipe()

	go func() {
		defer resp.Body.Close()
		defer pw.Close()

		// Real implementation of parsing the JSON stream logic will be written below separately
		err := parseGeminiAudioStream(resp.Body, pw)
		if err != nil {
			pw.CloseWithError(err)
		}
	}()

	return pr, nil
}
