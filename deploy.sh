#!/bin/bash

# Exit on any error
set -e

# Path to .env file
ENV_FILE=".env"

if [ ! -f "$ENV_FILE" ]; then
    echo "Error: $ENV_FILE not found."
    echo "Please create a .env file from .env.example"
    exit 1
fi

echo "Loading variables from $ENV_FILE..."

# Parse the .env file ensuring to handle quotes and export them to local shell for flag building
export $(grep -v '^#' "$ENV_FILE" | xargs)

# Verify keys are loaded
if [ -z "$TELEGRAM_BOT_TOKEN" ] || [ -z "$GROQ_API_KEY" ] || [ -z "$GEMINI_API_KEY" ]; then
    echo "Error: Missing required environment variables in $ENV_FILE."
    exit 1
fi

echo "Deploying Cloud Run service 'islahmebot'..."

gcloud run deploy islahmebot \
  --source=. \
  --region=europe-west4 \
  --allow-unauthenticated \
  --set-env-vars="TELEGRAM_BOT_TOKEN=${TELEGRAM_BOT_TOKEN},GROQ_API_KEY=${GROQ_API_KEY},GEMINI_API_KEY=${GEMINI_API_KEY}" \
  --port=8080 \
  --memory=512Mi \
  --cpu=1 \
  --timeout=120 \
  --min-instances=0 \
  --max-instances=3 \
  --concurrency=80 \
  --execution-environment=gen2 \
  --cpu-boost \
  --labels=managed-by=gcloud,tier=backend \
  --liveness-probe=httpGet.path=/health,initialDelaySeconds=2,timeoutSeconds=5,periodSeconds=15 \
  --startup-probe=httpGet.path=/health,initialDelaySeconds=2,timeoutSeconds=5,periodSeconds=15

echo "Deployment finished."

echo "Fetching deployed Cloud Run URL..."
SERVICE_URL=$(gcloud run services describe islahmebot --region=europe-west4 --format="value(status.url)")
WEBHOOK_URL="${SERVICE_URL}/webhook"

echo "Setting Telegram Webhook to: $WEBHOOK_URL"
curl -s -F "url=${WEBHOOK_URL}" "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/setWebhook" | grep -q '"ok":true' && echo "Webhook linked successfully!" || echo "Failed to link webhook."
