# TestInbox Software Requirements Specification

Status: M0 review draft. Approved constraints are binding; resource/retention defaults, SQLite BLOB storage, plaintext local SMTP, OTP heuristic, and embedded frontend are approved; remaining review items are listed below. No application implementation is included in M0.

## Purpose and goals

TestInbox is a local-first developer tool that captures development email and simulated SMS, displays them, and lets automated tests retrieve verification codes. It should make OTP workflows repeatable without contacting real recipients or relying on external services.

Goals: simple Docker startup, durable local capture, useful inspection of original and parsed messages, deterministic test retrieval, and bounded resource use.

## Scope and constraints

The first release includes SMTP capture on port 1025; SMS capture through a local HTTP API; a dashboard and API sharing port 8025; SQLite persistence; search and deletion; raw MIME, parsed headers, text, HTML, and attachments; and OTP extraction/retrieval introduced in M5 after both capture channels.

One Go application owns capture, storage, API, and background cleanup. The frontend uses React, TypeScript, and Vite and is embedded in the Go binary for distribution. Native listeners bind to 127.0.0.1 by default. Docker and Docker Compose are the primary distribution methods.

Non-goals: real email or SMS delivery, SMTP relay, provider integrations, user accounts, multi-user cloud hosting, public internet exposure, inbound SMS from carriers, outbound webhooks, OTP generation/verification, and production mailbox hosting. Redis, MongoDB, SQS, and microservices require an approved requirement.

## Functional requirements

| ID | Requirement |
| --- | --- |
| E1 | Accept SMTP envelope sender and recipients, including multiple recipients and an empty reverse path. Do not resolve recipients or relay mail. |
| E2 | Preserve accepted DATA bytes as raw MIME after SMTP framing removal. Store envelope metadata separately from MIME headers. Preserve duplicate headers and header order. |
| E3 | Parse subjects, addresses, text/HTML alternatives, MIME charsets and transfer encodings, and attachments. Preserve original bytes even when parsing fails; show a parse warning and available fields. |
| E4 | Acknowledge SMTP success only after durable database commit. Reject oversized DATA and return temporary failure for storage/capacity failures. |
| S1 | Accept simulated SMS via JSON with required non-blank string `to` and non-empty string `body`, and optional string `from`. Omitted or empty `from` means unknown sender; reject null/non-string fields and blank `to` or empty `body`. Preserve body whitespace. Validate content type and configured size limits; return the committed message ID. |
| S2 | Store SMS locally only. Never invoke carrier/provider APIs. |
| M1 | Assign a unique message ID and server UTC receipt timestamp. Persist messages across application/container restarts with a mounted database volume. |
| M2 | List and inspect both channels with pagination, channel, sender, recipient, received-time filters, and case-insensitive substring search across subject and decoded text/body. HTML-only email contributes parsed visible text. |
| M3 | Support individual deletion and explicit bulk deletion using the same filters. Return the affected count. Dashboard bulk deletion requires confirmation. Delete raw content, attachment bytes, and OTP candidates with the message. |
| M4 | Offer original email and attachment downloads. Never render downloaded attachments inline by default. Missing/deleted IDs return a clear not-found response. |
| O1 | In M5, extract and persist OTP candidates from email subject, decoded text, HTML visible text, and SMS body. Raw MIME and attachment content are excluded. |
| O2 | Return candidate values, source, confidence/rank, and extraction version. Extraction is heuristic and must not imply that a code is valid or uniquely correct. |
| O3 | Require both non-blank `recipient` and valid RFC3339 `since` for automated-test retrieval (400 if absent/invalid). Optional filters are `channel`, `sender`, `before`, and `q` (subject/text substring); all combine with AND. Select the newest unexpired matching message containing candidates by receipt timestamp descending, then ID descending; select its highest-ranked candidate. Rank by keyword proximity within 32 characters of case-insensitive `otp`, `code`, or `verification`, then source (subject, SMS body, plain text, HTML visible text), then first occurrence. Deduplicate values using their strongest source. |
| O4 | Support cancellable waiting: `wait_seconds` defaults to 0, maximum 30, with at most 32 concurrent waiting requests. Query immediately, poll every 250 ms when waiting, and query at the deadline. No match returns 404 `otp_not_found` immediately for zero wait or after timeout; invalid bounds return 400 and waiter saturation returns 429. A result does not consume a message; ambiguous candidate sets remain visible. |
| D1 | Dashboard provides channel filters, paginated search, message details, envelope/header inspection, plain text and safe HTML preview, attachment downloads, parse warnings, deletion, and OTP display/copy after M5. |
| D2 | Show loading, empty, validation, timeout, and server-error states. Core actions must work by keyboard with labeled controls. Propose manual refresh for release one; live updates are deferred. |

