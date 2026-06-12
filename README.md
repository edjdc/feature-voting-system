# Feature Voting System

An authenticated REST/JSON API for submitting, browsing, and voting on feature requests. Built with Go, PostgreSQL, and Redis. Implements atomic vote counting, keyset pagination, Top/Trending rankings, and ETag-based conditional polling.

## Architecture

**Option A — Synchronous Atomic Increments**: PostgreSQL is the single source of truth. Each vote and its denormalized `vote_count` move atomically in one transaction. Redis caches counts and maintains `top`/`trending` ZSETs for fast reads.

See [`specs/001-feature-voting-system/plan.md`](specs/001-feature-voting-system/plan.md) for full architecture.

## Requirements

- Docker + Docker Compose
- Go 1.23+ (for local development/testing)

## Quick Start

```bash
cp deploy/.env.example deploy/.env
# Edit deploy/.env — set real JWT secrets before deploying

docker compose -f deploy/docker-compose.yml up --build
```

Wait for readiness:
```bash
curl -fsS http://localhost:8080/readyz
```

## Development

```bash
# Unit tests
make test

# Integration tests (requires Docker for testcontainers)
make test-integration

# Lint
make lint

# Vulnerability check
make vuln

# Build binary
make build

# Build Docker image
make docker
```

## API

Base URL: `http://localhost:8080`

| Method | Path | Description |
|--------|------|-------------|
| POST | `/auth/register` | Register a new user |
| POST | `/auth/login` | Login |
| POST | `/auth/refresh` | Refresh access token |
| GET | `/requests` | List requests (`sort=new\|top\|trending`, `cursor`, `limit`) |
| POST | `/requests` | Submit a new feature request |
| GET | `/requests/{id}` | Get a single feature request |
| PUT | `/requests/{id}/vote` | Upvote (idempotent) |
| DELETE | `/requests/{id}/vote` | Remove upvote |
| GET | `/healthz` | Liveness probe |
| GET | `/readyz` | Readiness probe (checks Postgres + Redis) |
| GET | `/metrics` | Prometheus metrics |

Full OpenAPI 3.1 contract: [`api/openapi.yaml`](api/openapi.yaml).
Client integration guide: [`api/README.md`](api/README.md).

## Project Structure

```text
cmd/server/         — entry point, composition root
internal/
  config/           — 12-factor env config
  handler/          — HTTP only: decode, validate, encode
  service/          — business logic
  repository/       — data access (postgres/, redis/)
  ranking/          — trending recompute + reconciliation jobs
  middleware/        — JWT auth, rate limit, security headers
  observability/    — slog, Prometheus, OTel
  platform/         — health probes, server bootstrap
migrations/         — golang-migrate SQL files
db/queries/         — sqlc SQL sources
api/                — OpenAPI contract copy + client docs
deploy/             — Dockerfile, docker-compose.yml, .env.example
tests/
  integration/      — testcontainers-based integration tests
  e2e/              — httptest API flow tests
```

## Validation Scenarios

See [`specs/001-feature-voting-system/quickstart.md`](specs/001-feature-voting-system/quickstart.md) for end-to-end validation scenarios A–F.
