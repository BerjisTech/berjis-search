# Custom Search Engine Project - Technical Specification

## Project Overview
Build a custom search engine with Go backend and Angular frontend, featuring web crawling, indexing, and full-text search capabilities.

---

## Backend (Go)

### Core Components

**1. Web Crawler**
- Implement concurrent crawling using goroutines
- Respect robots.txt and crawl delays
- Handle URL normalization and deduplication
- Support depth-limited crawling
- Implement polite crawling with rate limiting

**2. Indexer**
- Build inverted index data structure
- Implement tokenization and text preprocessing
- Support stemming/lemmatization (use Snowball stemmer)
- Calculate TF-IDF scores for ranking
- Store metadata (title, description, URL, timestamp)

**3. Search Query Processor**
- Parse search queries (boolean operators: AND, OR, NOT)
- Support phrase queries with quotes
- Implement query expansion and spell correction
- Handle stop words removal

**4. Ranking Algorithm**
- Implement TF-IDF scoring
- Add PageRank-style link analysis
- Consider document freshness
- Support custom relevance signals

**5. API Server**
- RESTful API using Gin or Echo framework
- Endpoints:
  - `POST /api/crawl` - Trigger crawling
  - `GET /api/search?q={query}&page={n}` - Search
  - `GET /api/suggest?q={query}` - Autocomplete
  - `GET /api/stats` - Index statistics
- CORS configuration for Angular frontend
- Request validation and error handling

### Data Storage

**Primary Database: PostgreSQL**
- Store crawled pages and metadata
- Maintain crawl queue and visited URLs
- Track index statistics

**Search Index: Elasticsearch or Bleve**
- Option 1: Elasticsearch for distributed search
- Option 2: Bleve (pure Go) for embedded search engine
- Full-text search with relevance scoring
- Faceted search support

**Cache Layer: Redis**
- Cache frequent queries
- Store autocomplete suggestions
- Rate limiting data

### Go Libraries/Packages
```go
// Core packages
"net/http"
"context"
"sync"

// Web framework
"github.com/gin-gonic/gin"

// HTML parsing
"golang.org/x/net/html"
"github.com/PuerkitoBio/goquery"

// Database
"github.com/lib/pq" // PostgreSQL
"gorm.io/gorm"

// Search (choose one)
"github.com/blevesearch/bleve/v2" // Pure Go
"github.com/elastic/go-elasticsearch/v8" // Elasticsearch

// Redis
"github.com/go-redis/redis/v8"

// Text processing
"github.com/kljensen/snowball" // Stemming
```

---

## Frontend (Angular)

### Components Structure

**1. Search Component**
- Search input with real-time validation
- Advanced search filters (date, domain, file type)
- Search suggestions/autocomplete
- Voice search integration (Web Speech API)

**2. Results Component**
- Paginated results display
- Result snippets with keyword highlighting
- Sorting options (relevance, date)
- Infinite scroll or load more button
- Result preview on hover

**3. Filters Sidebar**
- Date range filter
- Domain/site filter
- Content type filter (web, images, documents)
- Custom facets

**4. Stats Dashboard** (Admin)
- Total indexed pages
- Crawl statistics
- Popular queries
- System health metrics

**5. Settings Component**
- Crawl configuration
- Index management
- Search preferences

### Angular Features

**Services**
- `SearchService` - API communication
- `CrawlerService` - Manage crawling operations
- `AutocompleteService` - Suggestions
- `StateManagementService` - RxJS/NgRx for state

**Routing**
```typescript
/search?q={query}
/results
/admin/dashboard
/admin/crawler
/settings
```

**Key Packages**
```json
{
  "@angular/core": "^17.x",
  "@angular/router": "^17.x",
  "@angular/forms": "^17.x",
  "rxjs": "^7.x",
  "@ngrx/store": "^17.x", // Optional state management
  "ngx-infinite-scroll": "^17.x",
  "highlight.js": "^11.x" // Syntax highlighting
}
```

### UI/UX Features
- Responsive design (mobile-first)
- Dark mode support
- Keyboard shortcuts (/ to focus search)
- Loading skeletons
- Error states with retry
- Search history (localStorage)

---

## Additional Technologies

### DevOps & Infrastructure

**Containerization**
```dockerfile
# Docker for Go backend
# Multi-stage build for minimal image size
```

**Message Queue**
- RabbitMQ or Apache Kafka for crawl job distribution
- Handle async processing of large crawl tasks

**Monitoring**
- Prometheus + Grafana for metrics
- Structured logging with Zap (Go)
- ELK stack for log aggregation

**API Documentation**
- Swagger/OpenAPI specification
- Auto-generated docs with swaggo

---

## System Architecture Flow

```
User → Angular Frontend → API Gateway (Go)
                              ↓
                    ┌─────────┼─────────┐
                    ↓         ↓         ↓
                 Crawler   Search    Cache
                 Service   Service   (Redis)
                    ↓         ↓
              PostgreSQL  Elasticsearch/Bleve
```

---

## Implementation Priorities

**Phase 1: MVP**
1. Basic web crawler with URL queue
2. Simple inverted index in PostgreSQL
3. Basic search API
4. Angular search interface with results display

**Phase 2: Enhanced Features**
1. Integrate Elasticsearch/Bleve
2. Implement ranking algorithms
3. Add autocomplete and suggestions
4. Admin dashboard

**Phase 3: Advanced**
1. Distributed crawling
2. Machine learning for ranking
3. Image/video search
4. Personalization

---

## Performance Targets
- Index: 10,000+ pages per hour
- Search latency: <100ms (p95)
- Concurrent crawlers: 50-100
- Support: 1000+ queries per second

Would you like me to elaborate on any specific component or create starter code for any part of this system?