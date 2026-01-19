// Package model defines shared data structures.
package model

import "time"

// Folder represents a hierarchical folder for organizing feeds.
type Folder struct {
	ID       int64
	Name     string
	ParentID *int64 // nullable for root folders
}

// Feed represents an RSS/Atom feed subscription.
type Feed struct {
	ID          int64
	FolderID    *int64 // nullable if not in a folder
	Title       string
	URL         string
	IconURL     string
	LastFetched time.Time
}

// Item represents a single article/entry from a feed.
type Item struct {
	ID          int64
	FeedID      int64
	GUID        string // unique identifier from feed
	Title       string
	Content     string
	Link        string
	PublishedAt time.Time
	FetchedAt   time.Time
	IsRead      bool
}

// FolderWithFeeds represents a folder containing its feeds for UI rendering.
type FolderWithFeeds struct {
	Folder
	Feeds []Feed
}

// Settings key constants.
const (
	SettingPollingInterval = "polling_interval_minutes"
)
