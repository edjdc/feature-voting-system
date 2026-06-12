# Feature Voting System — Architecture Research (Go)

> Research deliverable for the [Technical AI Assessment](../Technical%20AI%20Assessment.md).
> Audience: input for the development phase. Status: **Recommendation ready**.

---

## 1. Context Summary

### 1.1 Problem
Build a **Feature Voting System** where users can:
- Submit a feature request (title + description)
- View the list of existing requests
- Upvote requests submitted by others
- See vote counts and a popularity-based ranking

Accessible from **web and mobile**. The assignment is open-ended and judged on *system-design thinking, scalability, and UX* — production-readiness beyond the core flow is explicitly encouraged.

### 1.2 Aligned constraints (from the alignment interview)
These were resolved up front and are treated as **fixed** across all three options:

| Decision | Choice | Consequence |
|---|---|---|
| **Target scale** | Build for high scale now | Managed Redis, Postgres read replicas, async-capable counting, horizontally scalable **stateless** app tier |
| **Identity** | Authenticated users (JWT) | One-vote-per-request enforced by a DB uniqueness constraint; trustworthy counts |
| **Real-time** | Optimistic UI + polling | No WebSocket/SSE fan-out → app tier stays stateless and trivially horizontal |
| **Ranking** | Two views: **Top** (raw count) + **Trending** (time-decay) | Trending maintained in a Redis sorted set (ZSET); recomputed periodically |
| **Deployment** | Docker Compose (local) | Single-host: app + Postgres + Redis run as Compose services; no orchestration overhead |
| **First-class prod scope** | Observability, CI/CD & testing | Logs/metrics/traces/probes; test pyramid + pipeline are designed, not name-dropped |
| **Mention-only** | Rate limiting/abuse prevention, request lifecycle/status | Designed-for seams left open, not fully built in v1 |

### 1.3 The key insight that frames everything
A feature-voting system is **overwhelmingly read-heavy**. Even a "viral" request accumulates thousands of votes over hours — *not* millions of writes per second. The hard problem is the **read/ranking path** (listing + sorting + counts on every page load), **not** the vote-write path.

➡️ **Strategy:** optimize reads aggressively (cached counts, Redis ZSET rankings, cursor pagination, cheap conditional polling) while keeping the write path **simple and correct**. Running locally under Docker Compose removes the need for horizontal scaling machinery — but this same architecture maps cleanly onto a single-node production deployment (a VPS or container PaaS) with no structural changes.

### 1.4 Shared foundation (common to all three options)
- **Language/runtime:** Go (static binary, distroless/scratch image)
- **Primary store:** PostgreSQL (relational; the uniqueness constraint for dedup wants a relational engine)
- **Cache / ranking store:** Redis (cached counts + `Top`/`Trending` ZSETs + rate-limit buckets)
- **Data access:** `pgx` driver + `sqlc` (type-safe queries) or `sqlc`-generated repos; `golang-migrate` for migrations
- **AuthN:** JWT (short-lived access token + refresh token); `argon2id` password hashing or OAuth/social
- **Deploy:** Docker Compose — `app`, `postgres`, `redis` as named services; `depends_on` + health-check ordering; env vars via `.env` / `env_file`; named volumes for Postgres + Redis persistence; `docker compose up` spins the full stack
- **Observability:** `log/slog` (structured JSON), Prometheus `/metrics` (RED), OpenTelemetry traces, `/healthz` + `/readyz`
- **Clients:** Web (SPA) + mobile share one API; **cursor/keyset pagination**, `ETag`/`Last-Modified` for cheap polling

