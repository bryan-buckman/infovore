// Package database provides PostgreSQL storage for the RSS reader.
package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/bryan-buckman/infovore/internal/model"
	_ "github.com/lib/pq"
)

// PostgresStore wraps the PostgreSQL connection.
type PostgresStore struct {
	conn *sql.DB
}

// Ensure PostgresStore implements Store interface.
var _ Store = (*PostgresStore)(nil)

// NewPostgres opens a PostgreSQL database connection.
// connStr format: "postgres://user:password@host:port/dbname?sslmode=disable"
func NewPostgres(connStr string) (*PostgresStore, error) {
	conn, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	// Test connection
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	// Set connection pool settings for better performance
	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(5 * time.Minute)

	db := &PostgresStore{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

// Close closes the database connection.
func (db *PostgresStore) Close() error {
	return db.conn.Close()
}

// DatabaseType returns the database backend name.
func (db *PostgresStore) DatabaseType() string {
	return "PostgreSQL"
}

// SupportsHighConcurrency returns true for PostgreSQL.
func (db *PostgresStore) SupportsHighConcurrency() bool {
	return true
}

func (db *PostgresStore) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS folders (
		id BIGSERIAL PRIMARY KEY,
		name TEXT NOT NULL,
		parent_id BIGINT REFERENCES folders(id)
	);
	CREATE TABLE IF NOT EXISTS feeds (
		id BIGSERIAL PRIMARY KEY,
		folder_id BIGINT REFERENCES folders(id),
		title TEXT NOT NULL,
		url TEXT NOT NULL UNIQUE,
		icon_url TEXT DEFAULT '',
		last_fetched TIMESTAMP,
		last_error TEXT DEFAULT ''
	);
	CREATE TABLE IF NOT EXISTS items (
		id BIGSERIAL PRIMARY KEY,
		feed_id BIGINT NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
		guid TEXT NOT NULL,
		title TEXT NOT NULL,
		content TEXT,
		link TEXT,
		published_at TIMESTAMP,
		fetched_at TIMESTAMP NOT NULL,
		is_read BOOLEAN DEFAULT FALSE,
		UNIQUE(feed_id, guid)
	);
	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);
	INSERT INTO settings (key, value) VALUES ('polling_interval_minutes', '15') ON CONFLICT (key) DO NOTHING;

	-- Create indexes for better query performance
	CREATE INDEX IF NOT EXISTS idx_items_feed_id ON items(feed_id);
	CREATE INDEX IF NOT EXISTS idx_items_published_at ON items(published_at DESC);
	CREATE INDEX IF NOT EXISTS idx_feeds_folder_id ON feeds(folder_id);
	CREATE INDEX IF NOT EXISTS idx_items_is_read ON items(is_read);
	`
	_, err := db.conn.Exec(schema)
	return err
}

// --- Folder Methods ---

func (db *PostgresStore) GetFolders() ([]model.Folder, error) {
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

func (db *PostgresStore) CreateFolder(name string, parentID *int64) (int64, error) {
	var id int64
	err := db.conn.QueryRow("INSERT INTO folders (name, parent_id) VALUES ($1, $2) RETURNING id", name, parentID).Scan(&id)
	return id, err
}

func (db *PostgresStore) GetOrCreateFolder(name string, parentID *int64) (int64, error) {
	var id int64
	var row *sql.Row
	if parentID == nil {
		row = db.conn.QueryRow("SELECT id FROM folders WHERE name = $1 AND parent_id IS NULL", name)
	} else {
		row = db.conn.QueryRow("SELECT id FROM folders WHERE name = $1 AND parent_id = $2", name, *parentID)
	}
	err := row.Scan(&id)
	if err == sql.ErrNoRows {
		return db.CreateFolder(name, parentID)
	}
	return id, err
}

func (db *PostgresStore) GetFolderByID(folderID int64) (*model.Folder, error) {
	var f model.Folder
	err := db.conn.QueryRow("SELECT id, name, parent_id FROM folders WHERE id = $1", folderID).
		Scan(&f.ID, &f.Name, &f.ParentID)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (db *PostgresStore) DeleteFolder(folderID int64) error {
	// First delete all feeds in the folder (items cascade via FK).
	if _, err := db.conn.Exec("DELETE FROM feeds WHERE folder_id = $1", folderID); err != nil {
		return err
	}
	_, err := db.conn.Exec("DELETE FROM folders WHERE id = $1", folderID)
	return err
}

// --- Feed Methods ---

func (db *PostgresStore) GetFeeds(folderID *int64) ([]model.Feed, error) {
	var rows *sql.Rows
	var err error
	query := `SELECT f.id, f.folder_id, f.title, f.url, f.icon_url, f.last_fetched, f.last_error,
		(SELECT COUNT(*) FROM items WHERE feed_id = f.id) as item_count
		FROM feeds f`
	if folderID == nil {
		rows, err = db.conn.Query(query + " ORDER BY f.title")
	} else {
		rows, err = db.conn.Query(query+" WHERE f.folder_id = $1 ORDER BY f.title", *folderID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFeeds(rows)
}

func (db *PostgresStore) GetAllFeeds() ([]model.Feed, error) {
	return db.GetFeeds(nil)
}

func (db *PostgresStore) GetFeedsByFolderID(folderID int64) ([]model.Feed, error) {
	rows, err := db.conn.Query("SELECT id, folder_id, title, url, icon_url, last_fetched, last_error FROM feeds WHERE folder_id = $1 ORDER BY title", folderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFeedsSimple(rows)
}

func (db *PostgresStore) GetUnfiledFeeds() ([]model.Feed, error) {
	rows, err := db.conn.Query("SELECT id, folder_id, title, url, icon_url, last_fetched, last_error FROM feeds WHERE folder_id IS NULL ORDER BY title")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFeedsSimple(rows)
}

func (db *PostgresStore) GetFoldersWithFeeds() ([]model.FolderWithFeeds, error) {
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

func (db *PostgresStore) CreateFeed(folderID *int64, title, url string) (int64, error) {
	var id int64
	err := db.conn.QueryRow("INSERT INTO feeds (folder_id, title, url) VALUES ($1, $2, $3) RETURNING id", folderID, title, url).Scan(&id)
	return id, err
}

func (db *PostgresStore) GetOrCreateFeed(folderID *int64, title, url string) (int64, bool, error) {
	var id int64
	err := db.conn.QueryRow("SELECT id FROM feeds WHERE url = $1", url).Scan(&id)
	if err == sql.ErrNoRows {
		id, err := db.CreateFeed(folderID, title, url)
		return id, true, err
	}
	return id, false, err
}

func (db *PostgresStore) UpdateFeedLastFetched(feedID int64, t time.Time) error {
	_, err := db.conn.Exec("UPDATE feeds SET last_fetched = $1, last_error = '' WHERE id = $2", t, feedID)
	return err
}

func (db *PostgresStore) UpdateFeedTitle(feedID int64, title string) error {
	_, err := db.conn.Exec("UPDATE feeds SET title = $1 WHERE id = $2", title, feedID)
	return err
}

func (db *PostgresStore) UpdateFeedError(feedID int64, errMsg string) error {
	_, err := db.conn.Exec("UPDATE feeds SET last_error = $1 WHERE id = $2", errMsg, feedID)
	return err
}

func (db *PostgresStore) GetFeedByID(feedID int64) (*model.Feed, error) {
	var f model.Feed
	var lastFetched sql.NullTime
	var lastError sql.NullString
	err := db.conn.QueryRow("SELECT id, folder_id, title, url, icon_url, last_fetched, last_error FROM feeds WHERE id = $1", feedID).
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

func (db *PostgresStore) DeleteFeed(feedID int64) error {
	_, err := db.conn.Exec("DELETE FROM feeds WHERE id = $1", feedID)
	return err
}

func (db *PostgresStore) MoveFeedToFolder(feedID int64, folderID *int64) error {
	_, err := db.conn.Exec("UPDATE feeds SET folder_id = $1 WHERE id = $2", folderID, feedID)
	return err
}

// --- Item Methods ---

func (db *PostgresStore) AddItem(item *model.Item) (int64, bool, error) {
	var id int64
	err := db.conn.QueryRow(`
		INSERT INTO items (feed_id, guid, title, content, link, published_at, fetched_at, is_read)
		VALUES ($1, $2, $3, $4, $5, $6, $7, FALSE)
		ON CONFLICT(feed_id, guid) DO NOTHING
		RETURNING id`,
		item.FeedID, item.GUID, item.Title, item.Content, item.Link, item.PublishedAt, item.FetchedAt).Scan(&id)
	if err == sql.ErrNoRows {
		// Conflict occurred, item already exists
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

func (db *PostgresStore) GetItems(feedID int64, onlyUnread bool) ([]model.Item, error) {
	query := "SELECT id, feed_id, guid, title, content, link, published_at, fetched_at, is_read FROM items WHERE feed_id = $1"
	if onlyUnread {
		query += " AND is_read = FALSE"
	}
	query += " ORDER BY published_at DESC"
	rows, err := db.conn.Query(query, feedID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanItemsPg(rows)
}

func (db *PostgresStore) GetAllItems(onlyUnread bool) ([]model.Item, error) {
	query := "SELECT id, feed_id, guid, title, content, link, published_at, fetched_at, is_read FROM items"
	if onlyUnread {
		query += " WHERE is_read = FALSE"
	}
	query += " ORDER BY published_at DESC"
	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanItemsPg(rows)
}

func (db *PostgresStore) GetItemsByFolderID(folderID int64, onlyUnread bool) ([]model.Item, error) {
	query := `SELECT i.id, i.feed_id, i.guid, i.title, i.content, i.link, i.published_at, i.fetched_at, i.is_read
		FROM items i
		JOIN feeds f ON i.feed_id = f.id
		WHERE f.folder_id = $1`
	if onlyUnread {
		query += " AND i.is_read = FALSE"
	}
	query += " ORDER BY i.published_at DESC"
	rows, err := db.conn.Query(query, folderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanItemsPg(rows)
}

func (db *PostgresStore) MarkItemRead(itemID int64) error {
	_, err := db.conn.Exec("UPDATE items SET is_read = TRUE WHERE id = $1", itemID)
	return err
}

func (db *PostgresStore) MarkItemsRead(itemIDs []int64) error {
	if len(itemIDs) == 0 {
		return nil
	}
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("UPDATE items SET is_read = TRUE WHERE id = $1")
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

func (db *PostgresStore) DeleteReadItems(itemIDs []int64) error {
	if len(itemIDs) == 0 {
		return nil
	}
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("DELETE FROM items WHERE id = $1 AND is_read = TRUE")
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

func (db *PostgresStore) CleanupReadItems() (int64, error) {
	res, err := db.conn.Exec("DELETE FROM items WHERE is_read = TRUE")
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// --- Settings Methods ---

func (db *PostgresStore) GetSetting(key string) (string, error) {
	var val string
	err := db.conn.QueryRow("SELECT value FROM settings WHERE key = $1", key).Scan(&val)
	return val, err
}

func (db *PostgresStore) SetSetting(key, value string) error {
	_, err := db.conn.Exec("INSERT INTO settings (key, value) VALUES ($1, $2) ON CONFLICT(key) DO UPDATE SET value = $2", key, value)
	return err
}

func (db *PostgresStore) GetPollingInterval() (int, error) {
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

// --- Helper functions ---

func scanFeeds(rows *sql.Rows) ([]model.Feed, error) {
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

func scanFeedsSimple(rows *sql.Rows) ([]model.Feed, error) {
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

func scanItemsPg(rows *sql.Rows) ([]model.Item, error) {
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