## Security, privacy, retention, and errors

- The application must never initiate message delivery. Local data may contain credentials and personal information; the dashboard must describe its development-only purpose.
- No authentication is proposed for loopback-only usage. Non-loopback binding requires explicit configuration and a warning; it does not make deployment safe for shared/untrusted networks.
- Do not log bodies, raw MIME, attachment contents, or OTP values. Log request IDs and diagnostic categories without sensitive payloads. Disable permissive CORS; require JSON on SMS writes and reject unexpected browser origins on API mutations. Do not trust a Host header outside configured hosts.
- Treat captured content as hostile. Render headers/text as text; sandbox HTML without scripts, same-origin privileges, forms, navigation, or remote resource loading. If isolation cannot be guaranteed, show escaped HTML source. Attachment downloads use safe filenames and content-disposition attachment.
- SQLite and Docker volumes contain plaintext data. Encryption at rest and filesystem/backup access control are host responsibilities in this release. Deletion removes logical records; secure erasure from WAL, free pages, host snapshots, and backups is not promised.
- Cleanup must be configurable, bounded, and restart-safe. Expiry uses server receipt time: a message expires when `received_at + retention <= query_time`; retention 0 disables expiry. All normal list/search/detail/raw/attachment and OTP queries exclude expired messages immediately, even before physical cleanup (ID lookups return 404). Background cleanup deletes expired records within two cleanup intervals while running. Quota admission safely reclaims eligible expired records before rejecting capture, without evicting unexpired records. Explicit deletion is immediate at the API level.
- Validation errors identify invalid fields without echoing sensitive data. API errors use stable codes, safe messages, and request IDs. Unexpected errors do not expose stack traces or database paths.

## Approved operational defaults

These defaults are approved. Overrides must be validated and documented; limitations remain as described below.

| Setting | Approved default | Reason / trade-off |
| --- | --- | --- |
| Retention | 7 days; cleanup every 60 seconds; 0 disables expiry | Bounded history versus long-running local investigations. |
| Email size | 10 MiB after SMTP framing removal | Supports typical development attachments; larger fixtures need an override. |
| SMS body / total JSON request | 16 KiB UTF-8 / 64 KiB | Allows test payloads while bounding allocation. |
| Message count / stored payload bytes | 10,000 / 1 GiB | Reclaim expired records first; reject new capture if either quota would still be exceeded; never silently evict unexpired data. Payload quota is not a filesystem-size guarantee. |
| MIME parsing | 100 parts, nesting depth 20 | Limits hostile multipart work; raw MIME remains available if parsing is truncated. |
| List page | 50 default, 200 maximum | Bounded responses; opaque keyset cursor. |
| OTPs | Standalone 4–8 ASCII digits; deduplicate per message | Simple baseline; misses alphanumeric/localized codes and can detect unrelated numbers. |
| OTP wait | 0 seconds default; 30 seconds maximum; 32 concurrent waiters | Immediate queries by default; bounded test convenience. |
| SMTP sessions | 32 concurrent, 60-second idle timeout | Limits slow-client resource use. |

