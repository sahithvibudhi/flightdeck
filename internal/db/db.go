package db

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

var migrations = []string{
	`CREATE TABLE IF NOT EXISTS config (
		id              INTEGER PRIMARY KEY DEFAULT 1,
		admin_username  TEXT NOT NULL,
		admin_password  TEXT NOT NULL,
		jwt_secret      TEXT NOT NULL,
		panel_domain    TEXT,
		git_token       TEXT,
		created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS apps (
		id          TEXT PRIMARY KEY,
		name        TEXT NOT NULL UNIQUE,
		repo_url    TEXT,
		branch      TEXT DEFAULT 'main',
		port        INTEGER NOT NULL UNIQUE,
		start_cmd   TEXT NOT NULL,
		status      TEXT DEFAULT 'stopped',
		pid         INTEGER,
		log_path    TEXT NOT NULL,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS envs (
		id      TEXT PRIMARY KEY,
		app_id  TEXT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
		key     TEXT NOT NULL,
		value   TEXT NOT NULL,
		UNIQUE(app_id, key)
	)`,
	`CREATE TABLE IF NOT EXISTS domains (
		id         TEXT PRIMARY KEY,
		app_id     TEXT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
		domain     TEXT NOT NULL UNIQUE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`ALTER TABLE apps ADD COLUMN build_cmd TEXT DEFAULT ''`,
	`ALTER TABLE apps ADD COLUMN work_dir TEXT DEFAULT ''`,
	`ALTER TABLE apps ADD COLUMN webhook_secret TEXT DEFAULT ''`,
	`ALTER TABLE apps ADD COLUMN health_path TEXT DEFAULT ''`,
	`ALTER TABLE apps ADD COLUMN active_port INTEGER DEFAULT 0`,
	`CREATE TABLE IF NOT EXISTS deployments (
		id           TEXT PRIMARY KEY,
		app_id       TEXT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
		triggered_by TEXT NOT NULL,
		status       TEXT NOT NULL DEFAULT 'running',
		detail       TEXT DEFAULT '',
		started_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
		finished_at  DATETIME
	)`,
	`ALTER TABLE deployments ADD COLUMN commit_sha TEXT DEFAULT ''`,
	`ALTER TABLE deployments ADD COLUMN commit_msg TEXT DEFAULT ''`,
	`CREATE TABLE IF NOT EXISTS api_tokens (
		id         TEXT PRIMARY KEY,
		name       TEXT NOT NULL,
		hash       TEXT NOT NULL UNIQUE,
		scope      TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_used  DATETIME
	)`,
}

func Open(path string) (*sql.DB, error) {
	// busy_timeout makes concurrent writers wait instead of failing with
	// SQLITE_BUSY — e.g. deleting an app races the process watcher's
	// status update when the app was just stopped.
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(wal)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			if strings.Contains(err.Error(), "duplicate column") {
				continue
			}
			return nil, fmt.Errorf("run migration: %w", err)
		}
	}

	return db, nil
}
