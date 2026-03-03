package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/th3204965/islahmebot/httpclient"
)

var telegramBaseURL = "https://api.telegram.org"

// TypingLoop keeps a chat action indicator alive until stopped.
type TypingLoop struct {
	stop chan struct{}
}

// StartTypingLoop sends the given action immediately and then every 4 seconds.
// Telegram indicators expire after ~5s, so 4s keeps them continuously visible.
func StartTypingLoop(chatID int64, action string) *TypingLoop {
	loop := &TypingLoop{stop: make(chan struct{})}
	sendChatAction(chatID, action)

	go func() {
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-loop.stop:
				return
			case <-ticker.C:
				sendChatAction(chatID, action)
			}
		}
	}()
	return loop
}

// Stop terminates the typing loop.
func (t *TypingLoop) Stop() {
	select {
	case <-t.stop:
	default:
		close(t.stop)
	}
}

// SendVoice sends audio via Telegram's sendVoice API (shows as voice bubble with waveform).
// The duration parameter ensures the correct length is displayed.
func SendVoice(ctx context.Context, chatID int64, audioReader io.Reader, durationSec int) error {
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		defer writer.Close()

		writer.WriteField("chat_id", fmt.Sprintf("%d", chatID))
		writer.WriteField("duration", fmt.Sprintf("%d", durationSec))

		part, err := writer.CreateFormFile("voice", "voice.ogg")
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		if _, err := io.Copy(part, audioReader); err != nil {
			pw.CloseWithError(err)
			return
		}
	}()

	apiURL := fmt.Sprintf("%s/bot%s/sendVoice", telegramBaseURL, os.Getenv("TELEGRAM_BOT_TOKEN"))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, pr)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := (&http.Client{Timeout: 0}).Do(req)
	if err != nil {
		return fmt.Errorf("failed to send voice: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram error %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// SendMessage sends a text message.
func SendMessage(ctx context.Context, chatID int64, text string) error {
	apiURL := fmt.Sprintf("%s/bot%s/sendMessage", telegramBaseURL, os.Getenv("TELEGRAM_BOT_TOKEN"))
	data := url.Values{"chat_id": {fmt.Sprintf("%d", chatID)}, "text": {text}}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpclient.Shared.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram error %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// GetFileURL fetches the download URL for a Telegram file ID.
func GetFileURL(ctx context.Context, fileID string) (string, error) {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	apiURL := fmt.Sprintf("%s/bot%s/getFile?file_id=%s", telegramBaseURL, token, fileID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := httpclient.Shared.Do(req)
	if err != nil {
		return "", fmt.Errorf("getFile failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("getFile status %d", resp.StatusCode)
	}

	var r FileResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", fmt.Errorf("failed to decode getFile: %w", err)
	}
	if !r.Ok {
		return "", fmt.Errorf("getFile not ok")
	}

	return fmt.Sprintf("%s/file/bot%s/%s", telegramBaseURL, token, r.Result.FilePath), nil
}

func sendChatAction(chatID int64, action string) {
	apiURL := fmt.Sprintf("%s/bot%s/sendChatAction", telegramBaseURL, os.Getenv("TELEGRAM_BOT_TOKEN"))
	resp, err := (&http.Client{Timeout: 5 * time.Second}).PostForm(apiURL,
		url.Values{"chat_id": {fmt.Sprintf("%d", chatID)}, "action": {action}})
	if err != nil {
		slog.Error("sendChatAction failed", "component", "telegram", "error", err)
		return
	}
	resp.Body.Close()
}
