package searchindex

import (
    "encoding/json"
    "math"
    "net/url"
    "os"
    "sort"
    "strconv"
    "fmt"
    "strings"
    "time"
)

type Doc struct {
    ID      string
    Title   string
    Url     string
    Snippet string
    Body    string
    Headings string
    Source  string
    Date    string // RFC3339 preferred
    Lang    string // ISO 639-1 (e.g., "en"), optional
    Meta    map[string]string `json:"meta,omitempty"`
}

type Index struct {
    docs   map[string]Doc
    titlePos   map[string]map[string][]int // token -> docID -> positions in title
    snippetPos map[string]map[string][]int // token -> docID -> positions in snippet
    bodyPos    map[string]map[string][]int // token -> docID -> positions in body
    urlPos     map[string]map[string][]int // token -> docID -> positions in URL (host/path)
    headingsPos map[string]map[string][]int // token -> docID -> positions in headings
    total  int
}

func New() *Index { return &Index{docs: map[string]Doc{}, titlePos: map[string]map[string][]int{}, snippetPos: map[string]map[string][]int{}, bodyPos: map[string]map[string][]int{}, urlPos: map[string]map[string][]int{}, headingsPos: map[string]map[string][]int{}, total: 0} }

var englishStopwords = map[string]struct{}{
    "the":{},"is":{},"at":{},"which":{},"on":{},"and":{},"a":{},"an":{},"for":{},"to":{},"in":{},"of":{},"with":{},"as":{},"by":{},"it":{},"this":{},"that":{},"be":{},"or":{},"not":{},
}
func getStopwords(lang string) map[string]struct{} {
    if lang == "en" { return englishStopwords }
    return map[string]struct{}{}
}

// Feature toggles via environment variables (defaults enabled)
var (
    enableStemming = getenvDefault("SEARCH_ENABLE_STEMMING", "1") != "0"
    enablePrefix   = getenvDefault("SEARCH_ENABLE_PREFIX", "1") != "0"
    enableFuzzy    = getenvDefault("SEARCH_ENABLE_FUZZY", "1") != "0"
    prefixMinLen   = getIntDefault("SEARCH_PREFIX_MINLEN", 2)
    synonymsMap    = map[string][]string{}
    // diversify result lists by limiting hits per source (domain)
    maxPerSource   = getIntDefault("SEARCH_MAX_PER_SOURCE", 3)
    domainPriors   = map[string]float64{}
    // field weights (overridable via env and admin)
    wTitle   = getFloatDefault("SEARCH_W_TITLE", 2.0)
    wHead    = getFloatDefault("SEARCH_W_HEADINGS", 1.4)
    wURL     = getFloatDefault("SEARCH_W_URL", 1.2)
    wSnip    = getFloatDefault("SEARCH_W_SNIPPET", 1.0)
    wBody    = getFloatDefault("SEARCH_W_BODY", 0.7)
)

// SetDomainPriors replaces the in-memory map of domain suffix -> multiplier (>1 boosts, <1 demotes).
// Keys are matched as lowercase suffixes against Doc.Source (host). Example: ".co.ke": 1.2, ".africa": 1.25.
func SetDomainPriors(m map[string]float64) {
    domainPriors = map[string]float64{}
    for k, v := range m {
        lk := strings.ToLower(strings.TrimSpace(k))
        if lk == "" { continue }
        if v <= 0 { continue }
        domainPriors[lk] = v
    }
}

func domainBoostFor(host string) float64 {
    if host == "" || len(domainPriors) == 0 { return 1.0 }
    h := strings.ToLower(host)
    best := 1.0
    for suf, mul := range domainPriors {
        if strings.HasSuffix(h, suf) {
            if mul > best { best = mul }
        }
    }
    return best
}

