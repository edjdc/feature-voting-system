---
description: "Dependency-ordered task list for the Feature Voting System (Option A — Synchronous Atomic Increments)"
---

# Tasks: Feature Voting System

**Input**: Design documents from `/specs/001-feature-voting-system/`

**Prerequisites**: [plan.md](./plan.md) (required), [spec.md](./spec.md) (user stories), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/openapi.yaml](./contracts/openapi.yaml), [quickstart.md](./quickstart.md)

**Tests**: INCLUDED. The project constitution mandates Test-First Discipline (Principle III) and quickstart.md defines explicit validation scenarios (A–F). Tests are written before implementation and must FAIL first.

**Architecture**: Option A — PostgreSQL is the source of truth; a vote and its denormalized `vote_count` move atomically in one transaction. Redis caches counts + maintains `top`/`trending` ZSETs. Read path uses keyset pagination + `ETag`/`304` conditional polling. Stack runs via `docker compose up`.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: Which user story the task belongs to (US1–US5)
- All paths are repository-root relative, per the plan's Project Structure.

## Path Conventions

Single Go web-service project (per plan.md): `cmd/server/`, `internal/{config,handler,service,repository,ranking,middleware,observability,platform}/`, `migrations/`, `db/`, `api/`, `deploy/`, `tests/{integration,e2e}/`. Unit tests live in `*_test.go` beside the code.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project skeleton, toolchain, container/CI scaffolding. No business logic.

- [ ] T001 Create the directory structure per plan.md (`cmd/server/`, `internal/{config,handler,service,repository/postgres,repository/redis,ranking,middleware,observability,platform}/`, `migrations/`, `db/queries/`, `api/`, `deploy/`, `tests/integration/`, `tests/e2e/`)
- [ ] T002 Initialize Go module in `go.mod` (Go 1.23+) and add pinned dependencies: `chi` v5.3.0, `pgx` v5.10.0, `go-redis`, `testify` v1.11.1, `testcontainers-go`, JWT lib, `golang.org/x/crypto/argon2`, Prometheus client, OpenTelemetry
- [ ] T003 [P] Configure linting/formatting in `.golangci.yml` (gofmt -s, goimports, go vet, staticcheck, gosec)
- [ ] T004 [P] Create `Makefile` targets: `build`, `test`, `test-integration`, `lint`, `vuln`, `sqlc`, `migrate`, `docker`
- [ ] T005 [P] Create `db/sqlc.yaml` (sqlc config pointing at `db/queries/` + `migrations/`, output to `internal/repository/postgres`)
- [ ] T006 [P] Create `deploy/Dockerfile` (multi-stage builder → distroless, non-root, `CGO_ENABLED=0` static binary, pinned base images)
- [ ] T007 [P] Create `deploy/docker-compose.yml` with `app`, `postgres` (PG 16), `redis` (7), and one-shot `migrate` service; healthchecks, `depends_on`, named volumes
- [ ] T008 [P] Create `deploy/.env.example` documenting `DATABASE_URL`, `REDIS_URL`, `JWT_ACCESS_SECRET`, `JWT_REFRESH_SECRET`, `JWT_ACCESS_TTL`, `JWT_REFRESH_TTL`, `TRENDING_RECOMPUTE_INTERVAL`, `PORT`
- [ ] T009 [P] Create `.github/workflows/ci.yml` running gates in order: golangci-lint → `go test -race ./...` → govulncheck → docker build
- [ ] T010 [P] Publish the API contract copy to `api/openapi.yaml` (source remains `specs/001-feature-voting-system/contracts/openapi.yaml`)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Database, config, observability, auth, middleware, and server bootstrap that every user story depends on. Auth lives here because **every** endpoint requires JWT (FR-001).

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [ ] T011 Create golang-migrate migration `migrations/0001_init.up.sql` and `migrations/0001_init.down.sql` per data-model.md DDL (citext extension; `users`, `feature_requests`, `votes` with composite PK; indexes `idx_fr_top`, `idx_fr_created`, `idx_votes_request`)
- [ ] T012 [P] Implement 12-factor env config loading in `internal/config/config.go` (parse all `.env.example` keys, fail fast on missing required)
- [ ] T013 [P] Implement slog JSON logger setup in `internal/observability/logging.go`
- [ ] T014 [P] Implement Prometheus RED metrics registry in `internal/observability/metrics.go`
- [ ] T015 [P] Implement OpenTelemetry tracer init in `internal/observability/tracing.go`
- [ ] T016 [P] Define domain sentinel errors in `internal/service/errors.go` (`ErrNotFound`, `ErrSelfVote`, `ErrAlreadyVoted`, `ErrValidation`)
- [ ] T017 Implement pgx connection pool (with configured limits) in `internal/repository/postgres/pool.go`
- [ ] T018 [P] Implement go-redis client wrapper in `internal/repository/redis/client.go`
- [ ] T019 [P] [Test] Unit test for argon2id hashing + JWT issue/verify in `internal/service/auth_test.go` (write first, must FAIL)
- [ ] T020 Implement argon2id password hashing and JWT access/refresh issue/verify in `internal/service/auth.go`
- [ ] T021 Add base sqlc auth queries in `db/queries/users.sql` (`CreateUser`, `GetUserByEmail`, `GetUserByID`) and run `make sqlc` to generate
- [ ] T022 Implement auth handlers (register, login, refresh) in `internal/handler/auth.go`
- [ ] T023 Implement JWT authentication middleware in `internal/middleware/auth.go` (extracts caller user ID into request context)
- [ ] T024 [P] Implement recover, request-logging, OTel, and metrics middleware in `internal/middleware/observability.go`
- [ ] T025 [P] Implement Redis token-bucket rate-limit middleware in `internal/middleware/ratelimit.go`
- [ ] T026 [P] Implement security-headers middleware in `internal/middleware/security.go`
- [ ] T027 Implement `/healthz`, `/readyz` (checks Postgres + Redis), `/metrics` probes in `internal/platform/health.go`
- [ ] T028 Implement server bootstrap, chi router, global middleware chain, ops + auth route registration, and graceful shutdown in `internal/platform/server.go` and `cmd/server/main.go`
- [ ] T029 [Test] Integration test for auth + migrations (register → login → refresh) using testcontainers in `tests/integration/auth_test.go` (`//go:build integration`)

