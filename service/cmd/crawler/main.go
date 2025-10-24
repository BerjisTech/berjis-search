package main

import (
    "bytes"
    "crypto/sha1"
    "encoding/json"
    "compress/gzip"
    "fmt"
    "io"
    "log"
    "net/http"
    "net/url"
    "os"
    "regexp"
    "strings"
    "time"

    "github.com/berjistech/berjis-ecosystem/search/service/internal/config"
    whatlang "github.com/abadojack/whatlanggo"
)

type item struct{ u string; depth int }

var (
    reTitle    = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
    reMetaDesc = regexp.MustCompile(`(?is)<meta\s+(?:name|property)=["']description["']\s+content=["']([^"']*)["'][^>]*>`) 
    reMetaOGImg  = regexp.MustCompile(`(?is)<meta\s+property=["']og:image["']\s+content=["']([^"']+)["'][^>]*>`) 
    reMetaOGVid  = regexp.MustCompile(`(?is)<meta\s+property=["']og:video["']\s+content=["']([^"']+)["'][^>]*>`) 
    reMetaOGType = regexp.MustCompile(`(?is)<meta\s+property=["']og:type["']\s+content=["']([^"']+)["'][^>]*>`) 
    reMetaArtPub = regexp.MustCompile(`(?is)<meta\s+property=["']article:published_time["']\s+content=["']([^"']+)["'][^>]*>`) 
    reLinks    = regexp.MustCompile(`(?is)<a\s+[^>]*href=["']([^"']+)["'][^>]*>`) 
    reImgTags  = regexp.MustCompile(`(?is)<img\s+[^>]*src=["']([^"']+)["'][^>]*>`) 
    reArticle  = regexp.MustCompile(`(?is)<article[^>]*>([\s\S]*?)</article>`) 
    reMain     = regexp.MustCompile(`(?is)<main[^>]*>([\s\S]*?)</main>`) 
    reBody     = regexp.MustCompile(`(?is)<body[^>]*>([\s\S]*?)</body>`) 
    reScript   = regexp.MustCompile(`(?is)<script[^>]*>[\s\S]*?</script>`) 
    reStyle    = regexp.MustCompile(`(?is)<style[^>]*>[\s\S]*?</style>`) 
    reH1       = regexp.MustCompile(`(?is)<h1[^>]*>([\s\S]*?)</h1>`) 
    reH2       = regexp.MustCompile(`(?is)<h2[^>]*>([\s\S]*?)</h2>`) 
    reH3       = regexp.MustCompile(`(?is)<h3[^>]*>([\s\S]*?)</h3>`) 
    reH4       = regexp.MustCompile(`(?is)<h4[^>]*>([\s\S]*?)</h4>`) 
    reH5       = regexp.MustCompile(`(?is)<h5[^>]*>([\s\S]*?)</h5>`) 
    reH6       = regexp.MustCompile(`(?is)<h6[^>]*>([\s\S]*?)</h6>`) 
    reTags     = regexp.MustCompile(`(?is)<[^>]+>`) 
    reComments = regexp.MustCompile(`(?s)<!--.*?-->`) 
)

type WebDoc struct { ID, Title, Url, Snippet, Body, Headings, Source, Date, Lang string }

