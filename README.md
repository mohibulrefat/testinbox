# TestInbox

A local-first developer tool for email, simulated SMS, and OTP testing.

M1 provides the Go HTTP lifecycle, configuration, health endpoints, and a React application shell. Storage and message capture are not implemented. No SMTP listener starts and no real messages are sent.

## Prerequisites

- Go 1.26.x (verified with 1.26.8).
- Node.js 24.x and npm 11.x (verified with Node 24.19.0/npm 11.17.0). `web/.nvmrc` pins the verified Node version for nvm users.
- No database, Docker, or CGO setup is needed for M1. See [dependency decisions](docs/DEVELOPMENT.md).

## Run locally

From the repository root:

```sh
go run ./cmd/testinbox
```

In another terminal:

```sh
cd web
npm ci
npm run dev
```

Open http://127.0.0.1:5173. Vite proxies `/api` and `/health` to http://127.0.0.1:8025. If the backend address changes, update the development proxy in `web/vite.config.ts`. API/health access works without the frontend. Port conflicts fail instead of silently selecting a different port.

```sh
curl -i http://127.0.0.1:8025/health/live
curl -i http://127.0.0.1:8025/health/ready
```

Liveness returns 200 JSON. Readiness intentionally returns 503 with `error.code=storage_not_initialized` until M2 adds SQLite initialization. Ctrl+C or SIGTERM stops acceptance and drains HTTP requests for up to 10 seconds, then forcibly closes remaining connections. Run a built binary for direct process/signal management:

```sh
go build -o bin/testinbox ./cmd/testinbox
./bin/testinbox
```

## Configuration

Environment variables only; unset values use defaults. Explicit empty values fail startup.

| Variable | Default | Validation / M1 behavior |
| --- | --- | --- |
| `TESTINBOX_HTTP_ADDR` | `127.0.0.1:8025` | Literal IP and numeric port 1–65535; bracket IPv6, e.g. `[::1]:8025`. Non-loopback binds log a warning. |
| `TESTINBOX_SMTP_ADDR` | `127.0.0.1:1025` | Same address validation; reserved configuration, no listener starts. |
| `TESTINBOX_DB_PATH` | `./data/testinbox.db` | Non-empty path without surrounding whitespace, NUL, CR, or LF; reserved configuration, no file is opened/created. Relative paths use the process working directory. |

Hostnames and implicit wildcard addresses such as `:8025` are rejected; use an explicit IP (`0.0.0.0` is an explicit wildcard override). Do not expose this unauthenticated development application to untrusted networks. Only the three documented settings are implemented; resource/retention configuration arrives with its owning milestone.

HTTP defaults: header read 5 seconds, whole request read 15 seconds, response write 40 seconds, idle connection 60 seconds, header budget 16 KiB (Go may allow parser overhead). These fixed M1 bounds avoid additional tuning options; the write timeout leaves room for M5's 30-second OTP waits. Revisit capture-specific deadlines in M3.

## Checks and production build

From the root:

```sh
gofmt -w cmd internal
go test ./...
go vet ./...
go test -race ./...
```

From `web/`:

```sh
npm run lint
npm run typecheck
npm test
npm run build
npm run preview
```

`npm run build` emits `web/dist`; preview serves it on http://127.0.0.1:4173 for inspection only, without the development API proxy. Embedding production assets and Docker distribution remain M8. Go has no third-party dependencies; `npm ci` reproduces the locked frontend tree. Tests need permission to bind ephemeral loopback sockets; race tests need a supported platform/C compiler.

## Planning

- [Requirements](docs/SRS.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Milestone plan](docs/PLAN.md)
- [Development and dependency decisions](docs/DEVELOPMENT.md)
