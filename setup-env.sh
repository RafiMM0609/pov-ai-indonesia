#!/bin/bash
# Extract OPENROUTER_API_KEY from hermes .env and write to project .env
HERMES_ENV="/home/anton/.hermes/.env"
PROJECT_ENV="/home/anton/Koding/ngawur/pov-ai-indonesia/.env"

if [ ! -f "$HERMES_ENV" ]; then
    echo "Hermes .env not found"
    exit 1
fi

KEY=$(grep "^OPENROUTER_API_KEY=*** "$HERMES_ENV" | head -1 | sed 's/^OPENROUTER_API_KEY=*** | sed 's/^"//' | sed 's/"$//')

if [ -z "$KEY" ]; then
    echo "OPENROUTER_API_KEY not found in $HERMES_ENV"
    exit 1
fi

printf 'OPENROUTER_API_KEY=*** POV_AI_MODEL=openrouter/owl-alpha\nSCRAPE_INTERVAL=6\nPORT=8080\n' "$KEY" > "$PROJECT_ENV"

echo "Created $PROJECT_ENV (${#KEY} chars)"
