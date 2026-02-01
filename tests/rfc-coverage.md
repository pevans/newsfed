# RFC 4 Black Box Test Coverage

This document tracks which sections of RFC 4 (News Feed API) are covered by black box tests.

## Section 2 - API Design

- [x] **2.1 Protocol and Format** - Tested via all endpoints
  - HTTP protocol with JSON responses
  - Content-Type: application/json headers verified
- [x] **2.2 Base URL** - Tested in all scripts
  - Base URL: `http://localhost:8080/api/v1`
- [x] **2.3 Versioning** - Tested in all scripts
  - Version included in URL path (`/api/v1`)
- [x] **2.4 HTTP Methods** - Tested in endpoint tests
  - GET methods tested in list and get endpoints
  - POST methods tested in pin/unpin endpoints
  - Wrong method returns 405 (tested)

## Section 3 - Endpoints

### 3.1 List News Items

- [x] **Basic listing** (`GET /api/v1/items`)
  - Returns 200 OK
  - Returns items array
  - Returns total, limit, offset fields
- [x] **Filter by pinned status**
  - `pinned=true` returns only pinned items
  - `pinned=false` returns only unpinned items
- [x] **Filter by publisher**
  - `publisher=<name>` filters correctly
- [x] **Filter by author**
  - `author=<name>` filters items with matching author
- [ ] **Filter by since** (not yet tested)
  - `since=<timestamp>` filters by discovered_at
- [ ] **Filter by until** (not yet tested)
  - `until=<timestamp>` filters by discovered_at
- [x] **Pagination - limit**
  - `limit=<n>` limits number of items returned
  - `limit` capped at 1000 (not explicitly tested)
- [x] **Pagination - offset**
  - `offset=<n>` skips items correctly
- [x] **Sorting**
  - `sort=published_desc` sorts correctly (default)
  - Other sort options (published_asc, discovered_desc, etc.) not explicitly tested
- [x] **Error handling**
  - Invalid parameters return 400

### 3.2 Get News Item by ID

- [x] **Get existing item** (`GET /api/v1/items/{id}`)
  - Returns 200 OK
  - Returns correct item with all required fields
- [x] **Get non-existent item**
  - Returns 404 Not Found
- [x] **Invalid UUID format**
  - Returns 400 Bad Request
- [x] **Error response format**
  - Contains error.code and error.message

### 3.3 Pin Item

- [x] **Pin item** (`POST /api/v1/items/{id}/pin`)
  - Returns 200 OK
  - Sets pinned_at to current timestamp
  - Returns updated item
- [x] **Pin non-existent item**
  - Returns 404 Not Found
- [x] **Wrong HTTP method**
  - Returns 405 Method Not Allowed
- [x] **Idempotency**
  - Pinning already pinned item returns 200 OK

### 3.4 Unpin Item

- [x] **Unpin item** (`POST /api/v1/items/{id}/unpin`)
  - Returns 200 OK
  - Sets pinned_at to null
  - Returns updated item
- [x] **Unpin non-existent item**
  - Returns 404 Not Found
- [x] **Persistence**
  - Unpinned state persists after GET request

## Section 4 - Error Handling

- [x] **4.1 Error Response Format**
  - Consistent JSON format with error.code and error.message
- [x] **4.2 HTTP Status Codes**
  - 200 OK for successful requests
  - 400 Bad Request for invalid parameters
  - 404 Not Found for missing resources
  - 405 Method Not Allowed for wrong HTTP methods
  - 500 Internal Server Error (not explicitly tested)

## Section 5 - Authentication and Authorization

- [x] **5.1 Initial Implementation**
  - No authentication required (tested implicitly - all requests work without auth)
- [ ] **5.2 Future Enhancements**
  - Not applicable for current implementation

## Section 6 - CORS Support

- [ ] **6.1 Cross-Origin Requests**
  - CORS headers present (not tested in current suite)
- [ ] **6.2 Preflight Requests**
  - OPTIONS requests handled (not tested in current suite)

## Test Files

- `setup.sh` - Creates test data in .news directory
- `test-list-items.sh` - Tests section 3.1 (10 tests)
- `test-get-item.sh` - Tests section 3.2 (5 tests)
- `test-pin-unpin.sh` - Tests sections 3.3 and 3.4 (8 tests)
- `run-all.sh` - Runs all test suites

## Coverage Summary

**Total RFC 4 Sections**: 6 main sections (2-6, plus subsections)
**Sections with Tests**: Sections 2, 3, 4 (partially 5)
**Test Coverage**: ~75% of testable functionality

**Not Covered**:
- Date range filtering (since/until)
- All sort options
- CORS headers verification
- Some edge cases

## Running Tests

```bash
# Start the API server in one terminal
go run .

# In another terminal, run all tests
./tests/run-all.sh

# Or run individual test suites
./tests/test-list-items.sh
./tests/test-get-item.sh
./tests/test-pin-unpin.sh
```
