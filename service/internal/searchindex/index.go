package searchindex

import (
    "sort"
    "strings"
)

type Doc struct {
    ID      string
    Title   string
    Url     string
    Snippet string
    Source  string
}

type Index struct {
    docs   map[string]Doc
    inv    map[string]map[string]int // token -> docID -> count
    total  int
}

func New() *Index {
    return &Index{docs: map[string]Doc{}, inv: map[string]map[string]int{}, total: 0}
}

func tokenize(s string) []string {
    // naive tokenization: split non-letters, lowercase, drop short tokens
    repl := func(r rune) rune {
        if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') { return r }
        return ' '
    }
    cleaned := strings.Map(repl, strings.ToLower(s))
    parts := strings.Fields(cleaned)
    out := make([]string, 0, len(parts))
    for _, p := range parts { if len(p) >= 2 { out = append(out, p) } }
    return out
}

func (ix *Index) Add(d Doc) {
    ix.docs[d.ID] = d
    ix.total++
    text := d.Title + " " + d.Snippet + " " + d.Url + " " + d.Source
    for _, tok := range tokenize(text) {
        m, ok := ix.inv[tok]
        if !ok { m = map[string]int{}; ix.inv[tok] = m }
        m[d.ID] = m[d.ID] + 1
    }
}

type Result struct {
    Doc   Doc
    Score float64
}

func (ix *Index) Search(q string, limit int) []Result {
    if limit <= 0 { limit = 10 }
    tokens := tokenize(q)
    if len(tokens) == 0 { return nil }
    scores := map[string]float64{}
    for _, t := range tokens {
        postings := ix.inv[t]
        for docID, tf := range postings {
            // simple tf score; could extend with idf and normalization
            scores[docID] += float64(tf)
        }
    }
    out := make([]Result, 0, len(scores))
    for id, sc := range scores {
        out = append(out, Result{Doc: ix.docs[id], Score: sc})
    }
    sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
    if len(out) > limit { out = out[:limit] }
    return out
}