**Checkpoint**: Foundation ready — `docker compose up` boots, migrations apply, `/readyz` returns 200, a user can register/login. User stories can now begin.

---

## Phase 3: User Story 1 - Submit a feature request (Priority: P1) 🎯 MVP

**Goal**: An authenticated user submits a feature request (title + description); it is persisted, attributed to them, and appears with `vote_count: 0`.

**Independent Test**: Sign in, POST a valid request → `201` with `vote_count: 0` and `author_id` = caller; POST empty title → `400 validation_failed`; POST without token → `401` (quickstart Scenario A).

### Tests for User Story 1 (write first, must FAIL) ⚠️

- [ ] T030 [P] [US1] Unit test for request submission validation (empty/whitespace/over-limit title & description) in `internal/service/request_test.go`
- [ ] T031 [P] [US1] Integration test for submit + auth-required (Scenario A) in `tests/integration/submit_test.go`
- [ ] T032 [P] [US1] E2E test for the submit flow via httptest in `tests/e2e/submit_test.go`

### Implementation for User Story 1

- [ ] T033 [US1] Add sqlc query `InsertFeatureRequest` in `db/queries/requests.sql` and run `make sqlc`
- [ ] T034 [P] [US1] Define `FeatureRequest` domain type in `internal/service/request.go`
- [ ] T035 [US1] Implement `RequestService.Submit` with title/description validation (FR-003) in `internal/service/request.go`
- [ ] T036 [US1] Implement `POST /requests` handler (decode, validate, encode `201` FeatureRequest) in `internal/handler/requests.go`
- [ ] T037 [US1] Register `/requests` POST route behind JWT middleware and wire `RequestService` in `cmd/server/main.go`
- [ ] T038 [US1] Map validation/domain errors to the `Error` schema (`validation_failed` → 400, `unauthorized` → 401) in `internal/handler/requests.go`

