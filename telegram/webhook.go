package telegram

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/user/islahmebot/gemini"
	"github.com/user/islahmebot/groq"
)

// HandleWebhook is the HTTP handler for Telegram Webhooks and Cloud Function entry
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

	// We only care about voice messages
	if update.Message == nil || update.Message.Voice == nil {
		// Respond 200 OK so Telegram doesn't retry
		w.WriteHeader(http.StatusOK)
		return
	}

	go processVoiceMessage(update.Message)

	// Acknowledge the webhook immediately so Telegram doesn't timeout
	w.WriteHeader(http.StatusOK)
}

func processVoiceMessage(msg *Message) {
	fileID := msg.Voice.FileID
	chatID := msg.Chat.ID

	// 1: Fetch the temporary file URL from Telegram
	fileURL, err := GetFileURL(fileID)
	if err != nil {
		log.Printf("Failed to get file URL: %v", err)
		return
	}

	// 2: Open an HTTP GET to the Telegram download URL
	audioResp, err := http.Get(fileURL)
	if err != nil {
		log.Printf("Failed to download audio from telegram: %v", err)
		return
	}
	defer audioResp.Body.Close()
	if audioResp.StatusCode != http.StatusOK {
		log.Printf("Failed to download audio, status %d", audioResp.StatusCode)
		return
	}

	// 3: Stream the Telegram audio directly to Groq STT
	transcriptionText, err := groq.TranscribeAudio(audioResp.Body)
	if err != nil {
		log.Printf("Groq transcription failed: %v", err)
		return
	}

	log.Printf("Transcription: %s", transcriptionText)

	// 4: Stream text through Gemini AI, which pipes audio out as `io.ReadCloser`
	audioStream, err := gemini.GenerateAudioStream(transcriptionText)
	if err != nil {
		log.Printf("Gemini audio generation failed: %v", err)
		return
	}
	defer audioStream.Close()

	// 5: Stream resulting audio stream back to Telegram
	if err := SendVoice(chatID, audioStream); err != nil {
		log.Printf("Failed to send voice to telegram: %v", err)
		return
	}

	log.Println("Voice message processed successfully.")
}
