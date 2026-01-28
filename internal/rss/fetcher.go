// Package rss provides feed fetching and parsing.
package rss

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"sync"
	"time"

	"github.com/bryan-buckman/infovore/internal/database"
	"github.com/bryan-buckman/infovore/internal/model"
	"github.com/mmcdole/gofeed"
)

// MinPollingIntervalMinutes is the minimum allowed interval.
const MinPollingIntervalMinutes = 15

// Concurrency settings
const (
	// MaxConcurrencyPostgres is the number of parallel fetches for PostgreSQL
	MaxConcurrencyPostgres = 10
	// MaxConcurrencySQLite is the number of parallel fetches for SQLite (limited due to locking)
	MaxConcurrencySQLite = 1
	// MaxConcurrencyPerDomain limits parallel requests to any single domain
	MaxConcurrencyPerDomain = 2
	// DelayBetweenDomainRequests is the minimum delay between requests to the same domain
	DelayBetweenDomainRequests = 500 * time.Millisecond
)

// domainLimiter controls rate limiting per domain to avoid overwhelming hosts.
type domainLimiter struct {
	mu          sync.Mutex
	semaphores  map[string]chan struct{}
	lastRequest map[string]time.Time
}

// newDomainLimiter creates a new per-domain rate limiter.
func newDomainLimiter() *domainLimiter {
	return &domainLimiter{
		semaphores:  make(map[string]chan struct{}),
		lastRequest: make(map[string]time.Time),
	}
}

// acquire gets a slot for the domain, blocking if necessary.
// It also enforces the minimum delay between requests to the same domain.
func (dl *domainLimiter) acquire(ctx context.Context, domain string) error {
	dl.mu.Lock()
	sem, ok := dl.semaphores[domain]
	if !ok {
		sem = make(chan struct{}, MaxConcurrencyPerDomain)
		dl.semaphores[domain] = sem
	}
	dl.mu.Unlock()

	// Acquire semaphore slot
	select {
	case sem <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}

	// Enforce delay between requests to same domain
	dl.mu.Lock()
	lastReq := dl.lastRequest[domain]
	dl.mu.Unlock()

	if !lastReq.IsZero() {
		elapsed := time.Since(lastReq)
		if elapsed < DelayBetweenDomainRequests {
			delay := DelayBetweenDomainRequests - elapsed
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				// Release the semaphore on cancel
				<-sem
				return ctx.Err()
			}
		}
	}

	return nil
}

// release returns a slot for the domain and records the request time.
func (dl *domainLimiter) release(domain string) {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	dl.lastRequest[domain] = time.Now()
	if sem, ok := dl.semaphores[domain]; ok {
		<-sem
	}
}

// extractDomain gets the host from a URL.
func extractDomain(feedURL string) string {
	u, err := url.Parse(feedURL)
	if err != nil {
		return feedURL // fallback to full URL
	}
	return u.Host
}

// Fetcher handles RSS feed fetching.
type Fetcher struct {
	db            database.Store
	parser        *gofeed.Parser
	concurrency   int
	domainLimiter *domainLimiter
}

// NewFetcher creates a new fetcher with concurrency based on database type.
func NewFetcher(db database.Store) *Fetcher {
	concurrency := MaxConcurrencySQLite
	if db.SupportsHighConcurrency() {
		concurrency = MaxConcurrencyPostgres
	}
	return &Fetcher{
		db:            db,
		parser:        gofeed.NewParser(),
		concurrency:   concurrency,
		domainLimiter: newDomainLimiter(),
	}
}

// FetchFeed fetches and parses a single feed, storing new items.
// Returns the number of new items added.
func (f *Fetcher) FetchFeed(ctx context.Context, feed model.Feed) (int, error) {
	// Apply per-domain rate limiting
	domain := extractDomain(feed.URL)
	if err := f.domainLimiter.acquire(ctx, domain); err != nil {
		return 0, fmt.Errorf("rate limit cancelled for %s: %w", feed.URL, err)
	}
	defer f.domainLimiter.release(domain)

	parsed, err := f.parser.ParseURLWithContext(feed.URL, ctx)
	if err != nil {
		// Record the error for UI display.
		errMsg := err.Error()
		if len(errMsg) > 200 {
			errMsg = errMsg[:200]
		}
		_ = f.db.UpdateFeedError(feed.ID, errMsg)
		return 0, fmt.Errorf("parse feed %s: %w", feed.URL, err)
	}

	// Update feed title from RSS if it differs and isn't just the URL.
	if parsed.Title != "" && parsed.Title != feed.Title && feed.Title == feed.URL {
		if err := f.db.UpdateFeedTitle(feed.ID, parsed.Title); err != nil {
			log.Printf("Error updating title for feed %d: %v", feed.ID, err)
		} else {
			log.Printf("Updated feed title: %s -> %s", feed.URL, parsed.Title)
		}
	}

	now := time.Now()
	newCount := 0
	for _, item := range parsed.Items {
		guid := item.GUID
		if guid == "" {
			guid = item.Link
		}
		if guid == "" {
			continue
		}
		pubDate := now
		if item.PublishedParsed != nil {
			pubDate = *item.PublishedParsed
		}
		dbItem := &model.Item{
			FeedID:      feed.ID,
			GUID:        guid,
			Title:       item.Title,
			Content:     item.Content,
			Link:        item.Link,
			PublishedAt: pubDate,
			FetchedAt:   now,
		}
		if dbItem.Content == "" {
			dbItem.Content = item.Description
		}
		_, isNew, err := f.db.AddItem(dbItem)
		if err != nil {
			log.Printf("Error adding item %s: %v", guid, err)
			continue
		}
		if isNew {
			newCount++
		}
	}

	// Update last fetched time (and clear any previous error).
	if err := f.db.UpdateFeedLastFetched(feed.ID, now); err != nil {
		log.Printf("Error updating last_fetched for feed %d: %v", feed.ID, err)
	}

	return newCount, nil
}

