# TestInbox Milestone Plan

Status: M0 review draft. This plan fixes M0–M9 order and places OTP work at M5 as approved. The M0–M9 sequence below is preserved; detailed milestone names remain review proposals. Review before implementation.

## Common completion rules

Implement only the explicitly authorized milestone. A milestone is complete when its deliverables exist, measurable acceptance checks pass, relevant tests/formatting/static checks have actually run, documentation reflects final behavior, and remaining limitations are reported. Do not claim unexecuted checks passed. Preserve unrelated changes; do not commit/push unless requested. No milestone enables real delivery, accounts, cloud hosting, or extra services.

For Go changes, run gofmt checks, `go test ./...`, and `go vet ./...`; run race tests for concurrency-sensitive changes where supported. For frontend changes, run the established lint, TypeScript, unit/component test, and production-build commands. Exact scripts/toolchain versions are established in M1. Integration tests use temporary databases and loopback ephemeral ports, never real recipients. Every behavior change gets relevant regression coverage. Docker checks begin once Docker artifacts exist; failures due to missing tools must be reported, not treated as passes.

## M0 — Product Foundation and Technical Design

Dependencies: repository assessment and approved constraints.

Tasks: produce SRS, architecture, and this plan; document constraints, proposal/default distinctions, risks, API/schema sketches, release criteria, and unresolved questions. Link documents from README. Review consistency and scope.

Acceptance: all three documents exist and cover requested topics; all approved constraints agree across documents; proposed choices are explicit; every milestone has dependencies, verification, and a completion gate. No application code/dependencies are introduced.

Verification: read all documents together; check local links, requirement coverage, milestone order, ports/binds, M5 sequencing, and Git diff scope.

Complete when: documents are delivered for review. Implementation remains gated on review and explicit authorization; review approval is recorded before M1 begins.

## M1 — Repository and Application Foundation

Dependencies: M0 review approval; resolve toolchain versions, dependency selection process, configuration contract, and HTTP deadlines.

Tasks: create Go entry point and React/TypeScript/Vite skeleton; development scripts; validated configuration; lifecycle/shutdown; health endpoints without SQLite initialization; documented commands and ignore rules. Choose minimal build/test dependencies with license and CGO implications recorded.

Acceptance: backend starts with loopback HTTP defaults; liveness is 200 and readiness is 503 `storage_not_initialized` until M2 implements storage; frontend production build succeeds; malformed configuration and port conflicts fail clearly; shutdown completes within configured grace period; no capture or OTP behavior yet.

Verification: configuration/lifecycle/health tests explicitly asserting M1 readiness never reports initialized storage; Go checks; frontend lint/type/build checks and a basic render test.

Complete when: a reproducible development build and startup are documented and verified, with selected versions recorded.

## M2 — SQLite Persistence and Message Model

Dependencies: M1; approved BLOB storage and retention/limits; finalize quota accounting and migration details.

Tasks: initialize SQLite and wire real readiness probes; migrations for messages/recipients/headers/attachments; storage CRUD and filters; atomic quota checks; bounded cleanup; durability settings; restart handling. OTP table/extraction remains M5.

Acceptance: temporary database migration and re-open preserve fixtures; duplicate ordered headers and multiple recipients round-trip; cascaded deletion removes related rows; failing transactions leave zero partial records; concurrent writes cannot exceed quotas; expiry uses receipt time, immediately excludes expired records from normal reads, and physically removes them within two cleanup intervals while running. Admission reclaims expired records safely before quota rejection without evicting unexpired data. Readiness is 200 only after initialization/migrations/reconciliation and a successful bounded storage probe; unavailable storage is 503, and failed startup initialization fails startup.

Verification: storage integration tests for migration/restart/rollback/foreign keys, quota boundaries, concurrent admission, expired-record reclamation before quota rejection, immediate expiry visibility with cleanup paused, storage-unavailable readiness, and retention with controllable time; Go checks and race checks.

Complete when: tested persistence primitives support later capture paths without requiring external services.

## M3 — SMTP Email Capture and Message API

Dependencies: M2; approved local plaintext SMTP scope; select SMTP/MIME libraries and resolve address matching and final API filters/errors.

