package db

import (
	"database/sql"

	"github.com/google/uuid"
)

// Config

type Config struct {
	AdminUsername string
	AdminPassword string
	JWTSecret     string
	PanelDomain   sql.NullString
}

func GetConfig(db *sql.DB) (*Config, error) {
	var c Config
	err := db.QueryRow(
		`SELECT admin_username, admin_password, jwt_secret, panel_domain FROM config WHERE id = 1`,
	).Scan(&c.AdminUsername, &c.AdminPassword, &c.JWTSecret, &c.PanelDomain)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func InsertConfig(db *sql.DB, c *Config) error {
	_, err := db.Exec(
		`INSERT INTO config (admin_username, admin_password, jwt_secret, panel_domain) VALUES (?, ?, ?, ?)`,
		c.AdminUsername, c.AdminPassword, c.JWTSecret, c.PanelDomain,
	)
	return err
}

func UpdatePanelDomain(db *sql.DB, domain sql.NullString) error {
	_, err := db.Exec(`UPDATE config SET panel_domain = ? WHERE id = 1`, domain)
	return err
}

func UpdatePassword(db *sql.DB, hash string) error {
	_, err := db.Exec(`UPDATE config SET admin_password = ? WHERE id = 1`, hash)
	return err
}

// Apps

type App struct {
	ID        string
	Name      string
	RepoURL   sql.NullString
	Port      int
	StartCmd  string
	Status    string
	PID       sql.NullInt64
	LogPath   string
	CreatedAt string
}

func ListApps(db *sql.DB) ([]App, error) {
	rows, err := db.Query(`SELECT id, name, repo_url, port, start_cmd, status, pid, log_path, created_at FROM apps ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apps []App
	for rows.Next() {
		var a App
		if err := rows.Scan(&a.ID, &a.Name, &a.RepoURL, &a.Port, &a.StartCmd, &a.Status, &a.PID, &a.LogPath, &a.CreatedAt); err != nil {
			return nil, err
		}
		apps = append(apps, a)
	}
	return apps, rows.Err()
}

func GetApp(db *sql.DB, id string) (*App, error) {
	var a App
	err := db.QueryRow(
		`SELECT id, name, repo_url, port, start_cmd, status, pid, log_path, created_at FROM apps WHERE id = ?`, id,
	).Scan(&a.ID, &a.Name, &a.RepoURL, &a.Port, &a.StartCmd, &a.Status, &a.PID, &a.LogPath, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func NextPort(db *sql.DB) (int, error) {
	var port sql.NullInt64
	err := db.QueryRow(`SELECT port FROM apps ORDER BY port DESC LIMIT 1`).Scan(&port)
	if err == sql.ErrNoRows || !port.Valid {
		return 4000, nil
	}
	if err != nil {
		return 0, err
	}
	return int(port.Int64) + 1, nil
}

func InsertApp(db *sql.DB, name, startCmd string, port int, logPath string) (*App, error) {
	id := uuid.New().String()
	_, err := db.Exec(
		`INSERT INTO apps (id, name, port, start_cmd, status, log_path) VALUES (?, ?, ?, ?, 'stopped', ?)`,
		id, name, port, startCmd, logPath,
	)
	if err != nil {
		return nil, err
	}
	return GetApp(db, id)
}

func UpdateAppStatus(db *sql.DB, id, status string, pid sql.NullInt64) error {
	_, err := db.Exec(`UPDATE apps SET status = ?, pid = ? WHERE id = ?`, status, pid, id)
	return err
}

func DeleteApp(db *sql.DB, id string) error {
	_, err := db.Exec(`DELETE FROM apps WHERE id = ?`, id)
	return err
}

// Envs

type Env struct {
	ID    string
	AppID string
	Key   string
	Value string
}

func ListEnvs(db *sql.DB, appID string) ([]Env, error) {
	rows, err := db.Query(`SELECT id, app_id, key, value FROM envs WHERE app_id = ?`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var envs []Env
	for rows.Next() {
		var e Env
		if err := rows.Scan(&e.ID, &e.AppID, &e.Key, &e.Value); err != nil {
			return nil, err
		}
		envs = append(envs, e)
	}
	return envs, rows.Err()
}

func ReplaceEnvs(db *sql.DB, appID string, envs []Env) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM envs WHERE app_id = ?`, appID); err != nil {
		return err
	}

	for _, e := range envs {
		id := uuid.New().String()
		if _, err := tx.Exec(`INSERT INTO envs (id, app_id, key, value) VALUES (?, ?, ?, ?)`, id, appID, e.Key, e.Value); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// Domains

type Domain struct {
	ID        string
	AppID     string
	Domain    string
	CreatedAt string
}

func ListDomains(db *sql.DB, appID string) ([]Domain, error) {
	rows, err := db.Query(`SELECT id, app_id, domain, created_at FROM domains WHERE app_id = ?`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var domains []Domain
	for rows.Next() {
		var d Domain
		if err := rows.Scan(&d.ID, &d.AppID, &d.Domain, &d.CreatedAt); err != nil {
			return nil, err
		}
		domains = append(domains, d)
	}
	return domains, rows.Err()
}

func InsertDomain(db *sql.DB, appID, domain string) (*Domain, error) {
	id := uuid.New().String()
	_, err := db.Exec(
		`INSERT INTO domains (id, app_id, domain) VALUES (?, ?, ?)`,
		id, appID, domain,
	)
	if err != nil {
		return nil, err
	}
	var d Domain
	err = db.QueryRow(`SELECT id, app_id, domain, created_at FROM domains WHERE id = ?`, id).
		Scan(&d.ID, &d.AppID, &d.Domain, &d.CreatedAt)
	return &d, err
}

func DeleteDomain(db *sql.DB, id string) error {
	_, err := db.Exec(`DELETE FROM domains WHERE id = ?`, id)
	return err
}

func GetDomainByName(db *sql.DB, domain string) (*Domain, error) {
	var d Domain
	err := db.QueryRow(`SELECT id, app_id, domain, created_at FROM domains WHERE domain = ?`, domain).
		Scan(&d.ID, &d.AppID, &d.Domain, &d.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func ListAllDomains(db *sql.DB) ([]Domain, error) {
	rows, err := db.Query(`SELECT id, app_id, domain, created_at FROM domains`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var domains []Domain
	for rows.Next() {
		var d Domain
		if err := rows.Scan(&d.ID, &d.AppID, &d.Domain, &d.CreatedAt); err != nil {
			return nil, err
		}
		domains = append(domains, d)
	}
	return domains, rows.Err()
}
