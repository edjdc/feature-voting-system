#!/usr/bin/env bash
#
# demo.sh — end-to-end walkthrough of the Feature Voting System API.
#
# Drives the full user journey against a locally running stack so the Loom
# demo is reproducible and typo-free:
#   1. readiness probe          (ops / production-readiness cameo)
#   2. register two users       (JWT auth, argon2id)
#   3. submit feature requests  (authenticated writes)
#   4. cast upvotes             (atomic vote + count, one tx)
#   5. self-vote is rejected    (FR-008 business rule, 403)
#   6. Top & Trending rankings  (Redis read-path / ZSETs)
#
# Prereq: stack up via `docker compose -f deploy/docker-compose.yml up --build`
# Tools:  curl, jq
#
set -euo pipefail

BASE="${BASE:-http://localhost:8080}"
STAMP="$(date +%s)"

hr()   { printf '\n\033[1;36m── %s\033[0m\n' "$1"; }
say()  { printf '\033[2m%s\033[0m\n' "$1"; }

# POST helper: $1 path, $2 json body, $3 (optional) bearer token
post() {
  local path="$1" body="$2" token="${3:-}"
  if [[ -n "$token" ]]; then
    curl -fsS -X POST "$BASE$path" -H 'Content-Type: application/json' \
      -H "Authorization: Bearer $token" -d "$body"
  else
    curl -fsS -X POST "$BASE$path" -H 'Content-Type: application/json' -d "$body"
  fi
}

# ---------------------------------------------------------------------------
hr "1. Readiness — is the stack live? (Postgres + Redis checked)"
curl -fsS -o /dev/null -w 'readyz → HTTP %{http_code}\n' "$BASE/readyz"

# ---------------------------------------------------------------------------
hr "2. Register two users — JWT issued, password hashed with argon2id"
ALICE_EMAIL="alice+${STAMP}@example.com"
BOB_EMAIL="bob+${STAMP}@example.com"

ALICE=$(post /auth/register "{\"email\":\"$ALICE_EMAIL\",\"password\":\"correcthorse1\"}" | jq -r .access_token)
BOB=$(post   /auth/register "{\"email\":\"$BOB_EMAIL\",\"password\":\"correcthorse1\"}"   | jq -r .access_token)
say "alice → ${ALICE:0:24}…   bob → ${BOB:0:24}…"

# ---------------------------------------------------------------------------
hr "3. Submit feature requests (authenticated writes)"
R1=$(post /requests '{"title":"Dark mode","description":"A system-wide dark theme."}'        "$ALICE" | jq -r .id)
R2=$(post /requests '{"title":"CSV export","description":"Export reports as CSV."}'           "$ALICE" | jq -r .id)
R3=$(post /requests '{"title":"Mobile push","description":"Native push notifications."}'      "$BOB"   | jq -r .id)
say "created: dark-mode=$R1  csv=$R2  mobile-push=$R3"

# ---------------------------------------------------------------------------
hr "4. Cast upvotes — vote + denormalized count move in ONE transaction"
say "bob upvotes dark mode:"
curl -fsS -X PUT "$BASE/requests/$R1/vote" -H "Authorization: Bearer $BOB" | jq -c
say "bob upvotes csv export:"
curl -fsS -X PUT "$BASE/requests/$R2/vote" -H "Authorization: Bearer $BOB" | jq -c
say "alice upvotes mobile push:"
curl -fsS -X PUT "$BASE/requests/$R3/vote" -H "Authorization: Bearer $ALICE" | jq -c
say "bob upvotes dark mode AGAIN — idempotent, count stays 1:"
curl -fsS -X PUT "$BASE/requests/$R1/vote" -H "Authorization: Bearer $BOB" | jq -c

# ---------------------------------------------------------------------------
hr "5. Business rule: you cannot vote on your OWN request (FR-008 → 403)"
code=$(curl -s -o /dev/null -w '%{http_code}' -X PUT "$BASE/requests/$R1/vote" -H "Authorization: Bearer $ALICE")
say "alice self-votes dark mode → HTTP $code (rejected as designed)"

# ---------------------------------------------------------------------------
hr "6. Rankings served from the Redis read-path"
say "TOP (raw vote count):"
curl -fsS "$BASE/requests?sort=top&limit=5"      -H "Authorization: Bearer $ALICE" \
  | jq -r '.items[] | "  \(.vote_count)  ▸ \(.title)"'
say "TRENDING (time-decayed score):"
curl -fsS "$BASE/requests?sort=trending&limit=5" -H "Authorization: Bearer $ALICE" \
  | jq -r '.items[] | "  \(.vote_count)  ▸ \(.title)"'

hr "Done."
