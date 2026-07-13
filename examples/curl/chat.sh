#!/usr/bin/env bash
# Sample EdgeOS client: plain curl against the OpenAI-compatible endpoint.
# Usage: ./chat.sh <model-id> "your prompt" [router-url]
set -euo pipefail

MODEL="${1:?usage: chat.sh <model-id> \"prompt\" [router-url]}"
PROMPT="${2:?usage: chat.sh <model-id> \"prompt\" [router-url]}"
ROUTER="${3:-http://localhost:8081}"

curl -sN "$ROUTER/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d "$(jq -n --arg model "$MODEL" --arg prompt "$PROMPT" \
        '{model: $model, messages: [{role: "user", content: $prompt}], stream: true, max_tokens: 300}')" \
  | while IFS= read -r line; do
      case "$line" in
        "data: [DONE]") break ;;
        data:\ *)
          # Reasoning models stream chain-of-thought via reasoning_content
          # before/instead of content -- print whichever is present.
          piece=$(echo "${line#data: }" | jq -r '.choices[0].delta.content // .choices[0].delta.reasoning_content // empty')
          printf '%s' "$piece"
          ;;
      esac
    done
echo
