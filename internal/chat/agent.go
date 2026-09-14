package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"gogen-rag/internal/config"
	"gogen-rag/internal/ollama"
	"gogen-rag/internal/store"
)

var userLookupRe = regexp.MustCompile(`(?i)\b(user|customer|account|profile)\b|@`)

var getUserParams = json.RawMessage(`{
	"type": "object",
	"properties": {
		"user_id": {
			"type": "string",
			"description": "Numeric or username id, if the person gave one"
		},
		"email": {
			"type": "string",
			"description": "Email if they did not give an id, e.g. ada@acme.com"
		}
	}
}`)

type Request struct {
	Message     string
	History     []ollama.Message
	ToolResults []ToolResult
}

type Turn struct {
	Type      string           `json:"type"`
	Reply     string           `json:"reply,omitempty"`
	ToolCalls []ToolCall       `json:"tool_calls,omitempty"`
	History   []ollama.Message `json:"history"`
}

type ToolCall struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Arguments map[string]string `json:"arguments"`
}

type ToolResult struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

type Agent struct {
	cfg   config.Config
	llm   *ollama.Client
	db    *store.Store
	tools []ollama.Tool
}

func New(cfg config.Config, llm *ollama.Client, db *store.Store) *Agent {
	return &Agent{
		cfg: cfg,
		llm: llm,
		db:  db,
		tools: []ollama.Tool{{
			Type: "function",
			Function: ollama.ToolFunction{
				Name:        "get_user",
				Description: "Ask the host service to fetch live customer/user details. Use when the user asks for a specific person's account, email, plan, or profile. Pass user_id or email. Never invent user data. Do not call this for general or policy questions.",
				Parameters:  getUserParams,
			},
		}},
	}
}

func (a *Agent) Reply(ctx context.Context, req Request) (Turn, error) {
	question := strings.TrimSpace(req.Message)
	if question == "" && len(req.ToolResults) == 0 {
		return Turn{}, fmt.Errorf("empty question")
	}

	ragQuery := question
	if ragQuery == "" {
		ragQuery = lastUserText(req.History)
	}

	contextText, err := a.companyContext(ctx, ragQuery)
	if err != nil {
		return Turn{}, err
	}

	messages := []ollama.Message{{
		Role:    "system",
		Content: systemPrompt(contextText),
	}}
	messages = append(messages, modelHistory(req.History)...)

	if question != "" {
		messages = append(messages, ollama.Message{Role: "user", Content: question})
	}
	for _, result := range req.ToolResults {
		name := strings.TrimSpace(result.Name)
		if name == "" {
			name = "get_user"
		}
		messages = append(messages, ollama.Message{
			Role:     "tool",
			ToolName: name,
			Content:  result.Content,
		})
	}

	activeTools := a.tools
	if question != "" && !looksLikeUserLookup(question) && len(req.ToolResults) == 0 {
		activeTools = nil
	}

	msg, err := a.llm.Chat(ctx, a.cfg.ChatModel, messages, activeTools)
	if err != nil {
		return Turn{}, err
	}

	history := append([]ollama.Message{}, req.History...)
	if question != "" {
		history = append(history, ollama.Message{Role: "user", Content: question})
	}
	for _, result := range req.ToolResults {
		name := strings.TrimSpace(result.Name)
		if name == "" {
			name = "get_user"
		}
		history = append(history, ollama.Message{
			Role:     "tool",
			ToolName: name,
			Content:  result.Content,
		})
	}
	history = append(history, msg)

	if calls := exportToolCalls(msg.ToolCalls); len(calls) > 0 {
		return Turn{Type: "tool_call", ToolCalls: calls, History: history}, nil
	}

	answer := strings.TrimSpace(msg.Content)
	if answer == "" {
		answer = "I do not know."
	}
	return Turn{Type: "message", Reply: answer, History: history}, nil
}

func (a *Agent) companyContext(ctx context.Context, question string) (string, error) {
	if a.db == nil || strings.TrimSpace(question) == "" {
		return "", nil
	}

	queryVec, err := a.llm.Embed(ctx, a.cfg.EmbedModel, question)
	if err != nil {
		return "", fmt.Errorf("embed question: %w", err)
	}

	hits, err := a.db.Search(ctx, queryVec, 3)
	if err != nil {
		return "", fmt.Errorf("search chunks: %w", err)
	}

	var bits []string
	for _, hit := range hits {
		bits = append(bits, fmt.Sprintf("Source: %s\n%s", hit.Source, hit.Content))
	}
	return strings.Join(bits, "\n\n---\n\n"), nil
}

func exportToolCalls(calls []ollama.ToolCall) []ToolCall {
	var out []ToolCall
	for i, call := range calls {
		args, err := ollama.ToolArgs(call.Function.Arguments)
		if err != nil {
			args = map[string]string{"error": err.Error()}
		}
		out = append(out, ToolCall{
			ID:        fmt.Sprintf("call_%d", i+1),
			Name:      call.Function.Name,
			Arguments: args,
		})
	}
	return out
}

func systemPrompt(companyContext string) string {
	if strings.TrimSpace(companyContext) == "" {
		companyContext = "(no matching company documents)"
	}

	return fmt.Sprintf(`You are a company chatbot.

You can do three things:
1. Answer general questions using your own knowledge.
2. Use the company document context below when the question is about company policy or ingested docs.
3. When the user asks for a specific customer's or user's live details, request the get_user tool with user_id or email. The host application will call the real GetUser API and send the JSON back. Never invent emails, names, plans, or other account data.

If the company context is unrelated to the question, ignore it.
If a tool result says the user was not found, say so.

Company document context:
%s`, companyContext)
}

func modelHistory(history []ollama.Message) []ollama.Message {
	var out []ollama.Message
	for _, msg := range history {
		switch msg.Role {
		case "user", "assistant", "tool":
			if strings.TrimSpace(msg.Content) == "" && len(msg.ToolCalls) == 0 {
				continue
			}
			out = append(out, msg)
		}
	}
	return out
}

func lastUserText(history []ollama.Message) string {
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "user" && strings.TrimSpace(history[i].Content) != "" {
			return history[i].Content
		}
	}
	return ""
}

func looksLikeUserLookup(question string) bool {
	return userLookupRe.MatchString(question)
}
