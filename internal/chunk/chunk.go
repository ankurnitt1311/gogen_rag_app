package chunk

import "strings"

func Split(text string, maxChars int) []string {
	if maxChars <= 0 {
		maxChars = 800
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	paragraphs := strings.Split(text, "\n\n")
	var chunks []string
	var current strings.Builder

	flush := func() {
		part := strings.TrimSpace(current.String())
		if part != "" {
			chunks = append(chunks, part)
		}
		current.Reset()
	}

	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		if current.Len() == 0 {
			current.WriteString(p)
			if current.Len() >= maxChars {
				flush()
			}
			continue
		}

		if current.Len()+len(p)+2 > maxChars {
			flush()
			current.WriteString(p)
			continue
		}

		current.WriteString("\n\n")
		current.WriteString(p)
	}

	flush()
	return chunks
}
