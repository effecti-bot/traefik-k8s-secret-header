# Traefik Kubernetes Secret Header Plugin

A Traefik middleware plugin that reads Kubernetes secrets and injects them as HTTP headers into requests. This plugin is designed to work seamlessly with both Traefik's native IngressRoute CRD and Kubernetes Gateway API HTTPRoute.

## Features

- Read secrets from Kubernetes cluster in real-time
- Inject secret values as HTTP headers
- Configurable secret name, key, and header name
- Support for cross-namespace secret access
- Built-in caching with configurable TTL to reduce Kubernetes API calls
- Compatible with both IngressRoute and HTTPRoute
- Comprehensive error handling and logging

## Use Cases

- Inject authentication tokens from secrets (e.g., Bearer tokens, API keys)
- Add authorization headers from centrally managed secrets
- Pass credentials to backend services without hardcoding
- Rotate secrets without redeploying applications

## Configuration

### Plugin Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `secretName` | string | Yes | - | Name of the Kubernetes secret |
| `secretKey` | string | Yes | - | Key within the secret to read |
| `headerName` | string | Yes | - | Name of the HTTP header to inject |
| `namespace` | string | No | `default` | Kubernetes namespace of the secret |
| `cacheTTL` | int | No | `300` | Cache TTL in seconds (0 to disable caching) |

## Installation

### Prerequisites

- Traefik v2.10+ or v3.x running in Kubernetes
- Kubernetes cluster with RBAC enabled
- Traefik configured to use plugins

### Step 1: Configure RBAC

The plugin requires permissions to read secrets from Kubernetes. Apply the appropriate RBAC configuration:

For single namespace access:
```bash
kubectl apply -f examples/rbac/rbac.yaml
```

For cross-namespace access, use the ClusterRole and ClusterRoleBinding defined in the same file.

### Step 2: Configure Traefik Static Configuration

Add the plugin to your Traefik static configuration:

#### Helm Values (Recommended for Helm deployments)

```yaml
experimental:
  plugins:
    k8s-secret-header:
      moduleName: github.com/ImDevinC/traefik-k8s-secret-header
      version: v1.0.0

# Ensure Traefik uses the service account with secret read permissions
serviceAccount:
  name: traefik
```

#### Traefik Configuration File

```yaml
experimental:
  plugins:
    k8s-secret-header:
      moduleName: github.com/ImDevinC/traefik-k8s-secret-header
      version: v1.0.0
```

#### Command Line Arguments

```bash
--experimental.plugins.k8s-secret-header.moduleName=github.com/ImDevinC/traefik-k8s-secret-header
--experimental.plugins.k8s-secret-header.version=v1.0.0
```

### Step 3: Create a Secret

Create a Kubernetes secret containing the value you want to inject:

```bash
kubectl apply -f examples/secret.yaml
```

Or create it directly:

```bash
kubectl create secret generic basic-auth-header \
  --from-literal=token="Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

## Usage

### With IngressRoute (Traefik CRD)

1. Create a Middleware resource:

```yaml
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata:
  name: secret-auth-header
  namespace: default
spec:
  plugin:
    k8s-secret-header:
      secretName: basic-auth-header
      secretKey: token
      headerName: Authorization
      namespace: default
      cacheTTL: 300
```

2. Reference the middleware in your IngressRoute:

```yaml
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: whoami-ingressroute
  namespace: default
spec:
  entryPoints:
    - web
  routes:
  - match: Host(`whoami.example.com`)
    kind: Rule
    services:
    - name: whoami
      port: 80
    middlewares:
    - name: secret-auth-header
```

3. Apply the manifests:

```bash
kubectl apply -f examples/ingressroute/
```

### With HTTPRoute (Kubernetes Gateway API)

1. Create a Middleware resource (same as above):

```yaml
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata:
  name: secret-auth-header
  namespace: default
spec:
  plugin:
    k8s-secret-header:
      secretName: basic-auth-header
      secretKey: token
      headerName: Authorization
      namespace: default
      cacheTTL: 300
