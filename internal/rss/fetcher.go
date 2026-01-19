// Package rss provides feed fetching and parsing.
package rss

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/bryan-buckman/infovore/internal/database"
	"github.com/bryan-buckman/infovore/internal/model"
	"github.com/mmcdole/gofeed"
)

// Note: sync is still used by Poller

// MinPollingIntervalMinutes is the minimum allowed interval.
const MinPollingIntervalMinutes = 15

// Fetcher handles RSS feed fetching.
type Fetcher struct {
	db     *database.DB
	parser *gofeed.Parser
}

// NewFetcher creates a new fetcher.
func NewFetcher(db *database.DB) *Fetcher {
	return &Fetcher{
		db:     db,
		parser: gofeed.NewParser(),
	}
}

// FetchFeed fetches and parses a single feed, storing new items.
// Returns the number of new items added.
func (f *Fetcher) FetchFeed(ctx context.Context, feed model.Feed) (int, error) {
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

	// Log successful fetch.
	log.Printf("Fetched %s: %d new items", feed.URL, newCount)

	return newCount, nil
}

// FetchAll fetches all feeds sequentially to avoid database locking.
// Returns a map of feed ID -> new item count.
func (f *Fetcher) FetchAll(ctx context.Context) (map[int64]int, error) {
	feeds, err := f.db.GetAllFeeds()
	if err != nil {
		return nil, err
	}

	results := make(map[int64]int)

	for _, feed := range feeds {
		// Check for context cancellation between feeds.
		select {
		case <-ctx.Done():
			log.Printf("FetchAll cancelled after %d feeds", len(results))
			return results, ctx.Err()
		default:
		}

		count, err := f.FetchFeed(ctx, feed)
		if err != nil {
			log.Printf("Failed to fetch %s: %v", feed.URL, err)
			continue
		}
		results[feed.ID] = count
	}

	return results, nil
}

// Poller runs continuous polling.
type Poller struct {
	fetcher  *Fetcher
	db       *database.DB
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewPoller creates a background poller.
func NewPoller(db *database.DB) *Poller {
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

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
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
