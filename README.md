# gogen-rag

Local chatbot: Llama in Docker and company docs in pgvector.

This is separate from `gogen-api`, which talks to remote models (Gemini / OpenAI).

The model can:

1. Answer general questions from its own knowledge
2. Answer from ingested company documents (RAG)
3. Return a `get_user` tool request when someone asks for live user details

This service does **not** call GetUser. The host (microservice 1) calls the user API, then posts the JSON back.

## Start services

```bash
docker compose up -d
docker compose exec ollama ollama pull llama3.2
docker compose exec ollama ollama pull nomic-embed-text
```

`usersvc` on `:8090` is a demo GetUser API for the host app (`GET /users/1`, `GET /users?email=ada@acme.com`). gogen-rag never calls it.

## Ingest a document

In the chat UI, drop a `.pdf`, `.md`, or `.txt` file on the left. Text-based PDFs work; scanned image-only PDFs do not.

Or from the CLI:

```bash
go run ./cmd/ingest data/sample-policy.md
go run ./cmd/ingest data/sample-policy.pdf
```

## Chat API

```bash
go run ./cmd/chat -serve
```

Open http://localhost:8080 — the page acts as microservice 1.

Contract for a real host:

```bash
# 1) ask
curl -s localhost:8080/api/chat -d '{"message":"Get details for user 1"}'
# → { "type": "tool_call", "tool_calls": [{ "name": "get_user", "arguments": { "user_id": "1" } }], "history": [...] }

# 2) host calls GetUser, then continues
curl -s localhost:8080/api/chat -d '{
  "history": [...from step 1...],
  "tool_results": [{ "id": "call_1", "name": "get_user", "content": "{\"id\":\"1\",\"name\":\"Ada Lovelace\"}" }]
}'
# → { "type": "message", "reply": "Ada Lovelace ..." }
```

A policy or general question returns `{ "type": "message", "reply": "..." }` immediately.

## CLI

```bash
go run ./cmd/ask "How many leave days do employees get?"
go run ./cmd/ask "Get details for user 1"   # prints the tool_call JSON
```
