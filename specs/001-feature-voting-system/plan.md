# Implementation Plan: Feature Voting System

**Branch**: `001-feature-voting-system` | **Date**: 2026-06-12 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/001-feature-voting-system/spec.md`

**Architecture decision**: Option A — Synchronous Atomic Increments (strong consistency), per [docs/research/feature-voting-architecture.md](../../docs/research/feature-voting-architecture.md) §4. Options B/C are explicitly out of scope for v1; a metrics-gated evolution seam to Option B is preserved behind a `VoteCounter` interface but not built.

## Summary

Build an authenticated Feature Voting System as a single Go REST/JSON service. Users submit feature requests (title + description), browse them with cursor pagination, cast/remove one upvote per request (never on their own), and view **Top** (raw count) and **Trending** (time-decay) rankings. Correctness under viral spikes is the core promise.

Technical approach (Option A): PostgreSQL is the single source of truth. A vote and its denormalized `vote_count` move together in one transaction (`INSERT ... ON CONFLICT DO NOTHING` for idempotent dedup + conditional `UPDATE`), making counts transactionally exact. The read path is optimized aggressively: Redis caches counts and maintains `top`/`trending` ZSETs (trending recomputed by a periodic job), with Postgres as the durable fallback on cache miss. List endpoints use keyset pagination and `ETag`/`304` conditional polling so the client can feel live via optimistic UI + periodic revalidation, with zero push infrastructure. The whole stack (`app`, `postgres`, `redis`) runs via `docker compose up`.

## Technical Context

**Language/Version**: Go 1.23+ (static binary, `CGO_ENABLED=0`)

**Primary Dependencies**:
- HTTP: `net/http` + `chi` v5.3.0 (router/middleware)
- Data: `pgx` v5.10.0 + `sqlc` (type-safe generated queries); `golang-migrate` for migrations
- Cache/ranking: `go-redis` (Redis counts cache + `top`/`trending` ZSETs)
- Auth: JWT access + refresh tokens; `argon2id` password hashing (`golang.org/x/crypto/argon2`)
- Observability: `log/slog` (JSON), Prometheus client (`/metrics`, RED), OpenTelemetry traces
- Testing: `testify` v1.11.1, `testcontainers-go` (real Postgres + Redis), `net/http/httptest`

**Storage**: PostgreSQL 16 (primary, source of truth) + Redis 7 (count cache + ranking ZSETs + rate-limit buckets). Schema per research §1.5: `users`, `feature_requests`, `votes` (composite PK), indexes `idx_fr_top`, `idx_fr_created`, `idx_votes_request`.

**Testing**: Unit (domain/services, table-driven), integration via testcontainers (`//go:build integration`), API/e2e via `httptest`. `go test -race ./...` in CI; ≥70% coverage on critical paths.

**Target Platform**: Linux server (single-host Docker Compose). Stateless app tier — maps cleanly to a VPS / container PaaS without code changes.

**Project Type**: Web service (REST/JSON API). Clients (web SPA + mobile) consume one API via an OpenAPI 3.1 contract; no frontend is built in this repo.

**Performance Goals**:
- Vote write completes within a few seconds even under a ≥1,000 upvotes/minute spike on one request (SC-004).
- Counts/rankings reconcile to authoritative state within ≤60 s revalidation interval (SC-005).
- List reads cheap at any depth: keyset pagination (O(1) at depth) + `ETag`/`304` collapse polling cost.

**Constraints**:
- One active vote per (user, request) enforced structurally by the `votes` composite PK — exact under concurrency (SC-003).
- No self-votes (FR-008); no public/anonymous path — every endpoint requires JWT (FR-001).
- Trending freshness may lag by the recompute interval (1–5 min); `Top` and per-request counts stay live.
- No WebSocket/SSE — optimistic UI + conditional polling only.

**Scale/Scope**: Single-host deployment. 18 functional requirements, 5 user stories, 8 success criteria. Read-heavy domain (reads dominate writes); the hard path is read/ranking, not vote-write.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-checked after Phase 1 design.*

Constitution v1.0.0 — evaluated against each principle:

| Principle | Status | How this plan complies |
|-----------|--------|------------------------|
| I. Idiomatic Go & Code Clarity | ✅ PASS | `gofmt`/`goimports`, `go vet`, `staticcheck` in CI; intent-revealing names; doc comments on exported symbols; no `util`/`common`/`helper` packages. |
| II. Explicit Error Handling (NON-NEGOTIABLE) | ✅ PASS | Every error checked and wrapped with `%w` at call boundaries; sentinel errors for domain failures (`ErrSelfVote`, `ErrAlreadyVoted`, `ErrNotFound`); `errors.Is/As` for inspection; errors logged only at I/O boundaries (handlers, repo). |
| III. Test-First Discipline | ✅ PASS | Tests written before implementation; table-driven; `go test -race ./...` in CI; ≥70% on handlers/services/repos; integration tests gated by `//go:build integration` + testcontainers; benchmarks for the hot vote path before any optimization. |
| IV. Concurrency Safety | ✅ PASS | `context.Context` first param on all I/O; periodic trending/reconciliation jobs have explicit cancellation/exit conditions (no leaks); no `time.Sleep` for synchronization; shared state via Postgres row locks (vote tx) + Redis atomics, not in-process mutable globals. |
| V. Layered Architecture & Separation of Concerns | ✅ PASS | `cmd/server/main.go` + `internal/handler` (HTTP only) / `internal/service` (business logic) / `internal/repository` (data only). Dependencies flow inward via interfaces; constructor injection (`NewService(repo, log)`); parameterized SQL only (`$1`,`$2`); early-return style. |
| VI. Security & Observability | ✅ PASS | No hardcoded secrets (`.env`/env_file); input validated at handler boundary; `govulncheck` in CI; rate limiting (Redis token bucket) seam on public endpoints; `slog` JSON logs; all I/O instrumented (OTel + Prometheus RED); `/healthz`+`/readyz`+`/metrics`; security headers; pgx pool limits configured; non-root distroless image, pinned versions, multi-stage `CGO_ENABLED=0`. |

