package main

import (
    "io"
    "log"
    "net/http"
    "os"
    "os/exec"
    "strconv"
    "strings"
    "time"
)

// A lightweight scheduler that periodically runs the bundled crawler binary.
// Env:
//  - CRAWL_INTERVAL: duration like 30m, 1h (default 30m)
//  - SEED_URLS: comma-separated URLs passed through to crawler via env
func main() {
    intervalStr := getenv("CRAWL_INTERVAL", "30m")
    d, err := time.ParseDuration(intervalStr)
    if err != nil { d = 30 * time.Minute }
    log.Printf("search-scheduler: starting, interval=%s", d)
    ticker := time.NewTicker(d)
    defer ticker.Stop()
    // kick off immediately
    runOnce()
    for range ticker.C {
        runOnce()
    }
}

func runOnce() {
    log.Println("search-scheduler: running crawler")
    // Wait for search API to be reachable before spawning the crawler
    base := getenv("SEARCH_API_BASE", "http://search-service:8092")
    if !waitForSearch(base, 90*time.Second) {
        log.Printf("search-scheduler: search API not reachable at %s; skipping run", base)
        return
    }
    cmd := exec.Command("/app/search-crawler")
    // propagate MEILI_* and SEED_URLS
    env := os.Environ()
    cmd.Env = env
    // Retry the crawler up to 2 additional times on failure
    attempts := 0
    for {
        out, err := cmd.CombinedOutput()
        if err != nil {
            attempts++
            log.Printf("search-scheduler: crawler error (attempt %d): %v, out=%s", attempts, err, string(out))
            if attempts >= 3 { break }
            time.Sleep(15 * time.Second)
            continue
        }
        log.Printf("search-scheduler: crawler done, bytes=%s", strconv.Itoa(len(out)))
        break
    }
}

func getenv(k, def string) string { if v := os.Getenv(k); v != "" { return v }; return def }

func waitForSearch(base string, timeout time.Duration) bool {
    deadline := time.Now().Add(timeout)
    health := strings.TrimRight(base, "/") + "/v1/health"
    client := &http.Client{ Timeout: 5 * time.Second }
    for time.Now().Before(deadline) {
        resp, err := client.Get(health)
        if err == nil {
            io.Copy(io.Discard, resp.Body)
            resp.Body.Close()
            if resp.StatusCode/100 == 2 { return true }
        }
        time.Sleep(3 * time.Second)
    }
    return false
}