**Checkpoint**: User Story 1 fully functional and independently testable — the system is a working idea-capture tool (MVP).

---

## Phase 4: User Story 2 - Browse the list of feature requests (Priority: P1)

**Goal**: An authenticated user browses requests one bounded page at a time (title, description, submitter, vote count) with keyset pagination and no duplicates/skips.

**Independent Test**: Seed >1 page, `GET /requests?sort=new&limit=20` → 20 items + non-null `next_cursor`; next page has no overlap/gap; page past the end → empty items + `next_cursor: null` (Scenario B, SC-007).

### Tests for User Story 2 (write first, must FAIL) ⚠️

- [ ] T039 [P] [US2] Unit test for keyset cursor encode/decode (round-trip + tamper handling) in `internal/handler/cursor_test.go`
- [ ] T040 [P] [US2] Integration test for pagination boundaries + no-duplicate/no-skip (Scenario B) in `tests/integration/browse_test.go`

### Implementation for User Story 2

- [ ] T041 [US2] Add sqlc keyset list queries `ListRequestsByNew` and `ListRequestsByTop` (with `viewer_has_voted` via votes join) in `db/queries/requests.sql` and run `make sqlc`
- [ ] T042 [P] [US2] Implement opaque keyset cursor encode/decode in `internal/handler/cursor.go`
- [ ] T043 [US2] Implement `RequestService.List` (page-size bound, cursor, deterministic `(…, created_at, id)` ordering) in `internal/service/request.go`
- [ ] T044 [US2] Implement `GET /requests` and `GET /requests/{id}` handlers (cursor params, empty-state, `RequestPage`/`FeatureRequest` encoding) in `internal/handler/requests.go`
- [ ] T045 [US2] Register `/requests` GET and `/requests/{id}` GET routes in `cmd/server/main.go`

**Checkpoint**: Users can submit AND browse. Stories 1 and 2 both work independently.

---

## Phase 5: User Story 3 - Upvote a feature request (Priority: P2)

**Goal**: An authenticated user upvotes another user's request exactly once (idempotent), cannot self-vote, and can remove the upvote — with counts transactionally exact under concurrency and viral spikes.

**Independent Test**: Bob upvotes Alice's request → `vote_count: 1`; repeat → still `1`; Alice self-votes → `403 forbidden_self_vote`; Bob removes → `0`; vote on random UUID → `404` (Scenario C). Spike: 1,000 distinct users → `vote_count == 1000` exactly (Scenario F, SC-003/SC-004).

### Tests for User Story 3 (write first, must FAIL) ⚠️

- [ ] T046 [P] [US3] Unit test for vote rules (self-vote rejection, idempotent dedup, remove) in `internal/service/vote_test.go`
- [ ] T047 [P] [US3] Integration test for upvote/remove/self-vote/not-found (Scenario C) in `tests/integration/vote_test.go`
- [ ] T048 [P] [US3] Integration load test for viral spike correctness — 1,000 concurrent distinct voters, final count exact, reconciliation `vote_count == COUNT(votes)` (Scenario F) in `tests/integration/spike_test.go`
- [ ] T049 [P] [US3] Benchmark for the hot vote-write path in `internal/service/vote_bench_test.go` (per Principle III, before any optimization)

### Implementation for User Story 3

- [ ] T050 [US3] Add atomic vote sqlc queries in `db/queries/votes.sql`: `InsertVote` (`INSERT ... ON CONFLICT DO NOTHING`) + conditional `vote_count` increment; `DeleteVote` + conditional decrement; run `make sqlc`
- [ ] T051 [P] [US3] Define `VoteCounter` interface (Option B evolution seam) + Postgres implementation in `internal/service/vote.go`
- [ ] T052 [US3] Implement `VoteService` with self-vote check (`author_id != user_id`, FR-008), atomic increment/decrement transaction, and `ErrNotFound` handling in `internal/service/vote.go`
- [ ] T053 [P] [US3] Implement Redis count cache + best-effort `ZINCRBY` on `top` post-commit in `internal/repository/redis/votes.go`
- [ ] T054 [US3] Implement `PUT /requests/{id}/vote` and `DELETE /requests/{id}/vote` handlers (idempotent `200`, `403` self-vote, `404`) returning `VoteResult` in `internal/handler/votes.go`
- [ ] T055 [US3] Register vote routes behind JWT middleware and wire `VoteService` in `cmd/server/main.go`

