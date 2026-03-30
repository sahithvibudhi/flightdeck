package process

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/nestops/nestops/internal/db"
)

type Manager struct {
	mu       sync.Mutex
	procs    map[string]*exec.Cmd
	database *sql.DB
	dataDir  string
}

func NewManager(database *sql.DB, dataDir string) *Manager {
	return &Manager{
		procs:    make(map[string]*exec.Cmd),
		database: database,
		dataDir:  dataDir,
	}
}

func (m *Manager) StartApp(app *db.App) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, running := m.procs[app.ID]; running {
		return fmt.Errorf("app %s is already running", app.Name)
	}

	appDir := filepath.Join(m.dataDir, "apps", app.Name)
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return fmt.Errorf("create app dir: %w", err)
	}

	if err := m.writeEnvFile(app.ID, appDir); err != nil {
		return fmt.Errorf("write env file: %w", err)
	}

	logFile, err := os.OpenFile(app.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}

	parts := strings.Fields(app.StartCmd)
	if len(parts) == 0 {
		return fmt.Errorf("empty start command")
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = appDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = m.buildEnv(app.ID, app.Port)

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("start process: %w", err)
	}

	m.procs[app.ID] = cmd
	pid := sql.NullInt64{Int64: int64(cmd.Process.Pid), Valid: true}
	if err := db.UpdateAppStatus(m.database, app.ID, "running", pid); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	go m.watchProcess(app.ID, cmd, logFile)

	return nil
}

func (m *Manager) StopApp(appID string) error {
	m.mu.Lock()
	cmd, running := m.procs[appID]
	m.mu.Unlock()

	if !running {
		return nil
	}

	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		cmd.Process.Kill()
	}

	cmd.Wait()

	m.mu.Lock()
	delete(m.procs, appID)
	m.mu.Unlock()

	return db.UpdateAppStatus(m.database, appID, "stopped", sql.NullInt64{})
}

func (m *Manager) RestartApp(app *db.App) error {
	if err := m.StopApp(app.ID); err != nil {
		return err
	}
	return m.StartApp(app)
}

func (m *Manager) IsRunning(appID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.procs[appID]
	return ok
}

func (m *Manager) RestoreRunning() error {
	apps, err := db.ListApps(m.database)
	if err != nil {
		return err
	}
	for _, app := range apps {
		if app.Status == "running" {
			a := app
			if err := m.StartApp(&a); err != nil {
				fmt.Fprintf(os.Stderr, "failed to restore app %s: %v\n", app.Name, err)
			}
		}
	}
	return nil
}

func (m *Manager) watchProcess(appID string, cmd *exec.Cmd, logFile *os.File) {
	cmd.Wait()
	logFile.Close()

	m.mu.Lock()
	delete(m.procs, appID)
	m.mu.Unlock()

	db.UpdateAppStatus(m.database, appID, "stopped", sql.NullInt64{})
}

func (m *Manager) writeEnvFile(appID, appDir string) error {
	envs, err := db.ListEnvs(m.database, appID)
	if err != nil {
		return err
	}

	envPath := filepath.Join(appDir, ".env")
	f, err := os.Create(envPath)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, e := range envs {
		fmt.Fprintf(f, "%s=%s\n", e.Key, e.Value)
	}
	return nil
}

func (m *Manager) buildEnv(appID string, port int) []string {
	env := os.Environ()
	env = append(env, fmt.Sprintf("PORT=%d", port))

	envs, err := db.ListEnvs(m.database, appID)
	if err != nil {
		return env
	}
	for _, e := range envs {
		env = append(env, fmt.Sprintf("%s=%s", e.Key, e.Value))
	}
	return env
}
