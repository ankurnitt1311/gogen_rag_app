package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"gogen-rag/internal/chat"
	"gogen-rag/internal/config"
	"gogen-rag/internal/ollama"
	"gogen-rag/internal/store"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: go run ./cmd/ask \"your question\"")
		os.Exit(1)
	}

	question := strings.Join(os.Args[1:], " ")

	cfg, err := config.Load()
	if err != nil {
		fmt.Println("config error:", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	db, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Println("database error:", err)
		os.Exit(1)
	}
	defer db.Close()

	httpClient := &http.Client{Timeout: 5 * time.Minute}
	llm := ollama.New(cfg.OllamaURL, httpClient)
	agent := chat.New(cfg, llm, db)

	turn, err := agent.Reply(ctx, chat.Request{Message: question})
	if err != nil {
		fmt.Println("chat error:", err)
		os.Exit(1)
	}

	if turn.Type == "tool_call" {
		out, err := json.MarshalIndent(turn, "", "  ")
		if err != nil {
			fmt.Println("tool_call:", turn.ToolCalls)
			os.Exit(0)
		}
		fmt.Println(string(out))
		return
	}

	fmt.Println(turn.Reply)
}
