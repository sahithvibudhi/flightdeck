package process

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/sahithvibudhi/flightdeck/internal/db"
	"github.com/sahithvibudhi/flightdeck/internal/notify"
)

const (
	// standbyOffset is added to an app's port for the temporary process
	// during zero-downtime restarts. Apps must listen on $PORT for this
	// to work.
	standbyOffset = 10000

	// After this many consecutive crashes the app is marked "crashed"
	// and no further restarts are attempted.
	maxRestarts = 5

	// A process that stays up this long resets its crash counter.
	stableUptime = 60 * time.Second

	healthTimeout  = 60 * time.Second
	healthInterval = 500 * time.Millisecond

	// Log files larger than this are truncated (keeping the tail) on the
	// next start, so a chatty app can't fill the disk.
	maxLogSize  = 5 << 20
	logKeepSize = 512 << 10
)

var restartBackoff = []time.Duration{time.Second, 5 * time.Second, 15 * time.Second, 30 * time.Second, time.Minute}

type proc struct {
	cmd       *exec.Cmd // nil for processes adopted after a flightdeck restart
	pid       int
	done      chan struct{}
	startedAt time.Time
	expected  bool // guarded by Manager.mu; true when the exit was requested
}

type Manager struct {
	mu       sync.Mutex
	procs    map[string]*proc
	restarts map[string]int
	database *sql.DB
	dataDir  string

	// switchRoutes re-points an app's domain routes at a new port.
	// Injected from main so this package stays decoupled from the proxy.
	switchRoutes func(appID string, port int) error
}

func NewManager(database *sql.DB, dataDir string) *Manager {
	return &Manager{
		procs:    make(map[string]*proc),
		restarts: make(map[string]int),
		database: database,
		dataDir:  dataDir,
	}
}

func (m *Manager) SetRouteSwitcher(fn func(appID string, port int) error) {
	m.switchRoutes = fn
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
	return m.startApp(app, true, nil)
}

func (m *Manager) startApp(app *db.App, runBuild bool, deployLog io.Writer) error {
	m.mu.Lock()
	if _, running := m.procs[app.ID]; running {
		m.mu.Unlock()
		return fmt.Errorf("app %s is already running", app.Name)
	}
	m.mu.Unlock()

	p, err := m.launch(app, app.Port, runBuild, deployLog)
	if err != nil {
		return err
	}

	m.mu.Lock()
	if _, running := m.procs[app.ID]; running {
		m.mu.Unlock()
		signalGroup(p.pid, syscall.SIGKILL)
		return fmt.Errorf("app %s is already running", app.Name)
	}
	m.procs[app.ID] = p
	m.mu.Unlock()

	// A regular start always runs on the primary port; clear any standby
	// port left over from a zero-downtime deploy and re-point domains.
	if app.ActivePort > 0 && app.ActivePort != app.Port {
		db.SetActivePort(m.database, app.ID, 0)
		if m.switchRoutes != nil {
			m.switchRoutes(app.ID, app.Port)
		}
	}

	pid := sql.NullInt64{Int64: int64(p.pid), Valid: true}
	if err := db.UpdateAppStatus(m.database, app.ID, "running", pid); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	go m.watchProcess(app.ID, p)

	return nil
}

/*
launch builds (optionally) and starts the app process on the given port.
It does not register the process in the procs map — callers decide how
the process participates in the app lifecycle. A single goroutine owns
cmd.Wait and closes done when the process exits.
*/
func (m *Manager) launch(app *db.App, port int, runBuild bool, deployLog io.Writer) (*proc, error) {
	appDir := m.AppDir(app)
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return nil, fmt.Errorf("create app dir: %w", err)
	}

	if err := m.writeEnvFile(app.ID, appDir); err != nil {
		return nil, fmt.Errorf("write env file: %w", err)
	}

	capLog(app.LogPath)

	logFile, err := os.OpenFile(app.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}

	if runBuild && app.BuildCmd != "" {
		// Build output lands in the app log as before, and also in the
		// deploy log when a deploy is being watched live.
		buildOut := io.Writer(logFile)
		if deployLog != nil {
			buildOut = io.MultiWriter(logFile, deployLog)
		}
		fmt.Fprintf(buildOut, "=== Running build: %s ===\n", app.BuildCmd)
		buildCmd := exec.Command("sh", "-c", app.BuildCmd)
		buildCmd.Dir = appDir
		buildCmd.Stdout = buildOut
		buildCmd.Stderr = buildOut
		buildCmd.Env = m.buildEnv(app.ID, port)
		if err := buildCmd.Run(); err != nil {
			fmt.Fprintf(buildOut, "=== Build failed: %v ===\n", err)
			logFile.Close()
			return nil, fmt.Errorf("build command failed: %w", err)
		}
		fmt.Fprintf(buildOut, "=== Build complete ===\n")
	}

	if strings.TrimSpace(app.StartCmd) == "" {
		logFile.Close()
		return nil, fmt.Errorf("empty start command")
	}

	// The start command runs through a shell (like the build command) so
	// quoting, pipes, && and env expansion all work. Setpgid puts the shell
	// and everything it spawns in one process group, so stop/kill reaches
	// the whole tree instead of orphaning grandchildren.
	cmd := exec.Command("sh", "-c", app.StartCmd)
	cmd.Dir = appDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = m.buildEnv(app.ID, port)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return nil, fmt.Errorf("start process: %w", err)
	}

	p := &proc{
		cmd:       cmd,
		pid:       cmd.Process.Pid,
		done:      make(chan struct{}),
		startedAt: time.Now(),
	}

	go func() {
		cmd.Wait()
		logFile.Close()
		close(p.done)
	}()

	return p, nil
}

