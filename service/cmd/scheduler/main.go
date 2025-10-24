package main

import (
    "log"
    "os"
    "time"
    "os/exec"
    "strconv"
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
    cmd := exec.Command("/app/search-crawler")
    // propagate MEILI_* and SEED_URLS
    env := os.Environ()
    cmd.Env = env
    out, err := cmd.CombinedOutput()
    if err != nil {
        log.Printf("search-scheduler: crawler error: %v, out=%s", err, string(out))
        return
    }
    log.Printf("search-scheduler: crawler done, bytes=%s", strconv.Itoa(len(out)))
}

func getenv(k, def string) string { if v := os.Getenv(k); v != "" { return v }; return def }

