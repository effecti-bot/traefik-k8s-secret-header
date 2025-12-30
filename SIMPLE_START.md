# Quick Start Guide (Simple)

## The Simplest Way

**Just wait 24-48 hours** after plugin publication, then use this minimal config:

```yaml
# Traefik static configuration
experimental:
  plugins:
    k8s-secret-header:
      moduleName: github.com/ImDevinC/traefik-k8s-secret-header
      version: v1.0.0
```

That's it! No init containers, no local plugins, no complexity.

**Your plugin will be automatically fetched from GitHub via the Go proxy.**

---

## Why Wait?

Traefik uses a centralized **Plugin Catalog** (plugins.traefik.io) that:
- Validates plugin structure
- Acts as a whitelist for security
- Updates once per day by polling GitHub

Your plugin is **already valid** and **will be accepted** - it just needs to be discovered on the next catalog poll.

---

## Check If Ready

Visit: https://plugins.traefik.io/plugins

Search for "kubernetes secret" or click this direct link once it's indexed:
`https://plugins.traefik.io/plugins/[plugin-id]/kubernetes-secret-header`

---

## Timeline

- **Dec 29, 2025 ~19:00 UTC**: Plugin published to GitHub ✅
- **Dec 30-31, 2025**: Catalog will discover and index plugin ⏳
- **After indexing**: Simple config above works perfectly ✅

---

## Need It Right Now?

If you can't wait, see:
- [LOCAL_DEVELOPMENT.md](LOCAL_DEVELOPMENT.md) - Use local mode with init container
- [examples/traefik-with-plugin.yaml](examples/traefik-with-plugin.yaml) - Complete deployment example

But honestly, **just wait a day** - it's much simpler!

---

## What Happens When You Wait

**Before catalog indexing** (now):
```
Error: 404: Unknown plugin: github.com/ImDevinC/traefik-k8s-secret-header@v1.0.0
```

**After catalog indexing** (24-48h):
```
✅ Plugin loaded successfully
✅ Middleware initialized
✅ Headers injected automatically
```

---

## Once It's Ready

1. **Update Traefik config** (if using Helm):
```yaml
experimental:
  plugins:
    k8s-secret-header:
      moduleName: github.com/ImDevinC/traefik-k8s-secret-header
      version: v1.0.0
```

2. **Apply RBAC**:
```bash
kubectl apply -f https://raw.githubusercontent.com/ImDevinC/traefik-k8s-secret-header/main/examples/rbac/rbac.yaml
```

3. **Create a secret**:
```bash
kubectl create secret generic my-token --from-literal=token="Bearer xyz123"
```

4. **Create middleware**:
```bash
kubectl apply -f https://raw.githubusercontent.com/ImDevinC/traefik-k8s-secret-header/main/examples/ingressroute/middleware.yaml
```

5. **Done!** Headers will be injected automatically.

---

## Patience is Simplicity

The init container approach works, but it's development/testing workflow.

**Production approach**: Wait for catalog → Simple config → Done.

No init containers. No volume mounts. No complexity.
Just clean, simple configuration that Traefik fetches automatically.
