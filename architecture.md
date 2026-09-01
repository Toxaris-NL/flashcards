# Architecture overview

This application is a small, self-hosted Go service that serves a static Dutch-language web UI and stores study data as JSON files on disk. The design is intentionally simple: one binary, no database, no frontend framework, and clear separation between network handling, app services, and persistence.

## 1. System boundary

```mermaid
flowchart LR
    User[Kid or admin browser]
    Web[Go HTTP server]
    Routes[Route groups /admin /student /study]
    Services[Auth + Content + Study + Review + Progress + Admin]
    Storage[(JSON files in data/)]

    User --> Web
    Web --> Routes
    Routes --> Services
    Services --> Storage
```

## 2. Runtime composition

The bootstrap path is centered in [cmd/flashcards/main.go](cmd/flashcards/main.go): it loads TOML config, instantiates the storage-backed services, mounts the routes, and starts the HTTP server.

```mermaid
flowchart TD
    A[Start process] --> B[Load config]
    B --> C[Create stores/services]
    C --> D[Build chi router]
    D --> E[Mount /admin, /student, /study]
    E --> F[ListenAndServe]
```

## 3. Service map

```mermaid
flowchart TB
    subgraph App
        Main[cmd/flashcards/main.go]
        Config[internal/config]
        Auth[internal/auth]
        Content[internal/content]
        Study[internal/study]
        Review[internal/review]
        Progress[internal/progress]
        Admin[internal/admin]
        Web[internal/web]
    end

    Main --> Config
    Main --> Auth
    Main --> Content
    Main --> Study
    Main --> Review
    Main --> Progress
    Main --> Admin
    Main --> Web
```

### Responsibilities

- [internal/config](internal/config): TOML config loading and validation
- [internal/auth](internal/auth): kid accounts, PIN login, approval, sessions
- [internal/content](internal/content): subjects, periods, lists, cards, CSV/JSON import
- [internal/study](internal/study): session selection and study behavior
- [internal/review](internal/review): scheduling and repeated review logic
- [internal/progress](internal/progress): progress persistence and summaries
- [internal/admin](internal/admin): admin dashboard data, settings, management operations
- [internal/web](internal/web): route handlers, rendering, and static asset serving

## 4. Request flow

### Admin flow

```mermaid
sequenceDiagram
    participant Browser
    participant Web as internal/web
    participant Admin as internal/admin
    participant Store as JSON store

    Browser->>Web: POST /admin/login
    Web->>Admin: Validate credentials
    Admin->>Store: Read kid/admin data
    Store-->>Admin: Data
    Admin-->>Web: Session cookie + auth result
    Web-->>Browser: Redirect to /admin

    Browser->>Web: GET /admin
    Web->>Admin: Load dashboard data
    Admin->>Store: Read settings + progress + kids
    Store-->>Admin: State
    Admin-->>Web: Render dashboard
    Web-->>Browser: HTML / JSON response
```

### Student study flow

```mermaid
sequenceDiagram
    participant Browser
    participant Web as internal/web
    participant Auth as internal/auth
    participant Content as internal/content
    participant Study as internal/study
    participant Progress as internal/progress

    Browser->>Web: Login or session check
    Web->>Auth: Validate kid session
    Auth-->>Web: Session ok

    Browser->>Web: Open subject / period / list
    Web->>Content: Load list data
    Content-->>Web: Cards + metadata

    Browser->>Web: Start study session
    Web->>Study: Select cards and session mode
    Study-->>Web: Session payload

    Browser->>Web: Stop study session
    Web->>Progress: Store session summary
    Progress-->>Web: Updated summary
    Web-->>Browser: Next card / completion result
```

## 5. Data architecture

The application stores content in a simple filesystem layout rather than a database.

```mermaid
flowchart TB
    Root[data/]
    Root --> Accounts[kids.json]
    Root --> Content[content/]
    Content --> KidA[student-id/]
    KidA --> SubjectA[subject-slug/]
    SubjectA --> PeriodA[period-name.json]

    PeriodA --> Cards[Card data + metadata]

    Root --> Admin[admin-settings.json]
    Root --> Progress[progress/]
    Progress --> History[student-id.json]
```

### Storage principles

- Each student gets an isolated area under `content/<student-id>`
- Subject and period are structured as folders and JSON files
- Lists contain cards, metadata, typing settings, and timestamps
- Account data is stored in `kids.json`; progress and admin settings use dedicated JSON files
- The model favors reviewability, backups, and easy debugging over complex relational structure

## 6. Frontend architecture

The UI is static HTML/CSS/JS served by the Go binary. There is no Node build pipeline and no JavaScript framework.

```mermaid
flowchart LR
    Browser[Browser]
    GoServer[Go binary]
    Static[internal/web/static]
    HTML[HTML pages]
    CSS[CSS styles]
    JS[Vanilla JS]

    Browser --> GoServer
    GoServer --> Static
    Static --> HTML
    Static --> CSS
    Static --> JS
```

### Why this matters

- deployment stays simple
- the app can run as a single binary
- UI assets are easy to inspect and tweak directly
- the project remains compatible with the v1 self-hosted deployment model

## 7. Boundary constraints

The architecture is intentionally narrow because of the repository constitution.

```mermaid
flowchart TB
    A[Architecture choices]
    A --> B[Single Go binary]
    A --> C[Static HTML/CSS/JS]
    A --> D[JSON file persistence]
    A --> E[No database]
    A --> F[No CGO dependencies]
    A --> G[Dutch UI strings]
    A --> H[Simple self-hosted deployment]
```

## 8. Design summary

The system follows a classic layered approach:

- web layer handles HTTP and page rendering
- domain services enforce business rules
- storage layer persists JSON state without a DB
- the static frontend keeps deployment and maintenance lightweight

This architecture is deliberately optimized for a small family study app: easy to run, easy to inspect, easy to deploy, and aligned with the product goals documented in [openspec/project.md](openspec/project.md).