Tasks: SMTP listener, bounded MIME parsing, raw storage, parsed fields/attachments/warnings; versioned list/detail/download/delete HTTP API; pagination/search; safe error handling, logging, origin/Host protections, and attachment headers.

Acceptance: SMTP acknowledgment follows commit; multiple recipients and empty sender work; supported MIME fixtures parse; malformed/bounded parsing preserves exact raw bytes; oversize and storage failures return documented SMTP errors without partial data. API search/filter/cursor boundaries and single/bulk deletion match SRS; unauthorized origins/hosts and unsafe filenames are handled as documented.

Verification: local SMTP protocol and MIME integration fixtures; byte-exact raw download tests; API pagination/filter/error/download/deletion tests including expired list/detail/raw/attachment exclusion before cleanup; size/concurrency/timeouts and storage failure tests; Go/static/race checks.

Complete when: email can be captured, persisted, inspected, searched, downloaded, and deleted through tested APIs. OTP endpoints are absent.

## M4 — Simulated SMS Capture

Dependencies: M3, reusing shared message storage and retrieval.

Tasks: POST `/api/v1/sms`; JSON/media-type/size validation with required non-blank string `to`, required non-empty string `body`, and optional string `from`; SMS metadata and error contract; document local integration examples and retry behavior.

Acceptance: SMS with only `to`/`body` and SMS with optional `from` both return 201 only after commit and survive restart; omitted/empty `from` means unknown sender; body whitespace is preserved; list/detail/search/delete work with SMS and channel filters; malformed, missing `to`/`body`, blank `to`, empty `body`, null/non-string fields, wrong-media-type, oversized, quota, and storage failures produce documented responses with no partial records. No provider credentials/network delivery path exists.

Verification: HTTP/storage integration tests for both documented SMS examples, omitted/empty sender, whitespace-only body, required-field/type/size boundaries, mixed-channel fixtures, restart and rollback tests; Go checks. Inspect capture path to confirm no delivery client.

Complete when: both capture channels are working independently of OTP extraction.

## M5 — OTP Extraction and Automated-Test Retrieval

Dependencies: M4; approved candidate rules/ranking, required recipient/since and bounded wait contract; finalize extraction version/backfill behavior.

Tasks: candidate migration/extractor; persistence on capture for both channels; bounded retained-message backfill; immediate and waiting `/otp` retrieval; candidate metadata and examples.

Acceptance: approved digit boundary/ranking/dedup rules pass positive, negative, and ambiguous fixtures; attachments are excluded; newest-message selection and timestamp/ID tie-break are deterministic; both `recipient` and `since` are mandatory (400 when missing/invalid); optional channel/sender/before/q combine with AND; selection uses unexpired messages ordered by receipt timestamp then ID descending, then keyword proximity/source/occurrence candidate ranking; timeout, cancellation, waiter cap, retention/deletion races, and backfill interruption behave as documented. Wait defaults to 0, max 30 seconds, polling every 250 ms with max 32 concurrent waiters; zero-wait misses return immediate 404 `otp_not_found`, positive waits query at the deadline before returning 404, saturation is 429, and reads do not consume results.

Verification: table-driven extractor tests, upgrade/backfill tests, concurrent API/wait tests with controllable deadlines, each missing required filter, optional-filter combinations, expiry before cleanup and during waits, ranking/timestamp ties, waiter bounds and cancellation, and one automated-test example for each channel; Go/race/static checks.

Complete when: a test can capture either channel and reliably query an OTP using documented filters, with heuristic limitations stated.

## M6 — Dashboard

Dependencies: M5 and M1 frontend skeleton; existing APIs supply all data.

Tasks: channel lists, filters/search/pagination, details, text/HTML source or safe preview, ordered headers/envelope view, attachments/raw download, parse warnings, OTP display/copy, deletion confirmation, and error/empty/loading states. Manual refresh initially.

Acceptance: both channels can be inspected/deleted through the UI; filtering matches API semantics; OTP candidates and ambiguity are visible; bulk deletion requires confirmation; core keyboard flows work at 360 px width and desktop sizes. Captured scripts, remote images/CSS, navigation, and unsafe attachment content cannot execute/load through the preview; escaped source is acceptable if isolation is incomplete.

