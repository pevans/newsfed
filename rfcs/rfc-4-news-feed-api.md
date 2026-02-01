---
Request For Comments: 4
Title: News Feed API
Drafted At: 2026-01-31
Authors:
  - Peter Evans
---

# 1. News Feed API

newsfed provides an HTTP API for clients to interact with the news feed.
Clients use this API to retrieve news items, pin items for later reference, and
manage the feed. The API follows REST principles and uses JSON for request and
response payloads.

# 2. API Design

## 2.1. Protocol and Format

The API uses HTTP/1.1 or HTTP/2 over HTTPS. All request and response bodies
use JSON format with `Content-Type: application/json`.

## 2.2. Base URL

The API is served at a configurable base URL. For local development, the
default is `http://localhost:8080/api/v1`. In production, this would be
configured to match the deployment environment.

## 2.3. Versioning

The API version is included in the URL path (e.g., `/api/v1`). This allows for
future API changes without breaking existing clients.

## 2.4. HTTP Methods

The API uses standard HTTP methods:

- `GET` -- Retrieve resources (items, lists)
- `POST` -- Pin items for later reference

# 3. Endpoints

## 3.1. List News Items

**Endpoint**: `GET /api/v1/items`

Retrieves a list of news items from the feed. Supports filtering and
pagination.

**Query Parameters**:

- `pinned` (optional): Filter by pinned status
  - `true` -- Only pinned items (pinned_at is not nil)
  - `false` -- Only unpinned items (pinned_at is nil)
  - Omit parameter to return all items
- `publisher` (optional): Filter by publisher name (exact match)
- `author` (optional): Filter by author name (matches any author in the
  authors array)
- `since` (optional): Only return items discovered after this timestamp (ISO
  8601 format). Filters by discovered_at field.
- `until` (optional): Only return items discovered before this timestamp (ISO
  8601 format). Filters by discovered_at field.
- `limit` (optional): Maximum number of items to return (default: 50, max:
  1000)
- `offset` (optional): Number of items to skip for pagination (default: 0)
- `sort` (optional): Sort order
  - `published_desc` -- Sort by published_at descending (newest first,
    default)
  - `published_asc` -- Sort by published_at ascending (oldest first)
  - `discovered_desc` -- Sort by discovered_at descending
  - `discovered_asc` -- Sort by discovered_at ascending
  - `pinned_desc` -- Sort by pinned_at descending (most recently pinned first)
  - `pinned_asc` -- Sort by pinned_at ascending (least recently pinned first)

**Response**: `200 OK`

```json
{
  "items": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "title": "Example News Item",
      "summary": "This is a summary of the news item...",
      "url": "https://example.com/article",
      "publisher": "Example Publication",
      "authors": ["Jane Doe", "John Smith"],
      "published_at": "2026-01-30T10:00:00Z",
      "discovered_at": "2026-01-30T12:00:00Z",
      "pinned_at": null
    }
  ],
  "total": 42,
  "limit": 50,
  "offset": 0
}
```

**Response Fields**:

- `items`: Array of news item objects
- `total`: Total number of items matching the filter (before pagination)
- `limit`: Maximum items returned in this response
- `offset`: Number of items skipped

## 3.2. Get News Item by ID

**Endpoint**: `GET /api/v1/items/{id}`

Retrieves a specific news item by its UUID.

**Path Parameters**:

- `id`: The UUID of the news item

**Response**: `200 OK`

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "title": "Example News Item",
  "summary": "This is a summary of the news item...",
  "url": "https://example.com/article",
  "publisher": "Example Publication",
  "authors": ["Jane Doe", "John Smith"],
  "published_at": "2026-01-30T10:00:00Z",
  "discovered_at": "2026-01-30T12:00:00Z",
  "pinned_at": null
}
```

**Error Responses**:

- `404 Not Found` -- Item with the given ID does not exist

## 3.3. Pin Item

**Endpoint**: `POST /api/v1/items/{id}/pin`

Pins an item for later reference by setting the `pinned_at` field to the
current timestamp.

**Path Parameters**:

- `id`: The UUID of the news item

**Response**: `200 OK`

Returns the updated news item with `pinned_at` set to the current timestamp.

**Error Responses**:

- `404 Not Found` -- Item with the given ID does not exist

## 3.4. Unpin Item

**Endpoint**: `POST /api/v1/items/{id}/unpin`

Unpins an item by setting the `pinned_at` field to nil.

**Path Parameters**:

- `id`: The UUID of the news item

**Response**: `200 OK`

Returns the updated news item with `pinned_at` set to nil.

**Error Responses**:

- `404 Not Found` -- Item with the given ID does not exist

# 4. Error Handling

## 4.1. Error Response Format

All error responses use a consistent JSON format:

```json
{
  "error": {
    "code": "not_found",
    "message": "News item with ID 550e8400-e29b-41d4-a716-446655440000 not found"
  }
}
```

## 4.2. HTTP Status Codes

The API uses standard HTTP status codes:

- `200 OK` -- Request succeeded
- `400 Bad Request` -- Invalid request (malformed JSON, invalid parameters)
- `404 Not Found` -- Resource not found
- `500 Internal Server Error` -- Server error

# 5. Authentication and Authorization

## 5.1. Initial Implementation

The initial implementation does not require authentication. The API is assumed
to run locally or in a trusted environment where all clients have full access.

## 5.2. Future Enhancements

Future versions may add:

- API key authentication
- OAuth 2.0 support for multi-user scenarios
- Rate limiting per client
- Read-only vs read-write permissions

# 6. CORS Support

## 6.1. Cross-Origin Requests

The API should support CORS (Cross-Origin Resource Sharing) to allow web-based
clients running on different origins to access the API.

**CORS Headers**:

- `Access-Control-Allow-Origin`: Configurable, default `*` for development
- `Access-Control-Allow-Methods`: `GET, POST, OPTIONS`
- `Access-Control-Allow-Headers`: `Content-Type, Authorization`

## 6.2. Preflight Requests

The API should respond to `OPTIONS` requests with appropriate CORS headers for
preflight checks.
