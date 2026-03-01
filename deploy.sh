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

echo "Deploying Cloud Function (2nd Gen) 'islahmebot'..."

gcloud functions deploy islahmebot \
  --gen2 \
  --runtime=go121 \
  --region=europe-west4 \
  --source=. \
  --entry-point=MainHandler \
  --trigger-http \
  --allow-unauthenticated \
  --set-env-vars="TELEGRAM_BOT_TOKEN=${TELEGRAM_BOT_TOKEN},GROQ_API_KEY=${GROQ_API_KEY},GEMINI_API_KEY=${GEMINI_API_KEY}"

echo "Deployment finished."
