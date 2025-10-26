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

$domains='allafrica.com,africanews.com,thenewhumanitarian.org,africareport.com,theelephant.info,dailymaverick.co.za,news24.com,mg.co.za,nation.africa,standardmedia.co.ke,premiumtimesng.com,punchng.com,vanguardngr.com,thecable.ng,egyptindependent.com,almasryalyoum.com,middleeasteye.net,theconversation.com,chronicle.co.zw,monitor.co.ug,newtimes.co.rw,ghanaiantimes.com.gh,graphic.com.gh,au.int,afdb.org,uneca.org,ecowas.int,comesa.int,sadc.int,gov.za,kenya.go.ke,gov.ng,egypt.gov.eg,gov.et,gov.gh,gov.rw,jumia.com.ng,jumia.co.ke,jumia.com.eg,takealot.com,konga.com,jiji.ng,jiji.co.ke,jiji.co.tz,tonaton.com,cheki.co.ke,cars.co.za,property24.com,privateproperty.co.za,africanacademy.africa,codesria.org,universityworldnews.com,universitiesafrica.org,uct.ac.za,wits.ac.za,uonbi.ac.ke,ui.edu.ng,cu.edu.eg,aau.edu.et,techcabal.com,disrupt-africa.com,venturesafrica.com,technext24.com,techpoint.africa,benjamindada.com,weetracker.com,businessday.ng,businessdailyafrica.com,bd.co.za,african.business,howwemadeitinafrica.com,ventures-africa.com,africantouristboard.com,safaribookings.com,magicalkenya.com,southafrica.net,experienceegypt.eg,rwandatourism.com,cafonline.com,completesports.com,kickoff.com,supersport.com,goal.com,okayafrica.com,africafilms.tv,nollywoodgists.com,musicinafrica.net,africanhiphop.com,africahealthnews.com,afro.who.int,africacdc.org,farmingfirst.org,cta.int,afaas-africa.org,jobberman.com,brightermonday.co.ke,careers24.com,myjobmag.com,fuzu.com,nairaland.com,africahunt.com,skyscrapercity.com,bbc.com,rfi.fr,jeune-afrique.com'; $arr=$domains -split ',' | ForEach-Object { $_.Trim().ToLower() } | Where-Object { $_ } | Select-Object -Unique; foreach($d in $arr) { $name='search-scheduler-claude-'+($d -replace '[:\.]','--'); if(-not(docker ps -a --format '{{.Names}}' | Select-String -SimpleMatch $name)){ docker compose run -d --no-deps --name $name -e SEARCH_API_BASE=http://search-service:8092 -e CRAWL_INTERVAL=3h -e SEED_URLS="https://$d" -e SAME_DOMAIN_ONLY=true -e MAX_PAGES=4000 -e MAX_DEPTH=3 -e MAX_PER_HOST=800 -e MAX_WORKERS=16 search-scheduler | Out-Null } }; docker ps --format 'table {{.Names}}\t{{.Status}}' | Where-Object { $_ -match 'search-scheduler-(deep|claude-)' }

$doamins='allAfrica.com,News24.com,Nation.africa,TheEastAfrican.co.ke,DailyMaverick.co.za,Punchng.com,Vanguardngr.com,Guardian.ng,Standardmedia.co.ke,CitizenDigital.co.ke,TimesLive.co.za,IOL.co.za,MailandGuardian.co.za,GhanaWeb.com,MyJoyOnline.com,TheChronicle.com.gh,DailyMonitor.co.ug,NewVision.co.ug,Herald.co.zw,Chronicle.co.zw,AfricaNews.com,Zimeye.net,BusinessDay.ng,ThisDayLive.com,TheCable.ng,Leadership.ng,EWN.co.za,SowetanLive.co.za,Capitalfm.co.ke,KBC.co.ke,K24TV.co.ke,Jumia.co.ke,Jumia.com.ng,Kilimall.co.ke,Takealot.com,Bidorbuy.co.za,Jiji.ng,Jiji.ke,Cheki.co.ke,PigiaMe.co.ke,Tonaton.com,OLX.co.za,MallforAfrica.com,Konga.com,Zasttra.com,PriceCheck.co.za,Zando.co.za,Yuppiechef.com,Shopit.co.ke,Sky.Garden,Masoko.com,Techweez.com,TechCabal.com,TechCrunchAfrica.com,TechTrendsKE.co.ke,DigestAfrica.com,DisruptAfrica.com,InnovationVillage.co.ug,Techzim.co.zw,Techpoint.africa,ITWeb.co.za,MyBroadband.co.za,BusinessTech.co.za,TechArena.co.ke,AppsAfrica.com,Gadget.co.za,TechInAfrica.com,StartupAfrica.news,Ventureburn.com,TechSafariAfrica.com,KenyanWallStreet.com,OkayAfrica.com,Ghafla.co.ke,Niaje.com,Naibuzz.com,Hapakenya.com,Potentash.com,Brostalk.com,NairobiWire.com,BellaNaija.com,LindaIkejiBlog.com,ZAlebs.com,HowAfrica.com,Pulse.ng,PulseLive.co.ke,MTVBaseAfrica.com,TheNativeMag.com,NotJustOk.com,AmeyawDebrah.com,YNaija.com,TrendyAfrica.com,UniversityWorldNews.com,Tuko.co.ke,Kenya.go.ke,Gov.za,Education.go.ug,UniversityofNairobi.ac.ke,Makerere.ac.ug,UCT.ac.za,Unisa.ac.za,MESTAfrica.org,Ashesi.edu.gh,AfricanUnion.org,NEPAD.org,AU.int,UN.org/Africa,Amref.org,RedCross.or.ke,SafaricomFoundation.org,Africacheck.org,StandardBank.co.za'; $arr=$domains -split ',' | ForEach-Object { $_.Trim().ToLower() } | Where-Object { $_ } | Select-Object -Unique; foreach($d in $arr) { $name='search-scheduler-claude-'+($d -replace '[:\.]','--'); if(-not(docker ps -a --format '{{.Names}}' | Select-String -SimpleMatch $name)){ docker compose run -d --no-deps --name $name -e SEARCH_API_BASE=http://search-service:8092 -e CRAWL_INTERVAL=3h -e SEED_URLS="https://$d" -e SAME_DOMAIN_ONLY=true -e MAX_PAGES=4000 -e MAX_DEPTH=3 -e MAX_PER_HOST=800 -e MAX_WORKERS=16 search-scheduler | Out-Null } }; docker ps --format 'table {{.Names}}\t{{.Status}}' | Where-Object { $_ -match 'search-scheduler-(deep|claude-)' }

$domains='kenyaadultblog.com,kenyatalk.com,lughayangu.com,dailymotion.com,mdundo.com,youtube.com,allafrica.com,africanews.com,thenewhumanitarian.org,africareport.com,theelephant.info,dailymaverick.co.za,news24.com,mg.co.za,nation.africa,standardmedia.co.ke';

$domains='premiumtimesng.com,punchng.com,vanguardngr.com,thecable.ng,egyptindependent.com,almasryalyoum.com,middleeasteye.net,theconversation.com,chronicle.co.zw,monitor.co.ug,newtimes.co.rw,ghanaiantimes.com.gh,graphic.com.gh,au.int,afdb.org,uneca.org,ecowas.int';

$domains='comesa.int,sadc.int,gov.za,kenya.go.ke,gov.ng,egypt.gov.eg,gov.et,gov.gh,gov.rw,jumia.com.ng,jumia.co.ke,jumia.com.eg,takealot.com,konga.com,jiji.ng,jiji.co.ke,jiji.co.tz,tonaton.com,cheki.co.ke,cars.co.za,property24.com,privateproperty.co.za,africanacademy.africa';

$domains='codesria.org,universityworldnews.com,universitiesafrica.org,uct.ac.za,wits.ac.za,uonbi.ac.ke,ui.edu.ng,cu.edu.eg,aau.edu.et,techcabal.com,disrupt-africa.com,venturesafrica.com,technext24.com,techpoint.africa,benjamindada.com,weetracker.com,businessday.ng';

$domains='businessdailyafrica.com,bd.co.za,african.business,howwemadeitinafrica.com,ventures-africa.com,africantouristboard.com,safaribookings.com,magicalkenya.com,southafrica.net,experienceegypt.eg,rwandatourism.com,cafonline.com,completesports.com,kickoff.com';

$domains='supersport.com,goal.com,okayafrica.com,africafilms.tv,nollywoodgists.com,musicinafrica.net,africanhiphop.com,africahealthnews.com,afro.who.int,africacdc.org,farmingfirst.org,cta.int,afaas-africa.org,jobberman.com,brightermonday.co.ke,careers24.com,myjobmag.com,fuzu.com';

$domains='nairaland.com,africahunt.com,skyscrapercity.com,bbc.com,rfi.fr,jeune-afrique.com,theeastafrican.co.ke,guardian.ng,citizendigital.co.ke,timeslive.co.za,iol.co.za,mailandguardian.co.za,ghanaweb.com,myjoyonline.com,thechronicle.com.gh,dailymonitor.co.ug,newvision.co.ug';

$domains='herald.co.zw,zimeye.net,thisdaylive.com,leadership.ng,ewn.co.za,sowetanlive.co.za,capitalfm.co.ke,kbc.co.ke,k24tv.co.ke,kilimall.co.ke,bidorbuy.co.za,jiji.ke,pigiame.co.ke,olx.co.za,mallforafrica.com,zasttra.com,pricecheck.co.za,zando.co.za,yuppiechef.com,shopit.co.ke';

$domains='sky.garden,masoko.com,techweez.com,techcrunchafrica.com,techtrendske.co.ke,digestafrica.com,innovationvillage.co.ug,techzim.co.zw,itweb.co.za,mybroadband.co.za,businesstech.co.za,techarena.co.ke,appsafrica.com,gadget.co.za,techinafrica.com,startupafrica.news';

$domains='ventureburn.com,techsafariafrica.com,kenyanwallstreet.com,ghafla.co.ke,niaje.com,naibuzz.com,hapakenya.com,potentash.com,brostalk.com,nairobiwire.com,bellanaija.com,lindaikejiblog.com,zalebs.com,howafrica.com,pulse.ng,pulselive.co.ke,mtvbaseafrica.com,thenativemag.com';

$domains='notjustok.com,ameyawdebrah.com,ynaija.com,trendyafrica.com,tuko.co.ke,education.go.ug,universityofnairob'; 


$arr=$domains -split ',' | ForEach-Object { $_.Trim().ToLower() } | Where-Object { $_ } | Select-Object -Unique; foreach($d in $arr) { $name='search-scheduler-claude-'+($d -replace '[:\.]','--'); if(-not(docker ps -a --format '{{.Names}}' | Select-String -SimpleMatch $name)){ docker compose run -d --no-deps --name $name -e SEARCH_API_BASE=http://search-service:8092 -e RUN_CRAWLER=true -e RUN_PROVIDERS=false -e CRAWL_INTERVAL=3h -e SEED_URLS="https://$d" -e SAME_DOMAIN_ONLY=true -e MAX_PAGES=4000 -e MAX_DEPTH=3 -e MAX_PER_HOST=800 -e MAX_WORKERS=16 search-scheduler | Out-Null } }; Start-Sleep -Seconds 25200; foreach($d in $arr) { $name='search-scheduler-claude-'+($d -replace '[:\.]','--'); docker stop $name | Out-Null; docker rm $name | Out-Null }; $headers=@{'Content-Type'='application/json'}; $body='{"from":"disk"}'; foreach($v in 'web','images','videos','news') { Invoke-WebRequest -UseBasicParsing -Method Post -Headers $headers -Body $body -Uri "http://localhost:8092/v1/admin/index/$v/reindex" | Out-Host }; foreach($v in 'web','images','videos','news') { (Invoke-WebRequest -UseBasicParsing "http://localhost:8092/v1/admin/index/$v/docs?limit=1").Content }