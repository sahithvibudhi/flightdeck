# Competitive research: where flightdeck fits (July 2026)

This document summarizes the research that shaped flightdeck's positioning and roadmap. Numbers and citations are as of July 2026.

## The landscape

| Tool | Stars | Stack | Docker required | Web UI | Positioning |
|---|---|---|---|---|---|
| [Coolify](https://github.com/coollabsio/coolify) | ~58k | PHP/Laravel + Postgres + Redis | Yes | Yes | Full PaaS: 280+ one-click services, teams, multi-server |
| [Dokploy](https://github.com/dokploy/dokploy) | ~36k | TypeScript | Yes (Swarm) | Yes | "Lighter Coolify", native Compose |
| [Dokku](https://github.com/dokku/dokku) | ~32k | Shell/Go | Yes (buildpacks) | No | CLI Heroku clone, the original (2013) |
| [CapRover](https://github.com/caprover/caprover) | ~15k | TypeScript | Yes (Swarm) | Yes | Mature but slow-moving |
| [Kamal](https://github.com/basecamp/kamal) | ~14k | Ruby (37signals) | Yes | No | CLI-over-SSH deploys, zero-downtime |
| [Sidekick](https://github.com/MightyMoud/sidekick) | ~7.5k | Go single binary | Yes | No | "Own Fly.io", stalled since 2024 |
| [Piku](https://github.com/piku/piku) | ~6.6k | Python (~1.7k SLOC) | **No** | No | Tiniest PaaS, git-push deploys, dormant |

**The gap:** every tool with a web dashboard requires Docker; every no-Docker tool has no dashboard (and Piku, the closest philosophical kin, is dormant). The "no-Docker + single-binary + web UI" quadrant is empty. That's flightdeck's lane.

## Coolify's pain points (validated opportunities)

- **Resource overhead**: the platform is a Laravel app + PostgreSQL + Redis + workers + proxy. Community comparisons measure ~5–6% idle CPU and 500–700 MB idle RAM; official minimum is 2 CPU / 2 GB / 30 GB *before your apps*. On a $5 VPS the platform is the biggest tenant.
- **Upgrade pain**: in-place self-updates have bricked instances (double-clicked update button corrupting state, lost env vars/keys — github.com/coollabsio/coolify/discussions/3687).
- **Security**: January 2026 disclosure of 11 critical CVEs, four rated CVSS 10.0 (command injection → root RCE), with ~52k instances exposed to the internet.
- **UX**: recurring HN feedback that the dashboard is "clunky"; config leaks Traefik internals; changing an env var forces a full rebuild (issue #2854).
- **Zero-downtime**: historically missing/partial — the most-cited dealbreaker in HN threads; Kamal and Haloy market zero-downtime as their headline feature against it.

## What indie hackers actually ask for (ranked)

1. One-command install, minutes to first deploy
2. Push-to-deploy (webhook or git push) — table stakes
3. Automatic HTTPS — table stakes
4. **Zero-downtime deploys with health checks and rollback** — the most-cited gap
5. Postgres/Redis provisioning with scheduled backups to S3
6. Live log streaming
7. Cron jobs with failure notifications
8. Env changes that don't trigger full rebuilds
9. Preview deployments per PR
10. Low idle footprint — the recurring reason people leave Coolify on small VPSes
11. (2026 trend) MCP/agent integration — deploy from Claude Code/Cursor with scoped tokens
12. "If the platform dies, my apps keep running"

Flightdeck ships 1–4, 6, 8, 10, and 12 today. 5, 7, 11 are on the [roadmap](ROADMAP.md).

## Marketing lessons from the winners

- **Concrete numbers beat adjectives.** "0.8% CPU / 350 MB" (Dokploy comparisons), "20-second deploys" (Kamal), "1.7k SLOC" (Piku). Flightdeck's: *one 17 MB binary, ~15 MB RAM idle, no Docker*.
- **The curl one-liner above the fold** — Coolify, Dokploy, and CapRover all do it.
- **A dashboard screenshot/GIF** is the single highest-value README asset for a UI product.
- **"Alternative to X" framing** captures search traffic (Coolify: "alternative to Heroku/Netlify/Vercel").
- **Comparison pages are the SEO battlefield** — hosting vendors publish dozens of "X vs Y" posts; an honest self-published comparison table captures that traffic.
- **Build-in-public works**: Coolify's growth was driven by its maintainer's relentless public dev-logging, revenue transparency (~$15.7k/mo), sponsor wall, and a Hetzner one-click partnership.
- **Credibility via specifics**: Kamal's minimal README works because "37signals runs HEY on this" — for flightdeck, the equivalent is publishing real measured numbers and honest limitations (see SECURITY.md's threat model).

## Positioning statement

> Everything an indie dev needs from Coolify on one cheap VPS — deploy from GitHub, automatic SSL, push-to-deploy, zero-downtime restarts — in one 17 MB Go binary with no Docker, no platform database, and ~15 MB of overhead. If flightdeck dies, your apps keep serving.
