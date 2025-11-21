package memory

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
)

type Database struct {
	db *sql.DB
}

func NewDatabase(dbPath string) (*Database, error) {
	// 存在しなかったらディレクトリを作成
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// connectionをテスト
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	database := &Database{db: db}

	// テーブルを初期化
	if err := database.initTables(); err != nil {
		return nil, fmt.Errorf("failed to initialize tables: %w", err)
	}

	return database, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) initTables() error {
	// sessions table
	sessionsTableSQL := `
	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		ended_at DATETIME,
		project_path TEXT NOT NULL,
		model_used TEXT NOT NULL
	);`

	if _, err := d.db.Exec(sessionsTableSQL); err != nil {
		return fmt.Errorf("failed to create sessions table: %w", err)
	}

	// messages table
	messagesTableSQL := `
	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT REFERENCES sessions(id),
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		role TEXT NOT NULL,
		content TEXT,
		tool_calls TEXT,
		tool_results TEXT
	);`

	if _, err := d.db.Exec(messagesTableSQL); err != nil {
		return fmt.Errorf("failed to create messages table: %w", err)
	}

	// indexes
	indexSQL := []string{
		"CREATE INDEX IF NOT EXISTS idx_sessions_project_path ON sessions(project_path);",
		"CREATE INDEX IF NOT EXISTS idx_messages_session_id ON messages(session_id);",
		"CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON messages(timestamp);",
	}

	for _, index := range indexSQL {
		if _, err := d.db.Exec(index); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	return nil
}

func (d *Database) GetDB() *sql.DB {
	return d.db
}
