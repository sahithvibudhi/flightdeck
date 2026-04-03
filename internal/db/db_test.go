package db

import (
	"database/sql"
	"testing"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func seedConfig(t *testing.T, database *sql.DB) {
	t.Helper()
	err := InsertConfig(database, &Config{
		AdminUsername: "admin",
		AdminPassword: "hashed-pw",
		JWTSecret:     "secret-123",
	})
	if err != nil {
		t.Fatalf("seed config: %v", err)
	}
}

func seedApp(t *testing.T, database *sql.DB, name string, port int) *App {
	t.Helper()
	app, err := InsertApp(database, name, "node index.js", "", port, "/tmp/"+name+".log", sql.NullString{}, sql.NullString{})
	if err != nil {
		t.Fatalf("seed app: %v", err)
	}
	return app
}

func TestOpen(t *testing.T) {
	db := testDB(t)
	if err := db.Ping(); err != nil {
		t.Fatalf("ping failed: %v", err)
	}
}

func TestConfig_InsertAndGet(t *testing.T) {
	db := testDB(t)
	seedConfig(t, db)

	cfg, err := GetConfig(db)
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if cfg.AdminUsername != "admin" {
		t.Errorf("expected admin, got %s", cfg.AdminUsername)
	}
	if cfg.JWTSecret != "secret-123" {
		t.Errorf("expected secret-123, got %s", cfg.JWTSecret)
	}
	if cfg.PanelDomain.Valid {
		t.Error("expected null panel domain")
	}
}

func TestUpdatePanelDomain(t *testing.T) {
	db := testDB(t)
	seedConfig(t, db)

	err := UpdatePanelDomain(db, sql.NullString{String: "admin.example.com", Valid: true})
	if err != nil {
		t.Fatalf("UpdatePanelDomain: %v", err)
	}

	cfg, _ := GetConfig(db)
	if !cfg.PanelDomain.Valid || cfg.PanelDomain.String != "admin.example.com" {
		t.Errorf("expected admin.example.com, got %v", cfg.PanelDomain)
	}

	UpdatePanelDomain(db, sql.NullString{})
	cfg, _ = GetConfig(db)
	if cfg.PanelDomain.Valid {
		t.Error("expected null after clearing")
	}
}

func TestUpdatePassword(t *testing.T) {
	db := testDB(t)
	seedConfig(t, db)

	UpdatePassword(db, "new-hash")
	cfg, _ := GetConfig(db)
	if cfg.AdminPassword != "new-hash" {
		t.Errorf("expected new-hash, got %s", cfg.AdminPassword)
	}
}

func TestUpdateGitToken(t *testing.T) {
	db := testDB(t)
	seedConfig(t, db)

	UpdateGitToken(db, sql.NullString{String: "ghp_abc123", Valid: true})
	cfg, _ := GetConfig(db)
	if !cfg.GitToken.Valid || cfg.GitToken.String != "ghp_abc123" {
		t.Errorf("expected ghp_abc123, got %v", cfg.GitToken)
	}

	UpdateGitToken(db, sql.NullString{})
	cfg, _ = GetConfig(db)
	if cfg.GitToken.Valid {
		t.Error("expected null after clearing")
	}
}

func TestInsertApp(t *testing.T) {
	db := testDB(t)
	app := seedApp(t, db, "myapp", 4000)

	if app.ID == "" {
		t.Error("expected non-empty ID")
	}
	if app.Name != "myapp" {
		t.Errorf("expected myapp, got %s", app.Name)
	}
	if app.Port != 4000 {
		t.Errorf("expected port 4000, got %d", app.Port)
	}
	if app.Status != "stopped" {
		t.Errorf("expected stopped, got %s", app.Status)
	}
}

func TestInsertApp_WithRepo(t *testing.T) {
	db := testDB(t)
	app, err := InsertApp(db, "gitapp", "npm start", "", 4001, "/tmp/gitapp.log",
		sql.NullString{String: "https://github.com/user/repo", Valid: true},
		sql.NullString{String: "main", Valid: true},
	)
	if err != nil {
		t.Fatalf("InsertApp: %v", err)
	}
	if !app.RepoURL.Valid || app.RepoURL.String != "https://github.com/user/repo" {
		t.Error("expected repo URL")
	}
	if !app.Branch.Valid || app.Branch.String != "main" {
		t.Error("expected branch main")
	}
}

func TestGetApp_NotFound(t *testing.T) {
	db := testDB(t)
	_, err := GetApp(db, "nonexistent-id")
	if err == nil {
		t.Error("expected error for missing app")
	}
}

func TestListApps(t *testing.T) {
	db := testDB(t)

	apps, err := ListApps(db)
	if err != nil {
		t.Fatalf("ListApps: %v", err)
	}
	if len(apps) != 0 {
		t.Errorf("expected 0 apps, got %d", len(apps))
	}

	seedApp(t, db, "app1", 4000)
	seedApp(t, db, "app2", 4001)

	apps, _ = ListApps(db)
	if len(apps) != 2 {
		t.Errorf("expected 2 apps, got %d", len(apps))
	}
}

func TestNextPort(t *testing.T) {
	db := testDB(t)

	port, err := NextPort(db)
	if err != nil {
		t.Fatalf("NextPort: %v", err)
	}
	if port != 4000 {
		t.Errorf("expected 4000, got %d", port)
	}

	seedApp(t, db, "app1", 4000)
	port, _ = NextPort(db)
	if port != 4001 {
		t.Errorf("expected 4001, got %d", port)
	}

	seedApp(t, db, "app2", 4005)
	port, _ = NextPort(db)
	if port != 4006 {
		t.Errorf("expected 4006 (max+1), got %d", port)
	}
}

func TestUpdateAppStatus(t *testing.T) {
	db := testDB(t)
	app := seedApp(t, db, "myapp", 4000)

	UpdateAppStatus(db, app.ID, "running", sql.NullInt64{Int64: 12345, Valid: true})
	updated, _ := GetApp(db, app.ID)
	if updated.Status != "running" {
		t.Errorf("expected running, got %s", updated.Status)
	}
	if !updated.PID.Valid || updated.PID.Int64 != 12345 {
		t.Errorf("expected PID 12345, got %v", updated.PID)
	}
}

func TestDeleteApp(t *testing.T) {
	db := testDB(t)
	app := seedApp(t, db, "myapp", 4000)

	DeleteApp(db, app.ID)
	_, err := GetApp(db, app.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestAppNameUnique(t *testing.T) {
	db := testDB(t)
	seedApp(t, db, "myapp", 4000)

	_, err := InsertApp(db, "myapp", "cmd", "", 4001, "/tmp/x.log", sql.NullString{}, sql.NullString{})
	if err == nil {
		t.Error("expected unique constraint error")
	}
}

func TestPortUnique(t *testing.T) {
	db := testDB(t)
	seedApp(t, db, "app1", 4000)

	_, err := InsertApp(db, "app2", "cmd", "", 4000, "/tmp/x.log", sql.NullString{}, sql.NullString{})
	if err == nil {
		t.Error("expected unique constraint error")
	}
}

func TestEnvs(t *testing.T) {
	db := testDB(t)
	app := seedApp(t, db, "myapp", 4000)

	envs, _ := ListEnvs(db, app.ID)
	if len(envs) != 0 {
		t.Errorf("expected 0 envs, got %d", len(envs))
	}

	err := ReplaceEnvs(db, app.ID, []Env{
		{Key: "PORT", Value: "3000"},
		{Key: "NODE_ENV", Value: "production"},
	})
	if err != nil {
		t.Fatalf("ReplaceEnvs: %v", err)
	}

	envs, _ = ListEnvs(db, app.ID)
	if len(envs) != 2 {
		t.Fatalf("expected 2 envs, got %d", len(envs))
	}

	err = ReplaceEnvs(db, app.ID, []Env{
		{Key: "DATABASE_URL", Value: "sqlite://db"},
	})
	if err != nil {
		t.Fatalf("ReplaceEnvs (replace): %v", err)
	}

	envs, _ = ListEnvs(db, app.ID)
	if len(envs) != 1 {
		t.Errorf("expected 1 env after replace, got %d", len(envs))
	}
	if envs[0].Key != "DATABASE_URL" {
		t.Errorf("expected DATABASE_URL, got %s", envs[0].Key)
	}
}

func TestDomains(t *testing.T) {
	db := testDB(t)
	app := seedApp(t, db, "myapp", 4000)

	domains, _ := ListDomains(db, app.ID)
	if len(domains) != 0 {
		t.Errorf("expected 0 domains, got %d", len(domains))
	}

	d, err := InsertDomain(db, app.ID, "example.com")
	if err != nil {
		t.Fatalf("InsertDomain: %v", err)
	}
	if d.Domain != "example.com" {
		t.Errorf("expected example.com, got %s", d.Domain)
	}

	domains, _ = ListDomains(db, app.ID)
	if len(domains) != 1 {
		t.Fatalf("expected 1 domain, got %d", len(domains))
	}

	found, err := GetDomainByName(db, "example.com")
	if err != nil {
		t.Fatalf("GetDomainByName: %v", err)
	}
	if found.AppID != app.ID {
		t.Error("domain should belong to the app")
	}

	DeleteDomain(db, d.ID)
	domains, _ = ListDomains(db, app.ID)
	if len(domains) != 0 {
		t.Errorf("expected 0 after delete, got %d", len(domains))
	}
}

func TestDomainUnique(t *testing.T) {
	db := testDB(t)
	app := seedApp(t, db, "myapp", 4000)

	InsertDomain(db, app.ID, "example.com")
	_, err := InsertDomain(db, app.ID, "example.com")
	if err == nil {
		t.Error("expected unique constraint error")
	}
}

func TestListAllDomains(t *testing.T) {
	db := testDB(t)
	app1 := seedApp(t, db, "app1", 4000)
	app2 := seedApp(t, db, "app2", 4001)

	InsertDomain(db, app1.ID, "one.com")
	InsertDomain(db, app2.ID, "two.com")

	all, _ := ListAllDomains(db)
	if len(all) != 2 {
		t.Errorf("expected 2, got %d", len(all))
	}
}

func TestCascadeDelete(t *testing.T) {
	db := testDB(t)
	app := seedApp(t, db, "myapp", 4000)

	ReplaceEnvs(db, app.ID, []Env{{Key: "A", Value: "1"}})
	InsertDomain(db, app.ID, "test.com")

	DeleteApp(db, app.ID)

	envs, _ := ListEnvs(db, app.ID)
	if len(envs) != 0 {
		t.Errorf("expected envs cascade deleted, got %d", len(envs))
	}

	domains, _ := ListDomains(db, app.ID)
	if len(domains) != 0 {
		t.Errorf("expected domains cascade deleted, got %d", len(domains))
	}
}