**Checkpoint**: Core voting signal works correctly under concurrency. Stories 1–3 independently functional.

---

## Phase 6: User Story 4 - View Top and Trending rankings (Priority: P3)

**Goal**: An authenticated user switches between "Top" (raw vote count) and "Trending" (time-decay) rankings, with a deterministic tiebreaker and stable ordering across pages.

**Independent Test**: With request X (old votes) and Y (recent votes, equal totals), `sort=top` orders by count desc with stable ties; after one recompute interval, `sort=trending` ranks Y above X (Scenario D, SC-006).

### Tests for User Story 4 (write first, must FAIL) ⚠️

- [ ] T056 [P] [US4] Unit test for the trending score function `vote_count/(age_hours+2)^1.5` and tiebreaker in `internal/ranking/trending_test.go`
- [ ] T057 [P] [US4] Integration test for Top vs Trending ordering + cross-page stability (Scenario D) in `tests/integration/ranking_test.go`

### Implementation for User Story 4

- [ ] T058 [P] [US4] Implement Redis `top`/`trending` ZSET read + Postgres fallback in `internal/repository/redis/ranking.go`
- [ ] T059 [P] [US4] Implement periodic trending recompute job (interval-driven, context-cancellable, no goroutine leak) in `internal/ranking/trending.go`
- [ ] T060 [P] [US4] Implement reconciliation job recomputing `vote_count` from `COUNT(votes)` and healing Redis drift (FR-010) in `internal/ranking/reconcile.go`
- [ ] T061 [US4] Implement `RankingService` selecting top/trending/new with deterministic tiebreaker in `internal/service/ranking.go`
- [ ] T062 [US4] Wire `sort=top|trending|new` into `GET /requests` (route to ranking read path) in `internal/handler/requests.go`
- [ ] T063 [US4] Start trending + reconciliation jobs with shared context and graceful shutdown in `cmd/server/main.go`

**Checkpoint**: Rankings turn raw votes into prioritization. Stories 1–4 independently functional.

---

## Phase 7: User Story 5 - Responsive updates without real-time push (Priority: P3)

**Goal**: The server supports the perceived-live experience via conditional polling — `ETag`/`If-None-Match`/`304` on list reads and `Cache-Control`, so optimistic UI + periodic revalidation reconcile within ≤60 s with zero push infrastructure.

**Independent Test**: `GET /requests?sort=top` returns an `ETag`; re-request with `If-None-Match` and no change → `304`; after a vote, same request → `200` with new ETag and updated counts (Scenario E, SC-005).

### Tests for User Story 5 (write first, must FAIL) ⚠️

- [ ] T064 [P] [US5] Integration test for ETag generation, `304 Not Modified`, and post-vote ETag change (Scenario E) in `tests/integration/conditional_test.go`

### Implementation for User Story 5

- [ ] T065 [US5] Implement deterministic ETag computation for list/page responses and `If-None-Match` → `304` handling in `internal/handler/requests.go`
- [ ] T066 [US5] Add `Cache-Control` headers on list responses to drive revalidation cadence (FR-015) in `internal/handler/requests.go`
- [ ] T067 [P] [US5] Document the optimistic-update + conditional-polling contract for clients in `api/README.md`

**Checkpoint**: All five user stories independently functional; client can feel live via polling.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Hardening, coverage, docs, and full end-to-end validation across all stories.

