package chat

import (
	"encoding/json"
	"testing"

	"gogen-rag/internal/ollama"
)

func TestLooksLikeUserLookup(t *testing.T) {
	t.Parallel()

	if !looksLikeUserLookup("Get details for user 1") {
		t.Fatal("expected user lookup")
	}
	if !looksLikeUserLookup("show customer 2 email") {
		t.Fatal("expected customer lookup")
	}
	if !looksLikeUserLookup("look up ada@acme.com") {
		t.Fatal("expected email lookup")
	}
	if looksLikeUserLookup("What is the capital of France?") {
		t.Fatal("generic question should not enable get_user")
	}
}

func TestExportToolCalls(t *testing.T) {
	t.Parallel()

	args, err := json.Marshal(map[string]string{"email": "ada@acme.com"})
	if err != nil {
		t.Fatal(err)
	}

	got := exportToolCalls([]ollama.ToolCall{{
		Function: ollama.ToolCallFunction{Name: "get_user", Arguments: args},
	}})
	if len(got) != 1 || got[0].Name != "get_user" || got[0].Arguments["email"] != "ada@acme.com" {
		t.Fatalf("unexpected export: %+v", got)
	}
	if got[0].ID != "call_1" {
		t.Fatalf("id = %q", got[0].ID)
	}
}
