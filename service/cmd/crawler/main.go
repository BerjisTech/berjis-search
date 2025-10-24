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
)

var (
    reTitle    = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
    reMetaDesc = regexp.MustCompile(`(?is)<meta\s+(?:name|property)=["']description["']\s+content=["']([^"']*)["'][^>]*>`) 
    reMetaOGImg  = regexp.MustCompile(`(?is)<meta\s+property=["']og:image["']\s+content=["']([^"']+)["'][^>]*>`) 
    reMetaOGVid  = regexp.MustCompile(`(?is)<meta\s+property=["']og:video["']\s+content=["']([^"']+)["'][^>]*>`) 
    reMetaOGType = regexp.MustCompile(`(?is)<meta\s+property=["']og:type["']\s+content=["']([^"']+)["'][^>]*>`) 
    reMetaArtPub = regexp.MustCompile(`(?is)<meta\s+property=["']article:published_time["']\s+content=["']([^"']+)["'][^>]*>`) 
    reLinks    = regexp.MustCompile(`(?is)<a\s+[^>]*href=["']([^"']+)["'][^>]*>`) 
    reImgTags  = regexp.MustCompile(`(?is)<img\s+[^>]*src=["']([^"']+)["'][^>]*>`) 
)

type WebDoc struct { ID, Title, Url, Snippet, Source, Date string }

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
    // Frontier crawl
    maxPages := atoi(getenv("MAX_PAGES", "50"))
    maxDepth := atoi(getenv("MAX_DEPTH", "2"))
    sameDomain := getenv("SAME_DOMAIN_ONLY", "true") == "true"
    allowed := map[string]bool{}
    for _, s := range seeds { if h := hostOf(strings.TrimSpace(s)); h != "" { allowed[h] = true } }
    type item struct{ u string; depth int }
    q := []item{}
    seen := map[string]bool{}
    for _, s := range seeds { s = strings.TrimSpace(s); if s != "" { q = append(q, item{u: s, depth: 0}) } }
    webBatch := []WebDoc{}
    imgBatch := []map[string]string{}
    vidBatch := []map[string]string{}
    newsBatch := []map[string]string{}
    count := 0
    hostPolicy := map[string]policy{}
    hostNext := map[string]time.Time{}
    hostCount := map[string]int{}
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
        if !pol.allowed(u) { continue }
        // per-host crawl delay
        if nxt := hostNext[h]; time.Now().Before(nxt) {
            time.Sleep(nxt.Sub(time.Now()))
        }
        if maxPerHost > 0 && hostCount[h] >= maxPerHost { continue }
        html, status := fetchHTML(client, ua, u)
        if status >= 400 || html == "" { continue }
        if pol.delay > 0 { hostNext[h] = time.Now().Add(pol.delay) }
        hostCount[h] = hostCount[h] + 1
        title, desc := extractTitleDesc(html)
        if title == "" { title = u }
        if desc == "" { desc = "Indexed by Berjis Crawler" }
        now := time.Now().UTC().Format(time.RFC3339)
        id := fmt.Sprintf("%x", sha1.Sum([]byte(u)))
        webBatch = append(webBatch, WebDoc{ID: id, Title: title, Url: u, Snippet: desc, Source: hostOf(u), Date: now})
        // OG image/video
        if img := firstGroup(reMetaOGImg.FindStringSubmatch(html)); img != "" {
            ai := absURL(u, img)
            imgBatch = append(imgBatch, map[string]string{"id": fmt.Sprintf("%x", sha1.Sum([]byte(ai))), "title": title, "thumbnailUrl": ai, "imageUrl": ai, "source": hostOf(ai), "date": now})
        }
        if vid := firstGroup(reMetaOGVid.FindStringSubmatch(html)); vid != "" {
            av := absURL(u, vid)
            vidBatch = append(vidBatch, map[string]string{"id": fmt.Sprintf("%x", sha1.Sum([]byte(av))), "title": title, "url": av, "duration": "", "source": hostOf(av), "date": now})
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
    // Bulk POST
    postJSON(base+"/v1/admin/index/web", map[string]any{"docs": webBatch})
    if len(imgBatch) > 0 { postJSON(base+"/v1/admin/index/images", map[string]any{"docs": imgBatch}) }
    if len(vidBatch) > 0 { postJSON(base+"/v1/admin/index/videos", map[string]any{"docs": vidBatch}) }
    if len(newsBatch) > 0 { postJSON(base+"/v1/admin/index/news", map[string]any{"docs": newsBatch}) }
    log.Printf("crawler: indexed web=%d images=%d videos=%d news=%d (visited=%d)", len(webBatch), len(imgBatch), len(vidBatch), len(newsBatch), len(seen))
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

func firstGroup(m []string) string { if len(m) >= 2 { return m[1] }; return "" }
func stripSpaces(s string) string { return regexp.MustCompile(`\s+`).ReplaceAllString(s, " ") }
func hostOf(u string) string { if i := strings.Index(u, "://"); i >= 0 { u = u[i+3:] }; if j := strings.Index(u, "/"); j >= 0 { u = u[:j] }; return u }
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