// SetSynonyms replaces the in-memory synonyms dictionary used for query expansion.
// Keys and values should be lowercase strings. Call at startup or via admin.
func SetSynonyms(m map[string][]string) {
    synonymsMap = map[string][]string{}
    for k, arr := range m {
        lk := strings.ToLower(strings.TrimSpace(k))
        if lk == "" { continue }
        vals := []string{}
        for _, v := range arr {
            lv := strings.ToLower(strings.TrimSpace(v))
            if lv != "" { vals = append(vals, lv) }
        }
        synonymsMap[lk] = vals
    }
}

func getenvDefault(k, def string) string {
    if v := os.Getenv(k); v != "" { return v }
    return def
}
func getIntDefault(k string, def int) int {
    v := getenvDefault(k, "")
    if v == "" { return def }
    if n, err := strconv.Atoi(v); err == nil { return n }
    return def
}
func getFloatDefault(k string, def float64) float64 {
    v := getenvDefault(k, "")
    if v == "" { return def }
    var f float64
    if _, err := fmt.Sscan(v, &f); err == nil { return f }
    return def
}

func asciiFold(s string) string {
    // Map common accented Latin characters to ASCII equivalents
    replacer := strings.NewReplacer(
        "à", "a", "á", "a", "â", "a", "ã", "a", "ä", "a", "å", "a",
        "ç", "c",
        "è", "e", "é", "e", "ê", "e", "ë", "e",
        "ì", "i", "í", "i", "î", "i", "ï", "i",
        "ñ", "n",
        "ò", "o", "ó", "o", "ô", "o", "õ", "o", "ö", "o",
        "ù", "u", "ú", "u", "û", "u", "ü", "u",
        "ý", "y", "ÿ", "y",
        "À", "a", "Á", "a", "Â", "a", "Ã", "a", "Ä", "a", "Å", "a",
        "Ç", "c",
        "È", "e", "É", "e", "Ê", "e", "Ë", "e",
        "Ì", "i", "Í", "i", "Î", "i", "Ï", "i",
        "Ñ", "n",
        "Ò", "o", "Ó", "o", "Ô", "o", "Õ", "o", "Ö", "o",
        "Ù", "u", "Ú", "u", "Û", "u", "Ü", "u",
        "Ÿ", "y",
        "œ", "oe", "Œ", "oe", "æ", "ae", "Æ", "ae", "ß", "ss",
    )
    s = replacer.Replace(s)
    // Replace any remaining non-ASCII letters/digits with space to split
    repl := func(r rune) rune {
        if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') { return r }
        return ' '
    }
    return strings.Map(repl, s)
}

func stemEn(tok string) string {
    if !enableStemming { return tok }
    s := tok
    n := len(s)
    if n <= 3 { return s }
    // Plurals
    if strings.HasSuffix(s, "ies") && n > 4 { // e.g., "bodies" -> "body"
        return s[:n-3] + "y"
    }
    if strings.HasSuffix(s, "sses") && n > 4 { // "classes" -> "class"
        return s[:n-2]
    }
    if strings.HasSuffix(s, "es") && n > 3 {
        base := s[:n-2]
        if strings.HasSuffix(base, "s") || strings.HasSuffix(base, "x") || strings.HasSuffix(base, "z") || strings.HasSuffix(base, "ch") || strings.HasSuffix(base, "sh") {
            s = base
            n = len(s)
        }
    }
    if strings.HasSuffix(s, "s") && !strings.HasSuffix(s, "ss") && n > 3 { // docs -> doc
        s = s[:len(s)-1]
        n = len(s)
    }
    // Past/continuous
    if strings.HasSuffix(s, "ing") && n > 5 {
        base := s[:n-3]
        if lb := len(base); lb >= 2 && base[lb-1] == base[lb-2] { base = base[:lb-1] }
        return base
    }
    if strings.HasSuffix(s, "ed") && n > 4 {
        base := s[:n-2]
        if lb := len(base); lb >= 2 && base[lb-1] == base[lb-2] { base = base[:lb-1] }
        return base
    }
    return s
}

func normalize(s string) string {
    s = strings.ToLower(s)
    s = asciiFold(s)
    return s
}

func tokenize(s string) []string { return tokenizeForLang(s, "en") }

