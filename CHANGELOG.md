# Changelog

## [0.1.0] - 2026-07-14

First versioned release.

### Added
- Browser-based first-run setup at `/setup`. Works under systemd with no terminal. Headless setup via `FLIGHTDECK_ADMIN_USER` and `FLIGHTDECK_ADMIN_PASSWORD`.
- Push-to-deploy webhooks per app (`POST /hooks/{id}`), GitHub signature compatible.
- Deployment history with trigger, status, and commit info.
- Live log streaming over SSE. Log files capped at 5MB.
- Crash auto-restart with backoff. Repeated failures mark the app crashed.
- Zero-downtime deploys for apps with a health check path.
- Process adoption: flightdeck restarts and upgrades do not touch running apps.
- Deploys from a server path (`work_dir`).
- Caddy and git installable from Settings. Installing Caddy starts the proxy and registers routes without a restart.
- Caddy download falls back to GitHub releases.
- One-click sample app on the empty dashboard.
- App URLs on cards, detail page, and the deploy success screen. Server IP in `/api/system`.
- Warning banner when Caddy is not running.
- Login rate limiting.
- Env var name validation. Domain validation and normalization.
- Toasts, confirm dialogs, light theme, status colors, a11y fixes.
- CI and release automation. Install script with checksum verification.
- LICENSE, CONTRIBUTING, SECURITY, issue and PR templates.

### Fixed
- Install script pointed at a broken release URL.
- Port edits now update Caddy routes and restart the process.
- Start commands run through a shell in their own process group.
- Reboots no longer re-run build commands.
- Settings token replace button.
- Git tokens no longer leak into process lists or error output.
- Deploys for the same app no longer run concurrently.
- Delete while running no longer fails on a busy database.