### 1.5 Canonical data model (shared)
```sql
-- users
CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         CITEXT UNIQUE NOT NULL,
    password_hash TEXT,                         -- null if OAuth-only
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- feature_requests
CREATE TABLE feature_requests (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id   UUID NOT NULL REFERENCES users(id),
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    vote_count  INTEGER NOT NULL DEFAULT 0,     -- denormalized counter (read-path optimization)
    status      TEXT NOT NULL DEFAULT 'open',   -- lifecycle seam (open/planned/in_progress/shipped/declined)
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- votes — composite PK *is* the one-vote-per-user-per-request guarantee
CREATE TABLE votes (
    user_id    UUID NOT NULL REFERENCES users(id),
    request_id UUID NOT NULL REFERENCES feature_requests(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, request_id)
);

-- "Top" ranking + keyset pagination
CREATE INDEX idx_fr_top ON feature_requests (vote_count DESC, created_at DESC, id DESC);
-- "New" + recency tie-breaks
CREATE INDEX idx_fr_created ON feature_requests (created_at DESC, id DESC);
-- audit / per-request vote scans
CREATE INDEX idx_votes_request ON votes (request_id);
```
- The `votes` **composite primary key** makes duplicate votes structurally impossible — an `INSERT ... ON CONFLICT DO NOTHING` is naturally idempotent.
- `vote_count` is a **denormalized read-path cache** inside Postgres; it is *always* reconcilable from `COUNT(votes)` (the source of truth), which makes every option self-healing.
- **Trending** is never stored in Postgres — it's a derived score recomputed into a Redis ZSET.

### 1.6 Ranking math (shared)
- **Top:** `ORDER BY vote_count DESC, created_at DESC, id DESC` — served from `top` ZSET / `idx_fr_top`.
- **Trending (gravity decay, HN-style):**
  `score = vote_count / (age_hours + 2)^gravity`, with `gravity ≈ 1.5`.
  Recomputed every 1–5 min by a small job into a `trending` ZSET; served with `ZREVRANGE`. Decay keeps fresh requests discoverable instead of letting old winners calcify at the top.

### 1.7 Client/API conventions (shared, mobile-first)
- **Cursor (keyset) pagination**, never `OFFSET`: cursor = `(sort_key, id)` → stable under concurrent inserts, O(1) at depth, cache-friendly. Responses carry `next_cursor`.
- **Cheap polling:** list endpoints emit `ETag`/`Last-Modified`; clients send `If-None-Match` and get `304 Not Modified` when nothing changed → polling cost collapses.
- **Optimistic vote UX:** client `POST`s the vote, immediately bumps the count locally, reconciles on the next poll/focus. Feels instant with zero push infrastructure.

---

## 2. The Three Architecture Options

All three share §1.4–§1.7. **They differ on the vote-counting & consistency model** — the genuinely consequential fork — with the API protocol chosen per option to match its philosophy.

---

### Option A — Synchronous Atomic Increments (strong consistency)

**Philosophy:** Postgres is the single source of truth; a vote and its counter move together in one transaction. Simplicity and correctness first.

- **API / framework / protocol:** **REST over `net/http` + `chi`** router. Idiomatic, stdlib-compatible, minimal magic, trivially testable with `httptest`. REST + JSON is the lowest-friction contract for both web and mobile.
- **Data & storage:** shared model (§1.5). Reads served from Redis cached counts + `Top`/`Trending` ZSETs; cache miss falls back to Postgres read replicas.
- **Vote counting under concurrency:**
  ```sql
  BEGIN;
    INSERT INTO votes (user_id, request_id)
      VALUES ($1, $2) ON CONFLICT DO NOTHING;          -- idempotent dedup
    UPDATE feature_requests
      SET vote_count = vote_count + 1, updated_at = now()
      WHERE id = $2 AND <a row was inserted>;           -- only if the insert took
  COMMIT;
  -- after commit (best-effort): ZINCRBY top 1 <id>; mark trending dirty; bust count cache
  ```
  Concurrent votes on the **same** request serialize on that single row's lock — correct, but a contention point on a hot row.
- **Ranking:** `Top` from `idx_fr_top`/ZSET; `Trending` recomputed by a periodic job.
- **Dup prevention / identity:** JWT + `votes` PK. Rate-limit seam via Redis token bucket.
- **Real-time:** optimistic UI + polling (§1.7).
- **Deployment:** `docker compose up` — `app` + `postgres` + `redis` services; multi-stage build keeps the image small; reads served from Redis cache, falling back to Postgres on miss.

