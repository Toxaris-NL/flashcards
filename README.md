# Flashcards

Flashcards is a self-hosted study app for Sander's kids. It focuses on individualized flashcard learning, subject-based study lists, progress tracking, and a simple admin workflow for approvals, credentials, and settings.

The app is built as a single Go binary with a static HTML/CSS/JS frontend. No database is used for v1; study content and user data are stored as JSON files on disk.

## Product goals

- Support multiple kids with separate content spaces
- Organize learning as Subject → Period → Card list
- Offer typed and mixed study sessions without forcing multiple-choice-only flows
- Keep content accessible after a period is completed instead of silently deleting it
- Keep the interface Dutch for all kid and parent-facing screens
- Deploy as one configurable Go service behind the existing stack

## Key capabilities

- Student accounts with PIN-based login, approval, and temporary-PIN replacement
- Content management for subjects, periods, lists, cards, and imports
- Study sessions with review logic and wrong-answer requeue behavior
- Progress tracking per kid and per list
- Admin dashboard for approvals, settings, and overview data
- Audio pronunciation support via the browser Web Speech API

## Technology

- Backend: Go
- Router: Chi
- Logging: slog
- Config: TOML
- Frontend: static HTML, CSS, and JavaScript served by the Go server
- Storage: JSON on disk
- Build requirement: CGO disabled

## Repository layout

- cmd/flashcards: application entrypoint and HTTP setup
- internal/admin: admin sessions, management, settings, and progress dashboard logic
- internal/auth: kid login/session and approval logic
- internal/config: TOML config loading and validation
- internal/content: subject/period/card storage and import parsing
- internal/progress: progress and review history persistence
- internal/review: spaced-repetition scheduling helpers
- internal/study: study session selection and logic
- internal/web: HTTP handlers, page rendering, and static assets
- data: runtime data directory used by the app
- openspec: product constitution and capability specs

## Local development

1. Run the app:

   go run ./cmd/flashcards

2. On first run, the application creates `flashcards.toml` when it is missing.
   The development credentials are `admin` / `example-password`.
3. Before exposing the application, replace the default admin password and session
   secret. The admin dashboard can update the password; the session secret remains
   a TOML or environment setting.
4. Open the app in the browser at the configured listen address.

For production deployment, use the Docker and compose setup described in [DEPLOYMENT.md](DEPLOYMENT.md).

## Configuration notes

The application reads settings from a TOML config file and supports environment overrides, for example:

- SERVER_LISTEN_ADDR
- DATA_DIR
- ADMIN_USERNAME
- ADMIN_PASSWORD_HASH
- SESSION_SECRET

The config loader validates required values before startup. The admin dashboard
stores password changes as a bcrypt hash in the active TOML configuration file.

## Standards and constraints

This project follows the repository constitution in [openspec/project.md](openspec/project.md). In particular:

- The frontend remains static and framework-free
- The backend stays dependency-light and pure Go
- No database is introduced for v1
- User-facing text stays in Dutch
- Specs remain brief and reviewable

## Documentation

- [openspec/project.md](openspec/project.md) — product constitution and scope
- [DEPLOYMENT.md](DEPLOYMENT.md) — Docker and deployment instructions
- [architecture.md](architecture.md) — system design and component overview

## Notes

The app is intentionally simple and operationally friendly: one binary, one data directory, static UI files, and JSON-backed persistence. That keeps deployment predictable on a small self-hosted machine while still supporting the required kid-study workflow.