```

2. Reference the middleware in your HTTPRoute using `extensionRef`:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: whoami-httproute
  namespace: default
spec:
  parentRefs:
  - name: traefik-gateway
  hostnames:
  - "whoami.example.com"
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /
    filters:
    - type: ExtensionRef
      extensionRef:
        group: traefik.io
        kind: Middleware
        name: secret-auth-header
    backendRefs:
    - name: whoami
      port: 80
```

3. Apply the manifests:

```bash
kubectl apply -f examples/httproute/
```

## Examples

### Example 1: Basic Authentication Header

```yaml
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata:
  name: api-auth
spec:
  plugin:
    k8s-secret-header:
      secretName: api-credentials
      secretKey: auth-token
      headerName: Authorization
```

### Example 2: Custom API Key Header

```yaml
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata:
  name: api-key-injector
spec:
  plugin:
    k8s-secret-header:
      secretName: api-keys
      secretKey: external-service-key
      headerName: X-API-Key
      namespace: production
      cacheTTL: 600
```

### Example 3: Cross-Namespace Secret Access

```yaml
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata:
  name: shared-secret-header
  namespace: app-namespace
spec:
  plugin:
    k8s-secret-header:
      secretName: shared-credentials
      secretKey: token
      headerName: X-Shared-Token
      namespace: shared-secrets  # Different namespace
      cacheTTL: 300
```

## Testing

You can test the plugin using the provided example manifests:

1. Deploy the test service:
```bash
kubectl apply -f examples/secret.yaml
kubectl apply -f examples/ingressroute/service.yaml
```

2. Deploy RBAC:
```bash
kubectl apply -f examples/rbac/rbac.yaml
```

3. Deploy the middleware and route:
```bash
kubectl apply -f examples/ingressroute/middleware.yaml
kubectl apply -f examples/ingressroute/ingressroute.yaml
```

4. Test the request:
```bash
curl -H "Host: whoami.example.com" http://localhost/
```

Check the response headers - you should see the `Authorization` header injected with the secret value.

## Local Development

To test the plugin locally before publishing to GitHub:

1. Place the plugin in Traefik's local plugins directory:
```
./plugins-local/
    └── src
        └── github.com
            └── yourusername
                └── traefik-k8s-secret-header/
```

2. Update Traefik static configuration:
```yaml
experimental:
  localPlugins:
    k8s-secret-header:
      moduleName: github.com/yourusername/traefik-k8s-secret-header
```

## Security Considerations

1. **Least Privilege**: Grant only necessary RBAC permissions. Use Role/RoleBinding for single namespace access instead of ClusterRole/ClusterRoleBinding when possible.

2. **Secret Encryption**: Ensure Kubernetes secrets are encrypted at rest in your cluster.

3. **Network Policies**: Consider implementing network policies to restrict access to the Kubernetes API.

4. **Audit Logging**: Enable Kubernetes audit logging to track secret access.

5. **Secret Rotation**: When rotating secrets, the cache will refresh after the TTL expires. Set a lower TTL for frequently rotated secrets.

## Troubleshooting

### Plugin fails to load
- Verify the plugin version matches a valid git tag in your repository
- Check that dependencies are vendored
- Ensure `.traefik.yml` manifest is present and valid

### "Failed to get secret" errors
- Verify RBAC permissions are correctly configured
- Check that the ServiceAccount is bound to the appropriate Role/ClusterRole
- Ensure the secret exists in the specified namespace

### Headers not appearing
- Check Traefik logs for plugin errors
- Verify the middleware is correctly referenced in your route
- Ensure the secret key exists in the secret
- Test with a lower cache TTL to rule out caching issues

### Permission denied errors
- Verify the Traefik pod is using the correct ServiceAccount
- Check that RoleBinding/ClusterRoleBinding references the correct ServiceAccount
- Ensure the namespace in the binding matches where Traefik is deployed

## Performance

The plugin implements caching to minimize Kubernetes API calls:

- Default cache TTL: 300 seconds (5 minutes)
- Cache is per-middleware instance
- Set `cacheTTL: 0` to disable caching (not recommended for production)
- Lower TTL values increase API calls but ensure fresher secrets

## Contributing

Contributions are welcome! Please follow these guidelines:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

Apache 2.0

## Author

[Your Name]

## Acknowledgments

- Traefik Team for the excellent plugin system
- Kubernetes community for client-go library
