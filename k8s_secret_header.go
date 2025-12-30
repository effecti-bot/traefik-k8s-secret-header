// Package k8ssecretheader is a Traefik plugin that injects HTTP headers from Kubernetes secrets.
package k8ssecretheader

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Config holds the plugin configuration.
type Config struct {
	SecretName string `json:"secretName,omitempty"`
	SecretKey  string `json:"secretKey,omitempty"`
	HeaderName string `json:"headerName,omitempty"`
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
	k8sClient *kubernetes.Clientset
	cache     *secretCache
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

	// Create in-cluster Kubernetes client
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create in-cluster config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	cache := &secretCache{
		ttl: time.Duration(config.CacheTTL) * time.Second,
	}

	os.Stdout.WriteString(fmt.Sprintf("[k8s-secret-header] Plugin '%s' initialized: secret=%s/%s key=%s header=%s ttl=%ds\n",
		name, config.Namespace, config.SecretName, config.SecretKey, config.HeaderName, config.CacheTTL))

	return &SecretHeader{
		next:      next,
		name:      name,
		config:    config,
		k8sClient: clientset,
		cache:     cache,
	}, nil
}

func (s *SecretHeader) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// Try to get from cache first
	if value, ok := s.cache.get(); ok {
		req.Header.Set(s.config.HeaderName, value)
		s.next.ServeHTTP(rw, req)
		return
	}

	// Cache miss - fetch from Kubernetes
	secret, err := s.k8sClient.CoreV1().Secrets(s.config.Namespace).Get(
		req.Context(),
		s.config.SecretName,
		metav1.GetOptions{},
	)
	if err != nil {
		os.Stderr.WriteString(fmt.Sprintf("[k8s-secret-header] Failed to get secret %s/%s: %v\n",
			s.config.Namespace, s.config.SecretName, err))
		http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Get the secret value
	secretValue, ok := secret.Data[s.config.SecretKey]
	if !ok {
		os.Stderr.WriteString(fmt.Sprintf("[k8s-secret-header] Secret key '%s' not found in secret %s/%s\n",
			s.config.SecretKey, s.config.Namespace, s.config.SecretName))
		http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Decode the secret value if it's base64 encoded (Kubernetes secrets are base64)
	// Since secret.Data returns []byte, we convert it to string
	value := string(secretValue)

	// Check if it's base64 encoded and needs decoding
	// For Opaque secrets, the data is already decoded by the client-go library
	// So we can use it directly

	// Cache the value
	s.cache.set(value)

	// Set the header
	req.Header.Set(s.config.HeaderName, value)

	s.next.ServeHTTP(rw, req)
}
