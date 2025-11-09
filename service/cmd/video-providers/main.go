package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type YTSearchResponse struct {
	Items []struct {
		ID struct {
			VideoID string `json:"videoId"`
		} `json:"id"`
		Snippet struct {
			Title       string    `json:"title"`
			PublishedAt time.Time `json:"publishedAt"`
			ChannelID   string    `json:"channelId"`
		} `json:"snippet"`
	} `json:"items"`
}

type DMSearchResponse struct {
	List []struct {
		Title    string `json:"title"`
		URL      string `json:"url"`
		Duration int    `json:"duration"`
		Thumb    string `json:"thumbnail_url"`
		// created_time is UNIX timestamp
		Created int64 `json:"created_time"`
	} `json:"list"`
}

func main() {
	base := getenv("SEARCH_API_BASE", "http://search-service:8092")

	var videos []map[string]string

	if ytKey := strings.TrimSpace(os.Getenv("YT_API_KEY")); ytKey != "" {
		// Queries
		for _, q := range splitCSV(os.Getenv("YT_QUERIES")) {
			vids := fetchYouTubeSearch(ytKey, q, "")
			videos = append(videos, vids...)
		}
		// Channels
		for _, ch := range splitCSV(os.Getenv("YT_CHANNEL_IDS")) {
			vids := fetchYouTubeSearch(ytKey, "", ch)
			videos = append(videos, vids...)
		}
	}

	// Dailymotion (no key required for basic search)
	for _, q := range splitCSV(os.Getenv("DM_QUERIES")) {
		vids := fetchDailymotionSearch(q)
		videos = append(videos, vids...)
	}

	// Post in batches of 100
	if len(videos) > 0 {
		for i := 0; i < len(videos); i += 100 {
			j := i + 100
			if j > len(videos) {
				j = len(videos)
			}
			postJSON(base+"/v1/admin/index/videos", map[string]any{"docs": videos[i:j]})
		}
		log.Printf("video-providers: upserted videos=%d", len(videos))
	} else {
		log.Printf("video-providers: no videos fetched (check API keys/queries)")
	}
}

func fetchYouTubeSearch(apiKey, q, channelID string) []map[string]string {
	// Build URL
	params := url.Values{}
	params.Set("key", apiKey)
	params.Set("part", "snippet")
	params.Set("type", "video")
	params.Set("maxResults", "25")
	if q != "" {
		params.Set("q", q)
	}
	if channelID != "" {
		params.Set("channelId", channelID)
	}
	u := "https://www.googleapis.com/youtube/v3/search?" + params.Encode()
	resp, err := http.Get(u)
	if err != nil {
		log.Printf("youtube search error: %v", err)
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(resp.Body)
		log.Printf("youtube search %d: %s", resp.StatusCode, string(b))
		return nil
	}
	var out YTSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		log.Printf("youtube decode: %v", err)
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res := []map[string]string{}
	for _, it := range out.Items {
		if it.ID.VideoID == "" {
			continue
		}
		url := "https://www.youtube.com/watch?v=" + it.ID.VideoID
		date := now
		if !it.Snippet.PublishedAt.IsZero() {
			date = it.Snippet.PublishedAt.UTC().Format(time.RFC3339)
		}
		res = append(res, map[string]string{
			"id":           fmt.Sprintf("%x", sha1.Sum([]byte(url))),
			"title":        it.Snippet.Title,
			"url":          url,
			"duration":     "",
			"thumbnailUrl": fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", it.ID.VideoID),
			"source":       hostOf(url),
			"date":         date,
		})
	}
	return res
}

func fetchDailymotionSearch(q string) []map[string]string {
	if strings.TrimSpace(q) == "" {
		return nil
	}
	v := url.Values{}
	v.Set("search", q)
	v.Set("limit", "25")
	v.Set("fields", "title,url,duration,created_time,thumbnail_url")
	u := "https://api.dailymotion.com/videos?" + v.Encode()
	resp, err := http.Get(u)
	if err != nil {
		log.Printf("dailymotion search error: %v", err)
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(resp.Body)
		log.Printf("dailymotion search %d: %s", resp.StatusCode, string(b))
		return nil
	}
	var out DMSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		log.Printf("dailymotion decode: %v", err)
		return nil
	}
	res := []map[string]string{}
	for _, it := range out.List {
		url := it.URL
		date := time.Unix(it.Created, 0).UTC().Format(time.RFC3339)
		res = append(res, map[string]string{
			"id":           fmt.Sprintf("%x", sha1.Sum([]byte(url))),
			"title":        it.Title,
			"url":          url,
			"duration":     fmt.Sprintf("%ds", it.Duration),
			"thumbnailUrl": it.Thumb,
			"source":       hostOf(url),
			"date":         date,
		})
	}
	return res
}

func postJSON(u string, v any) {
	b, _ := json.Marshal(v)
	req, err := http.NewRequest("POST", u, bytes.NewReader(b))
	if err != nil {
		log.Printf("post %s newrequest error: %v", u, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("post %s error: %v", u, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		out, _ := io.ReadAll(resp.Body)
		log.Printf("post %s failed: %s", u, string(out))
	}
}

func splitCSV(s string) []string {
	parts := []string{}
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func hostOf(u string) string {
	pu, err := url.Parse(u)
	if err != nil {
		return ""
	}
	return pu.Host
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
