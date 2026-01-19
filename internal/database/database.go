// Package database provides SQLite storage for the RSS reader.
package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/bryan-buckman/infovore/internal/model"
	_ "modernc.org/sqlite"
)

// DB wraps the SQLite connection.
type DB struct {
	conn *sql.DB
}

// New opens or creates an SQLite database at the given path.
func New(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	// Enable foreign key constraints.
	if _, err := conn.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}
	// Enable WAL mode for better concurrency.
	if _, err := conn.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("set wal mode: %w", err)
	}
	// Set busy timeout to wait up to 5 seconds when database is locked.
	if _, err := conn.Exec("PRAGMA busy_timeout=5000;"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}
	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS folders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		parent_id INTEGER REFERENCES folders(id)
	);
	CREATE TABLE IF NOT EXISTS feeds (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		folder_id INTEGER REFERENCES folders(id),
		title TEXT NOT NULL,
		url TEXT NOT NULL UNIQUE,
		icon_url TEXT DEFAULT '',
		last_fetched DATETIME,
		last_error TEXT DEFAULT ''
	);
	CREATE TABLE IF NOT EXISTS items (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		feed_id INTEGER NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
		guid TEXT NOT NULL,
		title TEXT NOT NULL,
		content TEXT,
		link TEXT,
		published_at DATETIME,
		fetched_at DATETIME NOT NULL,
		is_read INTEGER DEFAULT 0,
		UNIQUE(feed_id, guid)
	);
	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);
	-- Default polling interval (15 minutes minimum).
	INSERT OR IGNORE INTO settings (key, value) VALUES ('polling_interval_minutes', '15');
	`
	if _, err := db.conn.Exec(schema); err != nil {
		return err
	}
	// Migration: add last_error column if it doesn't exist.
	_, _ = db.conn.Exec("ALTER TABLE feeds ADD COLUMN last_error TEXT DEFAULT ''")
	return nil
}

// --- Folder Methods ---

// GetFolders returns all folders ordered by name.
func (db *DB) GetFolders() ([]model.Folder, error) {
	rows, err := db.conn.Query("SELECT id, name, parent_id FROM folders ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var folders []model.Folder
	for rows.Next() {
		var f model.Folder
		if err := rows.Scan(&f.ID, &f.Name, &f.ParentID); err != nil {
			return nil, err
		}
		folders = append(folders, f)
	}
	return folders, rows.Err()
}

// CreateFolder creates a new folder. Returns the ID.
func (db *DB) CreateFolder(name string, parentID *int64) (int64, error) {
	res, err := db.conn.Exec("INSERT INTO folders (name, parent_id) VALUES (?, ?)", name, parentID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetOrCreateFolder finds a folder by name and parent, or creates it.
func (db *DB) GetOrCreateFolder(name string, parentID *int64) (int64, error) {
	var id int64
	var row *sql.Row
	if parentID == nil {
		row = db.conn.QueryRow("SELECT id FROM folders WHERE name = ? AND parent_id IS NULL", name)
	} else {
		row = db.conn.QueryRow("SELECT id FROM folders WHERE name = ? AND parent_id = ?", name, *parentID)
	}
	err := row.Scan(&id)
	if err == sql.ErrNoRows {
		return db.CreateFolder(name, parentID)
	}
	return id, err
}

// --- Feed Methods ---

// GetFeeds returns all feeds, optionally filtered by folder.
func (db *DB) GetFeeds(folderID *int64) ([]model.Feed, error) {
	var rows *sql.Rows
	var err error
	query := `SELECT f.id, f.folder_id, f.title, f.url, f.icon_url, f.last_fetched, f.last_error,
		(SELECT COUNT(*) FROM items WHERE feed_id = f.id) as item_count
		FROM feeds f`
	if folderID == nil {
		rows, err = db.conn.Query(query + " ORDER BY f.title")
	} else {
		rows, err = db.conn.Query(query + " WHERE f.folder_id = ? ORDER BY f.title", *folderID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var feeds []model.Feed
	for rows.Next() {
		var f model.Feed
		var lastFetched sql.NullTime
		var lastError sql.NullString
		if err := rows.Scan(&f.ID, &f.FolderID, &f.Title, &f.URL, &f.IconURL, &lastFetched, &lastError, &f.ItemCount); err != nil {
			return nil, err
		}
		if lastFetched.Valid {
			f.LastFetched = lastFetched.Time
		}
		if lastError.Valid {
			f.LastError = lastError.String
		}
		feeds = append(feeds, f)
	}
	return feeds, rows.Err()
}

// GetAllFeeds returns all feeds regardless of folder.
func (db *DB) GetAllFeeds() ([]model.Feed, error) {
	return db.GetFeeds(nil)
}

// GetFeedsByFolderID returns feeds belonging to a specific folder.
func (db *DB) GetFeedsByFolderID(folderID int64) ([]model.Feed, error) {
	rows, err := db.conn.Query("SELECT id, folder_id, title, url, icon_url, last_fetched, last_error FROM feeds WHERE folder_id = ? ORDER BY title", folderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var feeds []model.Feed
	for rows.Next() {
		var f model.Feed
		var lastFetched sql.NullTime
		var lastError sql.NullString
		if err := rows.Scan(&f.ID, &f.FolderID, &f.Title, &f.URL, &f.IconURL, &lastFetched, &lastError); err != nil {
			return nil, err
		}
		if lastFetched.Valid {
			f.LastFetched = lastFetched.Time
		}
		if lastError.Valid {
			f.LastError = lastError.String
		}
		feeds = append(feeds, f)
	}
	return feeds, rows.Err()
}

// GetUnfiledFeeds returns feeds that don't belong to any folder.
func (db *DB) GetUnfiledFeeds() ([]model.Feed, error) {
	rows, err := db.conn.Query("SELECT id, folder_id, title, url, icon_url, last_fetched, last_error FROM feeds WHERE folder_id IS NULL ORDER BY title")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var feeds []model.Feed
	for rows.Next() {
		var f model.Feed
		var lastFetched sql.NullTime
		var lastError sql.NullString
		if err := rows.Scan(&f.ID, &f.FolderID, &f.Title, &f.URL, &f.IconURL, &lastFetched, &lastError); err != nil {
			return nil, err
		}
		if lastFetched.Valid {
			f.LastFetched = lastFetched.Time
		}
		if lastError.Valid {
			f.LastError = lastError.String
		}
		feeds = append(feeds, f)
	}
	return feeds, rows.Err()
}

// GetFoldersWithFeeds returns all folders with their feeds populated.
func (db *DB) GetFoldersWithFeeds() ([]model.FolderWithFeeds, error) {
	folders, err := db.GetFolders()
	if err != nil {
		return nil, err
	}

	var result []model.FolderWithFeeds
	for _, folder := range folders {
		feeds, err := db.GetFeedsByFolderID(folder.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, model.FolderWithFeeds{
			Folder: folder,
			Feeds:  feeds,
		})
	}
	return result, nil
}

// CreateFeed adds a new feed. Returns the ID.
func (db *DB) CreateFeed(folderID *int64, title, url string) (int64, error) {
	res, err := db.conn.Exec("INSERT INTO feeds (folder_id, title, url) VALUES (?, ?, ?)", folderID, title, url)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetOrCreateFeed finds a feed by URL, or creates it.
func (db *DB) GetOrCreateFeed(folderID *int64, title, url string) (int64, bool, error) {
	var id int64
	err := db.conn.QueryRow("SELECT id FROM feeds WHERE url = ?", url).Scan(&id)
	if err == sql.ErrNoRows {
		id, err := db.CreateFeed(folderID, title, url)
		return id, true, err
	}
	return id, false, err
}

// UpdateFeedLastFetched updates the last_fetched timestamp for a feed.
func (db *DB) UpdateFeedLastFetched(feedID int64, t time.Time) error {
	_, err := db.conn.Exec("UPDATE feeds SET last_fetched = ?, last_error = '' WHERE id = ?", t, feedID)
	return err
}

// UpdateFeedTitle updates the title for a feed.
func (db *DB) UpdateFeedTitle(feedID int64, title string) error {
	_, err := db.conn.Exec("UPDATE feeds SET title = ? WHERE id = ?", title, feedID)
	return err
}

// UpdateFeedError sets the last error message for a feed.
func (db *DB) UpdateFeedError(feedID int64, errMsg string) error {
	_, err := db.conn.Exec("UPDATE feeds SET last_error = ? WHERE id = ?", errMsg, feedID)
	return err
}

// GetFeedByID returns a single feed by its ID.
func (db *DB) GetFeedByID(feedID int64) (*model.Feed, error) {
	var f model.Feed
	var lastFetched sql.NullTime
	var lastError sql.NullString
	err := db.conn.QueryRow("SELECT id, folder_id, title, url, icon_url, last_fetched, last_error FROM feeds WHERE id = ?", feedID).
		Scan(&f.ID, &f.FolderID, &f.Title, &f.URL, &f.IconURL, &lastFetched, &lastError)
	if err != nil {
		return nil, err
	}
	if lastFetched.Valid {
		f.LastFetched = lastFetched.Time
	}
	if lastError.Valid {
		f.LastError = lastError.String
	}
	return &f, nil
}

// GetFolderByID returns a single folder by its ID.
func (db *DB) GetFolderByID(folderID int64) (*model.Folder, error) {
	var f model.Folder
	err := db.conn.QueryRow("SELECT id, name, parent_id FROM folders WHERE id = ?", folderID).
		Scan(&f.ID, &f.Name, &f.ParentID)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// DeleteFeed removes a feed and all its items (cascading).
func (db *DB) DeleteFeed(feedID int64) error {
	_, err := db.conn.Exec("DELETE FROM feeds WHERE id = ?", feedID)
	return err
}

// DeleteFolder removes a folder and all its feeds (and their items).
func (db *DB) DeleteFolder(folderID int64) error {
	// First delete all feeds in the folder (items cascade via FK).
	if _, err := db.conn.Exec("DELETE FROM feeds WHERE folder_id = ?", folderID); err != nil {
		return err
	}
	// Then delete the folder itself.
	_, err := db.conn.Exec("DELETE FROM folders WHERE id = ?", folderID)
	return err
}

// MoveFeedToFolder updates a feed's folder assignment.
func (db *DB) MoveFeedToFolder(feedID int64, folderID *int64) error {
	_, err := db.conn.Exec("UPDATE feeds SET folder_id = ? WHERE id = ?", folderID, feedID)
	return err
}

// DeleteReadItems deletes specific read items by their IDs.
func (db *DB) DeleteReadItems(itemIDs []int64) error {
	if len(itemIDs) == 0 {
		return nil
	}
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("DELETE FROM items WHERE id = ? AND is_read = 1")
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, id := range itemIDs {
		if _, err := stmt.Exec(id); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// GetItemsByFolderID returns all items for feeds in a specific folder.
func (db *DB) GetItemsByFolderID(folderID int64, onlyUnread bool) ([]model.Item, error) {
	query := `SELECT i.id, i.feed_id, i.guid, i.title, i.content, i.link, i.published_at, i.fetched_at, i.is_read
		FROM items i
		JOIN feeds f ON i.feed_id = f.id
		WHERE f.folder_id = ?`
	if onlyUnread {
		query += " AND i.is_read = 0"
	}
	query += " ORDER BY i.published_at DESC"
	rows, err := db.conn.Query(query, folderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanItems(rows)
}

// --- Item Methods ---

// AddItem inserts a new item if GUID doesn't exist for that feed. Returns ID and whether it was new.
func (db *DB) AddItem(item *model.Item) (int64, bool, error) {
	res, err := db.conn.Exec(`
		INSERT INTO items (feed_id, guid, title, content, link, published_at, fetched_at, is_read)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(feed_id, guid) DO NOTHING`,
		item.FeedID, item.GUID, item.Title, item.Content, item.Link, item.PublishedAt, item.FetchedAt, 0)
	if err != nil {
		return 0, false, err
	}
	id, _ := res.LastInsertId()
	affected, _ := res.RowsAffected()
	return id, affected > 0, nil
}

// GetItems returns items for a feed, ordered by published date desc.
func (db *DB) GetItems(feedID int64, onlyUnread bool) ([]model.Item, error) {
	query := "SELECT id, feed_id, guid, title, content, link, published_at, fetched_at, is_read FROM items WHERE feed_id = ?"
	if onlyUnread {
		query += " AND is_read = 0"
	}
	query += " ORDER BY published_at DESC"
	rows, err := db.conn.Query(query, feedID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanItems(rows)
}

// GetAllItems returns all items for the sidebar/home stream.
func (db *DB) GetAllItems(onlyUnread bool) ([]model.Item, error) {
	query := "SELECT id, feed_id, guid, title, content, link, published_at, fetched_at, is_read FROM items"
	if onlyUnread {
		query += " WHERE is_read = 0"
	}
	query += " ORDER BY published_at DESC"
	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanItems(rows)
}

func scanItems(rows *sql.Rows) ([]model.Item, error) {
	var items []model.Item
	for rows.Next() {
		var it model.Item
		var publishedAt, fetchedAt sql.NullTime
		if err := rows.Scan(&it.ID, &it.FeedID, &it.GUID, &it.Title, &it.Content, &it.Link, &publishedAt, &fetchedAt, &it.IsRead); err != nil {
			return nil, err
		}
		if publishedAt.Valid {
			it.PublishedAt = publishedAt.Time
		}
		if fetchedAt.Valid {
			it.FetchedAt = fetchedAt.Time
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// MarkItemRead marks an item as read.
func (db *DB) MarkItemRead(itemID int64) error {
	_, err := db.conn.Exec("UPDATE items SET is_read = 1 WHERE id = ?", itemID)
	return err
}

// MarkItemsRead marks multiple items as read.
func (db *DB) MarkItemsRead(itemIDs []int64) error {
	if len(itemIDs) == 0 {
		return nil
	}
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("UPDATE items SET is_read = 1 WHERE id = ?")
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, id := range itemIDs {
		if _, err := stmt.Exec(id); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// CleanupReadItems deletes all items marked as read.
func (db *DB) CleanupReadItems() (int64, error) {
	res, err := db.conn.Exec("DELETE FROM items WHERE is_read = 1")
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// --- Settings Methods ---

// GetSetting retrieves a setting value.
func (db *DB) GetSetting(key string) (string, error) {
	var val string
	err := db.conn.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&val)
	return val, err
}

// SetSetting saves a setting.
func (db *DB) SetSetting(key, value string) error {
	_, err := db.conn.Exec("INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = ?", key, value, value)
	return err
}

// GetPollingInterval returns the polling interval in minutes, with a minimum of 15.
func (db *DB) GetPollingInterval() (int, error) {
	val, err := db.GetSetting(model.SettingPollingInterval)
	if err != nil {
		return 15, nil // default
	}
	var mins int
	fmt.Sscanf(val, "%d", &mins)
	if mins < 15 {
		mins = 15
	}
	return mins, nil
}