- [ ] T068 [P] Add `/health` alias for liveness (constitution naming compliance) in `internal/platform/health.go`
- [ ] T069 [P] Backfill unit tests to reach ≥70% coverage on handlers/services/repositories (constitution Principle III)
- [ ] T070 [P] Run `make vuln` (govulncheck) and remediate any findings
- [ ] T071 [P] Update root `README.md` and `docs/` with run/build/test instructions and architecture overview
- [ ] T072 Review pgx pool limits, Redis timeouts, and rate-limit defaults for spike resilience (SC-004)
- [ ] T073 Execute `quickstart.md` Scenarios A–F end-to-end against `docker compose up` and confirm all expected results

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately.
- **Foundational (Phase 2)**: Depends on Setup — BLOCKS all user stories.
- **User Stories (Phases 3–7)**: All depend on Foundational. US1 and US2 are both P1 (MVP). US3 (P2) is logically after US1/US2. US4 and US5 (P3) build on US3's vote data and US2's list path.
- **Polish (Phase 8)**: Depends on all targeted user stories.

### User Story Dependencies

- **US1 (P1)**: After Foundational. No dependency on other stories.
- **US2 (P1)**: After Foundational. Independent of US1 (seeds its own data in tests), though they share `internal/handler/requests.go` and `db/queries/requests.sql`.
- **US3 (P2)**: After Foundational. Mutates `vote_count`; needs requests to exist (uses US1's table, not its code).
- **US4 (P3)**: After Foundational. Consumes vote data; reuses US2's list endpoint for `sort` routing.
- **US5 (P3)**: After Foundational. Adds conditional-GET behavior to US2's list endpoint.

### Within Each User Story

- Tests written first and FAIL before implementation (constitution Principle III).
- sqlc queries / models → services → handlers → route wiring.
- Story complete and independently testable before moving to next priority.

### Parallel Opportunities

- All `[P]` Setup tasks (T003–T010) run in parallel after T001/T002.
- All `[P]` Foundational tasks (T012–T016, T018, T024–T026) run in parallel; T017/T020–T023/T027/T028 have ordering constraints.
- All `[P]` tests within a story run in parallel and before that story's implementation.
- Models/queries marked `[P]` within a story run in parallel.
- With capacity, different stories proceed in parallel after Foundational — coordinate shared files (`internal/handler/requests.go`, `db/queries/requests.sql`, `cmd/server/main.go`).

---

## Parallel Example: User Story 1

```bash
# Launch all US1 tests together (write first, must FAIL):
Task: "Unit test for request validation in internal/service/request_test.go"
Task: "Integration test for submit in tests/integration/submit_test.go"
Task: "E2E test for submit flow in tests/e2e/submit_test.go"

# Then parallelizable implementation pieces:
Task: "Define FeatureRequest domain type in internal/service/request.go"
# (T033 sqlc query precedes the service that uses it)
```

---

## Implementation Strategy

### MVP First

1. Phase 1: Setup → 2. Phase 2: Foundational (CRITICAL) → 3. Phase 3: US1 (Submit) → 4. Phase 4: US2 (Browse).
   US1 + US2 together (both P1) are the minimum browsable idea-capture product.
5. **STOP and VALIDATE** with quickstart Scenarios A & B, then deploy/demo.

### Incremental Delivery

1. Setup + Foundational → foundation ready (`/readyz` green).
2. US1 → submit works → demo.
3. US2 → browse works → demo (MVP complete).
4. US3 → voting works correctly under spikes → demo.
5. US4 → Top/Trending rankings → demo.
6. US5 → conditional polling / perceived-live → demo.
7. Polish → harden, cover, validate end-to-end.

### Parallel Team Strategy

After Foundational: Dev A takes US1, Dev B takes US2 (coordinate `requests.go`/`requests.sql`), then US3 (highest-risk concurrency path) gets a focused owner before US4/US5 layer on top.

---

## Notes

- `[P]` = different files, no dependency on incomplete tasks.
- `[Story]` label maps each task to a user story for traceability.
- Every error checked and wrapped with `%w`; sentinel errors for domain failures (Principle II).
- `context.Context` first param on all I/O; jobs cancellable, no leaks (Principle IV).
- Verify tests FAIL before implementing; commit after each task or logical group.
- Stop at any checkpoint to validate a story independently.
