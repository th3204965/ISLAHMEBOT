package gemini

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestGenerateTextResponse_MissingAPIKey(t *testing.T) {
	os.Unsetenv("GEMINI_API_KEY")
	_, err := GenerateTextResponse("test")
	if err == nil || !strings.Contains(err.Error(), "GEMINI_API_KEY") {
		t.Fatalf("expected API key error, got: %v", err)
	}
}

func TestGenerateAudio_MissingAPIKey(t *testing.T) {
	os.Unsetenv("GEMINI_API_KEY")
	_, err := GenerateAudio("test")
	if err == nil || !strings.Contains(err.Error(), "GEMINI_API_KEY") {
		t.Fatalf("expected API key error, got: %v", err)
	}
}

func TestTextRequest_JSONStructure(t *testing.T) {
	var req textRequest
	req.SystemInstruction = content{Parts: []part{{Text: "system"}}}
	req.Contents = []content{{Parts: []part{{Text: "question"}}}}
	req.GenerationConfig.ResponseModalities = []string{"TEXT"}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if strings.Contains(s, "speechConfig") {
		t.Error("text request should not have speechConfig")
	}
	if !strings.Contains(s, `"TEXT"`) {
		t.Error("should contain TEXT modality")
	}
}

func TestTTSRequest_JSONStructure(t *testing.T) {
	var req ttsRequest
	req.Contents = []content{{Parts: []part{{Text: "hello"}}}}
	req.GenerationConfig.ResponseModalities = []string{"AUDIO"}
	req.GenerationConfig.SpeechConfig.VoiceConfig.PrebuiltVoiceConfig.VoiceName = "Kore"
	req.Model = "gemini-2.5-flash-preview-tts"

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, `"AUDIO"`) {
		t.Error("should contain AUDIO modality")
	}
	if !strings.Contains(s, `"Kore"`) {
		t.Error("should contain Kore voice")
	}
	if strings.Contains(s, "systemInstruction") {
		t.Error("TTS request should not have systemInstruction")
	}
}

func TestTextResponse_Parsing(t *testing.T) {
	j := `{"candidates":[{"content":{"parts":[{"text":"five daily prayers"}]}}]}`
	var r textResponse
	if err := json.Unmarshal([]byte(j), &r); err != nil {
		t.Fatal(err)
	}
	if len(r.Candidates) != 1 || r.Candidates[0].Content.Parts[0].Text != "five daily prayers" {
		t.Error("unexpected parse result")
	}
}

func TestTextResponse_Empty(t *testing.T) {
	j := `{"candidates":[]}`
	var r textResponse
	json.Unmarshal([]byte(j), &r)
	if len(r.Candidates) != 0 {
		t.Error("expected empty")
	}
}

func TestTTSResponse_Parsing(t *testing.T) {
	audio := []byte("fake-pcm-data")
	b64 := base64.StdEncoding.EncodeToString(audio)
	j := `{"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"audio/L16","data":"` + b64 + `"}}]}}]}`

	var r ttsResponse
	if err := json.Unmarshal([]byte(j), &r); err != nil {
		t.Fatal(err)
	}
	decoded, _ := base64.StdEncoding.DecodeString(r.Candidates[0].Content.Parts[0].InlineData.Data)
	if string(decoded) != "fake-pcm-data" {
		t.Error("audio data mismatch")
	}
}

func TestIsRetryable(t *testing.T) {
	cases := map[int]bool{200: false, 400: false, 429: true, 500: true, 502: true, 503: true}
	for code, want := range cases {
		if got := isRetryable(code); got != want {
			t.Errorf("isRetryable(%d) = %v, want %v", code, got, want)
		}
	}
}

func TestPcmToWAV_Header(t *testing.T) {
	pcm := make([]byte, 48000) // 1 second of 24kHz 16-bit mono
	wav := pcmToWAV(pcm)

	// Check RIFF header
	if string(wav[:4]) != "RIFF" {
		t.Error("missing RIFF header")
	}
	if string(wav[8:12]) != "WAVE" {
		t.Error("missing WAVE format")
	}
	// WAV = 44 byte header + pcm data
	if len(wav) != 44+len(pcm) {
		t.Errorf("WAV size: got %d, want %d", len(wav), 44+len(pcm))
	}
}

func TestAudioDuration(t *testing.T) {
	// 24000 Hz * 2 bytes * 1 channel = 48000 bytes/sec
	pcm := make([]byte, 48000*5) // 5 seconds
	duration := len(pcm) / (pcmSampleRate * pcmChannels * pcmBitsPerSample / 8)
	if duration != 5 {
		t.Errorf("duration: got %d, want 5", duration)
	}
}
