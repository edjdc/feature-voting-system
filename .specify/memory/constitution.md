<!--
===========================================================================
SYNC IMPACT REPORT
===========================================================================
Version change:    (unversioned template) → 1.0.0
Bump rationale:    MAJOR — first complete fill; all placeholder tokens
                   replaced with concrete, project-specific principles.

Principles added:
  I.   Idiomatic Go & Code Clarity
  II.  Explicit Error Handling (NON-NEGOTIABLE)
  III. Test-First Discipline
  IV.  Concurrency Safety
  V.   Layered Architecture & Separation of Concerns
  VI.  Security & Observability

Sections added:
  - Technology Stack
  - Pre-Commit Quality Gates
  - Governance

Templates reviewed:
  ✅ .specify/templates/plan-template.md — "Constitution Check" gate reads
     "Gates determined based on constitution file"; generic reference
     remains valid and correct. No update required.
  ✅ .specify/templates/spec-template.md — No principle-driven mandatory
     sections to add; existing structure is compatible.
  ✅ .specify/templates/tasks-template.md — Task categories (Setup,
     Foundational, Polish with security hardening & logging) align with
     Principles III and VI. No update required.

Deferred TODOs:    None. All fields resolved.
Follow-up:         If team adds a second language or framework, amend
                   the Technology Stack section under a MINOR bump.
===========================================================================
-->

# Feature Voting System Constitution

## Core Principles

### I. Idiomatic Go & Code Clarity

All code MUST be formatted with `gofmt` and `goimports` on every save — no exceptions.
Names MUST communicate intent: `MixedCaps` for exported types, `camelCase` for unexported
identifiers, all-uppercase acronyms (`HTTP`, `URL`, `ID`), single-word lowercase package
names. Generic package names (`util`, `common`, `helper`) are PROHIBITED. Developers MUST
NOT use unsafe casts or reflection outside of explicitly justified, documented cases.

- Code clarity takes precedence over cleverness; a simple loop beats a complex abstraction.
- `go vet ./...` and `staticcheck ./...` MUST pass before every commit.
- Doc comments are REQUIRED on all exported symbols; package-level comments MUST state purpose.
- Comments explain "why", not "what" — self-documenting code is the first goal.

**Rationale**: Go's strength comes from enforced uniformity. `gofmt` eliminates style debates;
intent-revealing names eliminate reading overhead. Any contributor MUST be able to read any
file cold without consulting the author.

### II. Explicit Error Handling (NON-NEGOTIABLE)

Every error return value MUST be checked. Silent error swallowing (`_ = fn()`, unchecked
returns, or `if err != nil { return }` without wrapping) is PROHIBITED. Errors MUST be
wrapped with context using `fmt.Errorf("operation: %w", err)` at each call-site boundary.
Sentinel errors (`var ErrX = errors.New(...)`) MUST represent expected domain failures.
Custom error types MUST be used for structured, domain-specific errors.

- `panic`/`recover` MUST NOT be used for routine error flows.
- Errors are logged only at I/O boundaries (HTTP handlers, database calls), not in every layer.
- `errors.Is()` and `errors.As()` MUST be used for error inspection; type assertions on errors
  are PROHIBITED.

**Rationale**: Explicit error handling is Go's primary correctness mechanism. Swallowed errors
cause silent data corruption and outages that are nearly impossible to diagnose in production.

### III. Test-First Discipline

All code changes MUST be accompanied by tests before implementation begins. Table-driven tests
are the REQUIRED format for any function with more than one input variant. `go test -race ./...`
MUST run in CI and pass cleanly. Coverage on critical paths (handlers, services, repositories)
MUST reach ≥70%. Integration tests MUST use build tags (`//go:build integration`) to stay
separate from unit tests.

- Tests live in `*_test.go` files alongside the code they test.
- `github.com/stretchr/testify` is the approved assertion library.
- Mocks MUST be interface-based: handwritten for small interfaces, testify/mock for larger ones.
- Benchmarks MUST be written and validated before optimizing any performance-critical path;
  `pprof` profiles MUST support any claimed improvement.

**Rationale**: Tests written after the fact cover happy paths only. Table-driven tests force
explicit enumeration of edge cases. Race detection surfaces concurrency bugs that only appear
under load — before they surface in production.

### IV. Concurrency Safety

Every goroutine MUST have a clearly defined exit condition. Goroutines that can leak (blocked
sends/receives without a cancellation path) are PROHIBITED. `context.Context` MUST be the first
parameter of any function that performs I/O, spawns goroutines, or crosses a service boundary.
Shared mutable state MUST be protected by `sync.Mutex` or `sync/atomic`; data races are
zero-tolerance. Channel ownership is fixed: the sender closes the channel; receivers MUST NOT
close it.

- `go test -race ./...` MUST run in CI.
- `time.Sleep` is PROHIBITED as a synchronization mechanism.
- `sync.WaitGroup` MUST be used when the caller needs to wait on goroutine completion.
- `sync.Pool` SHOULD be used for frequently allocated temporary objects in hot paths.

**Rationale**: Goroutine leaks and data races are the most common causes of production outages
in Go services. Enforcing context propagation and strict channel ownership rules prevents both.

### V. Layered Architecture & Separation of Concerns

Code MUST be organized into three distinct layers:
- **Handler** — HTTP concerns only (decode request, call service, encode response).
- **Service** — Business logic only (no HTTP, no direct SQL).
- **Repository** — Data access only (no business logic, no HTTP).

