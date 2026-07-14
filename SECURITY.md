# Security Policy

## Reporting a vulnerability

Please report security issues privately via GitHub's **Report a vulnerability** (Security → Advisories) on this repository, or by emailing the maintainer. Do not open public issues for security problems.

You can expect an acknowledgement within a few days. Please include reproduction steps and the version/commit you tested.

## Threat model — read before deploying

Flightdeck is a **single-admin control plane with intentional remote-command capability**: anyone who can log into the dashboard can run arbitrary commands on your server (that's what start/build commands are). Treat panel credentials like SSH keys.

What flightdeck does to protect you:

- Setup endpoint permanently closes after the first admin is created
- Per-IP rate limiting on login; bcrypt (cost 12) password hashes
- JWT auth with HMAC algorithm pinning
- Webhooks require an HMAC signature (GitHub `X-Hub-Signature-256`) or the per-app secret
- Git tokens are passed to git via environment, never embedded in URLs, and redacted from error output
- Zip uploads are protected against path traversal (zip-slip)

What you should do:

- Put the dashboard behind a domain (Settings → panel domain) so it's served over HTTPS by Caddy, or firewall port 3000
- Use a strong admin password; rotate your GitHub token if it may have leaked
- Keep flightdeck updated

Known limitations (roadmap items): app processes run as the same user as flightdeck (typically root via systemd) without inter-app isolation, and secrets are stored unencrypted in the SQLite database file. If your threat model includes hostile co-tenants or database-file exfiltration, flightdeck is not yet the right tool.
