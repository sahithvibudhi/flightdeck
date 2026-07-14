# Architecture

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

## How the pieces fit

- **Apps are plain OS processes.** The process manager (`internal/process`) starts each app through `sh -c` in its own process group, injects `PORT` and the app's env vars, and streams output to a capped log file. Crashes restart with exponential backoff; repeated failures mark the app crashed.
- **Flightdeck restarts never touch your apps.** On boot, the manager checks each recorded PID and adopts processes that are still alive instead of spawning duplicates. Stop, restart, and metrics keep working on adopted processes.
- **Zero-downtime deploys.** When an app has a health check path, a deploy starts the new process on a standby port, waits for the health check to pass, switches the Caddy route, and only then stops the old process. If the new process never becomes healthy, the old one keeps serving.
- **Caddy owns 80/443.** Domain routes are managed live through Caddy's admin API. The panel can install and start Caddy at any time; routes re-register without a restart.
- **SQLite holds everything.** Config, apps, env vars, domains, and deployment history. Migrations are an append-only list in `internal/db/db.go`.
