# Phase 1 Data Model: Feature Voting System

**Plan**: [plan.md](./plan.md) | **Research**: [research.md](./research.md) | **Date**: 2026-06-12

Schema per research §1.5 (canonical, shared). PostgreSQL is the source of truth;
Redis holds derived read-path state (cached counts + ranking ZSETs) that is always
reconcilable from Postgres.

---

## Entities

### User
An authenticated participant who can submit feature requests and cast/remove upvotes.
The authority for "one vote per user" and "no self-vote" rules.

| Field | Type | Constraints / Notes |
|-------|------|---------------------|
| `id` | UUID | PK, `DEFAULT gen_random_uuid()` |
| `email` | CITEXT | `UNIQUE NOT NULL` (case-insensitive) |
| `password_hash` | TEXT | argon2id encoded hash; nullable if OAuth-only |
| `created_at` | TIMESTAMPTZ | `NOT NULL DEFAULT now()` |

### Feature Request
A proposed idea with a title, description, author, creation time, and a denormalized
current vote count. The unit that is browsed, ranked, and voted on.

| Field | Type | Constraints / Notes |
|-------|------|---------------------|
| `id` | UUID | PK, `DEFAULT gen_random_uuid()` |
| `author_id` | UUID | `NOT NULL REFERENCES users(id)` |
| `title` | TEXT | `NOT NULL`; non-empty, length-limited (validated at handler) |
| `description` | TEXT | `NOT NULL DEFAULT ''`; non-empty on submit, length-limited |
| `vote_count` | INTEGER | `NOT NULL DEFAULT 0`; denormalized read-path counter, reconcilable from `COUNT(votes)` |
| `created_at` | TIMESTAMPTZ | `NOT NULL DEFAULT now()` |
| `updated_at` | TIMESTAMPTZ | `NOT NULL DEFAULT now()`; bumped on vote count change |

**Validation (FR-003)**: `title` and `description` must be non-empty (not
whitespace-only) and within defined length limits; reject with actionable message.
Enforced at the handler boundary (Principle VI), not only by the DB.

### Vote
A record that a specific user holds an **active** upvote on a specific feature
request. At most one per (user, request) pair; its recency feeds Trending.

| Field | Type | Constraints / Notes |
|-------|------|---------------------|
| `user_id` | UUID | `NOT NULL REFERENCES users(id)`; part of composite PK |
| `request_id` | UUID | `NOT NULL REFERENCES feature_requests(id) ON DELETE CASCADE`; part of composite PK |
| `created_at` | TIMESTAMPTZ | `NOT NULL DEFAULT now()` |

**Composite PK `(user_id, request_id)`** *is* the one-vote-per-user-per-request
guarantee — duplicate votes are structurally impossible (FR-007). Removing a vote
deletes the row (FR-009); only the current active state is tracked, no toggle
history (spec Assumptions).

---

## Relationships

```text
users (1) ──< feature_requests (author_id)        one user authors many requests
users (1) ──< votes (user_id)                      one user casts many votes
feature_requests (1) ──< votes (request_id)        one request receives many votes
                         ON DELETE CASCADE          deleting a request removes its votes
```

`votes` is the join between `users` and `feature_requests` carrying the
many-to-many "who upvoted what", with the composite PK enforcing uniqueness.

---

## DDL (authoritative — golang-migrate `0001_init.up.sql`)

```sql
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         CITEXT UNIQUE NOT NULL,
    password_hash TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE feature_requests (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id   UUID NOT NULL REFERENCES users(id),
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    vote_count  INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE votes (
    user_id    UUID NOT NULL REFERENCES users(id),
    request_id UUID NOT NULL REFERENCES feature_requests(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, request_id)
);

-- "Top" ranking + keyset pagination
CREATE INDEX idx_fr_top     ON feature_requests (vote_count DESC, created_at DESC, id DESC);
-- "New" + recency tie-breaks
CREATE INDEX idx_fr_created ON feature_requests (created_at DESC, id DESC);
-- audit / per-request vote scans
CREATE INDEX idx_votes_request ON votes (request_id);
```

`0001_init.down.sql` drops `votes`, `feature_requests`, `users` (reverse order).

---

## State transitions

**Vote (per user × request)** — two states only; no history retained:

```text
                 upvote (INSERT ON CONFLICT DO NOTHING + count+1)
   NOT VOTED  ───────────────────────────────────────────────▶  ACTIVE VOTE
        ▲                                                              │
        └──────────────────────────────────────────────────────────--┘
                 remove  (DELETE + count-1; eligible to re-vote)   (FR-009)
```

- Duplicate upvote while `ACTIVE` → no-op (count unchanged) — FR-007, scenario US3-2.
- Self-vote attempt (`author_id == user_id`) → rejected, no state change — FR-008.
- Upvote on a removed request → fails gracefully, no orphan vote — FR-018.

**Feature Request**: created → (vote_count mutates with votes) → may be removed
(cascades to its votes). Editing/deleting content is out of scope for v1 (spec
Assumptions); `vote_count` is the only field that changes after creation.

---

## Derived read-path state (Redis — not authoritative)

| Key | Type | Purpose | Maintenance |
|-----|------|---------|-------------|
| `count:<request_id>` | string (int) | cached `vote_count` | busted on vote tx commit; backfilled on read miss |
| `top` | ZSET | Top ranking, score = `vote_count` | `ZINCRBY` best-effort post-commit; rebuildable from `idx_fr_top` |
| `trending` | ZSET | Trending ranking, score = `vote_count/(age_hours+2)^1.5` | recomputed by periodic job every 1–5 min |
| `ratelimit:<user_id>:<window>` | string (counter) | token-bucket rate limiting | per-request, TTL-expired (seam) |

**Reconciliation (FR-010)**: a periodic job recomputes `vote_count` from
`COUNT(votes)` (source of truth) and refreshes Redis, healing any drift from
best-effort cache busts.

---

## Requirement → model traceability

| Requirement | Enforced by |
|-------------|-------------|
| FR-002 submit (title, description, author, created_at) | `feature_requests` columns |
| FR-003 validation | handler-boundary validation + `NOT NULL` |
| FR-005 display vote count | `feature_requests.vote_count` / Redis `count:` |
| FR-007 one vote per user | `votes` composite PK |
| FR-008 no self-vote | service check `author_id != user_id` |
| FR-009 remove upvote | `DELETE FROM votes` + conditional decrement |
| FR-010 count == distinct active voters | conditional tx update + reconciliation job |
| FR-011 Top ranking | `idx_fr_top` / `top` ZSET |
| FR-012 Trending ranking | `trending` ZSET + recompute job |
| FR-013 deterministic tiebreaker | index sort tuple `(…, created_at, id)` + keyset cursor |
| FR-018 graceful on removed request | FK + `ON DELETE CASCADE`, `ErrNotFound` |
