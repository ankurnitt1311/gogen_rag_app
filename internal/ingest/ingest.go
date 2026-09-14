package ingest

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"gogen-rag/internal/chunk"
	"gogen-rag/internal/extract"
	"gogen-rag/internal/ollama"
	"gogen-rag/internal/store"
)

type Result struct {
	Source string `json:"source"`
	Chunks int    `json:"chunks"`
}

func File(ctx context.Context, db *store.Store, llm *ollama.Client, embedModel, filename string, data []byte) (Result, error) {
	source := filepath.Base(strings.TrimSpace(filename))
	if source == "" || source == "." || source == "/" {
		return Result{}, fmt.Errorf("missing file name")
	}

	text, err := extract.Text(source, data)
	if err != nil {
		return Result{}, err
	}

	parts := chunk.Split(text, 800)
	if len(parts) == 0 {
		return Result{}, fmt.Errorf("no text found in %s", source)
	}

	if err := db.ReplaceSource(ctx, source); err != nil {
		return Result{}, err
	}

	for _, part := range parts {
		embedding, err := llm.Embed(ctx, embedModel, part)
		if err != nil {
			return Result{}, fmt.Errorf("embed chunk: %w", err)
		}
		if err := db.Insert(ctx, source, part, embedding); err != nil {
			return Result{}, err
		}
	}

	return Result{Source: source, Chunks: len(parts)}, nil
}
