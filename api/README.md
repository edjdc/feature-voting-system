# Feature Voting System API

REST/JSON API for the Feature Voting System. Full contract: [`openapi.yaml`](./openapi.yaml).

## Authentication

All endpoints except `/auth/register`, `/auth/login`, and `/auth/refresh` require a JWT Bearer token.

```bash
TOKEN=$(curl -s -X POST $BASE_URL/auth/register \
  -H 'content-type: application/json' \
  -d '{"email":"alice@example.com","password":"secret"}' | jq -r .access_token)
```

## Optimistic UI + Conditional Polling Contract

The `GET /requests` endpoint supports **ETag-based conditional polling** so clients can feel live without WebSockets.

### Flow

1. **Initial load**: `GET /requests?sort=top` returns a page with an `ETag` header.
2. **Periodic revalidation**: Re-request with `If-None-Match: <etag>`.
   - `304 Not Modified` → data unchanged, use cached state.
   - `200 OK` → new `ETag` + updated counts in response body.
3. **Optimistic update**: When a user votes, increment the counter in the client immediately, then revalidate within ≤60 s to confirm or roll back.

### Headers

| Header | Description |
|--------|-------------|
| `ETag` | Opaque version identifier for the page content |
| `Cache-Control: private, max-age=0, must-revalidate` | Client must revalidate before use |
| `If-None-Match` | Send the stored ETag; server returns 304 if unchanged |

### Recommended polling interval

60 seconds or less. The reconciliation job heals any Redis drift within that window (SC-005).

## Pagination (keyset)

Use cursor-based pagination for stable, O(1)-at-depth list reads.

```bash
# First page
GET /requests?sort=new&limit=20

# Next page using cursor from response
GET /requests?sort=new&limit=20&cursor=<next_cursor>
```

- `next_cursor: null` means you've reached the last page.
- Cursors are opaque — do not parse or construct them.
- Changing `sort` invalidates a cursor (different ordering key).

## Sort modes

| `sort` | Description |
|--------|-------------|
| `new` | Newest first (default) |
| `top` | Most votes first, stable tie-break |
| `trending` | Time-decay score: `vote_count / (age_hours + 2)^1.5` |
