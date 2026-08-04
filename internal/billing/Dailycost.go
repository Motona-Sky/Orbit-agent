package billing

import (
	"database/sql"
	"errors"
	"fmt"
	"orbit/internal/config"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

var (
	ErrDailyUsageQuery    = errors.New("query daily usage")
	dailyCostDatabases    sync.Map
	dailyCostDatabaseOpen sync.Mutex
)

type DailyUsage struct {
	TotalTokens  int64
	PromptTokens int64
	CachedTokens int64
}

func (usage DailyUsage) CacheHitRate() float64 {
	if usage.PromptTokens == 0 {
		return 0
	}
	return float64(usage.CachedTokens) / float64(usage.PromptTokens) * 100
}

func CreateTable() error {
	configfolder, err := config.GetConfigFolderPath()
	if err != nil {
		return err
	}
	dbpath := filepath.Join(configfolder["ConfigFolder"], "dailycost.db")
	_, err = openDailyCostDatabase(dbpath)
	return err
}

const dailyCostTableSchema = `
CREATE TABLE IF NOT EXISTS Dailycost (
    sessionid TEXT NOT NULL
        CHECK (
            length(sessionid) = 19
            AND sessionid GLOB '????-????-????-????'
        ),

    "Date" TEXT NOT NULL
        DEFAULT (date('now', 'localtime')),

    prompt_tokens INTEGER NOT NULL DEFAULT 0
        CHECK (prompt_tokens >= 0),

    completion_tokens INTEGER NOT NULL DEFAULT 0
        CHECK (completion_tokens >= 0),

    total_tokens INTEGER NOT NULL DEFAULT 0
        CHECK (total_tokens >= 0),

    prompt_tokens_details INTEGER NOT NULL DEFAULT 0
        CHECK (prompt_tokens_details >= 0),

    reasoning_tokens INTEGER NOT NULL DEFAULT 0
        CHECK (reasoning_tokens >= 0),

    prompt_cache_miss_tokens INTEGER NOT NULL DEFAULT 0
        CHECK (prompt_cache_miss_tokens >= 0),

    PRIMARY KEY (sessionid, "Date")
);`

func createSchema(db *sql.DB) error {
	if _, err := db.Exec(dailyCostTableSchema); err != nil {
		return err
	}
	legacy, err := hasLegacySessionPrimaryKey(db)
	if err != nil {
		return err
	}
	if legacy {
		if err := migrateLegacySchema(db); err != nil {
			return err
		}
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_dailycost_date ON Dailycost("Date");`)
	return err
}

func hasLegacySessionPrimaryKey(db *sql.DB) (bool, error) {
	rows, err := db.Query(`PRAGMA table_info(Dailycost)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	sessionPrimaryKey := 0
	datePrimaryKey := 0
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, dataType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			return false, err
		}
		switch name {
		case "sessionid":
			sessionPrimaryKey = primaryKey
		case "Date":
			datePrimaryKey = primaryKey
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return sessionPrimaryKey == 1 && datePrimaryKey == 0, nil
}

func migrateLegacySchema(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`ALTER TABLE Dailycost RENAME TO Dailycost_legacy`); err != nil {
		return err
	}
	if _, err := tx.Exec(dailyCostTableSchema); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		INSERT INTO Dailycost (
			sessionid, "Date", prompt_tokens, completion_tokens, total_tokens,
			prompt_tokens_details, reasoning_tokens, prompt_cache_miss_tokens
		)
		SELECT
			sessionid, "Date", prompt_tokens, completion_tokens, total_tokens,
			prompt_tokens_details, reasoning_tokens, prompt_cache_miss_tokens
		FROM Dailycost_legacy
	`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DROP TABLE Dailycost_legacy`); err != nil {
		return err
	}
	return tx.Commit()
}

func queryUsageForDate(db *sql.DB, date string) (DailyUsage, error) {
	var usage DailyUsage

	const query = `
		SELECT
			COALESCE(SUM(total_tokens), 0),
			COALESCE(SUM(prompt_tokens), 0),
			COALESCE(SUM(prompt_tokens_details), 0)
		FROM Dailycost
		WHERE "Date" = ?
	`

	err := db.QueryRow(query, date).Scan(
		&usage.TotalTokens,
		&usage.PromptTokens,
		&usage.CachedTokens,
	)
	if err != nil {
		return DailyUsage{}, err
	}

	return usage, nil
}

