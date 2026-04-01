# flightdeck

One binary. One command. Your VPS, under control.

Flightdeck is a self-hosted VPS control plane. Install it on a fresh server, answer three questions, and get a dashboard to deploy and manage multiple apps with automatic SSL, domain routing, environment variables, and process control.

## Install

```bash
curl -sSL https://raw.githubusercontent.com/sahithvibudhi/flightdeck/main/scripts/install.sh | sudo bash
```

Then run the setup wizard:

```bash
/usr/local/bin/nestops
```

Start the service:

```bash
sudo systemctl start flightdeck
```

Access the dashboard at `http://your-server-ip:3000`.

## What it does

- **Deploy apps** from GitHub or a local directory on your server
- **Automatic SSL** via Caddy reverse proxy — assign a domain and it just works
- **Environment variables** per app, managed from the UI
- **Process control** — start, stop, restart from the dashboard
- **Git pull** to update apps without redeploying
- **Server monitoring** — CPU, memory, disk usage with sparkline trends
- **Runtime detection** — shows which tools are installed (Node, Python, Go, Docker, etc.)

## Dashboard

The dashboard shows four server stat cards (CPU, memory, disk, running apps) with sparkline charts, plus a card grid of all deployed apps with per-process CPU and memory.

## Deploy wizard

Navigate to `/deploy` for a three-step wizard:

1. **Source** — local directory or GitHub repository (with branch selection)
2. **Config** — app name, start command
3. **Environment** — key/value pairs (optional, can add later)

Private repos work with a GitHub personal access token configured in Settings.

## Architecture

```
flightdeck binary
├── Go API server (:3000)
│   ├── REST API (/api/*)
│   └── Embedded React UI (/)
├── SQLite database (nestops.db)
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
POST   /api/apps                    Create app {name, start_command, repo_url?, branch?}
GET    /api/apps/:id                Get app detail
DELETE /api/apps/:id                Stop and remove app
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
```

## Development

```bash
# Run the API server
NESTOPS_DATA_DIR=./data go run ./cmd/nestops

# Run the UI dev server (proxies /api to :3000)
cd ui && npm run dev

# Build everything
make build

# Run tests
make test

# Cross-compile for Linux
make release
```

## Project structure

```
cmd/nestops/main.go          Entry point, startup sequence
internal/
  api/                       HTTP handlers (chi router)
  auth/                      bcrypt + JWT
  db/                        SQLite schema + queries
  git/                       Clone and pull operations
  process/                   App lifecycle, logs, metrics
  proxy/                     Caddy subprocess + route management
  setup/                     First-boot wizard
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