func main() {
    _ = config.Load() // reserved for future use
    base := getenv("SEARCH_API_BASE", "http://search-service:8092")
    seeds := strings.Split(os.Getenv("SEED_URLS"), ",")
    if len(seeds) == 0 || (len(seeds) == 1 && strings.TrimSpace(seeds[0]) == "") {
        seeds = []string{
            "http://berjis.test",
            "http://logistics.berjis.test",
            "http://docs.berjis.test",
            "http://sheets.berjis.test",
            "http://slides.berjis.test",
            "http://notes.berjis.test",
            "http://communities.berjis.test",
            "http://architect.berjis.test",
        }
    }
    // Frontier crawl (persistent frontier)
    maxPages := atoi(getenv("MAX_PAGES", "250"))
    maxDepth := atoi(getenv("MAX_DEPTH", "4"))
    sameDomain := getenv("SAME_DOMAIN_ONLY", "true") == "true"
    frontierPath := getenv("FRONTIER_PATH", "/data/search/frontier.json")
    allowed := map[string]bool{}
    for _, s := range seeds { if h := hostOf(strings.TrimSpace(s)); h != "" { allowed[h] = true } }
    q := []item{}
    seen := map[string]bool{}
    // Load previous frontier
    loadFrontier(frontierPath, &q, &seen)
    // Merge in seeds at depth 0
    for _, s := range seeds { s = strings.TrimSpace(s); if s != "" && !seen[s] { q = append(q, item{u: s, depth: 0}) } }
    webBatch := []WebDoc{}
    imgBatch := []map[string]string{}
    vidBatch := []map[string]string{}
    newsBatch := []map[string]string{}
    batchSize := atoi(getenv("BATCH_SIZE", "100"))
    if batchSize <= 0 { batchSize = 100 }
    count := 0
    hostPolicy := map[string]policy{}
    hostNext := map[string]time.Time{}
    hostCount := map[string]int{}
    hostBlocked := map[string]int{}
    hostLast := map[string]string{}
    maxPerHost := atoi(getenv("MAX_PER_HOST", "20"))
    ua := getenv("CRAWLER_USER_AGENT", "BerjisBot/1.0")
    robotsUA := getenv("ROBOTS_USER_AGENT", "BerjisBot")
    client := &http.Client{ Timeout: 20 * time.Second }
    for len(q) > 0 && count < maxPages {
        it := q[0]; q = q[1:]
        u := it.u
        if seen[u] { continue }
        seen[u] = true
        count++
        h := hostOf(u)
        // robots.txt
        pol := hostPolicy[h]
        if !pol.inited {
            pol = fetchRobots(client, h, robotsUA)
            hostPolicy[h] = pol
        }
        if !pol.allowed(u) { hostBlocked[h] = hostBlocked[h] + 1; continue }
        // per-host crawl delay
        if nxt := hostNext[h]; time.Now().Before(nxt) {
            time.Sleep(nxt.Sub(time.Now()))
        }
        if maxPerHost > 0 && hostCount[h] >= maxPerHost { continue }
        html, status := fetchHTML(client, ua, u)
        if status >= 400 || html == "" { continue }
        if pol.delay > 0 { hostNext[h] = time.Now().Add(pol.delay) }
        hostCount[h] = hostCount[h] + 1
        hostLast[h] = time.Now().UTC().Format(time.RFC3339)
        title, desc := extractTitleDesc(html)
        if title == "" { title = u }
        if desc == "" { desc = title }
        body := extractMainText(html)
        headings := extractHeadings(html)
        lang := detectLang(title+" "+desc+" "+body)
        now := time.Now().UTC().Format(time.RFC3339)
        id := fmt.Sprintf("%x", sha1.Sum([]byte(u)))
        webBatch = append(webBatch, WebDoc{ID: id, Title: title, Url: u, Snippet: desc, Body: body, Headings: headings, Source: hostOf(u), Date: now, Lang: lang})
        if len(webBatch) >= batchSize {
            postJSON(base+"/v1/admin/index/web", map[string]any{"docs": webBatch})
            webBatch = webBatch[:0]
        }
        // OG image/video
        if img := firstGroup(reMetaOGImg.FindStringSubmatch(html)); img != "" {
            ai := absURL(u, img)
            imgBatch = append(imgBatch, map[string]string{"id": fmt.Sprintf("%x", sha1.Sum([]byte(ai))), "title": title, "thumbnailUrl": ai, "imageUrl": ai, "source": hostOf(ai), "date": now})
            if len(imgBatch) >= batchSize { postJSON(base+"/v1/admin/index/images", map[string]any{"docs": imgBatch}); imgBatch = imgBatch[:0] }
        }
        if vid := firstGroup(reMetaOGVid.FindStringSubmatch(html)); vid != "" {
            av := absURL(u, vid)
            vidBatch = append(vidBatch, map[string]string{"id": fmt.Sprintf("%x", sha1.Sum([]byte(av))), "title": title, "url": av, "duration": "", "source": hostOf(av), "date": now})
            if len(vidBatch) >= batchSize { postJSON(base+"/v1/admin/index/videos", map[string]any{"docs": vidBatch}); vidBatch = vidBatch[:0] }
        }
        // News detection
        ogType := strings.ToLower(firstGroup(reMetaOGType.FindStringSubmatch(html)))
        if ogType == "article" {
            pub := firstGroup(reMetaArtPub.FindStringSubmatch(html))
            if pub == "" { pub = now }
            newsBatch = append(newsBatch, map[string]string{"id": id, "title": title, "url": u, "snippet": desc, "source": hostOf(u), "date": pub})
        }
        // Images from <img>
        for _, m := range reImgTags.FindAllStringSubmatch(html, -1) {
            src := absURL(u, m[1])
            if src == "" { continue }
            imgBatch = append(imgBatch, map[string]string{"id": fmt.Sprintf("%x", sha1.Sum([]byte(src))), "title": title, "thumbnailUrl": src, "imageUrl": src, "source": hostOf(src), "date": now})
        }
        // Enqueue links
        if it.depth < maxDepth {
            for _, m := range reLinks.FindAllStringSubmatch(html, -1) {
                href := absURL(u, m[1])
                if href == "" { continue }
                if sameDomain && !allowed[hostOf(href)] { continue }
                if !seen[href] { q = append(q, item{u: href, depth: it.depth + 1}) }
            }
        }
        // Sitemap discovery via robots
        if len(pol.sitemaps) > 0 {
            for _, sm := range pol.sitemaps {
                urls := fetchSitemap(client, sm)
                for _, su := range urls {
                    if sameDomain && !allowed[hostOf(su)] { continue }
                    if !seen[su] { q = append(q, item{u: su, depth: it.depth + 1}) }
                }
            }
        }
    }
    // Final flush
    if len(webBatch) > 0 { postJSON(base+"/v1/admin/index/web", map[string]any{"docs": webBatch}) }
    if len(imgBatch) > 0 { postJSON(base+"/v1/admin/index/images", map[string]any{"docs": imgBatch}) }
    if len(vidBatch) > 0 { postJSON(base+"/v1/admin/index/videos", map[string]any{"docs": vidBatch}) }
    if len(newsBatch) > 0 { postJSON(base+"/v1/admin/index/news", map[string]any{"docs": newsBatch}) }
    // Save updated frontier
    saveFrontier(frontierPath, q, seen)
    // Post crawl stats per host
    stats := map[string]any{}
    for h, c := range hostCount {
        stats[h] = map[string]any{
            "pagesFetched": c,
            "blockedByRobots": hostBlocked[h],
            "lastFetch": hostLast[h],
            "crawlDelaySeconds": int(hostPolicy[h].delay.Seconds()),
            "sitemapCount": len(hostPolicy[h].sitemaps),
        }
    }
    postJSON(base+"/v1/admin/crawler/stats", map[string]any{"hosts": stats, "timestamp": time.Now().UTC().Format(time.RFC3339)})
    log.Printf("crawler: batches flushed; visited=%d hosts=%d", len(seen), len(hostCount))
}

