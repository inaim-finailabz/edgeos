#!/usr/bin/env python3
"""
Sample EdgeOS client using the official OpenAI Python SDK.

The whole point of an "OpenAI-compatible endpoint" is that this needs
no EdgeOS-specific code at all -- just point the SDK's base_url at the
router. api_key is required by the SDK but ignored by the router itself
in v0 (no auth on the inference path).

    pip install openai
    python3 chat.py --model qwen3-1.7b-Q4_K_M.gguf "Say hello in exactly three words"

Run with no prompt for an interactive loop. --router defaults to
http://localhost:8081/v1 (the SDK appends /chat/completions itself, same
as it would for api.openai.com/v1); --model must match a model a
discovered node reports as "loaded" (see `edgeos nodes` or GET /v0/nodes).
"""
import argparse
import sys

from openai import OpenAI


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("prompt", nargs="?", help="single prompt; omit for an interactive loop")
    parser.add_argument("--router", default="http://localhost:8081/v1", help="EdgeOS router base URL, including /v1 (the SDK appends /chat/completions itself)")
    parser.add_argument("--model", required=True, help="model id as reported by GET /v0/nodes")
    args = parser.parse_args()

    client = OpenAI(base_url=args.router, api_key="unused-in-v0")

    def ask(prompt, history):
        history.append({"role": "user", "content": prompt})
        stream = client.chat.completions.create(
            model=args.model,
            messages=history,
            stream=True,
            max_tokens=300,
        )
        reply = ""
        for chunk in stream:
            delta = chunk.choices[0].delta
            # Reasoning models stream chain-of-thought via a
            # provider-specific reasoning_content field alongside content.
            piece = getattr(delta, "content", None) or getattr(delta, "reasoning_content", None)
            if piece:
                print(piece, end="", flush=True)
                reply += piece
        print()
        history.append({"role": "assistant", "content": reply})

    history = []
    if args.prompt:
        ask(args.prompt, history)
        return

    print(f"EdgeOS chat -- model={args.model} router={args.router} (Ctrl-D to exit)")
    while True:
        try:
            prompt = input("> ")
        except EOFError:
            print()
            break
        if prompt.strip():
            ask(prompt, history)


if __name__ == "__main__":
    sys.exit(main())
