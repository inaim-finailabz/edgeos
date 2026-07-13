#!/usr/bin/env bash
# Hits a running llama-server's native /completion endpoint and extracts
# measured tok/s from its "timings" block — the same mechanism the agent's
# load-time benchmark (docs/CAPABILITY_SCHEMA.md) uses to fill tok_per_sec.
set -euo pipefail

URL="http://localhost:8080"
N_PREDICT=50
PROMPT="The quick brown fox jumps over the lazy dog. Tell me a short story about"

usage() {
	echo "Usage: $0 [-u llama-server URL] [-n n_predict] [-p prompt]" >&2
	exit 1
}

while getopts "u:n:p:h" opt; do
	case "$opt" in
	u) URL="$OPTARG" ;;
	n) N_PREDICT="$OPTARG" ;;
	p) PROMPT="$OPTARG" ;;
	*) usage ;;
	esac
done

if ! command -v jq >/dev/null 2>&1; then
	echo "error: jq is required (brew install jq / apt install jq)" >&2
	exit 1
fi

response=$(curl -sS "$URL/completion" \
	-H "Content-Type: application/json" \
	-d "$(jq -n --arg prompt "$PROMPT" --argjson n_predict "$N_PREDICT" \
		'{prompt: $prompt, n_predict: $n_predict, stream: false}')")

echo "$response" | jq -r '
  .timings as $t |
  "prompt_n=\($t.prompt_n) prompt_tok_per_sec=\($t.prompt_per_second)\n" +
  "predicted_n=\($t.predicted_n) predicted_tok_per_sec=\($t.predicted_per_second)"
'
