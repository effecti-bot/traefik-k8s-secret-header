# Quick Test Without Init Containers

The unfortunate truth: **There is NO way to pull plugins directly from GitHub without either:**
1. Waiting for Plugin Catalog approval (24-48h)
2. Using local mode (requires plugin files in the container)

## Why This Limitation Exists

Traefik's plugin system has two modes:

**Mode 1: `experimental.plugins`** (Remote)
- Queries Traefik Plugin Catalog API
- Catalog validates and whitelists plugins
- Only works for catalog-approved plugins
- **This is what you want, but can't use yet**

**Mode 2: `experimental.localPlugins`** (Local)
- Reads plugin from local filesystem
- Requires plugin files in `/plugins-local/`
- **This is what you're stuck with until catalog approval**

## The Absolute Simplest Workaround

Since you MUST use local mode, here's the least painful way:

### Option A: Single-File Mount (Simplest)

Download just the plugin code and mount it:

```bash
# 1. Download plugin
curl -o /tmp/k8s_secret_header.go https://raw.githubusercontent.com/ImDevinC/traefik-k8s-secret-header/v1.0.0/k8s_secret_header.go

# 2. Create ConfigMap
kubectl create configmap traefik-plugin-code \
  --from-file=k8s_secret_header.go=/tmp/k8s_secret_header.go \
  --from-file=go.mod=<(curl -s https://raw.githubusercontent.com/ImDevinC/traefik-k8s-secret-header/v1.0.0/go.mod) \
  --from-file=.traefik.yml=<(curl -s https://raw.githubusercontent.com/ImDevinC/traefik-k8s-secret-header/v1.0.0/.traefik.yml) \
  -n traefik

# 3. Patch Traefik to mount it
kubectl patch deployment traefik -n traefik --patch '
spec:
  template:
    spec:
      volumes:
      - name: plugin
        configMap:
          name: traefik-plugin-code
      containers:
      - name: traefik
        volumeMounts:
        - name: plugin
          mountPath: /plugins-local/src/github.com/ImDevinC/traefik-k8s-secret-header
'
```

**Note**: This won't work because the plugin needs vendored dependencies too (2800+ files). ConfigMap has size limits.

### Option B: Accept Init Container Reality

The init container IS actually the standard approach. Here's why it's not as bad as it seems:

**One-time setup** - Add this to your Traefik deployment ONCE:

```yaml
initContainers:
- name: plugin
  image: alpine/git
  command: [sh, -c, "git clone --depth 1 https://github.com/ImDevinC/traefik-k8s-secret-header.git /plugins/src/github.com/ImDevinC/traefik-k8s-secret-header"]
  volumeMounts:
  - {name: plugins, mountPath: /plugins}

volumes:
- name: plugins
  emptyDir: {}
```

Then in Traefik config:
```yaml
experimental:
  localPlugins:
    k8s-secret-header:
      moduleName: github.com/ImDevinC/traefik-k8s-secret-header
```

**That's it.** Pod restarts are fast because git clone is quick.

### Option C: Wait 24 Hours

Honestly, this is the cleanest option. Your plugin meets all requirements. Check tomorrow and use:

```yaml
experimental:
  plugins:
    k8s-secret-header:
      moduleName: github.com/ImDevinC/traefik-k8s-secret-header
      version: v1.0.0
```

No init containers, no local mode, no complexity.

## Why No Direct GitHub Fetch?

**Security.** Traefik doesn't want to allow arbitrary code execution from any GitHub repo. The Plugin Catalog acts as a security gate that:
- Validates plugin structure
- Checks for malicious code patterns
- Maintains a whitelist of trusted plugins

You can't bypass this by design.

## Bottom Line

Your only real options are:

1. **Init container** (~20 lines of YAML, works immediately)
2. **Wait 24h** (0 lines of YAML, cleanest solution)

Pick your poison based on urgency. Both are legitimate approaches used by Traefik plugin developers.

## Checking Catalog Status

```bash
# Check if your plugin has been indexed yet
curl -s https://plugins.traefik.io/api/plugins | jq '.[] | select(.author == "ImDevinC")'
```

Once it returns data, you're good to go with the simple config.
