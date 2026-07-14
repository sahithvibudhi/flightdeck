# flightdeck

[![CI](https://github.com/sahithvibudhi/flightdeck/actions/workflows/ci.yml/badge.svg)](https://github.com/sahithvibudhi/flightdeck/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/sahithvibudhi/flightdeck?include_prereleases)](https://github.com/sahithvibudhi/flightdeck/releases)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/sahithvibudhi/flightdeck)](https://goreportcard.com/report/github.com/sahithvibudhi/flightdeck)

One binary. One command. Your VPS, under control.

Flightdeck is a control plane for indie developers who deploy to a VPS. Install it on a $5 server, open the dashboard, and deploy apps with automatic SSL, push-to-deploy webhooks, zero-downtime restarts, and crash recovery — **without Docker**.

```bash
curl -sSL https://raw.githubusercontent.com/sahithvibudhi/flightdeck/main/scripts/install.sh | sudo bash
```

Then open `http://your-server-ip:3000` and create your admin account in the browser. That's the whole install.

![flightdeck dashboard](docs/screenshots/dashboard-dark.png)

## Why flightdeck?

Most deployment tools assume you're running Kubernetes, or Docker, or a managed platform that costs $20/month per app. Coolify — great as it is — wants 2 CPUs and 2 GB of RAM *before your apps start*, because the platform itself is a PHP app plus PostgreSQL plus Redis plus workers. If you're an indie hacker shipping a side project on a cheap VPS, the platform shouldn't be your biggest tenant.

Flightdeck is one **17 MB static binary** that idles at about **15 MB of RAM**. Your 512 MB VPS stays yours.

**No Docker.** This is intentional. Docker adds memory overhead, image build times, and complexity that doesn't pay off when you're running a few apps on a small server. Flightdeck runs your code directly — the same way you'd run it if you SSH'd in and typed `node server.js`.

**No lock-in, no platform risk.** Everything lives in SQLite next to a single binary. Your apps are plain OS processes: if flightdeck stops, crashes, or is upgraded, **your apps keep serving traffic** — flightdeck re-adopts the running processes when it comes back. If you stop using flightdeck entirely, your apps are still just processes on your server.

**Small attack surface.** One Go binary, one SQLite file, Caddy for TLS. No exposed database, no queue, no PHP runtime. Login is rate-limited, webhooks are HMAC-signed, and the setup endpoint permanently closes after first use.

## How it compares

| | flightdeck | Coolify | Dokploy | Kamal | Piku |
|---|---|---|---|---|---|
| Platform footprint (idle) | **~15 MB RAM** | 500–700 MB RAM | ~350 MB RAM | none (CLI) | minimal |
| Requires Docker | **No** | Yes | Yes | Yes | No |
| Web dashboard | **Yes** | Yes | Yes | No | No |
| Install | 1 command + browser | 1 command | 1 command | Ruby gem + config | git hooks setup |
| Zero-downtime deploys | **Yes** (health-checked) | Partial | Yes | Yes | No |
| Push-to-deploy webhooks | **Yes** | Yes | Yes | No (CLI push) | git push |
| Apps survive platform restart | **Yes** (process adoption) | Containers keep running | Containers keep running | Yes | Yes |
| Automatic SSL | **Yes** (Caddy) | Yes (Traefik/Caddy) | Yes (Traefik) | Yes (kamal-proxy) | Yes (acme.sh) |
| Minimum server | **512 MB** | 2 GB | 1 GB | any | 256 MB |

If you want 280+ one-click services, teams, and multi-server orchestration, use Coolify — it's excellent at that scale. Flightdeck is for the other case: a couple of apps, one cheap VPS, and no patience for platform overhead.

## What it does

- **Deploy apps** from GitHub, a zip upload, or a directory already on the server
- **Push-to-deploy** — every app gets an HMAC-signed webhook URL; push to GitHub and it pulls, rebuilds, restarts
- **Zero-downtime deploys** — set a health check path and deploys start the new process, wait for it to pass, switch traffic, then stop the old one
- **Crash recovery** — processes that die are restarted with exponential backoff; persistent failures are marked crashed with the reason in the logs
- **Automatic SSL** via Caddy reverse proxy — add a domain and certificates just work
- **Live logs** — streamed to the dashboard over SSE, not polled snapshots
- **Deployment history** — who/what triggered each deploy and whether it succeeded
- **Environment variables** per app, managed from the UI
- **Build commands** — run `npm install && npm run build` before start
- **Process control** — start, stop, restart from the dashboard; stop reaches the whole process group
- **Server monitoring** — CPU, memory, disk with sparkline trends; per-app CPU/memory
- **Runtime management** — detect and install Node, Python, Go, Bun, Deno from Settings
- **Survives itself** — flightdeck restarts and upgrades never touch your running apps

![app detail with push-to-deploy and live logs](docs/screenshots/app-detail-dark.png)

## Install

```bash
curl -sSL https://raw.githubusercontent.com/sahithvibudhi/flightdeck/main/scripts/install.sh | sudo bash
```

The installer downloads the binary for your architecture (amd64/arm64), verifies its checksum, sets up a systemd service, and starts it. Finish setup in the browser at `http://your-server-ip:3000`.

Prefer the terminal? Run `sudo flightdeck` interactively and the setup wizard runs there instead. Automating with cloud-init or Ansible? Set `FLIGHTDECK_ADMIN_USER` and `FLIGHTDECK_ADMIN_PASSWORD` in the service environment and setup is fully headless.

**Requirements:** a Linux VPS (amd64 or arm64), ports 80/443 open for SSL, port 3000 for the dashboard (or put it behind a domain in Settings).

## Deploying an app

Navigate to **Deploy**, pick a source tab:

- **GitHub** — clone a repository (private repos work with a token from Settings)
- **Upload Zip** — drag and drop a `.zip`
- **Server Path** — run an app from a directory already on the server

Fill in the app name and start command, optionally a build command, port, health check path, and environment variables. One click and you're watching the live deploy log.

To enable push-to-deploy, copy the webhook URL from the app page into your GitHub repo's webhook settings. Every push pulls, rebuilds, and restarts — zero-downtime if you set a health check.

## Architecture

```
flightdeck binary (~17 MB, idles at ~15 MB RAM)
├── Go API server (:3000)
│   ├── REST API (/api/*)
│   ├── Webhooks (/hooks/*)
│   └── Embedded React UI (/)
├── SQLite database (flightdeck.db)
├── Caddy subprocess (:80/:443)
│   └── Admin API (:2019)
└── App processes (ports 4000+)
    └── survive flightdeck restarts; re-adopted on boot
```

Everything runs as a single binary. The React UI is compiled and embedded at build time. SQLite is the only database. Caddy handles SSL certificates automatically. Zero CGO, no external runtime dependencies.

## API

All routes under `/api`. All except login and setup require `Authorization: Bearer <jwt>`.

### Setup & auth

```
GET    /api/setup/status            {needs_setup}
POST   /api/setup                   {username, password, domain?} → {token}   (first run only)
POST   /api/auth/login              {username, password} → {token}            (rate limited)
POST   /api/auth/password           {current, new}
```

### Apps

```
GET    /api/apps                    List all apps
POST   /api/apps                    Create {name, start_command, build_command?, port?, repo_url?, branch?, work_dir?, health_path?}
GET    /api/apps/:id                Get app detail
PUT    /api/apps/:id                Update config (port changes re-route domains and restart)
DELETE /api/apps/:id                Stop and remove app
POST   /api/apps/:id/upload         Upload zip to app directory
POST   /api/apps/:id/start          Start app
POST   /api/apps/:id/stop           Stop app
POST   /api/apps/:id/restart        Restart app
POST   /api/apps/:id/pull           Git pull latest changes
POST   /api/apps/:id/deploy         Pull + build + restart, recorded in history
GET    /api/apps/:id/deployments    Deployment history
GET    /api/apps/:id/logs?lines=100 Log snapshot
GET    /api/apps/:id/logs/stream    Live log tail (SSE; pass ?token=)
```

### Push-to-deploy

```
POST   /hooks/:id                   GitHub-compatible webhook (X-Hub-Signature-256
                                    HMAC, or ?secret= for curl/CI)
```

### Environment variables, domains, settings

```
GET/PUT /api/apps/:id/envs          List / replace env vars [{key, value}]
GET/POST/DELETE /api/apps/:id/domains[/:name]
GET    /api/settings                Settings
PUT    /api/settings/domain         Panel domain
PUT    /api/settings/git-token      GitHub token
GET    /api/system                  Runtime detection + Caddy status
GET    /api/system/metrics          Server metrics history
POST   /api/system/install          Install a runtime
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
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for conventions, project structure, and the OrbStack workflow for testing on macOS.

## Project structure

```
cmd/flightdeck/main.go       Entry point, startup sequence
internal/
  api/                       HTTP handlers (chi router), SSE, webhooks
  auth/                      bcrypt + JWT
  db/                        SQLite schema + queries
  git/                       Clone and pull (tokens via env, never in URLs)
  process/                   App lifecycle, crash recovery, zero-downtime, adoption
  proxy/                     Caddy subprocess + route management
  setup/                     First-boot setup (web, env, or terminal wizard)
  system/                    Runtime detection, server metrics
ui/
  src/pages/                 React pages
  src/components/            Shared components (toasts, dialogs, icons)
  src/api.ts                 Typed fetch client
  src/style.css              Design system (dark + light)
scripts/install.sh           Production install script
.github/workflows/           CI + release automation
```

## Tech stack

| Component | Choice |
|---|---|
| Language | Go |
| UI | React + TypeScript (Vite), embedded via go:embed |
| Database | SQLite (modernc.org/sqlite, pure Go) |
| Reverse proxy | Caddy (subprocess, auto SSL) |
| Router | chi |
| Auth | bcrypt + JWT, per-IP login rate limiting |
| Process management | os/exec, process groups, PID adoption |

## Roadmap

Postgres/Redis provisioning with scheduled backups, cron jobs, deploy notifications (Discord/Telegram), and an MCP server for agent-driven deploys are next. See [docs/ROADMAP.md](docs/ROADMAP.md) for the full plan and reasoning, and [docs/RESEARCH.md](docs/RESEARCH.md) for the competitive research behind it.

## License

[MIT](LICENSE)