func fetchHTML(client *http.Client, ua, u string) (string, int) {
    req, _ := http.NewRequest("GET", u, nil)
    if ua != "" { req.Header.Set("User-Agent", ua) }
    resp, err := client.Do(req)
    if err != nil { return "", 0 }
    defer resp.Body.Close()
    b, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
    return string(b), resp.StatusCode
}
func extractTitleDesc(html string) (string, string) {
    title := firstGroup(reTitle.FindStringSubmatch(html))
    if title != "" { title = strings.TrimSpace(stripSpaces(title)) }
    desc := firstGroup(reMetaDesc.FindStringSubmatch(html))
    if desc != "" { desc = strings.TrimSpace(stripSpaces(desc)) }
    return title, desc
}

// extractHeadings grabs H1-H6 texts, strips nested tags/comments, and collapses whitespace.
func extractHeadings(html string) string {
    parts := []string{}
    caps := [][][]string{ reH1.FindAllStringSubmatch(html, -1), reH2.FindAllStringSubmatch(html, -1), reH3.FindAllStringSubmatch(html, -1), reH4.FindAllStringSubmatch(html, -1), reH5.FindAllStringSubmatch(html, -1), reH6.FindAllStringSubmatch(html, -1) }
    for _, arr := range caps {
        for _, m := range arr {
            if len(m) >= 2 {
                s := reComments.ReplaceAllString(m[1], " ")
                s = reScript.ReplaceAllString(s, " ")
                s = reStyle.ReplaceAllString(s, " ")
                s = reTags.ReplaceAllString(s, " ")
                s = stripSpaces(s)
                s = strings.TrimSpace(s)
                if s != "" { parts = append(parts, s) }
            }
        }
    }
    out := strings.Join(parts, " \n ")
    if len(out) > 2000 { out = out[:2000] }
    return out
}

