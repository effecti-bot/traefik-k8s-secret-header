package traefik_k8s_secret_header

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mockK8sServer creates a mock Kubernetes API server for testing.
func mockK8sServer(t *testing.T, secretData map[string]string, secretExists bool) *httptest.Server {
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify authorization header
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if !secretExists {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"kind":"Status","apiVersion":"v1","status":"Failure","message":"secrets \"missing-secret\" not found","reason":"NotFound","code":404}`))
			return
		}

		// Encode secret data as base64 (like Kubernetes does)
		encodedData := make(map[string]string)
		for k, v := range secretData {
			encodedData[k] = base64.StdEncoding.EncodeToString([]byte(v))
		}

		secret := k8sSecret{
			Data: encodedData,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(secret)
	}))
}

// TestServeHTTP tests the HTTP handler with a mocked Kubernetes API server.
func TestServeHTTP(t *testing.T) {
	tests := []struct {
		name           string
		secretData     map[string]string
		secretExists   bool
		config         *Config
		expectedHeader string
		expectedStatus int
		expectError    bool
	}{
		{
			name: "successful secret retrieval",
			secretData: map[string]string{
				"token": "my-secret-token",
			},
			secretExists: true,
			config: &Config{
				SecretName: "my-secret",
				SecretKey:  "token",
				HeaderName: "X-Auth-Token",
				Namespace:  "default",
				CacheTTL:   300,
			},
			expectedHeader: "my-secret-token",
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:         "secret does not exist",
			secretExists: false,
			config: &Config{
				SecretName: "missing-secret",
				SecretKey:  "token",
				HeaderName: "X-Auth-Token",
				Namespace:  "default",
				CacheTTL:   300,
			},
			expectedStatus: http.StatusInternalServerError,
			expectError:    true,
		},
		{
			name: "secret key does not exist",
			secretData: map[string]string{
				"other-key": "some-value",
			},
			secretExists: true,
			config: &Config{
				SecretName: "my-secret",
				SecretKey:  "missing-key",
				HeaderName: "X-Auth-Token",
				Namespace:  "default",
				CacheTTL:   300,
			},
			expectedStatus: http.StatusInternalServerError,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock Kubernetes API server
			mockServer := mockK8sServer(t, tt.secretData, tt.secretExists)
			defer mockServer.Close()

			// Create a next handler that records if it was called
			nextCalled := false
			var capturedHeader string
			next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				nextCalled = true
				capturedHeader = req.Header.Get(tt.config.HeaderName)
				rw.WriteHeader(http.StatusOK)
			})

			// Create k8s client with mock server
			k8sClient := &k8sClient{
				httpClient: mockServer.Client(),
				baseURL:    mockServer.URL,
				token:      "test-token",
			}

			// Create the middleware
			handler := &SecretHeader{
				next:      next,
				name:      "test-middleware",
				config:    tt.config,
				k8sClient: k8sClient,
				cache: &secretCache{
					ttl: time.Duration(tt.config.CacheTTL) * time.Second,
				},
			}

			// Create a test request
			req := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
			rw := httptest.NewRecorder()

			// Execute the handler
			handler.ServeHTTP(rw, req)

			// Verify results
			if tt.expectError {
				if rw.Code != tt.expectedStatus {
					t.Errorf("Expected status %d, got %d", tt.expectedStatus, rw.Code)
				}
				if nextCalled {
					t.Error("Expected next handler not to be called on error, but it was called")
				}
			} else {
				if !nextCalled {
					t.Error("Expected next handler to be called, but it wasn't")
				}
				if capturedHeader != tt.expectedHeader {
					t.Errorf("Expected header value %q, got %q", tt.expectedHeader, capturedHeader)
				}
			}
		})
	}
}

// TestServeHTTPWithCache tests that cached values are used on subsequent requests.
func TestServeHTTPWithCache(t *testing.T) {
	secretData := map[string]string{
		"token": "my-secret-token",
	}

	mockServer := mockK8sServer(t, secretData, true)
	defer mockServer.Close()

	config := &Config{
		SecretName: "my-secret",
		SecretKey:  "token",
		HeaderName: "X-Auth-Token",
		Namespace:  "default",
		CacheTTL:   300,
	}

	requestCount := 0
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		requestCount++
		headerValue := req.Header.Get(config.HeaderName)
		if headerValue != "my-secret-token" {
			t.Errorf("Expected header value 'my-secret-token', got %q", headerValue)
		}
		rw.WriteHeader(http.StatusOK)
	})

	// Track API calls to mock server
	apiCallCount := 0
	trackedServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCallCount++
		// Forward to original mock server handler
		mockServer.Config.Handler.ServeHTTP(w, r)
	}))
	defer trackedServer.Close()

	k8sClient := &k8sClient{
		httpClient: trackedServer.Client(),
		baseURL:    trackedServer.URL,
		token:      "test-token",
	}

	handler := &SecretHeader{
		next:      next,
		name:      "test-middleware",
		config:    config,
		k8sClient: k8sClient,
		cache: &secretCache{
			ttl: time.Duration(config.CacheTTL) * time.Second,
		},
	}

	// First request - should fetch from K8s
	req1 := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
	rw1 := httptest.NewRecorder()
	handler.ServeHTTP(rw1, req1)

	if rw1.Code != http.StatusOK {
		t.Errorf("First request failed with status %d", rw1.Code)
	}

	initialAPICallCount := apiCallCount

	// Second request - should use cache (no new K8s call)
	req2 := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
	rw2 := httptest.NewRecorder()
	handler.ServeHTTP(rw2, req2)

	if rw2.Code != http.StatusOK {
		t.Errorf("Second request failed with status %d", rw2.Code)
	}

	// Verify cache was used (no new K8s API calls)
	if apiCallCount != initialAPICallCount {
		t.Errorf("Expected cache to be used, but K8s was called again. API calls: %d vs %d", apiCallCount, initialAPICallCount)
	}

	// Verify both requests were processed
	if requestCount != 2 {
		t.Errorf("Expected 2 requests to be processed, got %d", requestCount)
	}
}

// TestServeHTTPCacheExpiration tests that cache expires and refetches.
func TestServeHTTPCacheExpiration(t *testing.T) {
	secretData := map[string]string{
		"token": "my-secret-token",
	}

	mockServer := mockK8sServer(t, secretData, true)
	defer mockServer.Close()

	config := &Config{
		SecretName: "my-secret",
		SecretKey:  "token",
		HeaderName: "X-Auth-Token",
		Namespace:  "default",
		CacheTTL:   0, // 0 seconds - cache immediately expires
	}

	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusOK)
	})

	// Track API calls
	apiCallCount := 0
	trackedServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCallCount++
		mockServer.Config.Handler.ServeHTTP(w, r)
	}))
	defer trackedServer.Close()

	k8sClient := &k8sClient{
		httpClient: trackedServer.Client(),
		baseURL:    trackedServer.URL,
		token:      "test-token",
	}

	handler := &SecretHeader{
		next:      next,
		name:      "test-middleware",
		config:    config,
		k8sClient: k8sClient,
		cache: &secretCache{
			ttl: time.Duration(config.CacheTTL) * time.Second,
		},
	}

	// First request
	req1 := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
	rw1 := httptest.NewRecorder()
	handler.ServeHTTP(rw1, req1)

	initialAPICallCount := apiCallCount

	// Second request - cache should be expired, should fetch again
	req2 := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
	rw2 := httptest.NewRecorder()
	handler.ServeHTTP(rw2, req2)

	// Verify K8s was called again (cache expired)
	if apiCallCount <= initialAPICallCount {
		t.Errorf("Expected cache to expire and K8s to be called again, but API call count didn't increase")
	}
}
