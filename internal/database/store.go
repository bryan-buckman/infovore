// Package database provides storage backends for the RSS reader.
package database

import (
	"time"

	"github.com/bryan-buckman/infovore/internal/model"
)

// ScanLogEntry represents a single Kalshi scan log record.
type ScanLogEntry struct {
	ID        int64
	Output    string
	Error     string
	CreatedAt time.Time
}

// Store defines the interface for database operations.
type Store interface {
	Close() error

	// DatabaseType returns the name of the database backend ("SQLite" or "PostgreSQL").
	DatabaseType() string

	// SupportsHighConcurrency returns true if the database can handle
	// many concurrent write operations (e.g., PostgreSQL).
	// SQLite returns false due to write locking limitations.
	SupportsHighConcurrency() bool

	// Folder operations
	GetFolders() ([]model.Folder, error)
	CreateFolder(name string, parentID *int64) (int64, error)
	GetOrCreateFolder(name string, parentID *int64) (int64, error)
	GetFolderByID(folderID int64) (*model.Folder, error)
	DeleteFolder(folderID int64) error

	// Feed operations
	GetFeeds(folderID *int64) ([]model.Feed, error)
	GetAllFeeds() ([]model.Feed, error)
	GetFeedsByFolderID(folderID int64) ([]model.Feed, error)
	GetUnfiledFeeds() ([]model.Feed, error)
	GetFoldersWithFeeds() ([]model.FolderWithFeeds, error)
	CreateFeed(folderID *int64, title, url string) (int64, error)
	GetOrCreateFeed(folderID *int64, title, url string) (int64, bool, error)
	UpdateFeedLastFetched(feedID int64, t time.Time) error
	UpdateFeedTitle(feedID int64, title string) error
	UpdateFeedError(feedID int64, errMsg string) error
	GetFeedByID(feedID int64) (*model.Feed, error)
	DeleteFeed(feedID int64) error
	MoveFeedToFolder(feedID int64, folderID *int64) error
	AddFeedToFolder(feedID, folderID int64) error
	RemoveFeedFromFolder(feedID, folderID int64) error
	GetFolderIDsForFeed(feedID int64) ([]int64, error)

	// Item operations
	AddItem(item *model.Item) (int64, bool, error)
	GetItems(feedID int64, onlyUnread bool) ([]model.Item, error)
	GetAllItems(onlyUnread bool) ([]model.Item, error)
	GetUnreadCounts() (map[int64]int, error)
	GetItemsByFolderID(folderID int64, onlyUnread bool) ([]model.Item, error)
	MarkItemRead(itemID int64) error
	MarkItemsRead(itemIDs []int64) error
	DeleteReadItems(itemIDs []int64) error
	CleanupReadItems() (int64, error)

	// Settings operations
	GetSetting(key string) (string, error)
	SetSetting(key, value string) error
	GetPollingInterval() (int, error)

	// Kalshi operations
	SaveKalshiReport(html string) error
	GetLatestKalshiReport() (html string, generatedAt time.Time, err error)
	AddKalshiScanLog(output, errMsg string) error
	GetKalshiScanLog(limit int) ([]ScanLogEntry, error)
	SetKalshiOverride(ticker, side, field string, value float64) error
	GetKalshiOverrides() (map[string]float64, error)
}
