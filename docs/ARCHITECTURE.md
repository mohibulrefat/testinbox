# TestInbox Architecture

Status: M0 review draft. Approved product constraints are preserved. Implementation choices marked as proposals need review before dependent work.

## Context and responsibilities

```text
Development application --SMTP :1025--> Go application
Development app / tests --HTTP :8025--> Go application
Browser dashboard      --HTTP :8025--> Go application
                                          |
                                        SQLite
```

One Go process handles SMTP, HTTP, persistence, and retention. SMTP cannot relay. SMS capture cannot send. No runtime cloud service or message broker is involved.

Proposed repository layout:

```text
cmd/testinbox/          startup and graceful shutdown
internal/config/       validated configuration
internal/smtp/         SMTP transport and framing
internal/httpapi/      API handlers, validation, static asset routing
internal/message/      capture, query, deletion, retention orchestration
internal/storage/      SQLite queries and embedded SQL migrations
internal/mime/         bounded MIME parsing
internal/otp/          extraction and retrieval (M5)
web/                   React, TypeScript, Vite
docs/                  requirements, architecture, milestones
```

Tests live beside the code; integration fixtures may use `testdata/`. Use concrete components and small interfaces at real test boundaries, without a generic repository framework. Go/Node versions, SQLite driver, SMTP and MIME libraries are unresolved: select toolchains in M1, SQLite driver in M2, and SMTP/MIME libraries in M3 as maintained, appropriately licensed packages and record their build, CGO, and security implications before adoption. No dependencies are installed in M0.

## Go process and capture lifecycle

M1 establishes configuration, HTTP lifecycle, shutdown, and health routes without SQLite initialization or capture listeners. Liveness returns 200 while the process is running; readiness returns 503 with `storage_not_initialized` throughout M1, never a fabricated storage success. M2 owns opening SQLite, migrations, storage checks, quotas, and retention. From M2 onward, readiness returns 200 only after migrations/reconciliation and a successful bounded storage probe; storage unavailability returns 503. Startup storage/migration failure fails startup. HTTP may start after initialization; the startup probe sees no successful readiness before initialization. SMTP starts in M3. Required listeners and (from M5) completed backfill are readiness prerequisites. Shutdown stops acceptance, cancels waits/cleanup, drains in-flight work within a proposed 10-second grace period, and closes storage. A failed required listener fails the application.

Email: enforce SMTP session/DATA limits; remove SMTP framing; retain raw bytes; parse within MIME limits; build a message record with warnings; safely reclaim expired records, then atomically check quotas and commit message, recipients, headers, attachments, and (from M5) OTP candidates; only then return SMTP 250. If storage fails, return 451; oversize DATA returns 552. Unsupported relay/auth/TLS commands return appropriate protocol errors. Approved release-one SMTP is plaintext local capture without AUTH or STARTTLS. It provides no transport confidentiality and must remain on trusted local interfaces/networks.

Malformed MIME is a successful raw capture with parse warnings, not a reason to discard mail. Attachment bytes already decoded before a parsing limit may be stored; partial/truncated parts are flagged and not offered as complete downloads. A raw download is the fallback for unsupported encodings.

SMS: require non-blank string `to` and non-empty string `body`; optional string `from` may be omitted or empty (unknown sender). Reject null/non-string fields and preserve body whitespace. Validate JSON and limits, construct a message, run the same persistence/quota path, commit, and return HTTP 201. Every submission is a distinct message; retry deduplication/idempotency keys are deferred, so an acknowledgment lost after commit can result in duplicates on retry. APIs and tests must account for this using receipt bounds and filters.

## SQLite storage and schema proposal

Approved: store all content in a single database initially, including raw MIME and decoded attachment BLOBs. This duplicates some attachment bytes but makes persistence, deletion, and backup simpler than coordinating filesystem blobs. Review if actual workloads require a separate blob store.

| Table | Proposed fields / purpose |
| --- | --- |
| messages | id (random UUID text primary key), channel, received_at_us (UTC integer), sender, subject, text_body, html_body, search_text, raw_mime (email only), payload_bytes, parse_warnings (JSON text), extraction_version (nullable until M5) |
| recipients | message_id, ordinal, address; supports multiple email recipients and one SMS recipient |
| headers | message_id, ordinal, name, value; preserves duplicate header order; raw bytes remain in raw_mime |
| attachments | id, message_id, ordinal, filename, media_type, content_id, disposition, decoded_size, content BLOB |
| otp_candidates (M5) | message_id, ordinal, value, source, score; unique message/value after deduplication |
| schema_migrations | version, checksum, applied_at |

Use foreign keys with ON DELETE CASCADE. SMS body occupies text_body; absent/empty SMS `from` is normalized to null sender and does not match a non-empty sender filter; raw_mime/html_body are null. Envelope sender/recipients are independent of parsed From/To headers. Payload accounting includes all stored raw/decoded content and metadata UTF-8 bytes, including candidates; transactionally maintained accounting must be reconciled on startup. SQLite page/index/WAL overhead is excluded from the 1 GiB quota.

