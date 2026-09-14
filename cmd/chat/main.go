package main

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gogen-rag/internal/chat"
	"gogen-rag/internal/config"
	"gogen-rag/internal/ingest"
	"gogen-rag/internal/ollama"
	"gogen-rag/internal/store"
)

//go:embed web
var webFS embed.FS

const maxUploadBytes = 10 << 20

func main() {
	os.Exit(run())
}

func run() int {
	serve := false
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "-serve" {
		serve = true
		args = args[1:]
	}

	app, cleanup, err := newApp()
	if err != nil {
		fmt.Println(err)
		return 1
	}
	defer cleanup()

	if serve {
		if err := serveChat(app); err != nil {
			fmt.Println("server error:", err)
			return 1
		}
		return 0
	}

	if len(args) > 0 {
		return oneShot(app.agent, strings.Join(args, " "))
	}
	return repl(app.agent)
}

type app struct {
	cfg   config.Config
	db    *store.Store
	llm   *ollama.Client
	agent *chat.Agent
}

func newApp() (*app, func(), error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("config error: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	db, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("database error: %w", err)
	}

	httpClient := &http.Client{Timeout: 5 * time.Minute}
	llm := ollama.New(cfg.OllamaURL, httpClient)
	return &app{
		cfg:   cfg,
		db:    db,
		llm:   llm,
		agent: chat.New(cfg, llm, db),
	}, db.Close, nil
}

func printTurn(turn chat.Turn) {
	if turn.Type == "tool_call" {
		out, err := json.MarshalIndent(turn, "", "  ")
		if err != nil {
			fmt.Println("tool_call:", turn.ToolCalls)
			return
		}
		fmt.Println(string(out))
		return
	}
	fmt.Println(turn.Reply)
}

func oneShot(agent *chat.Agent, question string) int {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	turn, err := agent.Reply(ctx, chat.Request{Message: question})
	if err != nil {
		fmt.Println("chat error:", err)
		return 1
	}
	printTurn(turn)
	return 0
}

func repl(agent *chat.Agent) int {
	fmt.Println("gogen-rag chat. Type quit to exit.")
	in := bufio.NewScanner(os.Stdin)
	var history []ollama.Message
	for {
		fmt.Print("you> ")
		if !in.Scan() {
			break
		}
		line := strings.TrimSpace(in.Text())
		if line == "" {
			continue
		}
		if line == "quit" || line == "exit" {
			return 0
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		turn, err := agent.Reply(ctx, chat.Request{Message: line, History: history})
		cancel()
		if err != nil {
			fmt.Println("chat error:", err)
			continue
		}
		history = turn.History
		if turn.Type == "tool_call" {
			fmt.Println("bot> host must call this tool and POST tool_results to /api/chat")
		} else {
			fmt.Print("bot> ")
		}
		printTurn(turn)
	}
	if err := in.Err(); err != nil {
		fmt.Println("input error:", err)
		return 1
	}
	return 0
}

type chatRequest struct {
	Message     string            `json:"message"`
	History     []ollama.Message  `json:"history"`
	ToolResults []chat.ToolResult `json:"tool_results"`
}

type apiError struct {
	Error string `json:"error"`
}

func serveChat(a *app) error {
	static, err := fs.Sub(webFS, "web")
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServerFS(static))
	mux.HandleFunc("GET /api/documents", a.handleDocuments)
	mux.HandleFunc("POST /api/ingest", a.handleIngest)
	mux.HandleFunc("POST /api/chat", a.handleChat)

	fmt.Println("chat UI: http://localhost" + a.cfg.ChatAddr)
	return http.ListenAndServe(a.cfg.ChatAddr, mux)
}

func (a *app) handleDocuments(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	docs, err := a.db.ListSources(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	if docs == nil {
		docs = []store.Source{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"documents": docs})
}

func (a *app) handleIngest(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "file is missing or larger than 10MB"})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "choose a .pdf, .md, or .txt file"})
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "could not read upload"})
		return
	}

	name := filepath.Base(header.Filename)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	result, err := ingest.File(ctx, a.db, a.llm, a.cfg.EmbedModel, name, data)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *app) handleChat(w http.ResponseWriter, r *http.Request) {
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid json"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	turn, err := a.agent.Reply(ctx, chat.Request{
		Message:     req.Message,
		History:     req.History,
		ToolResults: req.ToolResults,
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, turn)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
