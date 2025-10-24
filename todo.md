Search Engine TODOs and Tuning Plan

Context

- Current index uses simple tokenization with stopword removal, exact token matching, TF‑IDF scoring, field weighting (title > snippet), and exact phrase boosts using positional indexes.
- Crawler fetches pages politely (robots.txt, Crawl‑delay), follows sitemaps/links to a limited depth, and posts results on run completion.

Observed gaps (examples from testing)

- Query “doc” returns no results while “docs” does.
  - Root cause: no stemming/lemmatization; exact token match only.
- Query “sanae takichi” does not match Wikipedia article.
  - Possible causes:
    - Coverage: page not crawled yet (depth/limits/host budgets).
    - Spelling/variant: current engine lacks fuzzy matching and accent/variant handling.
    - Tokenization/language: case/diacritics/Unicode normalization not applied.

Near‑term improvements

1) Stemming/Lemmatization
   - Integrate a light stemmer (e.g., Porter/Snowball) per language for English first.
   - Map plural/singular (docs↔doc), verb forms, etc.

2) Prefix / Partial Matching
   - Add optional prefix queries for short tokens (e.g., doc*), or character n‑gram index.
   - Guard with length threshold to avoid noise.

3) Fuzzy Matching
   - Add edit‑distance (Levenshtein) fallback for low‑hit queries; boost near matches.
   - Consider normalized/ASCII‑folded versions for matching (café→cafe).

4) Unicode Normalization & Accent Folding
   - Normalize to NFC/NFKD and fold accents for matching while preserving display.

5) Query Parsing Enhancements
   - Better phrase handling with word boundaries; escape special characters.
   - Add synonyms/aliases (MDN↔Mozilla Docs, Docs↔Documentation) via a small dictionary.

6) Index Coverage & Freshness
   - Increase depth/limits or add domain budgets to ensure key pages like specific Wikipedia articles are crawled.
   - Switch crawler to batch uploads (e.g., every 100 pages) for quicker feedback.
   - Incremental re‑crawl scheduling and per‑URL freshness metadata.

7) Field Expansion
   - Index page body (cleaned main content) in addition to title/snippet for recall.
   - Add URL/path token boosts (e.g., /docs/).

8) Ranking Tuning
   - Adjust field weights (title, headings, body, URL).
   - Add recency boost and site/domain priors.

9) Language & Locale
   - Detect language per page; route stemmer/stopwords accordingly.
   - Add language filter/facet in UI.

10) Admin & Ops
   - Add per‑host/domain stats (pages fetched, last crawl, blocked by robots).
   - Add batch clear by host with preview of impact.

Action items

- [ ] Add stemming (English) in tokenizer; keep original for display.
- [ ] Add ASCII‑folded copy for matching; keep original for scoring/display.
- [ ] Add simple prefix/n‑gram index for short tokens (min length 3).
- [ ] Add optional fuzzy fallback (edit distance 1) for rare queries.
- [ ] Extend crawler to extract and index main body text (readability/boilerplate removal).
- [ ] Switch crawler to periodic batch upserts (every N pages) and final flush.
- [ ] Add per‑host/domain crawl stats in admin (pages, sitemaps, delay, last fetch).
- [ ] Add language detection and per‑language stopword lists.
- [ ] Add UI facet for language and a toggle for fuzzy/prefix matching.

Notes

- All new matching logic should be guarded behind flags/toggles to evaluate impact.
- Keep politeness and robots.txt compliance as a priority when expanding breadth.

