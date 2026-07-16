# Production deployment inputs

This directory is a **reference boundary**, not a claim that a high-availability platform is provisioned by this repository. The root Compose file is for local/single-host validation. Production should deploy immutable backend/frontend images on the chosen orchestrator, run one migration task, and connect them to TLS PostgreSQL/RustFS and an OTLP collector using injected secrets.

Files:

- `env.example` lists production configuration classes without usable secrets;
- `Caddyfile` is an optional TLS edge example for a small single-host deployment. It proxies to the frontend, whose nginx config proxies `/api/` to the API. A managed load balancer/ingress can replace it.

Required gates are documented in [`../../docs/deployment.md`](../../docs/deployment.md) and [`../../docs/security.md`](../../docs/security.md): database PITR/restore, redundant RustFS/backup, secret manager, image digests/SBOM/scanning, exact CORS/cookies/trusted proxies, resource/network policy, migrations, alerts and an acceptance record.

Do not copy this example into production unchanged. In particular, replace all `__INJECT_*__` values at runtime; never make them Docker build args or commit the rendered file.
