package extract

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
)

func Text(filename string, data []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".md", ".txt":
		if !utf8.Valid(data) {
			return "", fmt.Errorf("%s is not valid UTF-8 text", filename)
		}
		text := strings.TrimSpace(string(data))
		if text == "" {
			return "", fmt.Errorf("no text found in %s", filename)
		}
		return text, nil
	case ".pdf":
		return pdfText(data)
	default:
		return "", fmt.Errorf("unsupported file type %q (use .pdf, .md, or .txt)", ext)
	}
}

func pdfText(data []byte) (string, error) {
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("read pdf: %w", err)
	}

	var b strings.Builder
	for i := 1; i <= reader.NumPage(); i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			return "", fmt.Errorf("read pdf page %d: %w", i, err)
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(text)
	}

	out := strings.TrimSpace(b.String())
	if out == "" {
		return "", fmt.Errorf("no extractable text in pdf (scanned image-only files are not supported)")
	}
	return out, nil
}