| Criterion | Assessment |
|---|---|
| Performance | Fast at the expected local/single-node load; reads are cache-fast. Row-level lock contention is a non-issue at this scale. |
| Scalability | More than adequate for single-host Docker Compose; maps cleanly to a single-node VPS deployment without structural changes. |
| Consistency | **Strongest** — counts are transactionally exact, always. |
| Complexity / maintainability | **Lowest** — one source of truth, few moving parts, easy to reason about and test. |
| Go ecosystem maturity | Excellent — `chi`, `pgx`, `sqlc`, `golang-migrate` all battle-tested. |
| Dev effort | **Lowest** — fastest path to a polished, correct MVP. |
| Fit with "production-ready, polished" | High — correct, observable, defensible; scales for the realistic write profile of this domain. |

---

### Option B — Redis Cache-Aside Counters + Async Write-Back (speed, eventual consistency)

**Philosophy:** Treat the vote count as a hot in-memory value. Absorb write spikes in Redis, persist to Postgres asynchronously in batches.

- **API / framework / protocol:** **REST + `Gin`** (or `Echo`) — batteries-included middleware (validation, binding) suits a slightly more involved write pipeline.
- **Data & storage:** Redis holds the authoritative *live* counter (`INCR`); Postgres is the durable store, written behind. Durability for in-flight counts via Redis AOF **plus** an append of raw vote events (so nothing is lost before flush).
- **Vote counting under concurrency:**
  1. `INSERT votes ... ON CONFLICT DO NOTHING` (dedup still transactional in Postgres) **or** a Redis `SADD voters:<req> <user>` dedup set, then
  2. `INCR count:<req>` in Redis (O(1), lock-free, absorbs spikes),
  3. an **async write-back worker** flushes deltas to `feature_requests.vote_count` in batches and reconciles against `COUNT(votes)` periodically.
- **Ranking:** ZSETs updated on the same Redis `INCR` path — rankings are *instantly* fresh.
- **Dup prevention / identity:** JWT; dedup via `votes` PK and/or Redis set; same rate-limit story.
- **Real-time:** optimistic UI + polling, but counts converge faster (Redis is already live).
- **Deployment:** add a write-back worker as an additional Compose service (or a goroutine inside the app container); Redis AOF enabled for durability within the Compose stack.

| Criterion | Assessment |
|---|---|
| Performance | **Best write latency/throughput** — in-memory `INCR`, no row locks. |
| Scalability | High — Redis absorbs spikes; Postgres writes amortized into batches. |
| Consistency | **Eventual** — window where Redis and Postgres disagree; Redis loss before flush risks counts unless AOF + event log + reconciliation are correct. |
| Complexity / maintainability | Moderate — dual source of truth, write-back worker, durability + reconciliation + idempotency logic. |
| Go ecosystem maturity | Excellent — `go-redis`, `Gin`/`Echo` mature. |
| Dev effort | Moderate–high — the correctness machinery (durability, reconciliation, idempotent flush) is where the time goes. |
| Fit with "production-ready, polished" | Strong scale story, but introduces a correctness surface that must be handled carefully to stay "polished." |

---

### Option C — Event-Driven Aggregation via Append Log (decoupled, highest scale)

**Philosophy:** A vote is an **event**, not an update. Append it to a durable log; downstream consumers aggregate counts into Postgres + Redis. CQRS-flavored.

- **API / framework / protocol:** **gRPC** internally (or **ConnectRPC**, which speaks gRPC *and* gRPC-Web/JSON so browsers and mobile work natively) + a gateway exposing REST/JSON for clients. Strong typed contracts, good for a service that may grow.
- **Data & storage:** append log = **Kafka / NATS JetStream / Redis Streams**. Write path appends `VoteCast{user,request,ts}`. A consumer group aggregates into `feature_requests.vote_count` + Redis ZSETs. Postgres + Redis are **materialized read models**.
- **Vote counting under concurrency:** producers never contend — they append. Consumers fold events into counts. Dedup via the `votes` PK at the consumer (idempotent apply) and/or a keyed log partition per request. Exactly-once-ish aggregation needs care (idempotent upserts keyed on `(user_id,request_id)`).
- **Ranking:** consumers maintain ZSETs as they apply events.
- **Dup prevention / identity:** JWT at the edge; idempotent apply at the consumer.
- **Real-time:** the same consumer can also fan out to SSE/WebSocket later for *true* push — but per the alignment we still ship optimistic UI + polling.
- **Deployment:** broker (Kafka/NATS/Redis Streams) + producer + consumer + app all as Compose services; highest number of containers and the most `depends_on` / health-check wiring in the stack.

