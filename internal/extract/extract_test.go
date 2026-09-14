package extract

import (
	"os"
	"strings"
	"testing"
)

func TestTextMarkdown(t *testing.T) {
	t.Parallel()

	got, err := Text("policy.md", []byte("# Leave\n\nEmployees get 20 days."))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "20 days") {
		t.Fatalf("got %q", got)
	}
}

func TestTextRejectsUnknownType(t *testing.T) {
	t.Parallel()

	if _, err := Text("photo.png", []byte("x")); err == nil {
		t.Fatal("expected unsupported type")
	}
}

func TestTextPDF(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/sample-policy.pdf")
	if err != nil {
		t.Fatal(err)
	}
	got, err := Text("sample-policy.pdf", data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "20 days") {
		t.Fatalf("pdf text %q", got)
	}
}
