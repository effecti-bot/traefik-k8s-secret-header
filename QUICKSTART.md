# Quick Start Guide

This guide will help you get started with the Traefik K8s Secret Header plugin in under 5 minutes.

## Prerequisites

- Running Kubernetes cluster
- Traefik v2.10+ or v3.x deployed in the cluster
- `kubectl` configured to access your cluster

## Quick Setup

### 1. Configure Plugin in Traefik

Add to your Traefik Helm values or static configuration:

```yaml
experimental:
  plugins:
    k8s-secret-header:
      moduleName: github.com/yourusername/traefik-k8s-secret-header
      version: v1.0.0
```

Restart Traefik to load the plugin.

### 2. Create RBAC

```bash
kubectl apply -f examples/rbac/rbac.yaml
```

### 3. Create a Secret

```bash
kubectl create secret generic my-api-token \
  --from-literal=token="Bearer my-secret-token-12345"
```

### 4. Create Middleware

```bash
cat <<EOF | kubectl apply -f -
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata:
  name: inject-api-token
  namespace: default
spec:
  plugin:
    k8s-secret-header:
      secretName: my-api-token
      secretKey: token
      headerName: Authorization
EOF
```

### 5. Use in Route

#### For IngressRoute:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: my-app
  namespace: default
spec:
  entryPoints:
    - web
  routes:
  - match: Host(\`myapp.example.com\`)
    kind: Rule
    services:
    - name: my-app-service
      port: 80
    middlewares:
    - name: inject-api-token
EOF
```

#### For HTTPRoute:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: my-app
  namespace: default
spec:
  parentRefs:
  - name: traefik-gateway
  hostnames:
  - "myapp.example.com"
  rules:
  - filters:
    - type: ExtensionRef
      extensionRef:
        group: traefik.io
        kind: Middleware
        name: inject-api-token
    backendRefs:
    - name: my-app-service
      port: 80
EOF
```

### 6. Test

```bash
curl -H "Host: myapp.example.com" http://your-traefik-ip/
```

The `Authorization: Bearer my-secret-token-12345` header should be automatically injected!

## What's Next?

- Read the full [README.md](README.md) for detailed configuration options
- Check out [examples/](examples/) for more use cases
- Configure cross-namespace access for shared secrets
- Adjust cache TTL based on your secret rotation frequency

## Troubleshooting

**Plugin not loading?**
- Check Traefik logs: `kubectl logs -n traefik deployment/traefik`
- Verify the plugin version exists in GitHub releases

**Headers not appearing?**
- Verify RBAC: `kubectl auth can-i get secrets --as=system:serviceaccount:traefik:traefik`
- Check middleware is referenced correctly in your route
- Ensure secret and key names are correct

**Permission errors?**
- Ensure Traefik pod uses the `traefik` ServiceAccount
- Verify RoleBinding is in the same namespace as your secrets
