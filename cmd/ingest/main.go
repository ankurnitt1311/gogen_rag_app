package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"gogen-rag/internal/config"
	"gogen-rag/internal/ingest"
	"gogen-rag/internal/ollama"
	"gogen-rag/internal/store"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: go run ./cmd/ingest <file.pdf|file.md|file.txt>")
		os.Exit(1)
	}

	path := os.Args[1]
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Println("read file:", err)
		os.Exit(1)
	}

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

	httpClient := &http.Client{Timeout: 2 * time.Minute}
	llm := ollama.New(cfg.OllamaURL, httpClient)

	result, err := ingest.File(ctx, db, llm, cfg.EmbedModel, path, data)
	if err != nil {
		fmt.Println("ingest error:", err)
		os.Exit(1)
	}

	fmt.Printf("stored %d chunk(s) from %s\n", result.Chunks, result.Source)
}
