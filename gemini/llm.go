package gemini

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	systemPrompt = `You are a respectful, warm, and comforting Islamic voice assistant.
Respond in natural Hindustani — a mix of Hindi and Urdu as spoken by Indian Muslims. Use Devanagari script for text.
Use accurate Arabic pronunciation for Islamic terms like Salah, Quran, Hadith, etc.
Ground your answers in the Quran and Sahih Hadith.
Keep responses concise (2-4 sentences) since they will be spoken aloud.
If asked about an uncertain or complex Fatwa, politely decline and advise consulting a qualified Islamic scholar.`

	maxRetries    = 3
	baseBackoff   = 500 * time.Millisecond
	ttsModel      = "gemini-2.5-flash-preview-tts"
	textModel     = "gemini-2.5-flash"
	geminiBaseURL = "https://generativelanguage.googleapis.com/v1beta/models"

	// PCM audio parameters (from Gemini TTS output)
	pcmSampleRate    = 24000
	pcmChannels      = 1
	pcmBitsPerSample = 16
)

// --- Request types ---

type content struct {
	Parts []part `json:"parts"`
}

type part struct {
	Text string `json:"text"`
}

type textRequest struct {
	SystemInstruction content   `json:"systemInstruction"`
	Contents          []content `json:"contents"`
	GenerationConfig  struct {
		ResponseModalities []string `json:"responseModalities"`
	} `json:"generationConfig"`
}

type ttsRequest struct {
	Contents         []content `json:"contents"`
	GenerationConfig struct {
		ResponseModalities []string `json:"responseModalities"`
		SpeechConfig       struct {
			VoiceConfig struct {
				PrebuiltVoiceConfig struct {
					VoiceName string `json:"voiceName"`
				} `json:"prebuiltVoiceConfig"`
			} `json:"voiceConfig"`
		} `json:"speechConfig"`
	} `json:"generationConfig"`
	Model string `json:"model"`
}

// --- Response types ---

type textResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

type ttsResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				InlineData struct {
					MimeType string `json:"mimeType"`
					Data     string `json:"data"`
				} `json:"inlineData"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

// --- Public API ---

// AudioResult holds the WAV audio bytes and duration.
type AudioResult struct {
	WAVData     []byte
	DurationSec int
}

// GenerateTextResponse generates a text answer using gemini-2.5-flash.
func GenerateTextResponse(text string) (string, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY is not set")
	}

	var req textRequest
	req.SystemInstruction = content{Parts: []part{{Text: systemPrompt}}}
	req.Contents = []content{{Parts: []part{{Text: text}}}}
	req.GenerationConfig.ResponseModalities = []string{"TEXT"}

	body, err := callGemini(apiKey, textModel, "generateContent", req)
	if err != nil {
		return "", err
	}

	var resp textResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to decode text response: %w", err)
	}

	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		return resp.Candidates[0].Content.Parts[0].Text, nil
	}
	return "", fmt.Errorf("empty text response")
}

// GenerateAudio converts text to speech and returns WAV audio with duration.
func GenerateAudio(text string) (*AudioResult, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY is not set")
	}

	var req ttsRequest
	req.Contents = []content{{Parts: []part{{Text: text}}}}
	req.GenerationConfig.ResponseModalities = []string{"AUDIO"}
	req.GenerationConfig.SpeechConfig.VoiceConfig.PrebuiltVoiceConfig.VoiceName = "Kore"
	req.Model = ttsModel

	body, err := callGemini(apiKey, ttsModel, "generateContent", req)
	if err != nil {
		return nil, err
	}

	var resp ttsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to decode TTS response: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty TTS response")
	}

	b64 := resp.Candidates[0].Content.Parts[0].InlineData.Data
	if b64 == "" {
		return nil, fmt.Errorf("empty audio data in TTS response")
	}

	pcm, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 audio: %w", err)
	}

	wav := pcmToWAV(pcm)
	duration := len(pcm) / (pcmSampleRate * pcmChannels * pcmBitsPerSample / 8)

	return &AudioResult{WAVData: wav, DurationSec: duration}, nil
}

// GenerateVoiceResponse runs the full pipeline: question → text answer → TTS audio.
// Returns audio result, answer text, and error.
func GenerateVoiceResponse(question string) (*AudioResult, string, error) {
	answer, err := GenerateTextResponse(question)
	if err != nil {
		return nil, "", fmt.Errorf("text generation failed: %w", err)
	}
	log.Printf("[gemini] Answer: %s", answer)

	audio, err := GenerateAudio(answer)
	if err != nil {
		return nil, answer, fmt.Errorf("TTS failed: %w", err)
	}

	return audio, answer, nil
}

// --- Internal helpers ---

func callGemini(apiKey, model, method string, reqBody any) ([]byte, error) {
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/%s:%s?key=%s", geminiBaseURL, model, method, apiKey)
	client := &http.Client{Timeout: 60 * time.Second}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := baseBackoff * time.Duration(1<<(attempt-1))
			log.Printf("[gemini] Retry %d/%d after %v", attempt+1, maxRetries, backoff)
			time.Sleep(backoff)
		}

		req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed (attempt %d): %w", attempt+1, err)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return body, nil
		}

		lastErr = fmt.Errorf("api error %d: %s", resp.StatusCode, string(body))
		if !isRetryable(resp.StatusCode) {
			return nil, lastErr
		}
		log.Printf("[gemini] Transient error %d (attempt %d/%d)", resp.StatusCode, attempt+1, maxRetries)
	}

	return nil, fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

func isRetryable(code int) bool {
	return code == 429 || code == 500 || code == 502 || code == 503
}

func pcmToWAV(pcm []byte) []byte {
	dataSize := len(pcm)
	byteRate := pcmSampleRate * pcmChannels * pcmBitsPerSample / 8
	blockAlign := pcmChannels * pcmBitsPerSample / 8

	buf := new(bytes.Buffer)
	buf.WriteString("RIFF")
	binary.Write(buf, binary.LittleEndian, int32(36+dataSize))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	binary.Write(buf, binary.LittleEndian, int32(16))
	binary.Write(buf, binary.LittleEndian, int16(1))
	binary.Write(buf, binary.LittleEndian, int16(pcmChannels))
	binary.Write(buf, binary.LittleEndian, int32(pcmSampleRate))
	binary.Write(buf, binary.LittleEndian, int32(byteRate))
	binary.Write(buf, binary.LittleEndian, int16(blockAlign))
	binary.Write(buf, binary.LittleEndian, int16(pcmBitsPerSample))
	buf.WriteString("data")
	binary.Write(buf, binary.LittleEndian, int32(dataSize))
	buf.Write(pcm)

	return buf.Bytes()
}