Verification: frontend lint/type/tests/build; browser end-to-end capture-to-display and deletion flows for each channel; hostile content fixtures and keyboard/responsive checks in documented browsers.

Complete when: dashboard requirements D1–D2 and safe rendering requirements pass, with limitations documented.

## M7 — Reliability and Resource Hardening

Dependencies: M6; existing security/limit checks from earlier milestones remain required, not deferred here.

Tasks: cross-component fault and boundary coverage; graceful shutdown under load; bounded cleanup/backfill behavior; sensitive-log review; tighten discovered reliability gaps without adding scope/services.

Acceptance: full/busy/unwritable database errors and interrupted transactions fail safely; slow SMTP clients, oversized MIME, deep multipart inputs, waiter saturation, and concurrent deletion/capture remain bounded; shutdown acknowledges only committed work; logs contain no fixture bodies/OTPs. Known unsupported encodings or platform restrictions are documented.

Verification: fault-injection integration tests, race checks, bounded stress runs, retention restart tests, and security regression fixtures; full relevant backend/frontend checks.

Complete when: release-critical reliability/security defects found by these checks are resolved and resource behavior matches SRS.

## M8 — Docker and Docker Compose Distribution

Dependencies: M7; choose runtime image/CGO strategy; embedded frontend packaging is approved.

Tasks: multi-stage build, embedded production assets, non-root runtime, persistent volume permissions, health checks, loopback host publishing, Compose configuration, and usage/upgrade/backup documentation.

Acceptance: Compose builds and serves dashboard/API on host 127.0.0.1:8025 and SMTP on 127.0.0.1:1025; container listeners use explicit interface overrides; no runtime Node/external service is needed; captured email/SMS and OTPs survive restart with the volume; clean-volume startup and existing-volume migrations work; API errors are not replaced by SPA HTML.

Verification: Docker build, Compose config validation, clean-start capture/OTP/UI smoke test, stop/start persistence, non-root write permissions, health checks, upgrade fixture, and published-port inspection. Use temporary project/volume names and preserve user data.

Complete when: documented Compose distribution works end to end and operational limits are disclosed.

## M9 — Release Validation and Documentation

Dependencies: M8 and all prior acceptance gates.

Tasks: complete quickstart/API/configuration/reference and troubleshooting docs; verify first-release SRS criteria; measure performance target; resolve release blockers; record supported platforms/browsers and remaining limitations.

Acceptance: a clean checkout follows documented build/run instructions; email and SMS automated-test examples pass; all seven SRS release acceptance items pass; separate 100-request list and search series each achieve p95 below 500 ms using the deterministic 10,000-message SRS fixture after 20 warmup requests per endpoint or a revised target is explicitly approved; no known release-blocking defects remain. No real delivery or out-of-scope services exist.

Verification: complete Go and frontend checks, concurrency/race suite where supported, browser end-to-end suite, Compose smoke/upgrade/persistence checks, and reproducible separate list/search performance reports with seed script, exact commands, raw samples, nearest-rank calculation, machine/configuration/fixture details as specified in SRS N4. Audit docs and dependency licenses/versions.

Complete when: release readiness evidence and limitations are presented for review. Publishing, tagging, committing, and pushing require separate explicit authorization.

## Decisions awaiting review

1. Confirm detailed milestone names; preserve the M0–M9 sequence and M5 OTP placement.
2. Confirm detailed API/schema choices beyond approved SMS and OTP contracts, address matching, and duplicate-submission policy.
3. Confirm forward-only migration/backup strategy and M5 backfill.
4. Confirm manual-refresh dashboard and unauthenticated loopback HTTP scope.
5. Select exact toolchains in M1, SQLite driver in M2, SMTP/MIME libraries in M3, and runtime image/CGO packaging in M8; finalize HTTP deadlines in M1/M3.

Already approved: resource/retention defaults, SQLite BLOB storage, plaintext local SMTP without AUTH/STARTTLS, OTP heuristic/ranking and mandatory recipient/since, embedded frontend, and separate list/search 500 ms p95 targets. M1/M2 storage readiness boundaries and immediate expiry exclusion/reclamation are required as specified above.