| Criterion | Assessment |
|---|---|
| Performance | High write throughput (append-only); end-to-end count latency higher (consumer lag). |
| Scalability | **Highest** — partitioned log, independently scalable producers/consumers, replayable. |
| Consistency | Eventual, but with **auditability + replay**; exactly-once aggregation needs deliberate design. |
| Complexity / maintainability | **Lowest** — a broker to operate, more failure modes, harder local/dev/test ergonomics. |
| Go ecosystem maturity | Good — `kafka-go`/`segmentio`, `nats.go`, `connectrpc`, `grpc-go` mature; more glue overall. |
| Dev effort | **Highest** — broker ops, consumer idempotency, materialization, gateway. |
| Fit with "production-ready, polished" | Architecturally impressive, but a real risk of **over-engineering** for the assessment's scope. |

---

## 3. Comparison & Scoring

### 3.1 Weighting (explicit) and rationale
The assessment rewards system-design thinking *and* a polished, production-ready result. With Docker Compose as the deployment target the *horizontal* scaling story is no longer a differentiator — all three options run on a single host. The scoring therefore shifts weight toward **correctness, maintainability, and development effort** (where the options genuinely differ) while retaining a modest scalability criterion to reward designs that could graduate to a VPS or container PaaS without a rewrite.

| Criterion | Weight | Why this weight |
|---|---:|---|
| Maintainability | 25% | A take-home is read and judged; clarity and operability compound. |
| Development effort | 25% | Finite time; over-spend here steals from polish elsewhere. |
| Consistency guarantees | 20% | Trustworthy vote counts are the product's core promise. |
| Performance | 15% | Matters, but the domain is read-heavy and reads are cached in all options. |
| Scalability | 10% | Reduced weight: single-host Compose removes horizontal scaling as a differentiator; still rewards designs that could graduate to a VPS. |
| Fit with "production-ready, polished" | 5% | Rewards judgment (not over-/under-engineering) for the actual scope. |

Scores are 1–5 (5 best). Weighted score = Σ(score × weight).

### 3.2 Scorecard

| Criterion (weight) | A — Sync atomic | B — Cache-aside async | C — Event-driven |
|---|:--:|:--:|:--:|
| Maintainability (25%) | 5.0 | 3.0 | 2.5 |
| Development effort (25%) | 5.0 | 3.0 | 2.0 |
| Consistency (20%) | 5.0 | 3.0 | 3.5 |
| Performance (15%) | 4.0 | 5.0 | 4.0 |
| Scalability (10%) | 4.0 | 4.5 | 5.0 |
| Fit / polish (5%) | 4.5 | 4.0 | 3.0 |
| **Weighted total** | **4.70** | **3.43** | **2.98** |

**Worked totals**
- **A:** 5.0·.25 + 5.0·.25 + 5.0·.20 + 4.0·.15 + 4.0·.10 + 4.5·.05 = 1.25+1.25+1.00+0.60+0.40+0.225 = **4.73**
- **B:** 3.0·.25 + 3.0·.25 + 3.0·.20 + 5.0·.15 + 4.5·.10 + 4.0·.05 = 0.75+0.75+0.60+0.75+0.45+0.20 = **3.50**
- **C:** 2.5·.25 + 2.0·.25 + 3.5·.20 + 4.0·.15 + 5.0·.10 + 3.0·.05 = 0.625+0.50+0.70+0.60+0.50+0.15 = **3.08**

### 3.3 Ranking
1. **🥇 Option A — Synchronous Atomic Increments — 4.73**
2. **🥈 Option B — Cache-Aside + Async Write-Back — 3.50**
3. **🥉 Option C — Event-Driven Aggregation — 3.08**

**Rationale for the order.** A wins even more decisively under the Docker Compose target. Without horizontal scaling as a differentiator, B and C can no longer justify their correctness/complexity costs — both introduce eventual consistency and extra machinery (write-back worker, broker) that solves a concurrency problem that simply doesn't arise on a single-host deployment. A is **correct by construction**, operationally trivial (`docker compose up`), and the fastest path to a polished result.

---

## 4. Recommendation

