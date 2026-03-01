package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/th3204965/islahmebot/gemini"
	"github.com/th3204965/islahmebot/groq"
)

// HandleWebhook is the HTTP handler for Telegram webhooks.
func HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var update Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		log.Printf("Error decoding update: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if update.Message == nil || update.Message.Voice == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	processVoiceMessage(update.Message)
	w.WriteHeader(http.StatusOK)
}

func processVoiceMessage(msg *Message) {
	chatID := msg.Chat.ID
	tag := fmt.Sprintf("[chat:%d]", chatID)

	// Start continuous indicator — stays on until we're done
	indicator := StartTypingLoop(chatID, "record_voice")
	defer indicator.Stop()

	// 1: Get file download URL
	fileURL, err := GetFileURL(msg.Voice.FileID)
	if err != nil {
		log.Printf("%s Failed to get file URL: %v", tag, err)
		return
	}

	// 2: Download audio
	dlClient := &http.Client{Timeout: 15 * time.Second}
	audioResp, err := dlClient.Get(fileURL)
	if err != nil {
		log.Printf("%s Download failed: %v", tag, err)
		return
	}
	defer audioResp.Body.Close()
	if audioResp.StatusCode != http.StatusOK {
		log.Printf("%s Download status %d", tag, audioResp.StatusCode)
		return
	}

	// 3: Transcribe via Groq STT
	log.Printf("%s Transcribing (%ds)...", tag, msg.Voice.Duration)
	text, err := groq.TranscribeAudio(audioResp.Body)
	if err != nil {
		log.Printf("%s Transcription failed: %v", tag, err)
		return
	}
	log.Printf("%s Transcription: %s", tag, text)

	// 4: Generate voice response (text → TTS)
	audio, answer, err := gemini.GenerateVoiceResponse(text)
	if err != nil {
		log.Printf("%s Voice pipeline failed: %v", tag, err)
		sendTextFallback(chatID, tag, answer, text)
		return
	}

	// 5: Send audio to Telegram
	indicator.Stop() // stop "recording" before uploading
	if err := SendVoice(chatID, bytes.NewReader(audio.WAVData), audio.DurationSec); err != nil {
		log.Printf("%s Send audio failed: %v", tag, err)
		sendTextFallback(chatID, tag, answer, text)
		return
	}
	log.Printf("%s Done (%ds audio)", tag, audio.DurationSec)
}

// sendTextFallback tries to send text when audio fails.
func sendTextFallback(chatID int64, tag, answer, originalText string) {
	// If we got an answer from text gen, send that
	if answer != "" {
		if err := SendMessage(chatID, answer); err != nil {
			log.Printf("%s Text fallback failed: %v", tag, err)
		} else {
			log.Printf("%s Text fallback sent", tag)
		}
		return
	}

	// Last resort: generate text-only
	log.Printf("%s Last resort text generation...", tag)
	textResp, err := gemini.GenerateTextResponse(originalText)
	if err != nil {
		log.Printf("%s All fallbacks failed: %v", tag, err)
		return
	}
	if err := SendMessage(chatID, textResp); err != nil {
		log.Printf("%s Send failed: %v", tag, err)
	}
}
