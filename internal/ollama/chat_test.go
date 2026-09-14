package ollama

import (
	"encoding/json"
	"testing"
)

func TestParseToolCallsFromContent(t *testing.T) {
	t.Parallel()

	calls := ParseToolCallsFromContent(`{"name":"get_user","parameters":{"user_id":"1"}}`)
	if len(calls) != 1 || calls[0].Function.Name != "get_user" {
		t.Fatalf("unexpected tool calls: %+v", calls)
	}

	args, err := ToolArgs(calls[0].Function.Arguments)
	if err != nil {
		t.Fatal(err)
	}
	if args["user_id"] != "1" {
		t.Fatalf("user_id = %q", args["user_id"])
	}
}

func TestToolArgsString(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(`{"user_id":"ada"}`)
	if err != nil {
		t.Fatal(err)
	}
	args, err := ToolArgs(raw)
	if err != nil {
		t.Fatal(err)
	}
	if args["user_id"] != "ada" {
		t.Fatalf("user_id = %q", args["user_id"])
	}
}
