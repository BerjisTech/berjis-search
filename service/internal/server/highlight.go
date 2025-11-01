package server

import (
	htmlpkg "html"
	"strings"

	"github.com/berjistech/berjis-ecosystem/search/service/internal/searchindex"
)

// highlight applies simple <mark> tags for query terms and phrases in a snippet.
func highlight(snippet string, q string) string {
	if strings.TrimSpace(snippet) == "" {
		return snippet
	}
	toks := searchindex.TokenizePublic(q)
	phrases := searchindex.ExtractPhrasesPublic(q)
	words := strings.Fields(snippet)
	hitIdx := -1
	lowerSnippet := strings.ToLower(snippet)
	for _, ph := range phrases {
		if ph == "" {
			continue
		}
		if strings.Contains(lowerSnippet, strings.ToLower(ph)) {
			for i, w := range words {
				if strings.Contains(strings.ToLower(w), strings.ToLower(ph)) {
					hitIdx = i
					break
				}
			}
			if hitIdx != -1 {
				break
			}
		}
	}
	if hitIdx == -1 {
		for i, w := range words {
			lw := strings.ToLower(w)
			for _, t := range toks {
				if t != "" && strings.Contains(lw, t) {
					hitIdx = i
					break
				}
			}
			if hitIdx != -1 {
				break
			}
		}
	}
	start := 0
	if hitIdx > 0 {
		start = hitIdx - 12
		if start < 0 {
			start = 0
		}
	}
	end := len(words)
	if start+24 < end {
		end = start + 24
	}
	slice := words[start:end]
	s := strings.Join(slice, " ")
	// Mark placeholders, then escape HTML, then replace placeholders with tags
	const mkStart = "\u0000MK_S\u0000"
	const mkEnd = "\u0000MK_E\u0000"
	for _, ph := range phrases {
		if ph != "" {
			s = strings.ReplaceAll(s, ph, mkStart+ph+mkEnd)
		}
	}
	for _, tok := range toks {
		if tok != "" {
			s = strings.ReplaceAll(s, tok, mkStart+tok+mkEnd)
		}
	}
	// Decode any existing HTML entities in snippet before escaping to avoid double-encoding
	s = htmlpkg.UnescapeString(s)
	s = escapeHTML(s)
	s = strings.ReplaceAll(s, escapeHTML(mkStart), "<mark>")
	s = strings.ReplaceAll(s, escapeHTML(mkEnd), "</mark>")
	if start > 0 {
		s = ". " + s
	}
	if end < len(words) {
		s = s + " ."
	}
	return s
}

func escapeHTML(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&#39;",
	)
	return r.Replace(s)
}