func detectLang(text string) string {
    t := strings.TrimSpace(text)
    if t == "" { return "" }
    info := whatlang.Detect(t)
    if info.IsReliable() {
        code := strings.ToLower(info.Lang.Iso6391())
        if code != "" && code != "un" { return code }
    }
    // Fallback heuristic: ASCII-heavy text -> likely English
    letters, ascii := 0, 0
    lower := strings.ToLower(t)
    for _, r := range lower {
        if (r >= 'a' && r <= 'z') || r == ' ' { letters++ }
        if r <= 0x7f { ascii++ }
    }
    if letters >= 20 && ascii*100/len([]rune(lower)) > 90 {
        // presence of common English function words boosts confidence
        if strings.Contains(lower, " the ") || strings.Contains(lower, " and ") || strings.Contains(lower, " of ") || strings.Contains(lower, " to ") {
            return "en"
        }
    }
    return ""
}

// extractMainText performs a naive readability-style extraction.
func extractMainText(html string) string {
    pick := ""
    if m := reArticle.FindStringSubmatch(html); len(m) >= 2 { pick = m[1] }
    if pick == "" { if m := reMain.FindStringSubmatch(html); len(m) >= 2 { pick = m[1] } }
    if pick == "" { if m := reBody.FindStringSubmatch(html); len(m) >= 2 { pick = m[1] } }
    if pick == "" { pick = html }
    pick = reComments.ReplaceAllString(pick, " ")
    pick = reScript.ReplaceAllString(pick, " ")
    pick = reStyle.ReplaceAllString(pick, " ")
    pick = strings.ReplaceAll(pick, "<p", "\n<p")
    pick = strings.ReplaceAll(pick, "<div", "\n<div")
    pick = strings.ReplaceAll(pick, "<br", "\n<br")
    pick = reTags.ReplaceAllString(pick, " ")
    pick = stripSpaces(pick)
    pick = strings.TrimSpace(pick)
    if len(pick) > 4000 { pick = pick[:4000] }
    return pick
}

func firstGroup(m []string) string { if len(m) >= 2 { return m[1] }; return "" }
func stripSpaces(s string) string { return regexp.MustCompile(`\s+`).ReplaceAllString(s, " ") }
func hostOf(u string) string {
    if i := strings.Index(u, "://"); i >= 0 { u = u[i+3:] }
    if j := strings.Index(u, "/"); j >= 0 { u = u[:j] }
    if k := strings.Index(u, "#"); k >= 0 { u = u[:k] }
    return u
}

// Persistent frontier helpers
type frontierFile struct {
    Queue    []struct{ U string `json:"u"`; D int `json:"depth"` } `json:"queue"`
    Seen     []string `json:"seen"`
    LastRun  string   `json:"lastRun"`
}

func loadFrontier(path string, q *[]item, seen *map[string]bool) {
    b, err := os.ReadFile(path)
    if err != nil || len(b) == 0 { return }
    var f frontierFile
    if json.Unmarshal(b, &f) != nil { return }
    for _, s := range f.Seen { (*seen)[s] = true }
    for _, e := range f.Queue { *q = append(*q, item{u: e.U, depth: e.D}) }
}

func saveFrontier(path string, q []item, seen map[string]bool) {
    f := frontierFile{Queue: []struct{ U string `json:"u"`; D int `json:"depth"` }{}, Seen: []string{}, LastRun: time.Now().UTC().Format(time.RFC3339)}
    for _, it := range q { f.Queue = append(f.Queue, struct{ U string `json:"u"`; D int `json:"depth"` }{U: it.u, D: it.depth}) }
    for s := range seen { f.Seen = append(f.Seen, s) }
    _ = os.MkdirAll(dirOf(path), 0o755)
    if b, err := json.MarshalIndent(f, "", "  "); err == nil { _ = os.WriteFile(path, b, 0o644) }
}

