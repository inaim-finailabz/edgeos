// Sample EdgeOS client in Go, stdlib only — no SDK dependency, matching
// examples/web/chat.html's philosophy (there's no single official OpenAI Go
// SDK the way there is for Python/Node, so this is a small reference
// implementation instead).
//
//	go run chat.go -model qwen3-1.7b-Q4_K_M.gguf "your prompt"
//
// -router defaults to http://localhost:8081.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
)

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model     string    `json:"model"`
	Messages  []message `json:"messages"`
	Stream    bool      `json:"stream"`
	MaxTokens int       `json:"max_tokens"`
}

type chunk struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"delta"`
	} `json:"choices"`
}

func main() {
	router := flag.String("router", "http://localhost:8081", "EdgeOS router base URL")
	model := flag.String("model", "", "model id as reported by GET /v0/nodes (required)")
	flag.Parse()
	prompt := strings.Join(flag.Args(), " ")

	if *model == "" {
		fmt.Fprintln(os.Stderr, "usage: chat -model <id> [-router <url>] \"prompt\"")
		os.Exit(1)
	}
	if prompt == "" {
		fmt.Fprintln(os.Stderr, "usage: chat -model <id> [-router <url>] \"prompt\"")
		os.Exit(1)
	}

	if err := ask(*router, *model, prompt); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Println()
}

func ask(router, model, prompt string) error {
	reqBody, err := json.Marshal(chatRequest{
		Model:     model,
		Messages:  []message{{Role: "user", Content: prompt}},
		Stream:    true,
		MaxTokens: 300,
	})
	if err != nil {
		return err
	}

	resp, err := http.Post(strings.TrimSuffix(router, "/")+"/v1/chat/completions", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var body bytes.Buffer
		body.ReadFrom(resp.Body)
		return fmt.Errorf("router returned %s: %s", resp.Status, body.String())
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			break
		}
		var c chunk
		if err := json.Unmarshal([]byte(payload), &c); err != nil {
			continue
		}
		if len(c.Choices) == 0 {
			continue
		}
		delta := c.Choices[0].Delta
		if delta.Content != "" {
			fmt.Print(delta.Content)
		} else if delta.ReasoningContent != "" {
			// Reasoning models stream chain-of-thought via this field
			// before/instead of content.
			fmt.Print(delta.ReasoningContent)
		}
	}
	return scanner.Err()
}