func (m *Manager) StopApp(appID string) error {
	m.mu.Lock()
	p, running := m.procs[appID]
	if running {
		p.expected = true
	}
	m.mu.Unlock()

	if !running {
		// Clear a pending "restarting"/"crashed" state so a stop always
		// lands the app in a clean stopped state.
		return db.UpdateAppStatus(m.database, appID, "stopped", sql.NullInt64{})
	}

	stopProc(p)

	return db.UpdateAppStatus(m.database, appID, "stopped", sql.NullInt64{})
}

func stopProc(p *proc) {
	signalGroup(p.pid, syscall.SIGINT)

	select {
	case <-p.done:
	case <-time.After(10 * time.Second):
		signalGroup(p.pid, syscall.SIGKILL)
		// Adopted processes are watched by a liveness poller rather than
		// cmd.Wait, so give it a moment to observe the kill.
		select {
		case <-p.done:
		case <-time.After(2 * adoptedPollInterval):
		}
	}
}

func (m *Manager) RestartApp(app *db.App) error {
	if err := m.StopApp(app.ID); err != nil {
		return err
	}
	return m.StartApp(app)
}

/*
DeployRestart restarts an app to pick up new code. When the app has a
health check configured and is currently running, the restart is
zero-downtime: the new process starts on a standby port, traffic is
switched only after the health check passes, and the old process is
stopped last. Without a health check it falls back to stop+start.

deployLog, when non-nil, receives stage progress and build output so
callers can surface a live view of the deploy. Pass nil when nobody is
watching.
*/
func (m *Manager) DeployRestart(app *db.App, deployLog io.Writer) error {
	if app.HealthPath == "" || !m.IsRunning(app.ID) {
		if m.IsRunning(app.ID) {
			logStage(deployLog, "Stopping current process")
			if err := m.StopApp(app.ID); err != nil {
				return err
			}
		}
		logStage(deployLog, "Starting on port %d", app.Port)
		return m.startApp(app, true, deployLog)
	}
	return m.zeroDowntimeRestart(app, deployLog)
}

// logStage writes a deploy progress line, tolerating a nil writer.
func logStage(w io.Writer, format string, args ...interface{}) {
	if w == nil {
		return
	}
	fmt.Fprintf(w, "=== "+format+" ===\n", args...)
}

func (m *Manager) zeroDowntimeRestart(app *db.App, deployLog io.Writer) error {
	oldPort := app.EffectivePort()
	newPort := app.Port
	if newPort == oldPort {
		newPort = app.Port + standbyOffset
	}

	logStage(deployLog, "Starting new process on standby port %d", newPort)
	p, err := m.launch(app, newPort, true, deployLog)
	if err != nil {
		return err
	}

	logStage(deployLog, "Waiting for health check on %s", app.HealthPath)
	if err := waitHealthy(newPort, app.HealthPath, p.done); err != nil {
		signalGroup(p.cmd.Process.Pid, syscall.SIGKILL)
		return fmt.Errorf("new process failed health check, old process still serving: %w", err)
	}

	// The new process is healthy: switch traffic, then retire the old one.
	logStage(deployLog, "Healthy, switching traffic to port %d", newPort)
	if m.switchRoutes != nil {
		if err := m.switchRoutes(app.ID, newPort); err != nil {
			signalGroup(p.cmd.Process.Pid, syscall.SIGKILL)
			return fmt.Errorf("failed to switch routes, old process still serving: %w", err)
		}
	}

	activePort := newPort
	if newPort == app.Port {
		activePort = 0
	}
	db.SetActivePort(m.database, app.ID, activePort)

	m.mu.Lock()
	old := m.procs[app.ID]
	if old != nil {
		old.expected = true
	}
	m.procs[app.ID] = p
	m.mu.Unlock()

	go m.watchProcess(app.ID, p)

	if old != nil {
		logStage(deployLog, "Stopping old process")
		stopProc(old)
	}

	pid := sql.NullInt64{Int64: int64(p.cmd.Process.Pid), Valid: true}
	return db.UpdateAppStatus(m.database, app.ID, "running", pid)
}

func waitHealthy(port int, path string, died <-chan struct{}) error {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	url := fmt.Sprintf("http://127.0.0.1:%d%s", port, path)
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(healthTimeout)

	for time.Now().Before(deadline) {
		select {
		case <-died:
			return fmt.Errorf("process exited before becoming healthy")
		default:
		}
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 400 {
				return nil
			}
		}
		time.Sleep(healthInterval)
	}
	return fmt.Errorf("health check %s did not pass within %s", url, healthTimeout)
}

