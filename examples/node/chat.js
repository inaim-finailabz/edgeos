#!/usr/bin/env node
// Sample EdgeOS client using the official OpenAI Node SDK.
//
// Same point as examples/python/chat.py: no EdgeOS-specific code, just
// base_url pointed at the router.
//
//   npm install
//   node chat.js --model qwen3-1.7b-Q4_K_M.gguf "Say hello in exactly three words"
//
// --router defaults to http://localhost:8081/v1 (the SDK appends
// /chat/completions itself, same as it would for api.openai.com/v1).

import OpenAI from "openai";

function parseArgs(argv) {
  const args = { router: "http://localhost:8081/v1", model: null, prompt: null };
  const rest = [];
  for (let i = 0; i < argv.length; i++) {
    if (argv[i] === "--router") args.router = argv[++i];
    else if (argv[i] === "--model") args.model = argv[++i];
    else rest.push(argv[i]);
  }
  args.prompt = rest.join(" ") || null;
  return args;
}

async function ask(client, model, history, prompt) {
  history.push({ role: "user", content: prompt });
  const stream = await client.chat.completions.create({
    model,
    messages: history,
    stream: true,
    max_tokens: 300,
  });

  let reply = "";
  for await (const chunk of stream) {
    const delta = chunk.choices[0].delta;
    // Reasoning models stream chain-of-thought via a provider-specific
    // reasoning_content field alongside content.
    const piece = delta.content || delta.reasoning_content;
    if (piece) {
      process.stdout.write(piece);
      reply += piece;
    }
  }
  process.stdout.write("\n");
  history.push({ role: "assistant", content: reply });
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  if (!args.model) {
    console.error("usage: node chat.js --model <id> [--router <url>] [prompt]");
    process.exit(1);
  }

  const client = new OpenAI({ baseURL: args.router, apiKey: "unused-in-v0" });
  const history = [];

  if (args.prompt) {
    await ask(client, args.model, history, args.prompt);
    return;
  }

  console.log(`EdgeOS chat -- model=${args.model} router=${args.router} (Ctrl-D to exit)`);
  const readline = await import("node:readline/promises");
  const rl = readline.createInterface({ input: process.stdin, output: process.stdout });
  while (true) {
    const prompt = await rl.question("> ").catch(() => null);
    if (prompt === null) break;
    if (prompt.trim()) await ask(client, args.model, history, prompt);
  }
  rl.close();
}

main();
