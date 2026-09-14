package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func New(baseURL string, httpClient *http.Client) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
	}
}

type generateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type generateResponse struct {
	Response string `json:"response"`
	Error    string `json:"error"`
}

func (c *Client) postJSON(ctx context.Context, path string, payload any) ([]byte, int, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}
	return raw, resp.StatusCode, nil
}

func (c *Client) Generate(ctx context.Context, model, prompt string) (string, error) {
	raw, status, err := c.postJSON(ctx, "/api/generate", generateRequest{
		Model:  model,
		Prompt: prompt,
		Stream: false,
	})
	if err != nil {
		return "", fmt.Errorf("call Ollama generate: %w", err)
	}

	var parsed generateResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("decode generate response: %w", err)
	}

	if parsed.Error != "" {
		return "", fmt.Errorf("Ollama generate error: %s (try: docker exec -it gogen-rag-ollama-1 ollama pull %s)", parsed.Error, model)
	}
	if status < 200 || status >= 300 {
		return "", fmt.Errorf("Ollama generate status %d: %s", status, string(raw))
	}

	return parsed.Response, nil
}

type embedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type embedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
	Error      string      `json:"error"`
}

func (c *Client) Embed(ctx context.Context, model, text string) ([]float32, error) {
	body, err := json.Marshal(embedRequest{
		Model: model,
		Input: text,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal embed request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call Ollama embed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read embed response: %w", err)
	}

	var parsed embedResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode embed response: %w", err)
	}

	if parsed.Error != "" {
		return nil, fmt.Errorf("Ollama embed error: %s (try: docker exec -it gogen-rag-ollama-1 ollama pull %s)", parsed.Error, model)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Ollama embed status %d: %s", resp.StatusCode, string(raw))
	}
	if len(parsed.Embeddings) == 0 {
		return nil, fmt.Errorf("Ollama returned no embedding")
	}

	return parsed.Embeddings[0], nil
}
