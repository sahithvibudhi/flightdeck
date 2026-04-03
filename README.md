# flightdeck

One binary. One command. Your VPS, under control.

Flightdeck is a self-hosted VPS control plane. Install it on a fresh server, answer three questions, and get a dashboard to deploy and manage multiple apps with automatic SSL, domain routing, environment variables, and process control.

## Install

```bash
curl -sSL https://raw.githubusercontent.com/sahithvibudhi/flightdeck/main/scripts/install.sh | sudo bash
```

Then run the setup wizard:

```bash
/usr/local/bin/flightdeck
```

Start the service:

```bash
sudo systemctl start flightdeck
```

Access the dashboard at `http://your-server-ip:3000`.

## What it does

- **Deploy apps** from GitHub, a local directory, or a zip upload
- **Build commands** — run `npm install`, `pip install`, etc. before starting
- **Automatic SSL** via Caddy reverse proxy — assign a domain and it just works
- **Environment variables** per app, managed from the UI
- **Process control** — start, stop, restart from the dashboard
- **Editable config** — change start command, build command, port, source after deploy
- **Git pull** to update apps without redeploying
- **Server monitoring** — CPU, memory, disk usage with sparkline trends
- **Runtime management** — detect and install runtimes (Node, Python, Go, Docker, Bun, Deno) from Settings

## Dashboard

The dashboard shows four server stat cards (CPU, memory, disk, running apps) with sparkline charts, plus a card grid of all deployed apps with per-process CPU and memory.

## Deploying an app

Navigate to `/deploy`. Pick a source tab:

- **Server Path** — point to a directory already on the server
- **Upload Zip** — drag and drop a `.zip` file
- **GitHub** — clone from a repository (with branch selection)

Then fill in the app name, start command, and optionally a build command (e.g. `npm install && npm run build`) and the port your app listens on. Environment variables can be added here or later from the app detail page.

Private repos work with a GitHub personal access token configured in Settings.

## Architecture

```
flightdeck binary
├── Go API server (:3000)
│   ├── REST API (/api/*)
│   └── Embedded React UI (/)
├── SQLite database (flightdeck.db)
├── Caddy subprocess (:80/:443)
│   └── Admin API (:2019)
└── App processes (ports 4000+)
```

Everything runs as a single binary. The React UI is compiled and embedded at build time. SQLite is the only database. Caddy handles SSL certificates automatically.

## API

All routes under `/api`. All except login require `Authorization: Bearer <jwt>`.

### Auth

```
POST   /api/auth/login              {username, password} → {token}
POST   /api/auth/password           {current, new}
```

### Apps

```
GET    /api/apps                    List all apps
POST   /api/apps                    Create app {name, start_command, build_command?, port?, repo_url?, branch?}
GET    /api/apps/:id                Get app detail
PUT    /api/apps/:id                Update app config
DELETE /api/apps/:id                Stop and remove app
POST   /api/apps/:id/upload         Upload zip to app directory
POST   /api/apps/:id/start          Start app
POST   /api/apps/:id/stop           Stop app
POST   /api/apps/:id/restart        Restart app
POST   /api/apps/:id/pull           Git pull latest changes
GET    /api/apps/:id/logs?lines=100 Get log output
```

### Environment variables

```
GET    /api/apps/:id/envs           List env vars
PUT    /api/apps/:id/envs           Replace all env vars [{key, value}]
```

### Domains

```
GET    /api/apps/:id/domains        List domains
POST   /api/apps/:id/domains        Add domain {domain}
DELETE /api/apps/:id/domains/:name  Remove domain
```

### Settings

```
GET    /api/settings                Get settings
PUT    /api/settings/domain         Set panel domain {domain}
PUT    /api/settings/git-token      Set GitHub token {token}
GET    /api/system                  Runtime detection + Caddy status
GET    /api/system/metrics          Server metrics history (last 120 snapshots)
POST   /api/system/install          Install a runtime {name}
```

## Development

```bash
# Run the API server
FLIGHTDECK_DATA_DIR=./data go run ./cmd/flightdeck

# Run the UI dev server (proxies /api to :3000)
cd ui && npm run dev

# Build everything
make build

# Run tests
make test

# Cross-compile for Linux
make release
```

### Testing with OrbStack

Flightdeck targets Linux but you can develop on macOS and test in an OrbStack VM. This is the fastest way to get a real Linux environment without leaving your Mac.

**Setup (one-time):**

```bash
# Create an Ubuntu VM
orb create ubuntu test-vm

# Point a local hostname to the VM (grab the IP from `orb list`)
sudo sh -c 'echo "192.168.139.25 flightdeck.local" >> /etc/hosts'
```

**Build and deploy:**

```bash
# Build the Linux binary
make build-linux-arm64    # or build-linux-amd64

# Copy to the VM
scp dist/flightdeck-linux-arm64 test-vm@orb:/tmp/flightdeck

# SSH in and run it
ssh test-vm@orb
chmod +x /tmp/flightdeck
sudo FLIGHTDECK_DATA_DIR=/tmp/flightdeck-data /tmp/flightdeck
```

Open `http://flightdeck.local:3000` in your browser.

To redeploy after changes: kill the old process, copy the new binary, run again. The data directory persists between runs so you don't need to re-run the setup wizard.

## Contributing

1. Fork the repo and create a branch off `main`
2. Make your changes — keep commits focused
3. Run `make test` and make sure everything passes
4. If you changed the UI, run `cd ui && npm run lint`
5. Open a PR with a short description of what and why

A few things to keep in mind:

- The binary should stay self-contained. No external runtime dependencies at build time beyond Go and Node.
- SQLite schema changes go in `internal/db/db.go` as new entries in the `migrations` slice. Use `ALTER TABLE` for additions — the migration runner handles "duplicate column" errors gracefully.
- API handlers live in `internal/api/`. Follow the existing pattern: request struct, handler method on the relevant handler type, route in `router.go`.
- Frontend is plain React with no state library. Keep it simple — `useState` and `fetch` calls to `api.ts`.
- CSS uses a single `style.css` with CSS variables. No CSS-in-JS, no utility classes beyond the small set already there.

## Project structure

```
cmd/flightdeck/main.go       Entry point, startup sequence
internal/
  api/                       HTTP handlers (chi router)
  auth/                      bcrypt + JWT
  db/                        SQLite schema + queries
  git/                       Clone and pull operations
  process/                   App lifecycle, logs, metrics
  proxy/                     Caddy subprocess + route management
  setup/                     First-boot wizard, runtime installer
  system/                    Runtime detection, server metrics
ui/
  src/pages/                 React pages (Apps, AppDetail, Deploy, Settings, Login)
  src/api.ts                 Typed fetch client
  src/style.css              Design system
scripts/install.sh           Production install script
Makefile                     Build targets
```

## Tech stack

| Component | Choice |
|---|---|
| Language | Go |
| UI | React + TypeScript (Vite) |
| Database | SQLite (modernc.org/sqlite, pure Go) |
| Reverse proxy | Caddy (subprocess, auto SSL) |
| Router | chi |
| Auth | bcrypt + JWT (30-day tokens) |
| Process management | os/exec |

Zero CGO. Single static binary. No Docker required.

## Requirements

- Linux VPS (amd64 or arm64)
- Port 80 and 443 open (for Caddy SSL)
- Port 3000 open (for dashboard, or use a domain)

## License

MIT
