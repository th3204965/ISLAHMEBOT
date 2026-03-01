# 🕌 IslahmBot — Islamic Voice Assistant

A voice-in/voice-out Telegram bot that answers Islamic questions. Send a voice message in Hindi, Urdu, or English — get a spoken Hindi response grounded in Quran and Sahih Hadith.

## How It Works

```
Voice Message → Groq Whisper STT → Gemini 2.5 Flash (text) → Gemini TTS (speech) → Audio Response
```

1. **Transcription** — [Groq Whisper](https://groq.com/) (`whisper-large-v3`) converts the voice message to text
2. **Answer Generation** — [Gemini 2.5 Flash](https://ai.google.dev/) generates a concise Hindi answer
3. **Text-to-Speech** — [Gemini TTS](https://ai.google.dev/gemini-api/docs/speech-generation) converts the answer to audio
4. **Delivery** — Audio is sent back as a Telegram audio message with correct title and duration

If audio generation fails, the bot automatically falls back to a text response.

## Architecture

```
islahmebot/
├── function.go              # Cloud Function entry point
├── cmd/local/main.go        # Local development server
├── deploy.sh                # One-command GCP deployment
├── gemini/
│   └── llm.go               # Gemini text + TTS with retry logic
├── groq/
│   └── stt.go               # Groq Whisper speech-to-text
└── telegram/
    ├── client.go             # Telegram API (SendAudio, SendMessage, TypingLoop)
    ├── models.go             # Telegram data types
    └── webhook.go            # Webhook handler + voice message processing
```

### Key Design Decisions

- **Zero-Disk I/O** — All audio streams via `io.Pipe`. No temp files, no disk writes.
- **Continuous Typing Indicator** — A background goroutine pings Telegram every 4s to keep the "recording voice" indicator alive during processing.
- **Retry with Backoff** — Gemini API calls retry 3 times on transient errors (500/502/503/429) with exponential backoff (500ms → 1s → 2s).
- **Three-Level Fallback** — Voice → text (if TTS fails) → full text regeneration (if everything fails).
- **WAV Output** — Gemini TTS outputs raw PCM (s16le, 24kHz, mono). We prepend a 44-byte WAV header and send via Telegram's `sendAudio` with proper title/duration metadata.

## Prerequisites

- Go 1.21+
- [Telegram Bot Token](https://core.telegram.org/bots#botfather) from BotFather
- [Groq API Key](https://console.groq.com/)
- [Gemini API Key](https://aistudio.google.com/apikey)
- [Google Cloud SDK](https://cloud.google.com/sdk) (`gcloud`) for deployment

## Setup

```bash
git clone <repository_url>
cd islahmebot
cp .env.example .env
```

Edit `.env`:
```env
TELEGRAM_BOT_TOKEN=your_token
GROQ_API_KEY=your_key
GEMINI_API_KEY=your_key
```

## Local Development

```bash
# Start the server (port 8080)
go run cmd/local/main.go

# Expose via ngrok
ngrok http 8080

# Set webhook to ngrok URL
curl "https://api.telegram.org/bot<TOKEN>/setWebhook?url=<NGROK_URL>"
```

## Deploy to GCP

```bash
./deploy.sh
```

Deploys as a 2nd Gen Cloud Function to `europe-west4` and configures the Telegram webhook automatically.

## Run Tests

```bash
go test ./... -v
```

## Models Used

| Service | Model | Purpose |
|---------|-------|---------|
| Groq | `whisper-large-v3` | Speech-to-text |
| Gemini | `gemini-2.5-flash` | Text answer generation |
| Gemini | `gemini-2.5-flash-preview-tts` | Text-to-speech (Kore voice) |

## License

MIT
