package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/berjistech/berjis-ecosystem/search/service/internal/config"
	"github.com/berjistech/berjis-ecosystem/search/service/internal/searchindex"
)

type HostStat struct {
	PagesFetched      int
	BlockedByRobots   int
	CrawlDelaySeconds int
	SitemapCount      int
	LastFetch         string
}

type CrawlerStats struct {
	Hosts     map[string]HostStat `json:"hosts"`
	Timestamp string              `json:"timestamp"`
}

type Synonyms map[string][]string
type Priors map[string]float64
type Weights map[string]float64

type serverState struct {
	cfg        config.Config
	store      *searchindex.Store
	persistDir string

	statsPath    string
	synonymsPath string
	priorsPath   string
	weightsPath  string

	muWeb    sync.Mutex
	muImages sync.Mutex
	muVideos sync.Mutex
    muNews   sync.Mutex
    muTransport sync.Mutex

	dirtyWeb    bool
	dirtyImages bool
	dirtyVideos bool
    dirtyNews   bool
    dirtyTransport bool

	crawlStats CrawlerStats
	synonyms   Synonyms
	priors     Priors
	weights    Weights
}

func newServerState(cfg config.Config, store *searchindex.Store) *serverState {
	state := &serverState{
		cfg:        cfg,
		store:      store,
		persistDir: cfg.PersistDir,
	}

	state.statsPath = filepath.Join(state.persistDir, "crawler-stats.json")
	state.synonymsPath = filepath.Join(state.persistDir, "synonyms.json")
	state.priorsPath = filepath.Join(state.persistDir, "priors.json")
	state.weightsPath = filepath.Join(state.persistDir, "weights.json")

	state.crawlStats = CrawlerStats{Hosts: map[string]HostStat{}, Timestamp: ""}
	state.synonyms = Synonyms{}
	state.priors = Priors{}
	state.weights = Weights{}

	state.loadIndexes()
	state.loadMetadata()
	state.startAutosave()

	return state
}

func (s *serverState) loadIndexes() {
    _ = s.store.Web.Load(s.snapshotPath("web.json"))
    _ = s.store.Images.Load(s.snapshotPath("images.json"))
    _ = s.store.Videos.Load(s.snapshotPath("videos.json"))
    _ = s.store.News.Load(s.snapshotPath("news.json"))
    _ = s.store.Transport.Load(s.snapshotPath("transport.json"))
}

func (s *serverState) loadMetadata() {
	if b, err := os.ReadFile(s.statsPath); err == nil {
		if err := json.Unmarshal(b, &s.crawlStats); err != nil {
			s.crawlStats = CrawlerStats{Hosts: map[string]HostStat{}, Timestamp: ""}
		} else if s.crawlStats.Hosts == nil {
			s.crawlStats.Hosts = map[string]HostStat{}
		}
	}

	if b, err := os.ReadFile(s.synonymsPath); err == nil {
		_ = json.Unmarshal(b, &s.synonyms)
	}
	searchindex.SetSynonyms(s.synonyms)

	if b, err := os.ReadFile(s.priorsPath); err == nil {
		_ = json.Unmarshal(b, &s.priors)
	}
	searchindex.SetDomainPriors(s.priors)

	if b, err := os.ReadFile(s.weightsPath); err == nil {
		_ = json.Unmarshal(b, &s.weights)
	}
	if len(s.weights) > 0 {
		searchindex.SetWeights(s.weights)
	}
}

func (s *serverState) startAutosave() {
	interval := 30 * time.Second
	if env := os.Getenv("SAVE_INTERVAL_SEC"); env != "" {
		if v, err := strconv.Atoi(env); err == nil && v > 0 {
			interval = time.Duration(v) * time.Second
		}
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
        for range ticker.C {
            s.flushDirty(&s.muWeb, &s.dirtyWeb, s.store.Web, "web.json")
            s.flushDirty(&s.muImages, &s.dirtyImages, s.store.Images, "images.json")
            s.flushDirty(&s.muVideos, &s.dirtyVideos, s.store.Videos, "videos.json")
            s.flushDirty(&s.muNews, &s.dirtyNews, s.store.News, "news.json")
            s.flushDirty(&s.muTransport, &s.dirtyTransport, s.store.Transport, "transport.json")
        }
    }()
}

func (s *serverState) flushDirty(mu *sync.Mutex, dirty *bool, ix *searchindex.Index, filename string) {
	mu.Lock()
	if !*dirty {
		mu.Unlock()
		return
	}
	s.saveIndexSnapshot(ix, filename)
	*dirty = false
	mu.Unlock()
}

func (s *serverState) ensurePersistDir() {
	if s.persistDir == "" {
		return
	}
	_ = os.MkdirAll(s.persistDir, 0o755)
}

func (s *serverState) saveIndexSnapshot(ix *searchindex.Index, filename string) {
	s.ensurePersistDir()
	_ = ix.Save(s.snapshotPath(filename))
}

func (s *serverState) saveCrawlerStats() {
	s.ensurePersistDir()
	if b, err := json.MarshalIndent(s.crawlStats, "", "  "); err == nil {
		_ = os.WriteFile(s.statsPath, b, 0o644)
	}
}

func (s *serverState) saveSynonyms() {
	s.ensurePersistDir()
	if b, err := json.MarshalIndent(s.synonyms, "", "  "); err == nil {
		_ = os.WriteFile(s.synonymsPath, b, 0o644)
	}
}

func (s *serverState) savePriors() {
	s.ensurePersistDir()
	if b, err := json.MarshalIndent(s.priors, "", "  "); err == nil {
		_ = os.WriteFile(s.priorsPath, b, 0o644)
	}
}

func (s *serverState) saveWeights() {
	s.ensurePersistDir()
	if b, err := json.MarshalIndent(s.weights, "", "  "); err == nil {
		_ = os.WriteFile(s.weightsPath, b, 0o644)
	}
}

func (s *serverState) snapshotPath(name string) string {
	return filepath.Join(s.persistDir, name)
}

func (s *serverState) resourcesFor(vertical string) (*searchindex.Index, *sync.Mutex, *bool, string, bool) {
    switch vertical {
    case "web":
        return s.store.Web, &s.muWeb, &s.dirtyWeb, "web.json", true
    case "images":
        return s.store.Images, &s.muImages, &s.dirtyImages, "images.json", true
    case "videos":
        return s.store.Videos, &s.muVideos, &s.dirtyVideos, "videos.json", true
    case "news":
        return s.store.News, &s.muNews, &s.dirtyNews, "news.json", true
    case "transport":
        return s.store.Transport, &s.muTransport, &s.dirtyTransport, "transport.json", true
    default:
        return nil, nil, nil, "", false
    }
}
