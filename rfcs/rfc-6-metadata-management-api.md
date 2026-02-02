---
Request For Comments: 6
Title: Metadata Management API
Drafted At: 2026-02-01
Authors:
  - Peter Evans
---

# 1. Metadata Management API

newsfed provides an HTTP API for managing metadata, including news source
configurations (RSS/Atom feeds and website scrapers) and user preferences.
This API enables programmatic configuration of the news aggregation system.

The Metadata Management API is separate from the News Feed API (RFC 4). The
News Feed API operates on news items, while this API operates on the metadata
that defines where news items come from.

# 2. API Design

## 2.1. Protocol and Format

The API uses HTTP/1.1 or HTTP/2 over HTTPS. All request and response bodies
use JSON format with `Content-Type: application/json`.

## 2.2. Base URL

The API is served at a configurable base URL. For local development, the
default is `http://localhost:8080/api/v1/meta`. In production, this would be
configured to match the deployment environment.

The `/meta` path distinguishes this API from the news feed API (`/api/v1`).

## 2.3. Versioning

The API version is included in the URL path (e.g., `/api/v1/meta`). This
allows for future API changes without breaking existing clients.

## 2.4. HTTP Methods

The API uses standard HTTP methods:

- `GET` -- Retrieve resources (sources, configuration)
- `POST` -- Create new sources
- `PUT` -- Update existing sources or configuration
- `DELETE` -- Remove sources

# 3. Endpoints

## 3.1. List Sources

**Endpoint**: `GET /api/v1/meta/sources`

Retrieves a list of all configured news sources.

**Query Parameters**:

- `type` (optional): Filter by source type
  - `rss` -- Only RSS sources
  - `atom` -- Only Atom sources
  - `website` -- Only website sources
  - Omit parameter to return all sources
- `enabled` (optional): Filter by enabled status
  - `true` -- Only enabled sources (enabled_at is not null)
  - `false` -- Only disabled sources (enabled_at is null)
  - Omit parameter to return all sources

**Response**: `200 OK`

```json
{
  "sources": [
    {
      "source_id": "550e8400-e29b-41d4-a716-446655440000",
      "source_type": "rss",
      "url": "https://example.com/feed.xml",
      "name": "Example Blog",
      "enabled_at": "2026-01-15T10:00:00Z",
      "created_at": "2026-01-15T10:00:00Z",
      "updated_at": "2026-01-20T14:30:00Z",
      "polling_interval": "1h",
      "last_fetched_at": "2026-01-31T09:00:00Z",
      "fetch_error_count": 0
    },
    {
      "source_id": "550e8400-e29b-41d4-a716-446655440001",
      "source_type": "website",
      "url": "https://example.com/articles",
      "name": "Example News Site",
      "enabled_at": "2026-01-15T10:00:00Z",
      "created_at": "2026-01-15T10:00:00Z",
      "updated_at": "2026-01-20T14:30:00Z",
      "polling_interval": "2h",
      "last_fetched_at": "2026-01-31T08:00:00Z",
      "fetch_error_count": 0,
      "scraper_config": {
        "discovery_mode": "list",
        "list_config": {
          "article_selector": ".article-link",
          "max_pages": 3
        },
        "article_config": {
          "title_selector": "h1.title",
          "content_selector": ".article-body",
          "author_selector": ".author-name",
          "date_selector": "time.published",
          "date_format": "2006-01-02"
        }
      }
    }
  ],
  "total": 2
}
```

**Response Fields**:

- `sources`: Array of source objects (see RFC 5, section 2)
- `total`: Total number of sources matching the filter

## 3.2. Get Source by ID

**Endpoint**: `GET /api/v1/meta/sources/{source_id}`

Retrieves detailed information about a specific source.

**Path Parameters**:

- `source_id`: UUID of the source

**Response**: `200 OK`

```json
{
  "source_id": "550e8400-e29b-41d4-a716-446655440000",
  "source_type": "rss",
  "url": "https://example.com/feed.xml",
  "name": "Example Blog",
  "enabled_at": "2026-01-15T10:00:00Z",
  "created_at": "2026-01-15T10:00:00Z",
  "updated_at": "2026-01-20T14:30:00Z",
  "polling_interval": "1h",
  "last_fetched_at": "2026-01-31T09:00:00Z",
  "etag": "\"abc123\"",
  "fetch_error_count": 0,
  "last_error": null
}
```

**Error Responses**:

- `404 Not Found` -- Source with given ID does not exist
- `400 Bad Request` -- Invalid UUID format

## 3.3. Create Source

**Endpoint**: `POST /api/v1/meta/sources`

Creates a new news source.

**Request Body**:

```json
{
  "source_type": "rss",
  "url": "https://example.com/feed.xml",
  "name": "Example Blog",
  "polling_interval": "1h"
}
```

**Required Fields**:

- `source_type`: Must be "rss", "atom", or "website"
- `url`: Valid URL for the source
- `name`: Human-readable name

**Optional Fields**:

- `polling_interval`: Duration string (e.g., "30m", "1h", "2h"). Defaults to
  system default.
