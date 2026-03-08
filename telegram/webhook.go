package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/th3204965/islahmebot/gemini"
	"github.com/th3204965/islahmebot/groq"
	"github.com/th3204965/islahmebot/httpclient"
)

// HandleWebhook is the HTTP handler for Telegram webhooks.
func HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var update Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		slog.Error("Error decoding update", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if update.Message == nil || update.Message.Voice == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 55 second strict timeout logic. Cloud Run drops the connection at 60s.
	// We want to force-cancel outgoing API requests slightly before the infrastructure executes a SIGKILL
	// to allow graceful error logging and text fallback attempts if possible.
	ctx, cancel := context.WithTimeout(r.Context(), 55*time.Second)
	defer cancel()

	processVoiceMessage(ctx, update.Message)
	w.WriteHeader(http.StatusOK)
}

func processVoiceMessage(ctx context.Context, msg *Message) {
	chatID := msg.Chat.ID
	tag := fmt.Sprintf("[chat:%d]", chatID)

	// Start continuous indicator — stays on until we're done
	indicator := StartTypingLoop(chatID, "record_voice")
	defer indicator.Stop()

	// 1: Get file download URL
	fileURL, err := GetFileURL(ctx, msg.Voice.FileID)
	if err != nil {
		slog.Error("Failed to get file URL", "chat", tag, "error", err)
		return
	}

	// 2: Download audio
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	audioResp, err := httpclient.Shared.Do(req)
	if err != nil {
		slog.Error("Download failed", "chat", tag, "error", err)
		return
	}
	defer audioResp.Body.Close()
	if audioResp.StatusCode != http.StatusOK {
		slog.Error("Download failed with bad status", "chat", tag, "status", audioResp.StatusCode)
		return
	}

	// 3: Transcribe via Groq STT
	slog.Info("Transcribing audio", "chat", tag, "duration", msg.Voice.Duration)
	text, err := groq.TranscribeAudio(ctx, audioResp.Body)
	if err != nil {
		slog.Error("Transcription failed", "chat", tag, "error", err)
		return
	}
	slog.Info("Transcription completed", "chat", tag, "text", text)

	// 4: Generate text response (Groq Llama 3) with Asynchronous TTS Streaming
	var audioChunks []chan []byte
	onSentence := func(sentence string) {
		ch := make(chan []byte, 1)
		audioChunks = append(audioChunks, ch)
		go func(s string, c chan []byte) {
			pcm, err := gemini.GeneratePCM(ctx, s)
			if err != nil {
				slog.Error("TTS stream chunk failed", "chat", tag, "error", err, "sentence", s)
			}
			c <- pcm
		}(sentence, ch)
	}

	answer, err := groq.GenerateTextStream(ctx, chatID, text, onSentence)
	if err != nil {
		slog.Error("Groq text generation failed", "chat", tag, "error", err)
		sendTextFallback(ctx, chatID, tag, "Maaf kijiye, mujhe samajhne mein dikkat hui.", text)
		return
	}
	slog.Info("Answer generated", "component", "groq", "answer", answer)

	// Wait for all audio chunk pipelines to complete, preserving original sentence order
	var finalPCM []byte
	for _, ch := range audioChunks {
		pcm := <-ch
		if len(pcm) > 0 {
			finalPCM = append(finalPCM, pcm...)
		}
	}

	if len(finalPCM) == 0 {
		slog.Error("All TTS chunks completely failed", "chat", tag)
		sendTextFallback(ctx, chatID, tag, answer, text)
		return
	}

	// 5: Transcode aggregated PCM to final Ogg Opus
	audio, err := gemini.EncodePCMToOggOpus(ctx, finalPCM)
	if err != nil {
		slog.Error("Final audio compression failed", "chat", tag, "error", err)
		sendTextFallback(ctx, chatID, tag, answer, text)
		return
	}

	// 6: Send audio to Telegram
	indicator.Stop() // stop "recording" before uploading
	if err := SendVoice(ctx, chatID, bytes.NewReader(audio.AudioData), audio.DurationSec); err != nil {
		slog.Error("Send audio failed", "chat", tag, "error", err)
		sendTextFallback(ctx, chatID, tag, answer, text)
		return
	}
	slog.Info("Audio sent successfully", "chat", tag, "duration", audio.DurationSec)
}

// sendTextFallback tries to send text when audio fails.
func sendTextFallback(ctx context.Context, chatID int64, tag, answer, originalText string) {
	// If we got an answer from text gen, send that
	if answer != "" {
		if err := SendMessage(ctx, chatID, answer); err != nil {
			slog.Error("Text fallback failed", "chat", tag, "error", err)
		} else {
			slog.Info("Text fallback sent", "chat", tag)
		}
		return
	}

	// Last resort: generate text-only
	slog.Info("Attempting last resort text generation", "chat", tag)
	textResp, err := groq.GenerateTextStream(ctx, chatID, originalText, func(string) {})
	if err != nil {
		slog.Error("All fallbacks failed", "chat", tag, "error", err)
		return
	}
	if err := SendMessage(ctx, chatID, textResp); err != nil {
		slog.Error("Send failed", "chat", tag, "error", err)
	}
}
