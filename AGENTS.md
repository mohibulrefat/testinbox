# TestInbox - Agent Instructions

## Project
TestInbox is a local-first developer tool for capturing email and SMS messages and testing OTP verification workflows.

## Technology
- Backend: Go
- Frontend: React, TypeScript, Vite
- Storage: SQLite
- Email capture: SMTP
- SMS capture: Local HTTP API
- Distribution: Docker and Docker Compose

## Working Rules
1. Read `docs/SRS.md`, `docs/PLAN.md`, and `docs/ARCHITECTURE.md` when they exist and are relevant to the task.
2. Work only on the milestone or task explicitly requested.
3. Inspect the existing codebase before making changes.
4. Do not redesign approved architecture without explaining the reason and requesting approval.
5. Prefer small, maintainable implementations over unnecessary abstractions.
6. Add or update tests for behavior changes.
7. Run relevant tests, formatting, and static checks before reporting completion.
8. Never claim a test passed unless it was actually executed successfully.
9. Do not introduce unnecessary services or dependencies.
10. Never enable real email or SMS delivery. TestInbox must capture messages locally.
11. Preserve unrelated changes in the working tree.
12. Do not commit or push changes unless explicitly requested.

## Reporting
At the end of each task, report:
- Changes made
- Important files modified
- Tests and checks executed
- Results and known limitations
- Any decisions requiring approval