func dirOf(p string) string {
    i := strings.LastIndex(p, "/")
    if i <= 0 { return "." }
    return p[:i]
}
func getenv(k, d string) string { if v := os.Getenv(k); v != "" { return v }; return d }
func atoi(s string) int { n := 0; for _, r := range s { if r<'0'||r>'9' { return n }; n = n*10 + int(r-'0') }; return n }
func absURL(baseStr, ref string) string {
    if strings.HasPrefix(ref, "data:") { return "" }
    bu, err := url.Parse(baseStr); if err != nil { return "" }
    ru, err := url.Parse(ref); if err != nil { return "" }
    return bu.ResolveReference(ru).String()
}
type policy struct{
    inited bool
    disallow []string
    allow    []string
    delay time.Duration
    sitemaps []string
}
func (p policy) allowed(u string) bool {
    if !p.inited { return true }
    // naive path prefix check
    path := "/"
    if uu, err := url.Parse(u); err == nil { path = uu.Path }
    // allow rules take precedence over disallow if more specific
    for _, a := range p.allow {
        if a == "" { continue }
        if strings.HasPrefix(path, a) { return true }
    }
    for _, d := range p.disallow {
        if d == "" { continue }
        if strings.HasPrefix(path, d) { return false }
    }
    return true
}
func fetchRobots(client *http.Client, host, agent string) policy {
    p := policy{inited: true}
    url := "http://"+host+"/robots.txt"
    req, _ := http.NewRequest("GET", url, nil)
    req.Header.Set("User-Agent", agent)
    resp, err := client.Do(req)
    if err != nil { return p }
    defer resp.Body.Close()
    b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
    lines := strings.Split(string(b), "\n")
    // Gather groups for our agent and for '*'
    currentUA := ""
    matchOur := false
    matchStar := false
    rulesOur := struct{ dis, allow []string; delay time.Duration }{}
    rulesStar := struct{ dis, allow []string; delay time.Duration }{}
    for _, ln := range lines {
        l := strings.TrimSpace(ln)
        if l == "" || strings.HasPrefix(l, "#") { continue }
        kv := strings.SplitN(l, ":", 2)
        if len(kv) != 2 { continue }
        key := strings.ToLower(strings.TrimSpace(kv[0]))
        val := strings.TrimSpace(kv[1])
        switch key {
        case "user-agent":
            currentUA = strings.ToLower(val)
            matchOur = (currentUA == strings.ToLower(agent))
            matchStar = (currentUA == "*")
        case "disallow":
            if matchOur { rulesOur.dis = append(rulesOur.dis, val) }
            if matchStar { rulesStar.dis = append(rulesStar.dis, val) }
        case "allow":
            if matchOur { rulesOur.allow = append(rulesOur.allow, val) }
            if matchStar { rulesStar.allow = append(rulesStar.allow, val) }
        case "crawl-delay":
            if matchOur { if d := atoi(val); d > 0 { rulesOur.delay = time.Duration(d) * time.Second } }
            if matchStar { if d := atoi(val); d > 0 { rulesStar.delay = time.Duration(d) * time.Second } }
        case "sitemap":
            p.sitemaps = append(p.sitemaps, val)
        }
    }
    // Prefer our agent rules if any, else star
    if len(rulesOur.dis) > 0 || len(rulesOur.allow) > 0 || rulesOur.delay > 0 {
        p.disallow = rulesOur.dis
        p.allow = rulesOur.allow
        p.delay = rulesOur.delay
    } else {
        p.disallow = rulesStar.dis
        p.allow = rulesStar.allow
        p.delay = rulesStar.delay
    }
    return p
}
func fetchSitemap(client *http.Client, u string) []string {
    req, _ := http.NewRequest("GET", u, nil)
    resp, err := client.Do(req)
    if err != nil { return nil }
    defer resp.Body.Close()
    var reader io.Reader = resp.Body
    // handle gzip
    if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "gzip") || strings.HasSuffix(strings.ToLower(u), ".gz") {
        // naive: try to ungzip
        gz, gzerr := gzip.NewReader(resp.Body)
        if gzerr == nil { defer gz.Close(); reader = gz }
    }
    b, _ := io.ReadAll(io.LimitReader(reader, 8<<20))
    // naive extract <loc> values
    re := regexp.MustCompile(`(?is)<loc>\s*([^<\s]+)\s*</loc>`)
    out := []string{}
    for _, m := range re.FindAllStringSubmatch(string(b), -1) {
        out = append(out, strings.TrimSpace(m[1]))
    }
    return out
}
func postJSON(u string, v any) {
    b, _ := json.Marshal(v)
    req, _ := http.NewRequest("POST", u, bytes.NewReader(b))
    req.Header.Set("Content-Type", "application/json")
    resp, err := http.DefaultClient.Do(req)
    if err != nil { log.Printf("post %s error: %v", u, err); return }
    defer resp.Body.Close()
    if resp.StatusCode >= 300 { out, _ := io.ReadAll(resp.Body); log.Printf("post %s failed: %s", u, string(out)) }
}
