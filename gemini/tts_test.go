package gemini

import (
	"bytes"
	"net/http"
	"testing"
)

func TestSanitizeForTTS(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no ligatures",
			input:    "Hello world",
			expected: "Hello world",
		},
		{
			name:     "contains Sallallahu Alaihi Wasallam",
			input:    "प्यारे नबी ﷺ की सुन्नत है",
			expected: "प्यारे नबी सल्लल्लाहु अलैहि वसल्लम की सुन्नत है",
		},
		{
			name:     "contains Jalla Jalaluhu",
			input:    "अल्लाह ﷻ",
			expected: "अल्लाह जल्ल जलालुहू",
		},
		{
			name:     "contains Bismillah",
			input:    "शुरू करते हैं ﷽",
			expected: "शुरू करते हैं बिस्मिल्लाहिर्रहमानिर्रहीम",
		},
		{
			name:     "multiple ligatures",
			input:    "﷽ अल्लाह ﷻ और नबी ﷺ",
			expected: "बिस्मिल्लाहिर्रहमानिर्रहीम अल्लाह जल्ल जलालुहू और नबी सल्लल्लाहु अलैहि वसल्लम",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := sanitizeForTTS(tc.input)
			if result != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestPCMToWAV(t *testing.T) {
	// A small dummy PCM buffer
	pcm := []byte{0x00, 0x01, 0x02, 0x03}
	wav := pcmToWAV(pcm)

	// WAV header is 44 bytes
	if len(wav) != 44+len(pcm) {
		t.Fatalf("expected length %d, got %d", 44+len(pcm), len(wav))
	}

	// Check RIFF header
	if string(wav[0:4]) != "RIFF" {
		t.Errorf("expected RIFF, got %s", string(wav[0:4]))
	}

	// Check WAVE format
	if string(wav[8:12]) != "WAVE" {
		t.Errorf("expected WAVE, got %s", string(wav[8:12]))
	}

	// Check data chunk matches original pcm
	if !bytes.Equal(wav[44:], pcm) {
		t.Errorf("pcm data mismatch in WAV file")
	}
}

func TestIsRetryable(t *testing.T) {
	retryableCodes := []int{http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout}
	for _, code := range retryableCodes {
		if !isRetryable(code) {
			t.Errorf("expected status %d to be retryable", code)
		}
	}

	nonRetryableCodes := []int{http.StatusOK, http.StatusBadRequest, http.StatusUnauthorized, http.StatusNotFound}
	for _, code := range nonRetryableCodes {
		if isRetryable(code) {
			t.Errorf("expected status %d to NOT be retryable", code)
		}
	}
}