**Technology Stack alignment**: All chosen technologies (Go 1.23+, chi v5.3.0, pgx v5.10.0, testify v1.11.1, slog, golangci-lint/vet/staticcheck, Docker Compose, golang-migrate, govulncheck) are exactly those approved in the constitution's Technology Stack table. Additions used by Option A — `sqlc`, `go-redis`, OpenTelemetry, Prometheus client, `golang-migrate`, JWT/argon2 libs — are read-path/observability/auth tooling consistent with Principle VI; `golang-migrate` and `govulncheck` are already listed. `go-redis`, `sqlc`, OTel, Prometheus, and JWT/argon2 libraries are net-new dependencies. They serve Principle VI (observability/security) and the research-mandated read path; if the team wants them formally enumerated, a MINOR constitution amendment to the Technology Stack table is the right vehicle — flagged, not blocking.

**Gate result**: ✅ PASS. No violations requiring Complexity Tracking. The constitution's `/health` naming vs. this plan's `/healthz` is a cosmetic alias — both are exposed; `/healthz` + `/readyz` is the research-specified split (liveness vs readiness) and `/health` can alias liveness if strict compliance is desired.

## Project Structure

### Documentation (this feature)

```text
specs/001-feature-voting-system/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (/speckit-plan command)
├── data-model.md        # Phase 1 output (/speckit-plan command)
├── quickstart.md        # Phase 1 output (/speckit-plan command)
├── contracts/           # Phase 1 output (/speckit-plan command)
│   └── openapi.yaml      # OpenAPI 3.1 REST contract for web/mobile clients
├── checklists/          # Pre-existing quality checklists
└── tasks.md             # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
cmd/
└── server/
    └── main.go                  # Composition root: config, DI wiring, server lifecycle, graceful shutdown

internal/
├── config/                      # 12-factor env loading (.env / env_file)
├── handler/                     # HTTP only: decode, call service, encode; ETag/304; cursor params
│   ├── auth.go                  # register, login, refresh
│   ├── requests.go              # submit + list (Top/Trending/New) with keyset pagination
│   └── votes.go                 # upvote / remove upvote
├── service/                     # Business logic only (no HTTP, no SQL)
│   ├── auth.go                  # JWT issue/verify, argon2id hashing
│   ├── request.go               # submission validation, listing orchestration
│   ├── vote.go                  # vote rules (self-vote, dedup) + VoteCounter interface
│   └── ranking.go               # Top/Trending read orchestration over Redis ZSETs
├── repository/                  # Data access only (sqlc-generated + thin wrappers)
│   ├── postgres/                # sqlc queries, the atomic vote transaction
│   └── redis/                   # count cache, top/trending ZSETs, rate-limit buckets
├── ranking/                     # Periodic trending recompute job + reconciliation job
├── middleware/                  # JWT auth, request logging, rate limit, recover, OTel, metrics
├── observability/               # slog setup, Prometheus RED metrics, OTel tracer init
└── platform/                    # health/readiness probes, server bootstrap helpers

migrations/                      # golang-migrate SQL (0001_init.up/down.sql ...)
db/
├── queries/                     # *.sql source for sqlc
└── sqlc.yaml                    # sqlc config

api/
└── openapi.yaml                 # Served/published copy of the contract (source in specs/.../contracts)

deploy/
├── Dockerfile                   # multi-stage builder -> distroless, non-root, static binary
├── docker-compose.yml           # app + postgres + redis; healthchecks, depends_on, named volumes
└── .env.example                 # documented config surface

tests/
├── integration/                 # //go:build integration — testcontainers (real PG + Redis)
└── e2e/                         # httptest-driven API flows

Makefile                         # build, test, lint, migrate, sqlc generate, docker targets
.github/workflows/ci.yml         # golangci-lint -> test -race -> govulncheck -> build image
```

**Structure Decision**: Single Go web-service project following the constitution's mandated `cmd/server` + `internal/{handler,service,repository}` layering (Principle V). Unit tests live in `*_test.go` beside the code they cover; cross-cutting integration and e2e suites live under `tests/`. No frontend in this repo — clients integrate through `contracts/openapi.yaml`.

## Complexity Tracking

> No constitution violations require justification. Table intentionally empty.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| — | — | — |