Additional HTTP request/header/read/write deadlines must be finalized in M1/M3; OTP wait routes need a write deadline greater than the maximum wait. Full-disk errors remain possible below logical quotas and must fail safely.

## Non-functional requirements

- N1: No external service is required for runtime capture, viewing, or OTP retrieval. Dependency installation/image builds may need internet access.
- N2: Committed messages survive clean restarts; interrupted writes leave no partially visible message. SQLite durability settings and migration failures must be documented.
- N3: Capture, decoding, list responses, and waiting are bounded by validated configuration. Invalid configuration fails startup with an actionable error.
- N4: On a documented reference machine with 2 CPU cores, 2 GiB RAM, and SSD storage, require separate list and search p95 latency below 500 ms. Use the reproducible benchmark fixture below and record both results in M9.
- N5: Support current stable desktop Chrome and Firefox at release verification; responsive layout from 360 px width. API use must not depend on the dashboard.
- N6: Versioned APIs, repeatable migrations, automated regression tests, and documented development commands are required.

## First-release acceptance

1. Compose starts one application with loopback host ports 1025/8025 and persistent SQLite storage; restarting preserves captured messages.
2. SMTP fixtures exercise multiple recipients, empty reverse path, plain text, HTML, multipart attachments, duplicate headers, and malformed MIME. Accepted raw bytes are downloadable and exact after framing removal.
3. SMS JSON capture with only `to` and `body`, or with optional `from`, returns an ID; missing/invalid required fields and oversized payloads fail without creating records. Neither capture path sends external messages.
4. API and dashboard list, filter, search, inspect, paginate, and delete both channels consistently. Deletion removes related logical records and downloads become unavailable.
5. M5 fixtures verify extraction, ambiguous candidates, deterministic newest-match selection, time bounds, no-match timeout, cancellation, and concurrent waiter limits.
6. Retention, quotas, size limits, parser bounds, full/storage failure behavior, and safe HTML/attachment handling have automated coverage.
7. Build, formatting, static checks, backend/frontend tests, Compose smoke tests, and the documented performance check pass in M9. There is no claim of unlimited performance, secure erasure, or production hosting readiness.

## Reproducible performance fixture

M9 must provide a deterministic seed script and benchmark command. Seed a fresh migrated database with 10,000 unexpired messages: alternating email/SMS; fixed IDs and receipt timestamps relative to a recorded benchmark clock; ten cyclic sender/recipient pairs; each decoded body exactly 1,024 UTF-8 bytes with fixed ASCII padding and `needle` in every tenth message; email subject `fixture-N` and matching minimal raw MIME; no attachments. Include normal M5 candidate extraction. Keep retention from expiring fixtures during measurement. Record seed/version, effective configuration, database size, SQLite settings, image/binary revision, OS, CPU/RAM/SSD, and browser-independent HTTP client version.

After seed and startup, warm each endpoint with 20 sequential requests. Measure 100 sequential requests separately for `GET /api/v1/messages?limit=50` and `GET /api/v1/messages?q=needle&limit=50`, without concurrent capture/cleanup or other load. Measure client elapsed time through complete JSON body receipt on loopback, validate 200 responses and expected IDs, sort each 100-sample series, and report its 95th value (nearest-rank p95). Both must be below 500 ms; report full series and command. This warm-cache small-body target does not cover maximum-size attachments or concurrent workloads.

## Remaining review decisions

Resource/retention defaults, BLOB storage, local plaintext SMTP without AUTH/STARTTLS, digit OTP heuristic/ranking, required OTP filters, performance target, and embedded frontend are approved. Remaining proposals: detailed API/schema choices beyond the decisions above, address matching, duplicate-submission policy, migration/backfill strategy, manual-refresh dashboard, and detailed milestone names. Toolchain/library versions, HTTP deadlines, runtime image, and CGO strategy must be selected in their assigned milestones. No implementation is authorized by this document.