Index messages by (received_at_us, id), (channel, received_at_us, id), and recipients by (address, message_id). Search uses parameterized substring queries over normalized search_text (subject plus decoded text and HTML visible text); no FTS service initially. Define case folding consistently in Go for indexed content and queries; address filters use trimmed exact matching to avoid silently rewriting local parts. Confirm address matching policy before M3.

Propose WAL, foreign_keys=ON, synchronous=FULL, a bounded busy timeout (5 seconds), and serialized writes. Reads use a small bounded pool; quota admission reclaims expired records and checks quotas/inserts using serialized transactions to prevent over-admission. Capture a consistent clock cutoff per transaction. If still over quota, expired-only reclamation may commit but the incoming message must not be partially stored. Reclamation is bounded in batches and repeated before final rejection; never delete unexpired messages. Disk-full or cleanup failures return storage failure instead of false quota success. Expensive parsing occurs before a short write transaction. Storage errors roll back all related rows.

Numbered embedded SQL migrations run once before traffic, in a transaction where supported; checksum mismatch/newer unsupported schema fails startup. Migrations are forward-only; require a backup before upgrades and restore the matching binary/database to roll back. M5 adds candidate storage and a bounded backfill of retained messages; readiness remains false until backfill completes, resumably if interrupted. Backfill must apply the same quota accounting; if candidate growth would exceed a quota, fail readiness with an actionable limit-adjustment error rather than discard messages or silently skip extraction. Schema/migration tests must cover upgrades and failure paths.

## Expiry visibility

Retention is 7 days by default; 0 disables expiry. A message expires at `received_at + retention <= query_time`. Every normal list/search/detail/raw/attachment and OTP storage query includes the unexpired predicate, independent of background cleanup. Use one clock value per query and recalculate on each wait poll; ID lookups for expired messages return 404. Background deletion cascades related records within two 60-second cleanup intervals while running. Explicit deletion can also remove expired records. Admission reclaims eligible expired records before quota rejection; WAL/free-page reclamation does not guarantee immediate filesystem shrinkage or secure erasure.

## HTTP API proposal

All endpoints below except health checks use `/api/v1`. JSON responses and errors are versioned; raw/attachment downloads are binary. Error shape: `error: {code, message, request_id, fields?}`. No sensitive input is echoed.

| Method / path | Behavior |
| --- | --- |
| POST /sms | JSON `{to: string, body: string, from?: string}`; `to` non-blank and `body` non-empty are required; 201 with message ID and received timestamp |
| GET /messages | Metadata summaries; filters `channel`, `sender`, `recipient`, `since`, `before`, `q`, `limit`, `cursor` |
| GET /messages/{id} | Parsed detail, recipients, ordered headers, warnings, attachment metadata, and candidates after M5; excludes BLOBs |
| GET /messages/{id}/raw | Original email MIME download; non-email yields 409 |
| GET /messages/{id}/attachments/{attachmentId} | Validates parent ownership; attachment download |
| DELETE /messages/{id} | 204; absent ID returns 404 |
| DELETE /messages | Filtered bulk deletion, or explicit `all=true`; reject an unfiltered request without `all=true`; 200 with deleted count |
| GET /otp | M5: filters plus `wait_seconds`; 200 selected code with message ID, receipt timestamp, source, and candidate list; 404 after no-match timeout |
| GET /health/live | Process liveness; no storage details |
| GET /health/ready | Storage/startup readiness; 503 if unavailable |

Filters combine with AND. `since` is inclusive and `before` exclusive, using RFC3339 timestamps converted to UTC microseconds. Queries order newest first by receipt time then ID descending. A cursor encodes the last sort tuple and filter fingerprint; mismatched/invalid cursors return 400. Concurrent deletion can shorten a page; pagination is not a snapshot. List summaries omit message bodies and raw data.

Status conventions: 400 invalid JSON/filter, 413 size limit, 415 unsupported media type, 404 missing/no OTP match, 409 unsupported channel operation, 429 waiter/concurrency limit, 507 logical storage quota, and 503 transient storage failure. SMTP quota failure is temporary 452. Document these distinctions and test that failures create no partial record.

OTP queries require both non-blank `recipient` and valid RFC3339 `since` to reduce stale cross-test matches. Reject missing required filters with 400. Optional OTP filters are `channel`, `sender`, `before`, and `q` (subject/text substring); combine with AND. No pagination parameters are accepted. A new request always selects from unexpired retained messages, never consumes candidates. Bounds and indexing apply equally to immediate and waiting queries.

### SMS and OTP examples

