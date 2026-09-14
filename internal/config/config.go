package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	OllamaURL   string
	ChatModel   string
	EmbedModel  string
	DatabaseURL string
	ChatAddr    string
}

func Load() (Config, error) {
	loadDotEnv()

	cfg := Config{
		OllamaURL:   getenv("OLLAMA_URL", "http://localhost:11434"),
		ChatModel:   getenv("OLLAMA_CHAT_MODEL", "llama3.2"),
		EmbedModel:  getenv("OLLAMA_EMBED_MODEL", "nomic-embed-text"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		ChatAddr:    getenv("CHAT_ADDR", ":8080"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is not set")
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func loadDotEnv() {
	wd, err := os.Getwd()
	if err != nil {
		return
	}

	dir := wd
	for i := 0; i < 5; i++ {
		if applyDotEnv(filepath.Join(dir, ".env")) {
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return
		}
		dir = parent
	}
}

func applyDotEnv(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}

	return true
}
