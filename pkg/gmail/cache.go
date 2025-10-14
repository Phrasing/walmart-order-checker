package gmail

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"walmart-order-checker/pkg/report"
)

type MessageCache struct {
	db       *sql.DB
	ttl      time.Duration
	getStmt  *sql.Stmt
	setStmt  *sql.Stmt
	stmtLock sync.RWMutex
}

type CachedResult struct {
	Order   *report.Order
	Shipped []*report.ShippedOrder
}

func NewMessageCache(cachePath string, ttl time.Duration) *MessageCache {
	dbPath := cachePath
	if filepath.Ext(cachePath) == "" {
		dbPath = filepath.Join(cachePath, "messages.db")
		if err := os.MkdirAll(cachePath, 0o755); err != nil {
			panic(fmt.Sprintf("failed to create cache directory: %v", err))
		}
	} else {
		dir := filepath.Dir(cachePath)
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				panic(fmt.Sprintf("failed to create cache directory: %v", err))
			}
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		panic(fmt.Sprintf("failed to open cache database: %v", err))
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA cache_size=-64000",
		"PRAGMA busy_timeout=5000",
		"PRAGMA temp_store=MEMORY",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			panic(fmt.Sprintf("failed to set pragma %s: %v", pragma, err))
		}
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS parsed_results (
			message_id TEXT PRIMARY KEY,
			result_data BLOB NOT NULL,
			created_at INTEGER NOT NULL
		) WITHOUT ROWID;
		CREATE INDEX IF NOT EXISTS idx_parsed_created_at ON parsed_results(created_at);
	`)
	if err != nil {
		panic(fmt.Sprintf("failed to create cache table: %v", err))
	}

	cache := &MessageCache{
		db:  db,
		ttl: ttl,
	}

	cache.prepareStatements()

	go cache.periodicCleanup()

	return cache
}

func (c *MessageCache) prepareStatements() {
	var err error

	c.getStmt, err = c.db.Prepare("SELECT result_data FROM parsed_results WHERE message_id = ? AND created_at > ?")
	if err != nil {
		panic(fmt.Sprintf("failed to prepare get statement: %v", err))
	}

	c.setStmt, err = c.db.Prepare("INSERT OR REPLACE INTO parsed_results (message_id, result_data, created_at) VALUES (?, ?, ?)")
	if err != nil {
		panic(fmt.Sprintf("failed to prepare set statement: %v", err))
	}
}

func (c *MessageCache) Get(msgID string) (*CachedResult, bool) {
	cutoff := time.Now().Add(-c.ttl).Unix()

	c.stmtLock.RLock()
	defer c.stmtLock.RUnlock()

	var data []byte
	err := c.getStmt.QueryRow(msgID, cutoff).Scan(&data)
	if err != nil {
		return nil, false
	}

	var result CachedResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, false
	}

	return &result, true
}

func (c *MessageCache) Set(msgID string, result *CachedResult) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}

	c.stmtLock.RLock()
	defer c.stmtLock.RUnlock()

	_, err = c.setStmt.Exec(msgID, data, time.Now().Unix())
	return err
}

func (c *MessageCache) Clear() error {
	_, err := c.db.Exec("DELETE FROM parsed_results")
	if err != nil {
		return err
	}
	_, err = c.db.Exec("VACUUM")
	return err
}

func (c *MessageCache) Stats() (total int, size int64, err error) {
	err = c.db.QueryRow(
		"SELECT COUNT(*), COALESCE(SUM(length(result_data)), 0) FROM parsed_results",
	).Scan(&total, &size)
	return
}

func (c *MessageCache) periodicCleanup() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		cutoff := time.Now().Add(-c.ttl).Unix()
		c.db.Exec("DELETE FROM parsed_results WHERE created_at <= ?", cutoff)
	}
}

func (c *MessageCache) Close() error {
	c.stmtLock.Lock()
	defer c.stmtLock.Unlock()

	if c.getStmt != nil {
		c.getStmt.Close()
	}
	if c.setStmt != nil {
		c.setStmt.Close()
	}
	return c.db.Close()
}
