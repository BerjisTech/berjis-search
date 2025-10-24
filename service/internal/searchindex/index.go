package searchindex

import (
    "encoding/json"
    "math"
    "net/url"
    "os"
    "sort"
    "strings"
    "time"
)

type Doc struct {
    ID      string
    Title   string
    Url     string
    Snippet string
    Source  string
    Date    string // RFC3339 preferred
}

type Index struct {
    docs   map[string]Doc
    titlePos   map[string]map[string][]int // token -> docID -> positions in title
    snippetPos map[string]map[string][]int // token -> docID -> positions in snippet
    total  int
}

func New() *Index { return &Index{docs: map[string]Doc{}, titlePos: map[string]map[string][]int{}, snippetPos: map[string]map[string][]int{}, total: 0} }

var stopwords = map[string]struct{}{
    "the":{},"is":{},"at":{},"which":{},"on":{},"and":{},"a":{},"an":{},"for":{},"to":{},"in":{},"of":{},"with":{},"as":{},"by":{},"it":{},"this":{},"that":{},"be":{},"or":{},"not":{},
}

func tokenize(s string) []string {
    // naive tokenization: split non-letters, lowercase, drop short tokens/stopwords
    repl := func(r rune) rune {
        if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') { return r }
        return ' '
    }
    cleaned := strings.Map(repl, strings.ToLower(s))
    parts := strings.Fields(cleaned)
    out := make([]string, 0, len(parts))
    for _, p := range parts {
        if len(p) < 2 { continue }
        if _, stop := stopwords[p]; stop { continue }
        out = append(out, p)
    }
    return out
}
// Internal adapters for other packages
func TokenizePublic(s string) []string { return tokenize(s) }
func ExtractPhrasesPublic(q string) []string { return extractPhrases(q) }

func (ix *Index) Add(d Doc) {
    ix.docs[d.ID] = d
    ix.total++
    // title positions
    p := 0
    for _, tok := range tokenize(d.Title) {
        m, ok := ix.titlePos[tok]
        if !ok { m = map[string][]int{}; ix.titlePos[tok] = m }
        m[d.ID] = append(m[d.ID], p)
        p++
    }
    // snippet positions
    p = 0
    for _, tok := range tokenize(d.Snippet) {
        m, ok := ix.snippetPos[tok]
        if !ok { m = map[string][]int{}; ix.snippetPos[tok] = m }
        m[d.ID] = append(m[d.ID], p)
        p++
    }
}

func (ix *Index) Remove(id string) bool {
    d, ok := ix.docs[id]
    if !ok { return false }
    // Remove from position maps
    for _, tok := range tokenize(d.Title) {
        if m, ok := ix.titlePos[tok]; ok {
            delete(m, id)
            if len(m) == 0 { delete(ix.titlePos, tok) }
        }
    }
    for _, tok := range tokenize(d.Snippet) {
        if m, ok := ix.snippetPos[tok]; ok {
            delete(m, id)
            if len(m) == 0 { delete(ix.snippetPos, tok) }
        }
    }
    delete(ix.docs, id)
    if ix.total > 0 { ix.total-- }
    return true
}

type Result struct {
    Doc   Doc
    Score float64
}

