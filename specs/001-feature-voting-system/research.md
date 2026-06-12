# Phase 0 Research: Feature Voting System

**Plan**: [plan.md](./plan.md) | **Date**: 2026-06-12

This document consolidates the technical decisions for v1. The architecture fork
(synchronous vs. async vs. event-driven) was already resolved upstream in
[docs/research/feature-voting-architecture.md](../../docs/research/feature-voting-architecture.md);
**Option A — Synchronous Atomic Increments** is taken as definitive. No
`NEEDS CLARIFICATION` markers remain; the items below record the remaining
implementation-level choices and their rationale.

---

## Decision 1: Vote-counting & consistency model — Synchronous Atomic Increments (Option A)

- **Decision**: Postgres is the single source of truth. A vote insert and the
  denormalized `vote_count` update occur in one transaction.
- **Rationale**: Highest-scoring option (4.73) in the upstream study under a
  single-host Docker Compose target. Correct by construction, lowest dev effort
  and complexity, strongest consistency — directly satisfies FR-007, FR-010,
  SC-003, SC-004. The domain is read-heavy; row-lock contention on a hot request
  is a non-issue at this scale.
- **Alternatives considered**: Option B (Redis cache-aside + async write-back) —
  rejected: introduces eventual consistency and reconciliation machinery for a
  concurrency problem that does not arise single-host. Option C (event-driven log)
  — rejected: broker ops and consumer idempotency are over-engineering for scope.
  Per the user directive, B/C are **not re-evaluated**; a metrics-gated seam to B
  is preserved (Decision 9) but not built.

## Decision 2: Vote transaction shape

- **Decision**: Single transaction:
  ```sql
  BEGIN;
    INSERT INTO votes (user_id, request_id) VALUES ($1, $2)
      ON CONFLICT DO NOTHING;                       -- idempotent dedup via composite PK
    UPDATE feature_requests
      SET vote_count = vote_count + 1, updated_at = now()
      WHERE id = $2 AND <insert affected a row>;     -- only increment if the vote was new
  COMMIT;
  -- after commit, best-effort: ZINCRBY top 1 <id>; mark trending dirty; bust count cache
  ```
  Detect "insert affected a row" via `INSERT ... ON CONFLICT DO NOTHING RETURNING`
  (a returned row means it was new) or the command tag's rows-affected count.
  **Vote removal** is the mirror: `DELETE FROM votes WHERE (user_id,request_id)=($1,$2)`
  and, only if a row was deleted, `vote_count = vote_count - 1` plus a best-effort
  `ZINCRBY top -1`.
- **Rationale**: The composite PK makes duplicate votes structurally impossible
  (FR-007, edge case "concurrent double-submit"). Conditional increment guarantees
  the count moves exactly once per distinct user (FR-010). Self-vote (FR-008) is
  enforced in the service layer before the tx (reject when `author_id == user_id`),
  keeping the SQL minimal. Concurrent votes on the same request serialize on that
  one row's lock — correct, acceptable at scale.
- **Alternatives considered**: Trigger-maintained counters (rejected: hides logic
  from the service layer, harder to test per Principle V); `COUNT(votes)` on every
  read (rejected: defeats the read-path optimization, though it remains the
  reconciliation source of truth).

## Decision 3: Read path — Redis cache + ZSET rankings, Postgres fallback

- **Decision**: Serve counts and rankings from Redis: a per-request count cache,
  a `top` ZSET (score = `vote_count`), and a `trending` ZSET. On cache miss, fall
  back to Postgres (`idx_fr_top` / `idx_fr_created`) and backfill Redis.
- **Rationale**: Reads are the hot path. ZSETs give O(log N) ranked reads; cached
  counts collapse per-request lookups. Postgres stays authoritative and
  reconcilable, so Redis loss is non-fatal (reads fall back; counts rebuild from
  `COUNT(votes)`).
- **Alternatives considered**: Postgres-only reads (rejected: misses the explicit
  read-optimization mandate); materialized views (rejected: refresh latency and
  operational weight exceed the ZSET approach for this scope).

## Decision 4: Trending ranking math & recompute job

- **Decision**: Gravity decay (HN-style): `score = vote_count / (age_hours + 2)^1.5`.
  A periodic job recomputes scores into the `trending` ZSET every 1–5 minutes
  (configurable). Served with `ZREVRANGE`.
- **Rationale**: Satisfies FR-012 / SC-006 — recent activity outranks equally-voted
  but older requests. Decay keeps fresh requests discoverable. Recompute cadence is
  well within the ≤60 s reconciliation budget for *counts* (SC-005); trending order
  is explicitly allowed to lag by the recompute interval (accepted trade-off).
- **Alternatives considered**: Per-vote score recompute (rejected: write
  amplification on the hot path); exponential time-bucketed decay (rejected: more
  state for negligible quality gain at this scope). Gravity `1.5` is a tunable
  constant, not user-facing.

## Decision 5: Deterministic ranking tiebreakers & keyset pagination

- **Decision**: **Top** orders by `(vote_count DESC, created_at DESC, id DESC)`
  (backed by `idx_fr_top`). **New** orders by `(created_at DESC, id DESC)`
  (`idx_fr_created`). Pagination is keyset/cursor — cursor encodes the sort tuple
  `(sort_key..., id)`, never `OFFSET`. Responses carry an opaque `next_cursor`.
  For **Trending**, the ZSET member set is paged by `(score, id)`; ties broken by
  `id` so paging is stable.
