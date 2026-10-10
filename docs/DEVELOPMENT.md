# M1 development decisions

M1 implements only the application foundation. The M0 requirements, architecture, and milestone order remain the product source of truth; the user's M1 implementation request authorizes this milestone despite historical “M0 review draft” status labels.

## Toolchains and dependencies

Use Go 1.26.x, Node.js 24.x, and npm 11.x. Tested patch versions are in README; `go.mod`, `web/.nvmrc`, package engines, and the npm lockfile record the supported build contract. Go 1.26 is a released toolchain ([release notes](https://go.dev/doc/go1.26)); Node 24 satisfies [Vite's documented Node requirements](https://vite.dev/guide/). npm dependency versions are saved exactly with a lockfile, rather than relying on moving latest tags at installation time. Builds require initial registry access; runtime health endpoints do not.

Backend: Go standard library only (BSD-3-Clause Go distribution); no third-party backend packages or go.sum are needed. net/http handles serving and graceful shutdown, slog handles diagnostics, and os/signal handles SIGINT/SIGTERM. Configuration validates explicit IP addresses rather than resolving names, keeping defaults and wildcard overrides predictable.

Frontend runtime: React and React DOM (MIT). Development: Vite (MIT) for TSX transformation and build, TypeScript (Apache-2.0) for type checks, React type definitions (MIT), ESLint/@eslint/js (MIT) with typescript-eslint (MIT) for linting, and Vitest (MIT) for a basic server-rendered shell regression test. No router, component library, React Vite plugin, simulated DOM, or application state package is needed for this shell. Vite's built-in TSX transform is sufficient; React Fast Refresh is not enabled. Transitive licenses are recorded in the lockfile where provided; review them again before distribution in M9. Dependencies should be deliberately updated and checks rerun, not floated on clean installs.

SQLite driver selection is deferred to M2. Compare maintained driver licenses, supported platforms, and CGO versus pure-Go build costs before choosing. M1 does not open a database or claim storage readiness. A CGO-enabled toolchain may be needed for race verification, independently of the later SQLite choice. Embedding frontend assets and choosing a minimal runtime container/CGO packaging are M8 work.

## Boundaries and limitations

- Health endpoints are the only backend routes. Readiness always returns 503 `storage_not_initialized`, including when a configured database file happens to exist. Unknown product routes return 404.
- HTTP lifecycle tests exercise port conflicts, draining requests, deadline-triggered forced close, and listener closure. Configuration tests exercise defaults, valid overrides, and invalid values. Health tests cover method routing and the readiness error contract.
- The frontend test renders the shell without a browser; it does not establish browser end-to-end or accessibility coverage for future product features. Those checks belong to M6.
- No message storage, SMTP listener, SMS endpoint, OTP logic, quotas, retention worker, Docker artifacts, or production frontend serving exists. Resource configuration should be added when behavior can be enforced, rather than accepted silently in M1.
- No scalability target is claimed for the shell. net/http handles concurrent requests with bounded I/O deadlines; message processing, persistence quotas, and load verification arrive in their planned milestones. No service abstraction or extra infrastructure is needed now.