func (m *Manager) IsRunning(appID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.procs[appID]
	return ok
}

const adoptedPollInterval = 3 * time.Second

func (m *Manager) RestoreRunning() error {
	apps, err := db.ListApps(m.database)
	if err != nil {
		return err
	}
	for _, app := range apps {
		if app.Status != "running" && app.Status != "restarting" {
			continue
		}
		a := app

		// App processes survive flightdeck restarts and upgrades. If the
		// recorded process is still alive, adopt it instead of spawning a
		// duplicate that would lose the port-bind fight.
		if a.PID.Valid && pidAlive(int(a.PID.Int64)) {
			m.adopt(a.ID, int(a.PID.Int64))
			continue
		}

		// Skip the build on restore: a reboot shouldn't re-run every
		// app's npm install. Builds happen on deploy, pull, and
		// explicit start/restart.
		if err := m.startApp(&a, false, nil); err != nil {
			fmt.Fprintf(os.Stderr, "failed to restore app %s: %v\n", app.Name, err)
		}
	}
	return nil
}

/*
adopt registers an already-running process (from before a flightdeck
restart) so stop/restart/metrics keep working. We can't cmd.Wait a
process we didn't spawn, so a poller watches liveness instead.
*/
func (m *Manager) adopt(appID string, pid int) {
	p := &proc{
		pid:       pid,
		done:      make(chan struct{}),
		startedAt: time.Now(),
	}

	m.mu.Lock()
	if _, running := m.procs[appID]; running {
		m.mu.Unlock()
		return
	}
	m.procs[appID] = p
	m.mu.Unlock()

	go func() {
		ticker := time.NewTicker(adoptedPollInterval)
		defer ticker.Stop()
		for range ticker.C {
			if !pidAlive(pid) {
				close(p.done)
				return
			}
		}
	}()

	go m.watchProcess(appID, p)
}

func pidAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
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
watchProcess reacts to a process exiting. If this proc is still the
app's current process it is deregistered; an exit that wasn't requested
(crash) triggers restart-with-backoff, giving up after maxRestarts
consecutive quick failures.
*/
func (m *Manager) watchProcess(appID string, p *proc) {
	<-p.done

	m.mu.Lock()
	expected := p.expected
	current := m.procs[appID] == p
	if current {
		delete(m.procs, appID)
	}
	m.mu.Unlock()

	if !current {
		// Superseded by a zero-downtime deploy; the new process owns the app.
		return
	}

	if expected {
		db.UpdateAppStatus(m.database, appID, "stopped", sql.NullInt64{})
		return
	}

	m.handleCrash(appID, p)
}

func (m *Manager) handleCrash(appID string, p *proc) {
	m.mu.Lock()
	if time.Since(p.startedAt) > stableUptime {
		m.restarts[appID] = 0
	}
	attempt := m.restarts[appID]
	m.restarts[appID] = attempt + 1
	m.mu.Unlock()

	if attempt >= maxRestarts {
		db.UpdateAppStatus(m.database, appID, "crashed", sql.NullInt64{})
		m.appendAppLog(appID, fmt.Sprintf("=== Still crashing after %d restart attempts, giving up. Fix the app and press Start. ===", maxRestarts))
		if app, err := db.GetApp(m.database, appID); err == nil {
			notify.Go(m.database, "App crashed: "+app.Name,
				fmt.Sprintf("Gave up after %d restart attempts. Check the logs and press Start.", maxRestarts))
		}
		return
	}

	delay := restartBackoff[attempt]
	db.UpdateAppStatus(m.database, appID, "restarting", sql.NullInt64{})
	m.appendAppLog(appID, fmt.Sprintf("=== Process exited unexpectedly, restarting in %s (attempt %d/%d) ===", delay, attempt+1, maxRestarts))

	time.AfterFunc(delay, func() {
		app, err := db.GetApp(m.database, appID)
		if err != nil {
			return // app was deleted
		}
		if app.Status != "restarting" {
			return // user intervened (stopped or started it manually)
		}
		if err := m.startApp(app, false, nil); err != nil {
			db.UpdateAppStatus(m.database, appID, "crashed", sql.NullInt64{})
		}
	})
}

func (m *Manager) appendAppLog(appID, line string) {
	app, err := db.GetApp(m.database, appID)
	if err != nil {
		return
	}
	f, err := os.OpenFile(app.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintln(f, line)
}

/*
capLog keeps log files bounded: once a log passes maxLogSize it is
rewritten in place keeping only the most recent logKeepSize bytes.
*/
func capLog(path string) {
	info, err := os.Stat(path)
	if err != nil || info.Size() <= maxLogSize {
		return
	}
	f, err := os.Open(path)
	if err != nil {
		return
	}
	buf := make([]byte, logKeepSize)
	n, err := f.ReadAt(buf, info.Size()-logKeepSize)
	f.Close()
	if err != nil && n == 0 {
		return
	}
	os.WriteFile(path, buf[:n], 0644)
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