Minimal SMS: `POST /api/v1/sms` with `{"to":"+15550000001","body":"Your code is 123456"}`. Optional sender: `{"from":"test-app","to":"+15550000001","body":"Your code is 123456"}`. Both return 201 after commit. Missing `to`/`body`, blank `to`, empty `body`, or null/non-string fields return 400 without storing a message; whitespace-only body is preserved and accepted.

OTP: `GET /api/v1/otp?recipient=%2B15550000001&since=2026-10-09T00%3A00%3A00Z&channel=sms&wait_seconds=10`. Tests must supply their own start timestamp, not reuse this illustrative value. Missing either required filter returns 400; timeout returns 404 `otp_not_found`.

## OTP design (M5)

Approved baseline: extract standalone 4–8 ASCII digit sequences; digit boundaries prevent matching substrings of longer numbers. Prefer candidates within 32 characters of case-insensitive terms `otp`, `code`, or `verification`; then prefer subject, SMS body, plain text, and HTML visible text, in that order; finally prefer first occurrence. Deduplicate values while keeping their strongest source. Scores are ordinal heuristic ranks, not statistical probabilities. Keep all candidates for inspection.

Choose the newest unexpired matching message with at least one candidate by receipt timestamp descending then ID descending, then its highest-ranked candidate; an older message with a higher score does not supersede a newer message. Return ambiguity metadata/list so callers can assert expectations. Automatic extraction from attachments, arbitrary user regexes, alphanumeric codes, and language-specific rules are deferred pending a requirement.

For bounded waiting, query immediately; `wait_seconds` defaults to 0 and must be between 0 and 30. Zero wait returns 404 `otp_not_found` immediately if absent. Positive waits use cancellable database polling every 250 ms with a waiter semaphore of 32; saturation returns 429 and invalid bounds return 400. Immediate requests do not occupy waiter slots. A query at the deadline determines the final result; no match returns 404 with code `otp_not_found`. Polling is simple and handles retention/deletion; latency and query load are the trade-off. Deletion can occur after selection, so a response is an observation, not a lock on message lifetime. All limits remain configurable within validated safe ranges.

## Frontend and captured content

Vite builds the React/TypeScript dashboard. In development, Vite proxies API requests to Go so browser writes remain same-origin. For distribution, embed build assets via Go embed as approved; production static routes and SPA fallback must never swallow `/api` errors. A separate web server is unnecessary.

Use paginated summaries and fetch details on selection; manual refresh initially. Render text safely. HTML preview must use an isolated sandbox, restrictive CSP, and blocked remote resources including CSS URLs; use escaped source if safe isolation cannot be guaranteed. Resolve local CID content only through controlled local resources if implemented; otherwise display placeholders. Sanitize filenames and download with attachment disposition and nosniff. Never execute email scripts or inline attachment content.

## Configuration, networking, and packaging

Proposed environment names: `TESTINBOX_SMTP_ADDR=127.0.0.1:1025`, `TESTINBOX_HTTP_ADDR=127.0.0.1:8025`, `TESTINBOX_DB_PATH=./data/testinbox.db`, with prefixed settings for SRS limits and retention. Document type, unit, range, default, and startup validation for every setting. Do not invent CLI/config-file precedence until needed.

Native defaults are loopback. Docker must explicitly bind application listeners to `0.0.0.0` inside the container for port publishing; supplied Compose maps `127.0.0.1:1025:1025` and `127.0.0.1:8025:8025`. A sibling container connects using the Compose service name, not its own localhost. Container interface binding is an explicit packaging override, not a change to native defaults. Warn on wildcard binding and document that manually publishing public host ports exposes an unauthenticated service.

Use one multi-stage Dockerfile for frontend/backend builds, a non-root runtime user, a writable persistent volume at `/data`, health checks, and no runtime Node requirement. Choose runtime image and CGO strategy with the SQLite driver; pin toolchain/image versions during M1/M8. Configure same-origin mutation checks, allowed hosts/origins, bounded HTTP deadlines, safe logging, and plaintext-storage disclosure. There is no TLS termination/authentication component in the proposed local release.

## Trade-offs and review gates

- One process and SQLite simplify startup and durability; serialized writes limit throughput. Measure before adding concurrency/services.
- Database BLOBs simplify atomic deletion but increase database size and raw/decoded duplication.
- Substring search avoids FTS complexity but scans matching content; M9 separately measures list and search against the approved 500 ms p95 target using the reproducible SRS fixture.
- Polling avoids an event broker but adds bounded latency/load; optimize only with measured need.
- No authentication fits loopback development, not shared-host deployment. No secure-erasure promise is made.
- Approved: SRS resource/retention defaults, BLOB storage, local plaintext SMTP, OTP heuristic/ranking and required recipient/since, embedded frontend, and separate list/search performance targets. Remaining review choices: address matching, detailed API/schema choices, duplicate-submission policy, migration/backfill strategy, manual refresh, milestone naming, and implementation-time toolchain/library/deadline/runtime choices. Architecture changes require approval under AGENTS.md.
