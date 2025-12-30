# HTTP Handler Tests for traefik-k8s-secret-header

## Overview

HTTP handler tests have been created to test the ServeHTTP functionality with a mocked Kubernetes API server.

## Test Coverage

Current coverage: **54.4%** of statements

## Test Implementation

The tests use `httptest.Server` to create a mock Kubernetes API server that simulates the actual Kubernetes API responses. This allows comprehensive testing without requiring a real Kubernetes cluster or using the heavy `client-go` library.

## Tests Included

### 1. TestServeHTTP
Tests the HTTP handler with various scenarios using table-driven tests:

- **successful secret retrieval**: Validates that secrets are retrieved from K8s API and injected as headers
- **secret does not exist**: Ensures proper error handling when secret doesn't exist (returns 500)
- **secret key does not exist**: Ensures proper error handling when secret key is missing (returns 500)

### 2. TestServeHTTPWithCache
Tests the caching mechanism:
- First request fetches from Kubernetes API
- Second request uses cached value (no K8s API call)
- Verifies both requests are processed correctly
- Validates that the header value is consistent

### 3. TestServeHTTPCacheExpiration
Tests cache expiration:
- Sets CacheTTL to 0 (immediate expiration)
- Verifies that expired cache triggers a new K8s API call
- Ensures secrets are re-fetched when cache expires

## Running the Tests

```bash
# Run all tests
go test -v -cover

# Run with race detector
go test -race -v

# Run specific test
go test -v -run TestServeHTTP
```

## Test Results

All tests pass successfully:

```
PASS
coverage: 54.4% of statements
ok      github.com/ImDevinC/traefik-k8s-secret-header   0.013s
```

Race detector also passes with no data races detected.

## What's Tested

✅ Successful secret retrieval and header injection  
✅ Base64 decoding of secret values  
✅ Error handling when secrets don't exist  
✅ Error handling when secret keys don't exist  
✅ Cache hit behavior (reuses cached values)  
✅ Cache expiration and refetch behavior  
✅ HTTP status codes (200 for success, 500 for errors)  
✅ Next handler is called only on success  
✅ Headers are properly set on requests  
✅ Authorization token handling  
✅ TLS/HTTPS communication with K8s API

## No External Dependencies

The plugin and tests now have **zero external dependencies**. They only use Go standard library packages:
- `crypto/tls` - TLS configuration
- `crypto/x509` - Certificate handling
- `encoding/base64` - Base64 decoding
- `encoding/json` - JSON parsing
- `net/http` - HTTP client and server
- `io` - I/O operations
- `os` - File and environment operations
- `sync` - Synchronization primitives
- `time` - Time operations

This makes the plugin:
- ✅ Compatible with Traefik Plugin Catalog (Yaegi)
- ✅ Lightweight and fast
- ✅ Easy to maintain
- ✅ No version conflicts

## Implementation Details

The plugin communicates directly with the Kubernetes API using HTTP requests:
1. Reads service account token from `/var/run/secrets/kubernetes.io/serviceaccount/token`
2. Reads CA certificate from `/var/run/secrets/kubernetes.io/serviceaccount/ca.crt`
3. Makes authenticated HTTPS requests to the Kubernetes API
4. Parses JSON responses
5. Decodes base64-encoded secret values
6. Caches values with configurable TTL