Cross-layer coupling is PROHIBITED; dependencies flow inward through interfaces. Dependency
injection MUST use constructor functions (e.g., `NewService(repo Repository, log *slog.Logger)`).
The standard directory layout MUST be followed:

```
cmd/server/main.go
internal/handler/
internal/service/
internal/repository/
```

- Each function MUST have a single responsibility; parameter count is limited to 3–4 (use a
  config struct beyond that).
- The early-return pattern MUST be used to reduce nesting; nesting deeper than 3 levels is a
  code smell requiring refactoring.
- SQL queries MUST use parameterized placeholders (`$1`, `$2`). String concatenation to build
  SQL is PROHIBITED unconditionally (SQL injection risk).

**Rationale**: Clear layer separation enables independent testing of each concern. It prevents
logic from becoming entangled across boundaries as the codebase grows and team composition changes.

### VI. Security & Observability

Secrets MUST NEVER be hardcoded; only environment variables or secret managers are acceptable.
All external input (request parameters, file uploads, API payloads) MUST be validated at the
handler boundary. `govulncheck ./...` MUST pass with zero vulnerabilities before any release.
Rate limiting MUST be applied to all public endpoints.

Structured logging via `log/slog` with JSON handler in production is REQUIRED. All I/O
operations (database queries, outbound HTTP calls) MUST be instrumented. Health (`/health`)
and metrics (`/metrics`) endpoints MUST be exposed. Log messages MUST use structured fields —
`fmt.Sprintf`-formatted log strings are PROHIBITED.

- HTTP security headers (Content-Security-Policy, X-Frame-Options) MUST be set on all responses.
- Database connection pooling MUST be configured (`SetMaxOpenConns`, `SetMaxIdleConns`).
- Docker images MUST run as non-root (`USER 1000`) and use pinned base image versions.
- Multi-stage Docker builds MUST be used; `CGO_ENABLED=0` MUST produce static binaries.

**Rationale**: Security and observability cannot be retrofitted. A service that cannot be
observed cannot be operated. A service that is not secure is a liability from day one.

## Technology Stack

The following stack is approved for this project. Additions or replacements require a
constitution amendment (minimum MINOR bump) with justification against: standard-library
sufficiency, maintenance health, and attack surface introduced.

| Concern | Technology | Version |
|---------|-----------|---------|
| Language | Go | 1.23+ |
| HTTP Router | chi | v5.3.0 |
| Database Driver | pgx (via `database/sql`) | v5.10.0 |
| Testing | testify | v1.11.1 |
| Logging | log/slog | stdlib (Go 1.21+) |
| Formatting | gofmt, goimports | stdlib |
| Linting | golangci-lint, go vet, staticcheck | latest stable |
| Build Automation | go build, make | stdlib |
| Containers | Docker + Docker Compose | pinned versions |
| Migrations | golang-migrate | latest stable |
| Vulnerability Scanner | govulncheck | latest stable |

## Pre-Commit Quality Gates

Every commit MUST pass all gates below before merging. CI enforces these automatically;
local pre-commit hooks are strongly encouraged.

**Code**
- [ ] `gofmt -s -w .` applied — zero diffs
- [ ] `go vet ./...` — zero errors
- [ ] `golangci-lint run ./...` — clean (gosec plugin enabled)
- [ ] `go build ./...` — compiles successfully

**Tests**
- [ ] `go test -race ./...` — all pass
- [ ] Coverage ≥70% on critical paths: `go test -coverprofile=out ./...`
- [ ] Integration tests pass: `go test -tags=integration ./...`
- [ ] Benchmarks re-validated if any performance-critical code changed

**Security**
- [ ] No hardcoded secrets, API keys, or passwords
- [ ] All errors explicitly handled — no `_ = fn()` or bare `return` on error
- [ ] `govulncheck ./...` — zero vulnerabilities
- [ ] All resources properly closed (`defer db.Close()`, `defer resp.Body.Close()`)

**Documentation**
- [ ] All exported functions and types have doc comments
- [ ] README updated for any API or behavioral changes
- [ ] Code comments explain "why", not "what"

**Docker**
- [ ] `docker compose build` — succeeds
- [ ] `docker compose up` — starts without errors
- [ ] Health check endpoint (`/health`) returns HTTP 200

## Governance

This constitution is the highest-authority document for the Feature Voting System.
It supersedes all local conventions, personal preferences, and informal agreements.

**Amendment Procedure**:
1. Propose the change with motivation in a PR or discussion thread.
2. At least one additional contributor must approve the amendment.
3. A migration plan MUST accompany any amendment that invalidates existing code.
4. Increment `CONSTITUTION_VERSION` using semantic versioning:
   - **MAJOR**: Principle removal, redefinition, or backward-incompatible governance change.
   - **MINOR**: New principle or section added, or material expansion of guidance.
   - **PATCH**: Clarification, wording correction, or non-semantic refinement.
5. Update `LAST_AMENDED_DATE` to today's date (ISO 8601).

**Compliance Review**: All PRs MUST verify compliance with each applicable principle.
Violations require explicit justification in the PR description. Unresolved violations block
merge. Complexity beyond what the principles permit MUST be recorded in the plan's Complexity
Tracking table with a clear rationale for why a simpler alternative was rejected.

**Runtime Guidance**: See `docs/Go-development-guidelines.md` for detailed patterns, code
examples, and tool commands supporting each principle.

**Version**: 1.0.0 | **Ratified**: 2026-06-12 | **Last Amended**: 2026-06-12