- `scraper_config`: Required for website sources, must conform to RFC 3
  section 2.2
- `enabled_at`: Timestamp when source should be enabled. Defaults to current
  time (source is enabled by default). Set to null to create a disabled
  source.

**Response**: `201 Created`

```json
{
  "source_id": "550e8400-e29b-41d4-a716-446655440002",
  "source_type": "rss",
  "url": "https://example.com/feed.xml",
  "name": "Example Blog",
  "enabled_at": "2026-02-01T10:00:00Z",
  "created_at": "2026-02-01T10:00:00Z",
  "updated_at": "2026-02-01T10:00:00Z",
  "polling_interval": "1h",
  "last_fetched_at": null,
  "fetch_error_count": 0
}
```

**Error Responses**:

- `400 Bad Request` -- Invalid request body, missing required fields, or
  validation errors
- `409 Conflict` -- Source with same URL already exists

## 3.4. Update Source

**Endpoint**: `PUT /api/v1/meta/sources/{source_id}`

Updates an existing source. Only provided fields are updated.

**Path Parameters**:

- `source_id`: UUID of the source to update

**Request Body** (all fields optional):

```json
{
  "name": "Updated Blog Name",
  "url": "https://example.com/new-feed.xml",
  "enabled_at": "2026-02-01T12:00:00Z",
  "polling_interval": "2h",
  "scraper_config": {
    "discovery_mode": "direct",
    "article_config": {
      "title_selector": "h1.title",
      "content_selector": ".content"
    }
  }
}
```

**Notes**:

- Setting `enabled_at` to null disables the source
- Setting `enabled_at` to a timestamp enables the source (typically set to
  current time)
- `source_type` cannot be changed after creation
- `scraper_config` can only be set for website sources
- Operational metadata (last_fetched_at, etag, etc.) cannot be updated through
  this endpoint

**Response**: `200 OK`

Returns the updated source object.

**Error Responses**:

- `404 Not Found` -- Source with given ID does not exist
- `400 Bad Request` -- Invalid request body or validation errors

## 3.5. Delete Source

**Endpoint**: `DELETE /api/v1/meta/sources/{source_id}`

Deletes a source. This removes the source configuration but does NOT delete
news items that were previously ingested from this source.

**Path Parameters**:

- `source_id`: UUID of the source to delete

**Response**: `204 No Content`

**Error Responses**:

- `404 Not Found` -- Source with given ID does not exist

## 3.6. Get Configuration

**Endpoint**: `GET /api/v1/meta/config`

Retrieves user configuration settings.

**Response**: `200 OK`

```json
{
  "default_polling_interval": "1h"
}
```

## 3.7. Update Configuration

**Endpoint**: `PUT /api/v1/meta/config`

Updates user configuration settings.

**Request Body** (all fields optional):

```json
{
  "default_polling_interval": "2h"
}
```

**Response**: `200 OK`

Returns the updated configuration object.

**Error Responses**:

- `400 Bad Request` -- Invalid configuration values

# 4. Error Handling

## 4.1. Error Response Format

All error responses follow a consistent JSON format:

```json
{
  "error": {
    "code": "not_found",
    "message": "Source with ID 550e8400-e29b-41d4-a716-446655440099 not found"
  }
}
```

**Error Codes**:

- `not_found` -- Resource does not exist
- `bad_request` -- Invalid request parameters or body
- `conflict` -- Resource already exists
- `validation_error` -- Request validation failed
- `internal_error` -- Unexpected server error

## 4.2. HTTP Status Codes

The API uses standard HTTP status codes:

- `200 OK` -- Successful GET or PUT request
- `201 Created` -- Successful POST request (resource created)
- `204 No Content` -- Successful DELETE request
- `400 Bad Request` -- Invalid request parameters or body
- `404 Not Found` -- Resource not found
- `409 Conflict` -- Resource conflict (e.g., duplicate URL)
- `500 Internal Server Error` -- Unexpected server error

## 4.3. Validation Errors

When validation fails, the error response includes details:

```json
{
  "error": {
    "code": "validation_error",
    "message": "Validation failed",
    "details": [
      {
        "field": "url",
        "message": "Invalid URL format"
      },
      {
        "field": "source_type",
        "message": "Must be one of: rss, atom, website"
      }
    ]
  }
}
```

# 5. Authentication and Authorization

## 5.1. Initial Implementation

The initial implementation requires no authentication. The API is intended for
local, single-user deployments where the user has full control over the
system.

## 5.2. Future Enhancements

For multi-user or networked deployments, authentication will be required:

- API keys or tokens for programmatic access
- OAuth 2.0 for user authentication
- Role-based access control (read-only vs. admin)

# 6. CORS Support

## 6.1. Cross-Origin Requests

The API includes CORS (Cross-Origin Resource Sharing) headers to enable web
clients from different origins to access the API.

All responses include:

```
Access-Control-Allow-Origin: *
```

## 6.2. Preflight Requests

The API handles OPTIONS preflight requests for CORS:

```
Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS
Access-Control-Allow-Headers: Content-Type, Authorization
```

