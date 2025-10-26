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

// A lightweight scheduler that periodically runs the bundled crawler and/or video providers.
// Env:
//  - CRAWL_INTERVAL: duration like 30m, 1h (default 30m)
//  - RUN_CRAWLER: true/false (default true)
//  - RUN_PROVIDERS: true/false (default false; or set VIDEO_PROVIDERS=true)
//  - SEED_URLS/VIDEO_SEEDS passed to crawler; YT_*/DM_* passed to providers.
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
    // Wait for search API to be reachable before spawning jobs
    base := getenv("SEARCH_API_BASE", "http://search-service:8092")
    if !waitForSearch(base, 90*time.Second) {
        log.Printf("search-scheduler: search API not reachable at %s; skipping run", base)
        return
    }

    runCrawler := strings.ToLower(getenv("RUN_CRAWLER", "true")) == "true"
    runProviders := strings.ToLower(getenv("RUN_PROVIDERS", getenv("VIDEO_PROVIDERS", "false"))) == "true"

    if runCrawler {
        log.Println("search-scheduler: running crawler")
        cmd := exec.Command("/app/search-crawler")
        cmd.Env = os.Environ()
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

    if runProviders {
        log.Println("search-scheduler: running video providers fetcher")
        vcmd := exec.Command("/app/search-video-providers")
        vcmd.Env = os.Environ()
        vout, verr := vcmd.CombinedOutput()
        if verr != nil {
            log.Printf("search-scheduler: video providers error: %v out=%s", verr, string(vout))
        } else {
            log.Printf("search-scheduler: video providers done, bytes=%s", strconv.Itoa(len(vout)))
        }
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