func (ix *Index) Search(q string, limit int) []Result {
    if limit <= 0 { limit = 10 }
    phrases := extractPhrases(q)
    qNoP := removePhrases(q)
    req, opt, not := parseBoolean(qNoP)
    // candidates
    cand := map[string]bool{}
    if len(req) > 0 {
        first := true
        for tok := range req {
            docsSet := map[string]struct{}{}
            for id := range ix.titlePos[tok] { docsSet[id] = struct{}{} }
            for id := range ix.snippetPos[tok] { docsSet[id] = struct{}{} }
            if first {
                for id := range docsSet { cand[id] = true }
                first = false
            } else {
                for id := range cand {
                    if _, ok := docsSet[id]; !ok { delete(cand, id) }
                }
            }
        }
    } else {
        for tok := range opt {
            for id := range ix.titlePos[tok] { cand[id] = true }
            for id := range ix.snippetPos[tok] { cand[id] = true }
        }
        if len(opt) == 0 {
            for id := range ix.docs { cand[id] = true }
        }
    }
    for tok := range not {
        for id := range ix.titlePos[tok] { delete(cand, id) }
        for id := range ix.snippetPos[tok] { delete(cand, id) }
    }
    if len(cand) == 0 { return nil }
    // TF-IDF with field weighting (title > snippet)
    scores := map[string]float64{}
    norms := map[string]float64{}
    N := float64(ix.total)
    allToks := map[string]struct{}{}
    for tok := range req { allToks[tok] = struct{}{} }
    for tok := range opt { allToks[tok] = struct{}{} }
    if len(allToks) == 0 {
        for _, t := range tokenize(qNoP) { allToks[t] = struct{}{} }
    }
    wTitle, wSnip := 2.0, 1.0
    for tok := range allToks {
        docsSet := map[string]struct{}{}
        for id := range ix.titlePos[tok] { docsSet[id] = struct{}{} }
        for id := range ix.snippetPos[tok] { docsSet[id] = struct{}{} }
        df := float64(len(docsSet))
        if df == 0 { continue }
        idf := math.Log(1 + (N / (1 + df)))
        for id := range docsSet {
            if !cand[id] { continue }
            tfT := float64(len(ix.titlePos[tok][id]))
            tfS := float64(len(ix.snippetPos[tok][id]))
            w := (tfT*wTitle + tfS*wSnip) * idf
            scores[id] += w
            norms[id] += w * w
        }
    }
    // Phrase boost
    for id := range cand {
        d := ix.docs[id]
        for _, ph := range phrases {
            if ph == "" { continue }
            toks := tokenize(ph)
            if len(toks) == 0 { continue }
            if hasPhrase(ix.titlePos, id, toks) || hasPhrase(ix.snippetPos, id, toks) {
                scores[id] += 2.0 // larger boost for exact phrase
            } else {
                // fallback substring boost (lower)
                corpus := strings.ToLower(d.Title + " " + d.Snippet)
                if strings.Contains(corpus, strings.ToLower(ph)) { scores[id] += 0.5 }
            }
        }
    }
    out := make([]Result, 0, len(scores))
    for id, sc := range scores {
        n := math.Sqrt(norms[id])
        if n > 0 { sc = sc / n }
        out = append(out, Result{Doc: ix.docs[id], Score: sc})
    }
    sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
    if len(out) > limit { out = out[:limit] }
    return out
}

// FilteredSearch supports optional source filter, date range, and sort by latest.
func (ix *Index) FilteredSearch(q string, source string, from string, to string, sortBy string, page int, size int) ([]Result, int, map[string]int64) {
    if size <= 0 { size = 10 }
    if page <= 0 { page = 1 }
    results := ix.Search(q, 10000)
    // Apply filters
    var fromT, toT time.Time
    var hasFrom, hasTo bool
    if strings.TrimSpace(from) != "" { if t, err := parseDate(from); err == nil { fromT = t; hasFrom = true } }
    if strings.TrimSpace(to) != "" { if t, err := parseDate(to); err == nil { toT = t; hasTo = true } }
    filtered := make([]Result, 0, len(results))
    facets := map[string]int64{}
    for _, r := range results {
        d := r.Doc
        if source != "" && d.Source != source { continue }
        if hasFrom || hasTo {
            if dt, err := parseDate(d.Date); err == nil {
                if hasFrom && dt.Before(fromT) { continue }
                if hasTo && dt.After(toT) { continue }
            }
        }
        filtered = append(filtered, r)
        facets[d.Source] = facets[d.Source] + 1
    }
    // Sort order
    if sortBy == "latest" {
        sort.SliceStable(filtered, func(i, j int) bool {
            di, dj := filtered[i].Doc, filtered[j].Doc
            ti, ei := parseDate(di.Date)
            tj, ej := parseDate(dj.Date)
            if ei != nil && ej != nil { return filtered[i].Score > filtered[j].Score }
            if ei != nil { return false }
            if ej != nil { return true }
            return ti.After(tj)
        })
    }
    total := len(filtered)
    // Pagination
    start := (page - 1) * size
    if start > total { return []Result{}, total, facets }
    end := start + size
    if end > total { end = total }
    return filtered[start:end], total, facets
}

func parseDate(s string) (time.Time, error) {
    if strings.TrimSpace(s) == "" { return time.Time{}, os.ErrInvalid }
    if t, err := time.Parse(time.RFC3339, s); err == nil { return t, nil }
    if t, err := time.Parse("2006-01-02", s); err == nil { return t, nil }
    return time.Time{}, os.ErrInvalid
}

// Boolean and phrase helpers
func extractPhrases(q string) []string {
    var ph []string
    s := q
    for {
        i := strings.IndexByte(s, '"')
        if i == -1 { break }
        s = s[i+1:]
        j := strings.IndexByte(s, '"')
        if j == -1 { break }
        ph = append(ph, s[:j])
        s = s[j+1:]
    }
    return ph
}
func removePhrases(q string) string {
    out := []rune{}
    in := false
    for _, r := range q {
        if r == '"' { in = !in; continue }
        if in { continue }
        out = append(out, r)
    }
    return string(out)
}
func parseBoolean(q string) (required, optional, negated map[string]struct{}) {
    required, optional, negated = map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}
    toks := strings.Fields(q)
    lastOp := "OR"
    for _, t := range toks {
        tt := strings.ToLower(t)
        switch tt {
        case "and", "or", "not":
            lastOp = strings.ToUpper(tt)
            continue
        }
        neg := false
        if strings.HasPrefix(tt, "-") { neg = true; tt = strings.TrimPrefix(tt, "-") }
        terms := tokenize(tt)
        if len(terms) == 0 { continue }
        tok := terms[0]
        if neg { negated[tok] = struct{}{}; continue }
        if lastOp == "AND" { required[tok] = struct{}{} } else { optional[tok] = struct{}{} }
    }
    return
}