// QueryTodayUsage 查询今日所有 session 的 token 消耗汇总。
func QueryTodayUsage() (DailyUsage, error) {
	configfolder, err := config.GetConfigFolderPath()
	if err != nil {
		return DailyUsage{}, err
	}
	dbpath := filepath.Join(configfolder["ConfigFolder"], "dailycost.db")
	db, err := openDailyCostDatabase(dbpath)
	if err != nil {
		return DailyUsage{}, err
	}
	return queryUsageForDate(db, time.Now().Format("2006-01-02"))
}

func getInt(usage map[string]any, key string) int {
	if v, ok := usage[key]; ok && v != nil {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		}
	}
	return 0
}

func getNestedInt(usage map[string]any, outerKey, innerKey string) int {
	if outer, ok := usage[outerKey].(map[string]any); ok && outer != nil {
		return getInt(outer, innerKey)
	}
	return 0
}

// 存储每日token消耗
func InsertCostData(sessionid string, usage map[string]any) (DailyUsage, error) {
	configfolder, err := config.GetConfigFolderPath()
	if err != nil {
		return DailyUsage{}, err
	}

	dbpath := filepath.Join(configfolder["ConfigFolder"], "dailycost.db")
	db, err := openDailyCostDatabase(dbpath)
	if err != nil {
		return DailyUsage{}, err
	}

	return insertCostData(db, sessionid, usage, time.Now().Format("2006-01-02"))
}

func openDailyCostDatabase(dbpath string) (*sql.DB, error) {
	if cached, ok := dailyCostDatabases.Load(dbpath); ok {
		return cached.(*sql.DB), nil
	}

	dailyCostDatabaseOpen.Lock()
	defer dailyCostDatabaseOpen.Unlock()
	if cached, ok := dailyCostDatabases.Load(dbpath); ok {
		return cached.(*sql.DB), nil
	}

	db, err := sql.Open("sqlite", dbpath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := createSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	dailyCostDatabases.Store(dbpath, db)
	return db, nil
}

func insertCostData(db *sql.DB, sessionid string, usage map[string]any, date string) (DailyUsage, error) {
	prompt_tokens := getInt(usage, "prompt_tokens")
	completion_tokens := getInt(usage, "completion_tokens")
	total_tokens := getInt(usage, "total_tokens")
	prompt_tokens_details := getNestedInt(usage, "prompt_tokens_details", "cached_tokens")
	reasoning_tokens := getNestedInt(usage, "completion_tokens_details", "reasoning_tokens")
	prompt_cache_miss_tokens := getInt(usage, "prompt_cache_miss_tokens")

	sqlStmt := `
	INSERT INTO Dailycost (
		sessionid,
		"Date",
		prompt_tokens,
		completion_tokens,
		total_tokens,
		prompt_tokens_details,
		reasoning_tokens,
		prompt_cache_miss_tokens
	)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(sessionid, "Date") DO UPDATE SET
		prompt_tokens =
			prompt_tokens + excluded.prompt_tokens,
		completion_tokens =
			completion_tokens + excluded.completion_tokens,
		total_tokens =
			total_tokens + excluded.total_tokens,
		prompt_tokens_details =
			prompt_tokens_details + excluded.prompt_tokens_details,
		reasoning_tokens =
			reasoning_tokens + excluded.reasoning_tokens,
		prompt_cache_miss_tokens =
			prompt_cache_miss_tokens + excluded.prompt_cache_miss_tokens;
`
	_, err := db.Exec(sqlStmt, sessionid, date, prompt_tokens, completion_tokens, total_tokens, prompt_tokens_details, reasoning_tokens, prompt_cache_miss_tokens)
	if err != nil {
		return DailyUsage{}, err
	}
	dailyUsage, err := queryUsageForDate(db, date)
	if err != nil {
		return DailyUsage{}, fmt.Errorf("%w: %v", ErrDailyUsageQuery, err)
	}
	return dailyUsage, nil
}
