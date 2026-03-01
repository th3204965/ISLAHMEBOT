# Telegram Islamic Voice Assistant (Ultra-Low Latency)

A lightning-fast, voice-in/voice-out Telegram bot designed as a dedicated Islamic assistant. It utilizes an extreme low-latency strategy with Zero-Disk I/O through Go streams.

## Features
- **Serverless Architecture:** Designed to run on Google Cloud Functions (2nd Gen).
- **Zero-Disk I/O:** Uses Go's `io.Pipe` and `io.Copy` to stream data directly between HTTP requests without saving to disk.
- **Speech-to-Text (STT):** Integrates with Groq's high-speed Whisper (`whisper-large-v3`) API.
- **Multimodal AI:** Utilizes Gemini 2.5 Flash (`gemini-2.5-flash`) via Vertex/Google AI Studio to directly generate streaming audio (using the "Kore" voice profile).
- **Islamic Persona:** Grounded in Quran and Sahih Hadith, providing warm, respectful responses while deferring complex fatwas.

## Prerequisites
- Go 1.21 or higher
- A Telegram Bot Token from BotFather
- A Groq API Key
- A Gemini API Key
- Google Cloud SDK (`gcloud`) CLI installed and configured (for deployment)

## Setup

1. **Clone the repository:**
   ```bash
   git clone <repository_url>
   cd islahmebot
   ```

2. **Configure Environment Variables:**
   Copy the example environment file:
   ```bash
   cp .env.example .env
   ```
   Edit `.env` and fill in your actual API keys:
   ```env
   TELEGRAM_BOT_TOKEN=your_telegram_bot_token_here
   GROQ_API_KEY=your_groq_api_key_here
   GEMINI_API_KEY=your_gemini_api_key_here
   ```

## Local Development & Testing

You can run the bot locally using the provided entry point mapping the Cloud Function logic to a standard `net/http` server.

1. **Run the local server:**
   ```bash
   go run cmd/local/main.go
   ```
   *The server defaults to port `8080`.*

2. **Expose locally using ngrok (or similar):**
   ```bash
   ngrok http 8080
   ```

3. **Set your Telegram Webhook:**
   Execute an HTTP GET replacing your tokens:
   ```bash
   curl "https://api.telegram.org/bot<TELEGRAM_BOT_TOKEN>/setWebhook?url=<NGROK_URL>"
   ```

## Deployment

The provided `deploy.sh` script deploys the application as a 2nd Gen Google Cloud Function to the `europe-west4` region (low latency to Telegram servers) and allows unauthenticated HTTP requests.

1. **Deploy to GCP:**
   ```bash
   ./deploy.sh
   ```

2. **Update Telegram Webhook to Cloud Function URL:**
   Once deployed, `gcloud` will output the Cloud Function endpoint. Set this URL for your bot:
   ```bash
   curl "https://api.telegram.org/bot<TELEGRAM_BOT_TOKEN>/setWebhook?url=<CLOUD_FUNCTION_URL>"
   ```