// hasPhrase checks if tokens appear in consecutive positions for a given doc in a field position index.
func hasPhrase(pos map[string]map[string][]int, docID string, toks []string) bool {
    if len(toks) == 0 { return false }
    // get positions list for first token
    p0, ok := pos[toks[0]][docID]
    if !ok || len(p0) == 0 { return false }
    // For each start position, attempt to advance consecutive positions
    for _, start := range p0 {
        okAll := true
        prev := start
        for i := 1; i < len(toks); i++ {
            lst, ok := pos[toks[i]][docID]
            if !ok { okAll = false; break }
            // find value prev+1 in lst
            found := false
            for _, v := range lst { if v == prev+1 { found = true; prev = v; break } }
            if !found { okAll = false; break }
        }
        if okAll { return true }
    }
    return false
}

// Persistence: save/load docs as JSON; rebuild index on load.
type snapshot struct {
    Docs []Doc `json:"docs"`
}

func (ix *Index) Save(path string) error {
    docs := make([]Doc, 0, len(ix.docs))
    for _, d := range ix.docs { docs = append(docs, d) }
    b, err := json.MarshalIndent(snapshot{Docs: docs}, "", "  ")
    if err != nil { return err }
    if err := os.MkdirAll(dirOf(path), 0o755); err != nil { return err }
    return os.WriteFile(path, b, 0o644)
}

func (ix *Index) Load(path string) error {
    b, err := os.ReadFile(path)
    if err != nil { return err }
    var snap snapshot
    if err := json.Unmarshal(b, &snap); err != nil { return err }
    ix.docs = map[string]Doc{}
    ix.titlePos = map[string]map[string][]int{}
    ix.snippetPos = map[string]map[string][]int{}
    ix.total = 0
    for _, d := range snap.Docs { ix.Add(d) }
    return nil
}

func dirOf(p string) string {
    i := strings.LastIndex(p, "/")
    if i <= 0 { return "." }
    return p[:i]
}

// Multi-vertical container holding separate indexes.
type Store struct {
    Web   *Index
    Images *Index
    Videos *Index
    News   *Index
}

func NewStore() *Store { return &Store{Web: New(), Images: New(), Videos: New(), News: New()} }

// Utilities for admin/inspection
func (ix *Index) Total() int { return ix.total }
func (ix *Index) FacetSource() map[string]int64 {
    m := map[string]int64{}
    for _, d := range ix.docs { m[d.Source] = m[d.Source] + 1 }
    return m
}
func (ix *Index) List(offset, limit int) []Doc {
    if limit <= 0 { limit = 50 }
    if offset < 0 { offset = 0 }
    // Deterministic order by ID
    keys := make([]string, 0, len(ix.docs))
    for k := range ix.docs { keys = append(keys, k) }
    sort.Strings(keys)
    out := []Doc{}
    for i := offset; i < len(keys) && len(out) < limit; i++ {
        out = append(out, ix.docs[keys[i]])
    }
    return out
}

// ExportDocs returns a copy of all docs for snapshot/export.
func (ix *Index) ExportDocs() []Doc {
    docs := make([]Doc, 0, len(ix.docs))
    for _, d := range ix.docs { docs = append(docs, d) }
    return docs
}

// ReplaceAll clears the index and adds the provided docs, rebuilding postings.
func (ix *Index) ReplaceAll(docs []Doc) {
    ix.docs = map[string]Doc{}
    ix.titlePos = map[string]map[string][]int{}
    ix.snippetPos = map[string]map[string][]int{}
    ix.total = 0
    for _, d := range docs { ix.Add(d) }
}

// ClearBy removes docs that match optional source or host filters.
// If source is non-empty, only docs with matching Source are removed.
// If host is non-empty, only docs with URL host matching are removed.
func (ix *Index) ClearBy(source, host string) int {
    removed := 0
    for id, d := range ix.docs {
        if source != "" && d.Source != source { continue }
        if host != "" && hostOf(d.Url) != host { continue }
        if ix.Remove(id) { removed++ }
    }
    return removed
}

func hostOf(u string) string {
    if u == "" { return "" }
    if uu, err := url.Parse(u); err == nil { return uu.Host }
    // fallback naive
    s := u
    if i := strings.Index(s, "://"); i >= 0 { s = s[i+3:] }
    if j := strings.Index(s, "/"); j >= 0 { s = s[:j] }
    return s
}
