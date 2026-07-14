package db

import (
	"database/sql"

	"github.com/google/uuid"
)

type Config struct {
	AdminUsername string
	AdminPassword string
	JWTSecret     string
	PanelDomain   sql.NullString
	GitToken      sql.NullString
}

func GetConfig(db *sql.DB) (*Config, error) {
	var c Config
	err := db.QueryRow(
		`SELECT admin_username, admin_password, jwt_secret, panel_domain, git_token FROM config WHERE id = 1`,
	).Scan(&c.AdminUsername, &c.AdminPassword, &c.JWTSecret, &c.PanelDomain, &c.GitToken)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func InsertConfig(db *sql.DB, c *Config) error {
	_, err := db.Exec(
		`INSERT INTO config (admin_username, admin_password, jwt_secret, panel_domain, git_token) VALUES (?, ?, ?, ?, ?)`,
		c.AdminUsername, c.AdminPassword, c.JWTSecret, c.PanelDomain, c.GitToken,
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

func UpdateGitToken(db *sql.DB, token sql.NullString) error {
	_, err := db.Exec(`UPDATE config SET git_token = ? WHERE id = 1`, token)
	return err
}

type App struct {
	ID            string
	Name          string
	RepoURL       sql.NullString
	Branch        sql.NullString
	Port          int
	StartCmd      string
	BuildCmd      string
	WorkDir       string
	WebhookSecret string
	HealthPath    string
	ActivePort    int
	Status        string
	PID           sql.NullInt64
	LogPath       string
	CreatedAt     string
}

/*
EffectivePort is the port the app is currently reachable on. During
zero-downtime deploys the process alternates between the configured
port and its standby, tracked in active_port.
*/
func (a *App) EffectivePort() int {
	if a.ActivePort > 0 {
		return a.ActivePort
	}
	return a.Port
}

func ListApps(db *sql.DB) ([]App, error) {
	rows, err := db.Query(
		`SELECT id, name, repo_url, branch, port, start_cmd, build_cmd, work_dir, webhook_secret, health_path, active_port, status, pid, log_path, created_at FROM apps ORDER BY created_at`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apps []App
	for rows.Next() {
		var a App
		if err := rows.Scan(&a.ID, &a.Name, &a.RepoURL, &a.Branch, &a.Port, &a.StartCmd, &a.BuildCmd, &a.WorkDir, &a.WebhookSecret, &a.HealthPath, &a.ActivePort, &a.Status, &a.PID, &a.LogPath, &a.CreatedAt); err != nil {
			return nil, err
		}
		apps = append(apps, a)
	}
	return apps, rows.Err()
}

func GetApp(db *sql.DB, id string) (*App, error) {
	var a App
	err := db.QueryRow(
		`SELECT id, name, repo_url, branch, port, start_cmd, build_cmd, work_dir, webhook_secret, health_path, active_port, status, pid, log_path, created_at FROM apps WHERE id = ?`, id,
	).Scan(&a.ID, &a.Name, &a.RepoURL, &a.Branch, &a.Port, &a.StartCmd, &a.BuildCmd, &a.WorkDir, &a.WebhookSecret, &a.HealthPath, &a.ActivePort, &a.Status, &a.PID, &a.LogPath, &a.CreatedAt)
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

func InsertApp(db *sql.DB, name, startCmd, buildCmd, workDir string, port int, logPath string, repoURL, branch sql.NullString) (*App, error) {
	id := uuid.New().String()
	_, err := db.Exec(
		`INSERT INTO apps (id, name, repo_url, branch, port, start_cmd, build_cmd, work_dir, status, log_path) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'stopped', ?)`,
		id, name, repoURL, branch, port, startCmd, buildCmd, workDir, logPath,
	)
	if err != nil {
		return nil, err
	}
	return GetApp(db, id)
}

func SetWebhookSecret(db *sql.DB, id, secret string) error {
	_, err := db.Exec(`UPDATE apps SET webhook_secret = ? WHERE id = ?`, secret, id)
	return err
}

func SetHealthPath(db *sql.DB, id, path string) error {
	_, err := db.Exec(`UPDATE apps SET health_path = ? WHERE id = ?`, path, id)
	return err
}

func SetActivePort(db *sql.DB, id string, port int) error {
	_, err := db.Exec(`UPDATE apps SET active_port = ? WHERE id = ?`, port, id)
	return err
}

func UpdateAppStatus(db *sql.DB, id, status string, pid sql.NullInt64) error {
	_, err := db.Exec(`UPDATE apps SET status = ?, pid = ? WHERE id = ?`, status, pid, id)
	return err
}

func DeleteApp(db *sql.DB, id string) error {
	_, err := db.Exec(`DELETE FROM apps WHERE id = ?`, id)
	return err
}

func UpdateApp(db *sql.DB, id, name, startCmd, buildCmd, workDir string, port int, logPath string, repoURL, branch sql.NullString) error {
	_, err := db.Exec(
		`UPDATE apps SET name = ?, start_cmd = ?, build_cmd = ?, work_dir = ?, port = ?, log_path = ?, repo_url = ?, branch = ? WHERE id = ?`,
		name, startCmd, buildCmd, workDir, port, logPath, repoURL, branch, id,
	)
	return err
}

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

type Deployment struct {
	ID          string
	AppID       string
	TriggeredBy string
	Status      string
	Detail      string
	StartedAt   string
	FinishedAt  sql.NullString
}

func InsertDeployment(db *sql.DB, appID, triggeredBy string) (string, error) {
	id := uuid.New().String()
	_, err := db.Exec(
		`INSERT INTO deployments (id, app_id, triggered_by, status) VALUES (?, ?, ?, 'running')`,
		id, appID, triggeredBy,
	)
	return id, err
}

func FinishDeployment(db *sql.DB, id, status, detail string) error {
	_, err := db.Exec(
		`UPDATE deployments SET status = ?, detail = ?, finished_at = CURRENT_TIMESTAMP WHERE id = ?`,
		status, detail, id,
	)
	return err
}

func ListDeployments(db *sql.DB, appID string, limit int) ([]Deployment, error) {
	rows, err := db.Query(
		`SELECT id, app_id, triggered_by, status, detail, started_at, finished_at FROM deployments WHERE app_id = ? ORDER BY started_at DESC LIMIT ?`,
		appID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deps []Deployment
	for rows.Next() {
		var d Deployment
		if err := rows.Scan(&d.ID, &d.AppID, &d.TriggeredBy, &d.Status, &d.Detail, &d.StartedAt, &d.FinishedAt); err != nil {
			return nil, err
		}
		deps = append(deps, d)
	}
	return deps, rows.Err()
}

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