# 7. Relationship to Other Components

## 7.1. Metadata Storage (RFC 5)

This API is a thin HTTP layer over the metadata storage system defined in RFC
5. API endpoints map directly to storage operations:

- `GET /sources` → `ListSources()`
- `GET /sources/{id}` → `GetSource(id)`
- `POST /sources` → `CreateSource(...)`
- `PUT /sources/{id}` → `UpdateSource(id, ...)`
- `DELETE /sources/{id}` → `DeleteSource(id)`
- `GET /config` → `GetConfig()`
- `PUT /config` → `UpdateConfig(...)`

## 7.2. News Feed API (RFC 4)

The Metadata Management API is completely separate from the News Feed API:

- **News Feed API** (`/api/v1/items`) -- Manages news items
- **Metadata API** (`/api/v1/meta/sources`) -- Manages sources

Clients may use both APIs, but they serve different purposes.

## 7.3. News Aggregator

A news aggregator component (not yet defined in an RFC) would:

1. Use the metadata storage directly (not through this API) to find enabled
   sources
2. Fetch and parse sources according to their type
3. Update operational metadata (last_fetched_at, fetch_error_count, etc.)
4. Store discovered items in the news feed

The aggregator operates at the storage layer, while this API is for
user/client configuration.

## 7.4. Web Clients

Web-based management interfaces would use this API to:

- Display all configured sources
- Add new RSS/Atom feeds or website scrapers
- Enable/disable sources
- Monitor fetch status and errors
- Adjust polling intervals

# 8. Implementation Considerations

## 8.1. Source Validation

Before creating or updating sources, the API should validate:

- URL format and accessibility
- source_type is valid ("rss", "atom", "website")
- scraper_config structure matches RFC 3 schema (for website sources)
- polling_interval is a valid duration string

## 8.2. Duplicate Detection

The API should prevent duplicate sources:

- Check if a source with the same URL already exists
- Return `409 Conflict` when attempting to create a duplicate
- Consider URL normalization (http vs https, trailing slashes, etc.)

## 8.3. Operational Metadata

Operational metadata (last_fetched_at, etag, fetch_error_count, last_error)
should NOT be user-modifiable through this API. These fields are managed by
the news aggregator component.

The API should return these fields in GET responses but ignore them in PUT
requests.

## 8.4. Transactional Updates

When updating sources, especially scraper_config, changes should be atomic:

- Either all updates succeed or none do
- Use database transactions where supported
- Validate entire request before making any changes

## 8.5. Audit Logging

Consider logging all create/update/delete operations:

- Who made the change (when authentication is added)
- What was changed
- When the change occurred

This helps with debugging and accountability.

# 9. Example Workflows

## 9.1. Adding a New RSS Feed

```bash
# Create a new RSS source
curl -X POST http://localhost:8080/api/v1/meta/sources \
  -H "Content-Type: application/json" \
  -d '{
    "source_type": "rss",
    "url": "https://blog.example.com/feed.xml",
    "name": "Example Tech Blog",
    "polling_interval": "30m"
  }'

# Response: 201 Created
# {
#   "source_id": "...",
#   "source_type": "rss",
#   "url": "https://blog.example.com/feed.xml",
#   "name": "Example Tech Blog",
#   "enabled_at": "2026-02-01T10:00:00Z",
#   ...
# }
```

## 9.2. Disabling a Source

```bash
# Disable by setting enabled_at to null
curl -X PUT http://localhost:8080/api/v1/meta/sources/{source_id} \
  -H "Content-Type: application/json" \
  -d '{
    "enabled_at": null
  }'
```

## 9.3. Listing All Website Sources

```bash
# Get all website sources
curl http://localhost:8080/api/v1/meta/sources?type=website

# Response: 200 OK
# {
#   "sources": [
#     {
#       "source_id": "...",
#       "source_type": "website",
#       "scraper_config": {...},
#       ...
#     }
#   ],
#   "total": 5
# }
```

## 9.4. Checking Source Status

```bash
# Get detailed source info including fetch status
curl http://localhost:8080/api/v1/meta/sources/{source_id}

# Response: 200 OK
# {
#   "source_id": "...",
#   "last_fetched_at": "2026-02-01T09:00:00Z",
#   "fetch_error_count": 2,
#   "last_error": "HTTP 404: Feed not found",
#   ...
# }
```

# 10. Security Considerations

## 10.1. Input Validation

All user inputs must be validated:

- URLs should be checked for valid schemes (http/https only)
- Prevent SSRF attacks by blocking internal/private IP ranges
- Validate JSON structure and field types
- Sanitize error messages to avoid information disclosure

## 10.2. Rate Limiting

Consider rate limiting API requests to prevent abuse:

- Limit number of source creation requests per time period
- Prevent rapid enable/disable cycling
- Return `429 Too Many Requests` when limits are exceeded

## 10.3. URL Safety

When users provide URLs:

- Validate against allow-lists if possible
- Block file:// and other non-HTTP schemes
- Consider blocking URLs pointing to localhost or private networks
- Warn about or block URLs with authentication credentials embedded