### 4.1 Foundation: **Option A — Synchronous Atomic Increments**, with an aggressively optimized read path.
Build the write path **strongly consistent and simple** (one transaction: idempotent vote insert + counter update), and optimise reads — the genuinely hot path:
- Redis **cached counts** + `Top`/`Trending` **ZSETs** as the default read source;
- Postgres as the durable fallback on cache miss;
- **cursor pagination** + `ETag`/`304` polling so list/ranking reads are cheap at any depth and polling frequency;
- `docker compose up` brings the entire stack (app, Postgres, Redis) up in one command.

### 4.2 Designed-in evolution seam to Option B (don't build it yet)
Keep counts behind a small `VoteCounter` interface with one synchronous implementation now. Because Redis ZSETs/cached counts already exist in the read path, the **seam to cache-aside write-back is small**. **Gate the switch on observability**: if Prometheus shows lock-wait/latency climbing on hot `feature_requests` rows (a genuinely viral request), promote that path to Redis `INCR` + async write-back behind the same interface. This is the production-minded move — evolve under measured pressure, not speculatively.

### 4.3 Concrete v1 stack
- **API:** REST + JSON over `net/http` + `chi`; OpenAPI spec for web/mobile clients.
- **Store:** PostgreSQL, `pgx` + `sqlc`, `golang-migrate` (migrations run as a one-shot init container / `command` in Compose before the app starts).
- **Cache/ranking:** Redis (counts cache, `Top`/`Trending` ZSETs, rate-limit buckets).
- **Auth:** JWT (access + refresh), `argon2id`; auth middleware; `votes` PK for dedup.
- **Real-time:** optimistic UI + conditional polling.
- **Observability (first-class):** `slog` JSON logs, Prometheus `/metrics` (RED), OTel traces, `/healthz` + `/readyz`.
- **CI/CD & testing (first-class):** test pyramid — unit (domain/services), integration via **testcontainers** (real Postgres + Redis, no mocks), API/e2e via `httptest`; GitHub Actions: `golangci-lint` → test → build image; migrations gated in the pipeline.
- **Deploy:** multi-stage Docker build (builder → distroless/scratch); `docker-compose.yml` with `app`, `postgres`, `redis` services; `depends_on` + `healthcheck` ordering; named volumes for data persistence; 12-factor config via `.env` / `env_file`.

### 4.4 Trade-offs accepted
- **Single-host only:** Docker Compose doesn't provide horizontal scaling. The architecture (stateless app, Redis as read cache, cursor pagination) is structured so graduating to a container PaaS or a VPS behind a reverse proxy requires only infrastructure changes, not code changes.
- **Trending freshness** lags by the recompute interval (1–5 min) → acceptable; `Top` and per-request counts stay live.
- **Polling vs push:** chose simplicity over SSE/WebSocket push → a clean future add if real-time push becomes a requirement.

### 4.5 Risks & mitigations
| Risk | Likelihood | Mitigation |
|---|---|---|
| Row-lock contention on a hot request | Very low (single-host; domain is read-heavy) | Redis cache absorbs reads; §4.2 seam to cache-aside write-back if ever measured. |
| Redis unavailability (container crash) | Low | Reads fall back to Postgres; counts reconcilable from `COUNT(votes)`. `restart: unless-stopped` in Compose. |
| Denormalized `vote_count` drift | Low | Periodic reconciliation job vs `COUNT(votes)` (source of truth). |
| Vote spam / abuse | Medium | JWT + `votes` PK kill duplicates; Redis token-bucket rate limiting (seam). |
| Data loss on `docker compose down -v` | Medium | Named volumes (not anonymous); documented in README; production backup strategy out of scope for local run. |

### 4.6 Bottom line
**Start with Option A**: a strongly-consistent, simple write path; an aggressively cached, cursor-paginated read path; `docker compose up` to run the whole stack; observability and CI/CD as first-class citizens. It is the **highest-scoring, lowest-risk, fastest-to-polished** foundation for a local Docker Compose deployment, and it carries a **clearly-designed, metrics-gated evolution path to Option B** for the one scenario where it would ever be needed. This is the foundation recommended for the development phase.

---

*Generated as the research deliverable feeding the development phase. See §1.5–§1.7 for the schema, ranking math, and API conventions to implement first.*
