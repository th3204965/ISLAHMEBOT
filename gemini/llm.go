package gemini

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	systemPrompt = `You are a respectful, warm, and comforting Islamic voice assistant.
CRITICAL INSTRUCTION: You must respond ONLY in conversational spoken Hindustani.
However, you MUST write your response using the English/Latin alphabet (Roman Urdu / Hinglish). 
Example: "Aap kaise hain? Din mein paanch farz namazein hoti hain."
Do NOT use Devanagari (अाप) or Arabic/Urdu scripts (ش). Do NOT provide English translations. Do NOT use prefixes like "Hindi:" or "Urdu:".
Output ONLY the raw conversational Roman text that should be immediately spoken aloud directly to the user.
Use accurate phonetic Arabic pronunciation for Islamic terms like Salah, Quran, Hadith, etc.
Ground your answers in the Quran and Sahih Hadith. Keep responses incredibly concise (1-3 sentences maximum) for lower latency.
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
	AudioData   []byte
	DurationSec int
}

// GenerateTextResponse generates a text answer using gemini-2.5-flash.
func GenerateTextResponse(ctx context.Context, text string) (string, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY is not set")
	}

	var req textRequest
	req.SystemInstruction = content{Parts: []part{{Text: systemPrompt}}}
	req.Contents = []content{{Parts: []part{{Text: text}}}}
	req.GenerationConfig.ResponseModalities = []string{"TEXT"}

	body, err := callGemini(ctx, apiKey, textModel, "generateContent", req)
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

// sanitizeForTTS replaces known problematic unicode ligatures that cause the Gemini TTS model to fail.
func sanitizeForTTS(text string) string {
	replacements := map[string]string{
		"ﷺ": "सल्लल्लाहु अलैहि वसल्लम",    // Sallallahu Alaihi Wasallam
		"ﷻ": "जल्ल जलालुहू",               // Jalla Jalaluhu
		"﷽": "बिस्मिल्लाहिर्रहमानिर्रहीम", // Bismillah
	}
	sanitized := text
	for ligature, replacement := range replacements {
		sanitized = strings.ReplaceAll(sanitized, ligature, replacement)
	}
	return sanitized
}

// GenerateAudio converts text to speech and returns WAV audio with duration.
func GenerateAudio(ctx context.Context, text string) (*AudioResult, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY is not set")
	}

	sanitizedText := sanitizeForTTS(text)

	var req ttsRequest
	req.Contents = []content{{Parts: []part{{Text: sanitizedText}}}}
	req.GenerationConfig.ResponseModalities = []string{"AUDIO"}
	req.GenerationConfig.SpeechConfig.VoiceConfig.PrebuiltVoiceConfig.VoiceName = "Kore"
	req.Model = ttsModel

	body, err := callGemini(ctx, apiKey, ttsModel, "generateContent", req)
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

	ogg, err := encodeOggOpus(ctx, wav)
	if err != nil {
		return nil, fmt.Errorf("failed to encode to ogg: %w", err)
	}

	return &AudioResult{AudioData: ogg, DurationSec: duration}, nil
}

func encodeOggOpus(ctx context.Context, wav []byte) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "ffmpeg", "-i", "pipe:0", "-c:a", "libopus", "-b:a", "32k", "-f", "ogg", "pipe:1")
	cmd.Stdin = bytes.NewReader(wav)

	var out bytes.Buffer
	cmd.Stdout = &out

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg failed: %w, stderr: %s", err, stderr.String())
	}

	return out.Bytes(), nil
}

// GenerateVoiceResponse runs the full pipeline: question → text answer → TTS audio.
// Returns audio result, answer text, and error.
func GenerateVoiceResponse(ctx context.Context, question string) (*AudioResult, string, error) {
	answer, err := GenerateTextResponse(ctx, question)
	if err != nil {
		return nil, "", fmt.Errorf("text generation failed: %w", err)
	}
	slog.Info("Answer generated", "component", "gemini", "answer", answer)

	audio, err := GenerateAudio(ctx, answer)
	if err != nil {
		return nil, answer, fmt.Errorf("TTS failed: %w", err)
	}

	return audio, answer, nil
}

// --- Internal helpers ---

func callGemini(ctx context.Context, apiKey, model, method string, reqBody any) ([]byte, error) {
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
			slog.Warn("Retrying gemini request", "component", "gemini", "attempt", attempt+1, "max_retries", maxRetries, "backoff", backoff)
			time.Sleep(backoff)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
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
		slog.Warn("Transient gemini error", "component", "gemini", "status", resp.StatusCode, "attempt", attempt+1)
	}

	return nil, fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

func isRetryable(code int) bool {
	return code == 429 || code == 500 || code == 502 || code == 503 || code == 504
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
