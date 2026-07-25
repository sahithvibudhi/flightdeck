# API reference

All routes live under `/api`. Every route except login and first-run setup requires `Authorization: Bearer <jwt>`.

## Setup and auth

```
GET    /api/setup/status            {needs_setup}
POST   /api/setup                   {username, password, domain?} -> {token}   (first run only)
POST   /api/auth/login              {username, password} -> {token}            (rate limited)
POST   /api/auth/password           {current, new}
```

## Apps

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
POST   /api/apps/:id/deployments/:depID/rollback     Roll back to that deployment's commit
GET    /api/apps/:id/deployments/:depID/logs         Captured deploy log {lines, running}
GET    /api/apps/:id/deployments/:depID/logs/stream  Live deploy log (SSE; ends with "done" event)
GET    /api/apps/:id/logs?lines=100 Log snapshot
GET    /api/apps/:id/logs/stream    Live log tail (SSE; pass ?token=)
POST   /api/apps/:id/webhook-secret Rotate the push-to-deploy secret
```

Running apps include `port_check` ("ok" or "mismatch") and `listening_ports`
in their responses: flightdeck reads /proc to verify the process group
actually listens on its assigned port, and the UI warns when it doesn't.

## Push-to-deploy

```
POST   /hooks/:id                   GitHub-compatible webhook (X-Hub-Signature-256
                                    HMAC, or ?secret= for curl/CI)
```

## Environment variables, domains, settings

```
GET/PUT /api/apps/:id/envs          List / replace env vars [{key, value}]
GET/POST/DELETE /api/apps/:id/domains[/:name]
GET    /api/settings                Settings
PUT    /api/settings/domain         Panel domain
PUT    /api/settings/git-token      GitHub token
GET    /api/system                  Runtime detection, Caddy status, server IP
GET    /api/system/metrics          Server metrics history
POST   /api/system/install          Install a runtime (node, python, go, bun, deno, docker, caddy, git)
```