func tokenizeForLang(s string, lang string) []string {
    // normalize, split non-alnum, drop short tokens/stopwords, apply stemming
    cleaned := normalize(s)
    parts := strings.Fields(cleaned)
    out := make([]string, 0, len(parts))
    sw := getStopwords(lang)
    for _, p := range parts {
        if len(p) < 2 { continue }
        if _, stop := sw[p]; stop { continue }
        if lang == "en" { p = stemEn(p) }
        if p == "" { continue }
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
    for _, tok := range tokenizeForLang(d.Title, d.Lang) {
        m, ok := ix.titlePos[tok]
        if !ok { m = map[string][]int{}; ix.titlePos[tok] = m }
        m[d.ID] = append(m[d.ID], p)
        p++
    }
    // snippet positions
    p = 0
    for _, tok := range tokenizeForLang(d.Snippet, d.Lang) {
        m, ok := ix.snippetPos[tok]
        if !ok { m = map[string][]int{}; ix.snippetPos[tok] = m }
        m[d.ID] = append(m[d.ID], p)
        p++
    }
    // body positions
    p = 0
    for _, tok := range tokenizeForLang(d.Body, d.Lang) {
        m, ok := ix.bodyPos[tok]
        if !ok { m = map[string][]int{}; ix.bodyPos[tok] = m }
        m[d.ID] = append(m[d.ID], p)
        p++
    }
    // headings positions
    p = 0
    for _, tok := range tokenizeForLang(d.Headings, d.Lang) {
        m, ok := ix.headingsPos[tok]
        if !ok { m = map[string][]int{}; ix.headingsPos[tok] = m }
        m[d.ID] = append(m[d.ID], p)
        p++
    }
    // url tokens (host/path)
    p = 0
    for _, tok := range tokenizeURL(d.Url) {
        m, ok := ix.urlPos[tok]
        if !ok { m = map[string][]int{}; ix.urlPos[tok] = m }
        m[d.ID] = append(m[d.ID], p)
        p++
    }
}

func (ix *Index) Remove(id string) bool {
    d, ok := ix.docs[id]
    if !ok { return false }
    // Remove from position maps
    for _, tok := range tokenizeForLang(d.Title, d.Lang) {
        if m, ok := ix.titlePos[tok]; ok {
            delete(m, id)
            if len(m) == 0 { delete(ix.titlePos, tok) }
        }
    }
    for _, tok := range tokenizeForLang(d.Snippet, d.Lang) {
        if m, ok := ix.snippetPos[tok]; ok {
            delete(m, id)
            if len(m) == 0 { delete(ix.snippetPos, tok) }
        }
    }
    for _, tok := range tokenizeForLang(d.Body, d.Lang) {
        if m, ok := ix.bodyPos[tok]; ok {
            delete(m, id)
            if len(m) == 0 { delete(ix.bodyPos, tok) }
        }
    }
    for _, tok := range tokenizeForLang(d.Headings, d.Lang) {
        if m, ok := ix.headingsPos[tok]; ok {
            delete(m, id)
            if len(m) == 0 { delete(ix.headingsPos, tok) }
        }
    }
    for _, tok := range tokenizeURL(d.Url) {
        if m, ok := ix.urlPos[tok]; ok {
            delete(m, id)
            if len(m) == 0 { delete(ix.urlPos, tok) }
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
    // build vocabulary once for fuzzy/prefix expansion
    vocab := ix.vocabulary()
    // candidates
    cand := map[string]bool{}
    if len(req) > 0 {
        first := true
        for tok := range req {
            // expand token variants (exact/prefix/fuzzy)
            variants := ix.expandTokenVariants(tok, vocab)
            docsSet := map[string]struct{}{}
            for vt := range variants {
                for id := range ix.titlePos[vt] { docsSet[id] = struct{}{} }
                for id := range ix.snippetPos[vt] { docsSet[id] = struct{}{} }
            }
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
            variants := ix.expandTokenVariants(tok, vocab)
            for vt := range variants {
                for id := range ix.titlePos[vt] { cand[id] = true }
                for id := range ix.snippetPos[vt] { cand[id] = true }
            }
        }
        if len(opt) == 0 {
            for id := range ix.docs { cand[id] = true }
        }
    }
    for tok := range not {
        // negations apply to all variants as well
        variants := ix.expandTokenVariants(tok, vocab)
        for vt := range variants {
            for id := range ix.titlePos[vt] { delete(cand, id) }
            for id := range ix.snippetPos[vt] { delete(cand, id) }
        }
    }
    if len(cand) == 0 { return nil }
    // TF-IDF with field weighting (title > snippet)
    scores := map[string]float64{}
    norms := map[string]float64{}
    N := float64(ix.total)
    // Use original query tokens, but expand to variants for scoring
    qTokens := map[string]struct{}{}
    for tok := range req { qTokens[tok] = struct{}{} }
    for tok := range opt { qTokens[tok] = struct{}{} }
    if len(qTokens) == 0 {
        for _, t := range tokenize(qNoP) { qTokens[t] = struct{}{} }
    }
    wTitle, wSnip, wBody, wURL, wHead := 2.0, 1.0, 0.7, 1.2, 1.4
    for tok := range qTokens {
        variants := ix.expandTokenVariants(tok, vocab)
        for vt, mult := range variants {
            docsSet := map[string]struct{}{}
            for id := range ix.titlePos[vt] { docsSet[id] = struct{}{} }
            for id := range ix.snippetPos[vt] { docsSet[id] = struct{}{} }
            for id := range ix.bodyPos[vt] { docsSet[id] = struct{}{} }
            for id := range ix.urlPos[vt] { docsSet[id] = struct{}{} }
            for id := range ix.headingsPos[vt] { docsSet[id] = struct{}{} }
            df := float64(len(docsSet))
            if df == 0 { continue }
            idf := math.Log(1 + (N / (1 + df)))
            for id := range docsSet {
                if !cand[id] { continue }
                tfT := float64(len(ix.titlePos[vt][id]))
                tfS := float64(len(ix.snippetPos[vt][id]))
                tfB := float64(len(ix.bodyPos[vt][id]))
                tfU := float64(len(ix.urlPos[vt][id]))
                tfH := float64(len(ix.headingsPos[vt][id]))
                w := (tfT*wTitle + tfS*wSnip + tfB*wBody + tfU*wURL + tfH*wHead) * idf * mult
                scores[id] += w
                norms[id] += w * w
            }
        }
    }
    // Phrase boost
    for id := range cand {
        d := ix.docs[id]
        for _, ph := range phrases {
            if ph == "" { continue }
            toks := tokenize(ph)
            if len(toks) == 0 { continue }
            if hasPhrase(ix.titlePos, id, toks) || hasPhrase(ix.snippetPos, id, toks) || hasPhrase(ix.bodyPos, id, toks) || hasPhrase(ix.urlPos, id, toks) || hasPhrase(ix.headingsPos, id, toks) {
                scores[id] += 2.0 // larger boost for exact phrase
            } else {
                // fallback substring boost (lower)
                corpus := strings.ToLower(d.Title + " " + d.Snippet + " " + d.Body + " " + d.Url + " " + d.Headings)
                if strings.Contains(corpus, strings.ToLower(ph)) { scores[id] += 0.5 }
            }
        }
    }

    // Intent boosts: detect common travel intents and add co-occurrence boosts
    // Patterns handled:
    //  - flights to <city>
    //  - flights from <city> to <city>
    //  - hotels/accommodation/lodging/stay in/near <place>
    // City/place detection is naive: the token(s) after prepositions are treated as targets.
    {
        qn := strings.ToLower(strings.TrimSpace(q))
        // Flights to <city>
        if strings.Contains(qn, "flights to ") || strings.Contains(qn, "flight to ") || strings.Contains(qn, "airfare to ") || strings.Contains(qn, "tickets to ") {
            city := afterPhraseToken(qn, "to")
            if city != "" {
                for id := range cand {
                    d := ix.docs[id]
                    corpus := strings.ToLower(d.Title + " " + d.Snippet + " " + d.Body + " " + d.Url + " " + d.Headings)
                    if (strings.Contains(corpus, "flight") || strings.Contains(corpus, "airline") || strings.Contains(corpus, "airfare") || strings.Contains(corpus, "ticket")) && strings.Contains(corpus, city) {
                        scores[id] += 1.2
                    }
                }
            }
        }
        // Flights from <cityA> to <cityB>
        if strings.Contains(qn, "flights from ") && strings.Contains(qn, " to ") {
            a, b := routeFromTo(qn)
            if a != "" && b != "" {
                for id := range cand {
                    d := ix.docs[id]
                    corpus := strings.ToLower(d.Title + " " + d.Snippet + " " + d.Body + " " + d.Url + " " + d.Headings)
                    if strings.Contains(corpus, "flight") && strings.Contains(corpus, a) && strings.Contains(corpus, b) {
                        scores[id] += 1.3
                    }
                }
            }
        }
        // Hotels/accommodation/stay/lodging in/near <place>
        if strings.Contains(qn, "hotels in ") || strings.Contains(qn, "hotel in ") || strings.Contains(qn, "accommodation in ") || strings.Contains(qn, "lodging in ") || strings.Contains(qn, "stay in ") || strings.Contains(qn, "hotels near ") || strings.Contains(qn, "hotel near ") {
            city := afterAny(qn, []string{"in", "near"})
            if city != "" {
                for id := range cand {
                    d := ix.docs[id]
                    corpus := strings.ToLower(d.Title + " " + d.Snippet + " " + d.Body + " " + d.Url + " " + d.Headings)
                    if (strings.Contains(corpus, "hotel") || strings.Contains(corpus, "accommodation") || strings.Contains(corpus, "lodging") || strings.Contains(corpus, "stay")) && strings.Contains(corpus, city) {
                        scores[id] += 1.0
                    }
                }
            }
        }
    }
    out := make([]Result, 0, len(scores))
    for id, sc := range scores {
        n := math.Sqrt(norms[id])
        if n > 0 { sc = sc / n }
        // apply domain prior multiplier
        d := ix.docs[id]
        sc = sc * domainBoostFor(d.Source)
        out = append(out, Result{Doc: d, Score: sc})
    }
    // Deterministic ordering: stable sort by score, tie-break by ID then URL
    sort.SliceStable(out, func(i, j int) bool {
        if out[i].Score == out[j].Score {
            if out[i].Doc.ID == out[j].Doc.ID {
                return out[i].Doc.Url < out[j].Doc.Url
            }
            return out[i].Doc.ID < out[j].Doc.ID
        }
        return out[i].Score > out[j].Score
    })
    if len(out) > limit { out = out[:limit] }
    return out
}

// afterPhraseToken extracts the first token after a given preposition-like token (e.g., "to", "in")
// from a simple natural-language query string. Returns lowercase token or empty.
func afterPhraseToken(q string, key string) string {
    parts := strings.Fields(q)
    for i := 0; i < len(parts)-1; i++ {
        if parts[i] == key {
            // Return next token stripped of punctuation
            nxt := parts[i+1]
            nxt = strings.Trim(nxt, ",.;:!?'\"")
            return nxt
        }
    }
    return ""
}

// afterAny returns the first token following the first matching key in keys.
func afterAny(q string, keys []string) string {
    parts := strings.Fields(q)
    for i := 0; i < len(parts)-1; i++ {
        for _, k := range keys {
            if parts[i] == k {
                nxt := parts[i+1]
                nxt = strings.Trim(nxt, ",.;:!?'\"")
                return nxt
            }
        }
    }
    return ""
}

// routeFromTo extracts tokens after "from" and after the next "to".
func routeFromTo(q string) (from string, to string) {
    parts := strings.Fields(q)
    for i := 0; i < len(parts)-1; i++ {
        if parts[i] == "from" && i+1 < len(parts) {
            from = strings.Trim(parts[i+1], ",.;:!?'\"")
            // find next "to"
            for j := i + 2; j < len(parts)-0; j++ {
                if parts[j] == "to" && j+1 < len(parts) {
                    to = strings.Trim(parts[j+1], ",.;:!?'\"")
                    return
                }
            }
            return
        }
    }
    return
}

// SetWeights updates field weights; keys: title, headings, url, snippet, body.
func SetWeights(m map[string]float64) {
    for k, v := range m {
        if v <= 0 { continue }
        switch strings.ToLower(strings.TrimSpace(k)) {
        case "title": wTitle = v
        case "headings": wHead = v
        case "url": wURL = v
        case "snippet": wSnip = v
        case "body": wBody = v
        }
    }
}

// FilteredSearch supports optional source filter, date range, and sort by latest.
func (ix *Index) FilteredSearch(q string, source string, lang string, from string, to string, sortBy string, page int, size int) ([]Result, int, map[string]int64) {
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
        if lang != "" && strings.ToLower(strings.TrimSpace(d.Lang)) != strings.ToLower(strings.TrimSpace(lang)) { continue }
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
    // Diversify by source domain
    diversified := diversifyBySource(filtered, maxPerSource)
    total := len(diversified)
    // Pagination
    start := (page - 1) * size
    if start > total { return []Result{}, total, facets }
    end := start + size
    if end > total { end = total }
    return diversified[start:end], total, facets
}

// FilteredSearchTransport adds support for filtering by transport meta fields: from, to, depart, return.
// Expect meta keys: from, to, depart, return (lowercase). Depart/return are YYYY-MM-DD or RFC3339.
func (ix *Index) FilteredSearchTransport(q string, meta map[string]string, sortBy string, page int, size int) ([]Result, int, map[string]int64) {
    if size <= 0 { size = 10 }
    if page <= 0 { page = 1 }
    results := ix.Search(q, 10000)
    filtered := make([]Result, 0, len(results))
    facets := map[string]int64{}
    wantFrom := strings.ToLower(strings.TrimSpace(meta["from"]))
    wantTo := strings.ToLower(strings.TrimSpace(meta["to"]))
    wantDepart := strings.TrimSpace(meta["depart"]) // raw compare substring or same-day match
    wantReturn := strings.TrimSpace(meta["return"]) // optional
    for _, r := range results {
        d := r.Doc
        if d.Meta == nil { d.Meta = map[string]string{} }
        if wantFrom != "" && strings.ToLower(d.Meta["from"]) != wantFrom { continue }
        if wantTo != "" && strings.ToLower(d.Meta["to"]) != wantTo { continue }
        if wantDepart != "" && !strings.Contains(d.Meta["depart"], wantDepart) { continue }
        if wantReturn != "" && !strings.Contains(d.Meta["return"], wantReturn) { continue }
        filtered = append(filtered, r)
        facets[d.Source] = facets[d.Source] + 1
    }
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
    diversified := diversifyBySource(filtered, maxPerSource)
    total := len(diversified)
    start := (page - 1) * size
    if start > total { return []Result{}, total, facets }
    end := start + size
    if end > total { end = total }
    return diversified[start:end], total, facets
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
    // Default to implicit AND to reduce spurious matches for multi-term queries
    lastOp := "AND"
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
    // Atomic-ish write: write to temp file then rename into place
    tmp := path + ".tmp"
    if err := os.WriteFile(tmp, b, 0o644); err != nil { return err }
    // On Windows, Rename does not replace existing files; remove if present
    _ = os.Remove(path)
    return os.Rename(tmp, path)
}

func (ix *Index) Load(path string) error {
    b, err := os.ReadFile(path)
    if err != nil { return err }
    var snap snapshot
    if err := json.Unmarshal(b, &snap); err != nil { return err }
    ix.docs = map[string]Doc{}
    ix.titlePos = map[string]map[string][]int{}
    ix.snippetPos = map[string]map[string][]int{}
    ix.bodyPos = map[string]map[string][]int{}
    ix.urlPos = map[string]map[string][]int{}
    ix.headingsPos = map[string]map[string][]int{}
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
    Transport *Index
}

func NewStore() *Store { return &Store{Web: New(), Images: New(), Videos: New(), News: New(), Transport: New()} }

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

// diversifyBySource caps the number of results per source while preserving order.
func diversifyBySource(in []Result, maxPer int) []Result {
    if maxPer <= 0 { return in }
    out := make([]Result, 0, len(in))
    per := map[string]int{}
    for _, r := range in {
        s := r.Doc.Source
        if per[s] < maxPer {
            out = append(out, r)
            per[s] = per[s] + 1
        }
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
    ix.bodyPos = map[string]map[string][]int{}
    ix.urlPos = map[string]map[string][]int{}
    ix.headingsPos = map[string]map[string][]int{}
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

// vocabulary returns a set of all tokens present in the index across fields.
func (ix *Index) vocabulary() map[string]struct{} {
    vocab := map[string]struct{}{}
    for t := range ix.titlePos { vocab[t] = struct{}{} }
    for t := range ix.snippetPos { vocab[t] = struct{}{} }
    for t := range ix.bodyPos { vocab[t] = struct{}{} }
    for t := range ix.urlPos { vocab[t] = struct{}{} }
    for t := range ix.headingsPos { vocab[t] = struct{}{} }
    return vocab
}

// tokenizeURL extracts tokens from host and path of a URL.
func tokenizeURL(u string) []string {
    if u == "" { return nil }
    // Lowercase and ASCII-fold then split by non-alnum
    s := normalize(u)
    parts := strings.Fields(s)
    out := make([]string, 0, len(parts))
    for _, p := range parts {
        if len(p) < 2 { continue }
        out = append(out, p)
    }
    return out
}

// expandTokenVariants returns candidate index tokens for a given query token with weights.
// Always includes the exact token (weight 1). May include prefix (weight 0.7) and fuzzy (weight 0.5) variants.
func (ix *Index) expandTokenVariants(tok string, vocab map[string]struct{}) map[string]float64 {
    out := map[string]float64{}
    out[tok] = 1.0
    l := len(tok)
    // Prefix expansion
    if enablePrefix && l >= prefixMinLen {
        for v := range vocab {
            if strings.HasPrefix(v, tok) {
                // skip exact (already present)
                if _, ok := out[v]; !ok {
                    weight := 0.7
                    if l <= 2 { weight = 0.4 }
                    out[v] = weight
                }
            }
        }
    }
    // Fuzzy expansion: only if we have no exact or prefix hits
    if enableFuzzy && l >= 3 {
        // count matches so far excluding the original token
        hasAlt := false
        for v := range out { if v != tok { hasAlt = true; break } }
        if !hasAlt {
            for v := range vocab {
                if v == tok { continue }
                if editDistanceLeq1(tok, v) { out[v] = 0.5 }
            }
        }
    }
    // Synonyms expansion (from in-memory dictionary)
    for _, syn := range synonyms(tok) {
        if _, ok := out[syn]; !ok { out[syn] = 0.8 }
    }
    return out
}

// synonyms returns a small set of hand-curated aliases.
func synonyms(tok string) []string {
    t := strings.ToLower(strings.TrimSpace(tok))
    if t == "" { return nil }
    if arr, ok := synonymsMap[t]; ok { return arr }
    return nil
}

// editDistanceLeq1 returns true if Levenshtein distance between a and b is <= 1.
func editDistanceLeq1(a, b string) bool {
    if a == b { return true }
    la, lb := len(a), len(b)
    if la-lb > 1 || lb-la > 1 { return false }
    i, j := 0, 0
    edits := 0
    for i < la && j < lb {
        if a[i] == b[j] { i++; j++; continue }
        edits++
        if edits > 1 { return false }
        if la == lb { // substitution
            i++; j++
        } else if la > lb { // deletion in a
            i++
        } else { // insertion in a
            j++
        }
    }
    // account for trailing extra char
    if i < la || j < lb { edits++ }
    return edits <= 1
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
