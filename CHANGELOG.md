# Changelog

All notable changes to flightdeck are documented here.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added
- Browser-based first-run setup at `/setup` — the systemd service now works out of the box with no terminal wizard required. Headless provisioning is supported via `FLIGHTDECK_ADMIN_USER` / `FLIGHTDECK_ADMIN_PASSWORD`.
- Push-to-deploy webhooks: per-app webhook URL (`POST /hooks/{id}`) with HMAC signature verification, GitHub-compatible. Pushing to your repo pulls, rebuilds, and restarts the app.
- Deployment history per app with trigger, status, and duration.
- Live log streaming over SSE (`GET /api/apps/{id}/logs/stream`) — the dashboard now tails logs in real time instead of polling.
- Crash auto-restart with exponential backoff; apps that keep failing are marked `crashed`.
- Optional health checks with zero-downtime restarts: new process starts on a standby port, traffic switches only after the health check passes.
- "Server Path" deploy source now actually works: apps can run from an existing directory on the server (`work_dir`).
- Login rate limiting.
- CI and release automation; installable binaries published to GitHub Releases with checksums.
- MIT LICENSE file, CONTRIBUTING, SECURITY policy, issue/PR templates.

### Fixed
- `install.sh` pointed at a nonexistent release URL; it now uses the correct `releases/latest/download` form, verifies checksums, and starts the service automatically.
- Editing an app's port now updates Caddy routes and restarts the running process; previously domains silently kept pointing at the old port.
- Start commands run through a shell, so quoting, pipes, and `&&` work (matching build commands).
- Rebooting the server no longer re-runs every app's build command.
- The "Replace" button for the GitHub token in Settings now works.
- Git tokens are no longer embedded in clone URLs (they leaked into `ps` output and error messages); credentials are passed via environment instead.
- Deleting an app that was just stopped no longer fails with a database-busy error (SQLite busy_timeout).
- Restarting or upgrading flightdeck no longer duplicates app processes: running apps are re-adopted by PID on boot and keep serving without interruption.
- Deleting an app now removes its managed directory and logs (a user-provided `work_dir` is never touched).
