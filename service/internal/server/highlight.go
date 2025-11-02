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
    words := strings.Fields(snippet)
    if len(words) == 0 {
        return snippet
    }
    // Normalize/stem each word similar to the indexer
    wordTok := make([]string, len(words))
    for i, w := range words {
        toks := searchindex.TokenizePublic(w)
        if len(toks) > 0 {
            wordTok[i] = toks[0]
        } else {
            wordTok[i] = ""
        }
    }
    // Query tokens and phrases (tokenized)
    qToks := searchindex.TokenizePublic(q)
    phrases := searchindex.ExtractPhrasesPublic(q)
    phToks := make([][]string, 0, len(phrases))
    for _, ph := range phrases {
        if strings.TrimSpace(ph) == "" {
            continue
        }
        toks := searchindex.TokenizePublic(ph)
        if len(toks) > 0 {
            phToks = append(phToks, toks)
        }
    }
    // Compute a window around the first phrase/token hit to keep snippets compact
    firstHit := -1
    marked := make([]bool, len(words))
    // Phrase matches first (exact token sequence)
    for _, seq := range phToks {
        L := len(seq)
        if L == 0 {
            continue
        }
        for i := 0; i+L <= len(wordTok); i++ {
            ok := true
            for k := 0; k < L; k++ {
                if wordTok[i+k] == "" || wordTok[i+k] != seq[k] {
                    ok = false
                    break
                }
            }
            if ok {
                for k := 0; k < L; k++ {
                    marked[i+k] = true
                }
                if firstHit == -1 {
                    firstHit = i
                }
            }
        }
    }
    // Single-token matches on word boundaries
    if len(qToks) > 0 {
        for i, wt := range wordTok {
            if wt == "" {
                continue
            }
            for _, qt := range qToks {
                if qt != "" && wt == qt {
                    marked[i] = true
                    if firstHit == -1 {
                        firstHit = i
                    }
                    break
                }
            }
        }
    }
    if firstHit == -1 {
        firstHit = 0
    }
    // Build a slice around the hit
    start := firstHit - 12
    if start < 0 {
        start = 0
    }
    end := start + 24
    if end > len(words) {
        end = len(words)
    }
    // Emit with placeholders around marked words
    const mkStart = "\u0000MK_S\u0000"
    const mkEnd = "\u0000MK_E\u0000"
    outWords := make([]string, 0, end-start)
    for i := start; i < end; i++ {
        w := words[i]
        if marked[i] {
            outWords = append(outWords, mkStart+w+mkEnd)
        } else {
            outWords = append(outWords, w)
        }
    }
    s := strings.Join(outWords, " ")
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
