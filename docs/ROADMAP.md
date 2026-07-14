# Roadmap

Flightdeck's north star: **everything an indie dev needs to run a few apps on one cheap VPS, in one small binary**. Features that require a heavier platform (multi-server orchestration, teams at scale, 280 one-click services) are explicitly out of scope — use Coolify for that.

## Shipped (this cycle)

- CI + tag-triggered release automation; working one-command installer with checksum verification
- Browser-based first-run setup (systemd-friendly), headless setup via env vars
- Push-to-deploy webhooks (GitHub HMAC-compatible) + deployment history
- Live log streaming over SSE, log size caps
- Crash auto-restart with exponential backoff and a terminal `crashed` state
- Zero-downtime deploys via health checks and standby-port traffic switching
- Process adoption: flightdeck restarts/upgrades never touch running apps
- Working "Server Path" deploys (`work_dir`), config-edit reconciliation, shell-parsed start commands
- Git tokens out of URLs/process lists; login rate limiting
- Toasts, confirm dialogs, light theme, a11y pass, empty states
- LICENSE, CONTRIBUTING, SECURITY, issue/PR templates, README with real measured numbers

## Next (rough order)

1. **Postgres provisioning + backups** — the top requested capability class. One-click install of Postgres (native package, not Docker), per-app database/user creation, `DATABASE_URL` injection, and `pg_dump` on a schedule with optional S3-compatible upload. Redis after.
2. **Cron jobs** — per-app scheduled commands with output capture and failure status; reuses the deployments-history UI pattern.
3. **Notifications** — Discord/Telegram/generic webhook on deploy success/failure and crash events. Small table + one goroutine.
4. **Instant rollback** — keep the previous git SHA/build per app; failed deploys (or one click) restore it. Pairs with zero-downtime.
5. **Preview environments** — deploy a branch as `<app>-preview` on an auto-assigned port + subdomain from a PR webhook; delete on merge/close.
6. **MCP server** — scoped agent tokens so Claude Code/Cursor can deploy and read logs; the 2026 differentiator with tiny surface area (the REST API already exists).
7. **Run as non-root** — dedicated `flightdeck` user with `CAP_NET_BIND_SERVICE` for Caddy, opt-in per-app users for isolation. Needs design for runtime installs.
8. **Upgrade command** — `flightdeck upgrade` (download, checksum, swap binary, restart service); safe because apps survive restarts by design.

## Marketing next steps

- Tag `v0.1.0` to publish the first real release (the installer depends on it)
- A 30–60s GIF of install → deploy → push-to-deploy for the README hero
- "flightdeck vs Coolify vs Dokploy" honest comparison post (SEO: this is the highest-traffic query family in the niche)
- Show HN / r/selfhosted launch once Postgres provisioning ships (the most-asked question will be "does it do databases?")
- Measure and publish idle RAM/CPU on a real $5 VPS alongside Coolify/Dokploy on the same box

## Explicitly not planned

- Docker as a deployment target (use Dokploy/Coolify)
- Multi-server orchestration
- Teams/RBAC beyond a single admin (until there's real demand)
- A separate cloud/SaaS offering
