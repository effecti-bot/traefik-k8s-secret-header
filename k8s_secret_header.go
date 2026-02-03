// Package traefik_k8s_secret_header is a Traefik plugin that injects HTTP headers from Kubernetes secrets.
package traefik_k8s_secret_header

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

// Config holds the plugin configuration.
type Config struct {
	SecretName string `json:"secretName,omitempty"`
	SecretKey  string `json:"secretKey,omitempty"`
	HeaderName string `json:"headerName,omitempty"`
	ValuePrefix string `json:"ValuePrefix,omitempty"` // Optional prefix to add before the secret value (e.g., "Bearer ")
	Namespace  string `json:"namespace,omitempty"`
	CacheTTL   int    `json:"cacheTTL,omitempty"` // Cache TTL in seconds, default 300 (5 minutes)
}

// CreateConfig creates the default plugin configuration.
func CreateConfig() *Config {
	return &Config{
		CacheTTL: 300, // 5 minutes default
	}
}

// SecretHeader is the middleware plugin.
type SecretHeader struct {
	next      http.Handler
	name      string
	config    *Config
	k8sClient *k8sClient
	cache     *secretCache
}

// k8sClient handles communication with the Kubernetes API.
type k8sClient struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

// k8sSecret represents the Kubernetes Secret API response.
type k8sSecret struct {
	Data map[string]string `json:"data"` // base64 encoded values
}

// secretCache provides caching for secret values.
type secretCache struct {
	mu        sync.RWMutex
	value     string
	lastFetch time.Time
	ttl       time.Duration
}

func (c *secretCache) get() (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if time.Since(c.lastFetch) > c.ttl {
		return "", false
	}
	return c.value, true
}

func (c *secretCache) set(value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.value = value
	c.lastFetch = time.Now()
}

// newK8sClient creates a new Kubernetes API client using in-cluster config.
func newK8sClient() (*k8sClient, error) {
	// Read the service account token
	tokenBytes, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return nil, fmt.Errorf("failed to read service account token: %w", err)
	}

	// Read the CA certificate
	caCert, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}

	// Create cert pool with CA
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	// Get Kubernetes API server URL
	host := os.Getenv("KUBERNETES_SERVICE_HOST")
	port := os.Getenv("KUBERNETES_SERVICE_PORT")
	if host == "" || port == "" {
		return nil, fmt.Errorf("KUBERNETES_SERVICE_HOST or KUBERNETES_SERVICE_PORT not set")
	}

	// Create HTTP client with TLS config
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:    caCertPool,
				MinVersion: tls.VersionTLS12,
			},
		},
	}

	return &k8sClient{
		httpClient: httpClient,
		baseURL:    fmt.Sprintf("https://%s:%s", host, port),
		token:      string(tokenBytes),
	}, nil
}

// getSecret retrieves a secret from the Kubernetes API.
func (c *k8sClient) getSecret(ctx context.Context, namespace, name string) (*k8sSecret, error) {
	url := fmt.Sprintf("%s/api/v1/namespaces/%s/secrets/%s", c.baseURL, namespace, name)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("kubernetes API returned status %d: %s", resp.StatusCode, string(body))
	}

	var secret k8sSecret
	if err := json.NewDecoder(resp.Body).Decode(&secret); err != nil {
		return nil, fmt.Errorf("failed to decode secret response: %w", err)
	}

	return &secret, nil
}

// New creates a new SecretHeader plugin.
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	if config.SecretName == "" {
		return nil, fmt.Errorf("secretName cannot be empty")
	}
	if config.SecretKey == "" {
		return nil, fmt.Errorf("secretKey cannot be empty")
	}
	if config.HeaderName == "" {
		return nil, fmt.Errorf("headerName cannot be empty")
	}

	// Default namespace to "default" if not specified
	if config.Namespace == "" {
		config.Namespace = "default"
	}

	// Create Kubernetes API client
	k8sClient, err := newK8sClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	cache := &secretCache{
		ttl: time.Duration(config.CacheTTL) * time.Second,
	}

	prefixInfo := ""
    if config.ValuePrefix != "" {
    	prefixInfo = fmt.Sprintf(" prefix='%s'", config.ValuePrefix)
    }
    fmt.Printf("[k8s-secret-header] Plugin '%s' initialized: secret=%s/%s key=%s header=%s%s ttl=%ds\n",
    	name, config.Namespace, config.SecretName, config.SecretKey, config.HeaderName, prefixInfo, config.CacheTTL)

	return &SecretHeader{
		next:      next,
		name:      name,
		config:    config,
		k8sClient: k8sClient,
		cache:     cache,
	}, nil
}

func (s *SecretHeader) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// Try to get from cache first
	if value, ok := s.cache.get(); ok {
		headerValue := s.config.ValuePrefix + value
		req.Header.Set(s.config.HeaderName, headerValue)
		s.next.ServeHTTP(rw, req)
		return
	}

	// Cache miss - fetch from Kubernetes
	secret, err := s.k8sClient.getSecret(req.Context(), s.config.Namespace, s.config.SecretName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[k8s-secret-header] Failed to get secret %s/%s: %v\n",
			s.config.Namespace, s.config.SecretName, err)
		http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Get the secret value (base64 encoded in the API response)
	encodedValue, ok := secret.Data[s.config.SecretKey]
	if !ok {
		fmt.Fprintf(os.Stderr, "[k8s-secret-header] Secret key '%s' not found in secret %s/%s\n",
			s.config.SecretKey, s.config.Namespace, s.config.SecretName)
		http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Decode base64 value
	// The Kubernetes API returns secret data as base64-encoded strings in JSON
	decodedValue, err := base64.StdEncoding.DecodeString(encodedValue)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[k8s-secret-header] Failed to decode secret value: %v\n", err)
		http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	value := string(decodedValue)

	// Cache the value
	s.cache.set(value)

	// Set the header with optional prefix
	headerValue := s.config.ValuePrefix + value
	req.Header.Set(s.config.HeaderName, headerValue)

	s.next.ServeHTTP(rw, req)
}
