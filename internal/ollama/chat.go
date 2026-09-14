package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type Message struct {
	Role      string     `json:"role"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	ToolName  string     `json:"tool_name,omitempty"`
}

type ToolCall struct {
	Type     string           `json:"type,omitempty"`
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
	Index     int             `json:"index,omitempty"`
}

type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
	Tools    []Tool    `json:"tools,omitempty"`
}

type chatResponse struct {
	Message Message `json:"message"`
	Error   string  `json:"error"`
}

func (c *Client) Chat(ctx context.Context, model string, messages []Message, tools []Tool) (Message, error) {
	raw, status, err := c.postJSON(ctx, "/api/chat", chatRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
		Tools:    tools,
	})
	if err != nil {
		return Message{}, fmt.Errorf("call Ollama chat: %w", err)
	}

	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return Message{}, fmt.Errorf("decode chat response: %w", err)
	}
	if parsed.Error != "" {
		return Message{}, fmt.Errorf("Ollama chat error: %s (try: docker exec -it gogen-rag-ollama-1 ollama pull %s)", parsed.Error, model)
	}
	if status < 200 || status >= 300 {
		return Message{}, fmt.Errorf("Ollama chat status %d: %s", status, string(raw))
	}

	msg := parsed.Message
	if len(msg.ToolCalls) == 0 {
		if calls := ParseToolCallsFromContent(msg.Content); len(calls) > 0 {
			msg.ToolCalls = calls
			msg.Content = ""
		}
	}
	return msg, nil
}

func ParseToolCallsFromContent(content string) []ToolCall {
	content = strings.TrimSpace(content)
	if content == "" || !strings.HasPrefix(content, "{") {
		return nil
	}

	var payload struct {
		Name       string          `json:"name"`
		Parameters json.RawMessage `json:"parameters"`
		Arguments  json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal([]byte(content), &payload); err != nil || payload.Name == "" {
		return nil
	}

	args := payload.Arguments
	if len(args) == 0 {
		args = payload.Parameters
	}
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}

	return []ToolCall{{
		Type: "function",
		Function: ToolCallFunction{
			Name:      payload.Name,
			Arguments: args,
		},
	}}
}

func ToolArgs(raw json.RawMessage) (map[string]string, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return map[string]string{}, nil
	}

	data := raw
	if len(data) > 0 && data[0] == '"' {
		var inner string
		if err := json.Unmarshal(data, &inner); err != nil {
			return nil, fmt.Errorf("decode tool arguments string: %w", err)
		}
		data = []byte(inner)
	}

	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, fmt.Errorf("decode tool arguments: %w", err)
	}

	out := make(map[string]string, len(object))
	for k, v := range object {
		out[k] = fmt.Sprint(v)
	}
	return out, nil
}