// FetchResult holds the result of fetching a single feed.
type FetchResult struct {
	FeedID   int64
	NewItems int
	Error    error
}

// FetchAll fetches all feeds with configurable concurrency.
// Uses parallel workers for PostgreSQL, sequential for SQLite.
// Returns a map of feed ID -> new item count.
func (f *Fetcher) FetchAll(ctx context.Context) (map[int64]int, error) {
	feeds, err := f.db.GetAllFeeds()
	if err != nil {
		return nil, err
	}

	if len(feeds) == 0 {
		return make(map[int64]int), nil
	}

	log.Printf("Fetching %d feeds with concurrency=%d", len(feeds), f.concurrency)

	// For sequential fetching (SQLite), use simple loop
	if f.concurrency <= 1 {
		return f.fetchSequential(ctx, feeds)
	}

	// For parallel fetching (PostgreSQL), use worker pool
	return f.fetchParallel(ctx, feeds)
}

// fetchSequential fetches feeds one at a time (for SQLite).
func (f *Fetcher) fetchSequential(ctx context.Context, feeds []model.Feed) (map[int64]int, error) {
	results := make(map[int64]int)

	for i, feed := range feeds {
		select {
		case <-ctx.Done():
			log.Printf("FetchAll cancelled after %d/%d feeds", i, len(feeds))
			return results, ctx.Err()
		default:
		}

		count, err := f.FetchFeed(ctx, feed)
		if err != nil {
			log.Printf("Failed to fetch %s: %v", feed.URL, err)
			continue
		}
		results[feed.ID] = count

		// Progress logging every 50 feeds
		if (i+1)%50 == 0 {
			log.Printf("Progress: %d/%d feeds fetched", i+1, len(feeds))
		}
	}

	return results, nil
}

// fetchParallel fetches feeds using a worker pool (for PostgreSQL).
func (f *Fetcher) fetchParallel(ctx context.Context, feeds []model.Feed) (map[int64]int, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex

	results := make(map[int64]int)
	feedChan := make(chan model.Feed, len(feeds))
	resultChan := make(chan FetchResult, len(feeds))

	// Start workers
	for i := 0; i < f.concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for feed := range feedChan {
				select {
				case <-ctx.Done():
					return
				default:
				}

				count, err := f.FetchFeed(ctx, feed)
				resultChan <- FetchResult{
					FeedID:   feed.ID,
					NewItems: count,
					Error:    err,
				}
			}
		}(i)
	}

	// Send feeds to workers
	go func() {
		for _, feed := range feeds {
			select {
			case <-ctx.Done():
				break
			case feedChan <- feed:
			}
		}
		close(feedChan)
	}()

	// Collect results in separate goroutine
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Process results
	completed := 0
	for result := range resultChan {
		if result.Error != nil {
			// Error already logged in FetchFeed
			continue
		}
		mu.Lock()
		results[result.FeedID] = result.NewItems
		completed++
		if completed%50 == 0 {
			log.Printf("Progress: %d/%d feeds fetched", completed, len(feeds))
		}
		mu.Unlock()
	}

	return results, nil
}

// Poller runs continuous polling.
type Poller struct {
	fetcher  *Fetcher
	db       database.Store
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewPoller creates a background poller.
func NewPoller(db database.Store) *Poller {
	return &Poller{
		fetcher:  NewFetcher(db),
		db:       db,
		stopChan: make(chan struct{}),
	}
}

// Start begins the polling loop.
func (p *Poller) Start() {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		for {
			interval, _ := p.db.GetPollingInterval()
			if interval < MinPollingIntervalMinutes {
				interval = MinPollingIntervalMinutes
			}
			log.Printf("Poller: Fetching all feeds (interval: %dm)", interval)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			results, err := p.fetcher.FetchAll(ctx)
			cancel()

			if err != nil {
				log.Printf("Poller error: %v", err)
			} else {
				total := 0
				for _, c := range results {
					total += c
				}
				log.Printf("Poller: Fetched %d new items from %d feeds", total, len(results))
			}

			select {
			case <-p.stopChan:
				return
			case <-time.After(time.Duration(interval) * time.Minute):
			}
		}
	}()
}

// Stop stops the poller gracefully.
func (p *Poller) Stop() {
	close(p.stopChan)
	p.wg.Wait()
}