- **Rationale**: Satisfies FR-013, SC-007 (every item exactly once across pages)
  and the "ranking ties" / "pagination boundaries" edge cases. Keyset is O(1) at
  depth and stable under concurrent inserts, unlike `OFFSET`.
- **Alternatives considered**: `OFFSET/LIMIT` (rejected: O(depth), unstable under
  concurrent inserts, duplicates/skips at page boundaries).

## Decision 6: Cheap polling — ETag / 304

- **Decision**: List endpoints emit `ETag` (and `Last-Modified`); clients send
  `If-None-Match` and receive `304 Not Modified` when nothing changed. ETag derived
  from a cheap version signal (e.g., a Redis per-list version counter or a hash of
  the page's `(id, vote_count)` tuples).
- **Rationale**: Satisfies FR-015 / SC-005 — periodic revalidation with no push
  channel. `304` responses make frequent polling nearly free.
- **Alternatives considered**: WebSocket/SSE push (rejected by upstream alignment —
  keeps the app tier stateless and trivially horizontal); unconditional re-fetch
  (rejected: wasteful under polling).

## Decision 7: Authentication — JWT (access + refresh) + argon2id

- **Decision**: JWT access (short-lived) + refresh tokens; passwords hashed with
  `argon2id`. Auth middleware validates the access token on every endpoint (FR-001,
  no anonymous path). User identity from the token drives one-vote-per-user and
  no-self-vote rules.
- **Rationale**: Stateless verification fits the horizontal-by-default app tier;
  `argon2id` is the current best-practice password KDF. Refresh tokens allow short
  access-token lifetimes without forcing frequent re-login.
- **Alternatives considered**: Server-side sessions (rejected: stateful, against
  the stateless-tier goal); bcrypt (acceptable, but `argon2id` is the stronger,
  research-specified choice).

## Decision 8: Data access — pgx + sqlc + golang-migrate

- **Decision**: `pgx` driver with `sqlc`-generated type-safe queries; schema via
  `golang-migrate` SQL migrations run as a one-shot Compose `command`/init step
  before the app starts. Parameterized placeholders only.
- **Rationale**: Constitution-approved stack; `sqlc` gives compile-time-checked,
  injection-safe queries (Principle V) with no ORM runtime cost. Migrations gated
  in CI.
- **Alternatives considered**: `database/sql` + hand-rolled queries (rejected: more
  boilerplate, weaker type safety); an ORM such as GORM (rejected: hidden queries,
  against the "clarity over cleverness" principle).

## Decision 9: Evolution seam to Option B (designed, not built)

- **Decision**: Counts sit behind a small `VoteCounter` interface with one
  synchronous implementation. Promotion to Redis `INCR` + async write-back is gated
  on Prometheus showing lock-wait/latency climbing on hot `feature_requests` rows.
- **Rationale**: Production-minded — evolve under measured pressure, not
  speculatively. Because Redis ZSETs/cached counts already exist, the seam is small.
- **Alternatives considered**: Building Option B now (rejected by directive and by
  the upstream scoring — unjustified complexity single-host).

## Decision 10: Observability, testing, deployment

- **Observability**: `log/slog` JSON; Prometheus `/metrics` (RED: Rate, Errors,
  Duration); OpenTelemetry traces on all I/O; `/healthz` (liveness) + `/readyz`
  (readiness, checks PG + Redis). Satisfies Principle VI.
- **Testing**: unit (table-driven, services/domain) → integration via
  `testcontainers-go` against **real** Postgres + Redis (`//go:build integration`)
  → API/e2e via `httptest`. `go test -race ./...` in CI; ≥70% on critical paths.
  Real dependencies (no mocks) for integration matches the upstream mandate.
- **Deployment**: multi-stage Dockerfile (builder → distroless, non-root,
  `CGO_ENABLED=0` static binary, pinned base); `docker-compose.yml` with `app`,
  `postgres`, `redis`, `healthcheck` + `depends_on` ordering, named volumes for
  persistence, `.env`/`env_file` for 12-factor config.
- **Rationale**: Each maps directly to a constitution requirement and to the v1
  stack the user specified. No open questions.

---

## Resolved unknowns summary

| Topic | Resolution |
|-------|------------|
| Consistency model | Synchronous atomic increment (Option A) — fixed |
| Self-vote enforcement | Service-layer check before tx (FR-008) |
| Vote removal | Mirror transaction; eligible to re-vote (FR-009) |
| Trending formula | `vote_count / (age_hours + 2)^1.5`, recompute 1–5 min |
| Tiebreakers | `(sort_key, created_at, id)` deterministic; keyset cursor |
| Polling freshness | ETag / 304 conditional revalidation, ≤60 s |
| Auth | JWT access+refresh, argon2id, every endpoint |
| Data access | pgx + sqlc + golang-migrate, parameterized |
| Scale seam | `VoteCounter` interface, metrics-gated to Option B |

**Phase 0 gate**: ✅ All decisions resolved. Proceed to Phase 1.
