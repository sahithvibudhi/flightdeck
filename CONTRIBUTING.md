# Contributing to flightdeck

Thanks for helping! Flightdeck aims to stay small, fast, and dependency-free — most contributions should keep it feeling like one coherent tool, not a platform.

## Getting started

1. Fork the repo and create a branch off `main`
2. Make your changes — keep commits focused
3. Run `make test` and make sure everything passes
4. If you changed the UI, run `cd ui && npm run lint && npm run build`
5. Open a PR with a short description of what and why

CI runs lint, tests, and a full build on every PR.

## Development

```bash
# API server with a local data dir
FLIGHTDECK_DATA_DIR=./data go run ./cmd/flightdeck

# UI dev server (proxies /api to :3000)
cd ui && npm run dev

# Full build (embeds the UI into the binary)
make build

# Tests
make test
```

## Conventions

- **The binary stays self-contained.** No external runtime dependencies at build time beyond Go and Node. No CGO.
- **SQLite schema changes** go in `internal/db/db.go` as new entries appended to the `migrations` slice. Use `ALTER TABLE` for additions — the migration runner handles "duplicate column" errors gracefully. Never edit an existing migration.
- **API handlers** live in `internal/api/`. Follow the existing pattern: request struct, handler method on the relevant handler type, route in `router.go`. Handlers that need the proxy go through the injected route functions (`SetRouteAdder`/`SetRouteRemover`), not `internal/proxy` directly, so they stay testable.
- **Process lifecycle** logic belongs in `internal/process`. The manager must never assume it spawned a process — flightdeck adopts app processes that survived a restart.
- **Frontend** is plain React with no state library. `useState` and `fetch` calls through `ui/src/api.ts`. Shared UI pieces live in `ui/src/components/`. Use `errMsg()` in catch blocks and `toast()` for success feedback.
- **CSS** is a single `ui/src/style.css` with CSS variables. Both dark and light themes come from the variables — don't hardcode colors in components.
- Write tests next to the code (`*_test.go`); `internal/testutil` has helpers for a seeded in-memory database.

## Testing on macOS with OrbStack

Flightdeck targets Linux, but you can develop on macOS and test in an OrbStack VM.

```bash
# One-time: create an Ubuntu VM and map a hostname
orb create ubuntu test-vm
sudo sh -c 'echo "192.168.139.25 flightdeck.local" >> /etc/hosts'   # IP from `orb list`

# Build and deploy
make build-linux-arm64            # or build-linux-amd64
scp dist/flightdeck-linux-arm64 test-vm@orb:/tmp/flightdeck
ssh test-vm@orb
chmod +x /tmp/flightdeck
sudo FLIGHTDECK_DATA_DIR=/tmp/flightdeck-data /tmp/flightdeck
```

Open `http://flightdeck.local:3000`. The data directory persists between runs so you don't need to redo setup.

## Releases

Tagging `v*` triggers the release workflow: it builds `flightdeck-linux-{amd64,arm64}`, generates checksums, and publishes a GitHub Release that `scripts/install.sh` consumes.
