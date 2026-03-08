# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in ts-bridge, please report it responsibly.

**Do not open a public issue.** Instead, email the maintainer directly or use [GitHub's private vulnerability reporting](https://github.com/mlorentedev/ts-bridge/security/advisories/new).

## Scope

ts-bridge handles sensitive data (Tailscale auth keys, network tunnels). The following are in scope:

- Auth key leakage (logs, error messages, process environment)
- Unauthorized tunnel access or connection hijacking
- Denial of service via resource exhaustion
- State directory permission issues

## Response Timeline

- **Acknowledgment:** within 48 hours
- **Assessment:** within 7 days
- **Fix release:** as soon as practical, coordinated with reporter
