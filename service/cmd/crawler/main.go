package main

import (
    "crypto/sha1"
    "fmt"
    "io"
    "log"
    "net/http"
    "os"
    "regexp"
    "strings"
    "time"

    "github.com/berjistech/berjis-ecosystem/search/service/internal/config"
    search "github.com/berjistech/berjis-ecosystem/search/service/internal/search"
)

var (
    reTitle = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`) 
    reMeta  = regexp.MustCompile(`(?is)<meta\s+name="description"\s+content="([^"]*)"[^>]*>`) 
)

func main() {
    cfg := config.Load()
    meili := search.NewMeili(cfg.MeiliHost, cfg.MeiliAPIKey)
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
    var docs []search.WebDoc
    for _, u := range seeds {
        u = strings.TrimSpace(u)
        if u == "" { continue }
        title, desc := fetchTitleDesc(u)
        if title == "" { title = u }
        if desc == "" { desc = "Indexed by Berjis Crawler" }
        id := fmt.Sprintf("%x", sha1.Sum([]byte(u)))
        docs = append(docs, search.WebDoc{ID: id, Title: title, Url: u, Snippet: desc, Source: hostOf(u), Date: time.Now().UTC().Format(time.RFC3339)})
    }
    if err := meili.IndexWeb(docs); err != nil {
        log.Fatalf("index error: %v", err)
    }
    log.Printf("crawler: indexed %d docs", len(docs))
}

func fetchTitleDesc(u string) (string, string) {
    resp, err := http.Get(u)
    if err != nil { return "", "" }
    defer resp.Body.Close()
    b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
    html := string(b)
    title := firstGroup(reTitle.FindStringSubmatch(html))
    if title != "" { title = strings.TrimSpace(stripSpaces(title)) }
    desc := firstGroup(reMeta.FindStringSubmatch(html))
    if desc != "" { desc = strings.TrimSpace(stripSpaces(desc)) }
    return title, desc
}

func firstGroup(m []string) string { if len(m) >= 2 { return m[1] }; return "" }
func stripSpaces(s string) string { return regexp.MustCompile(`\s+`).ReplaceAllString(s, " ") }
func hostOf(u string) string { if i := strings.Index(u, "://"); i >= 0 { u = u[i+3:] }; if j := strings.Index(u, "/"); j >= 0 { u = u[:j] }; return u }
