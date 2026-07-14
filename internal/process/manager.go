package process

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/sahithvibudhi/flightdeck/internal/db"
)

type proc struct {
	cmd     *exec.Cmd
	done    chan struct{}
	logFile *os.File
}

type Manager struct {
	mu       sync.Mutex
	procs    map[string]*proc
	database *sql.DB
	dataDir  string
}

func NewManager(database *sql.DB, dataDir string) *Manager {
	return &Manager{
		procs:    make(map[string]*proc),
		database: database,
		dataDir:  dataDir,
	}
}

/*
AppDir returns the directory an app runs in: its configured work_dir
when deployed from an existing server path, otherwise the managed
directory under the data dir.
*/
func (m *Manager) AppDir(app *db.App) string {
	if app.WorkDir != "" {
		return app.WorkDir
	}
	return filepath.Join(m.dataDir, "apps", app.Name)
}

func (m *Manager) StartApp(app *db.App) error {
	return m.startApp(app, true)
}

func (m *Manager) startApp(app *db.App, runBuild bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, running := m.procs[app.ID]; running {
		return fmt.Errorf("app %s is already running", app.Name)
	}

	appDir := m.AppDir(app)
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

	if runBuild && app.BuildCmd != "" {
		fmt.Fprintf(logFile, "=== Running build: %s ===\n", app.BuildCmd)
		buildCmd := exec.Command("sh", "-c", app.BuildCmd)
		buildCmd.Dir = appDir
		buildCmd.Stdout = logFile
		buildCmd.Stderr = logFile
		buildCmd.Env = m.buildEnv(app.ID, app.Port)
		if err := buildCmd.Run(); err != nil {
			fmt.Fprintf(logFile, "=== Build failed: %v ===\n", err)
			logFile.Close()
			return fmt.Errorf("build command failed: %w", err)
		}
		fmt.Fprintf(logFile, "=== Build complete ===\n")
	}

	if strings.TrimSpace(app.StartCmd) == "" {
		logFile.Close()
		return fmt.Errorf("empty start command")
	}

	// The start command runs through a shell (like the build command) so
	// quoting, pipes, && and env expansion all work. Setpgid puts the shell
	// and everything it spawns in one process group, so stop/kill reaches
	// the whole tree instead of orphaning grandchildren.
	cmd := exec.Command("sh", "-c", app.StartCmd)
	cmd.Dir = appDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = m.buildEnv(app.ID, app.Port)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("start process: %w", err)
	}

	p := &proc{
		cmd:     cmd,
		done:    make(chan struct{}),
		logFile: logFile,
	}
	m.procs[app.ID] = p

	pid := sql.NullInt64{Int64: int64(cmd.Process.Pid), Valid: true}
	if err := db.UpdateAppStatus(m.database, app.ID, "running", pid); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	go m.watchProcess(app.ID, p)

	return nil
}

func (m *Manager) StopApp(appID string) error {
	m.mu.Lock()
	p, running := m.procs[appID]
	m.mu.Unlock()

	if !running {
		return nil
	}

	signalGroup(p.cmd.Process.Pid, syscall.SIGINT)

	select {
	case <-p.done:
	case <-time.After(10 * time.Second):
		signalGroup(p.cmd.Process.Pid, syscall.SIGKILL)
		<-p.done
	}

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
			// Skip the build on restore: a reboot shouldn't re-run every
			// app's npm install. Builds happen on deploy, pull, and
			// explicit start/restart.
			if err := m.startApp(&a, false); err != nil {
				fmt.Fprintf(os.Stderr, "failed to restore app %s: %v\n", app.Name, err)
			}
		}
	}
	return nil
}

/*
signalGroup signals the entire process group so children spawned by the
start command's shell receive it too. Falls back to the single process
if the group signal fails.
*/
func signalGroup(pid int, sig syscall.Signal) {
	if err := syscall.Kill(-pid, sig); err != nil {
		syscall.Kill(pid, sig)
	}
}

/*
watchProcess waits for the process to exit, then cleans up the
log file handle and removes the process from the map. The done
channel signals StopApp that the process has fully exited, avoiding
the race where both watchProcess and StopApp call cmd.Wait().
*/
func (m *Manager) watchProcess(appID string, p *proc) {
	p.cmd.Wait()
	p.logFile.Close()
	close(p.done)

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
