# Plugin Catalog Status and Workarounds

## The Real Issue

When you use `experimental.plugins`, Traefik queries the **Traefik Plugin Catalog API** (plugins.traefik.io), which acts as a whitelist/registry. Your plugin isn't in that catalog yet because:

1. The catalog polls GitHub once per day
2. It needs to validate your plugin structure
3. It can take 24-48 hours for new plugins to appear

## Better Solution: Wait for Plugin Catalog

The **cleanest and recommended approach** is to wait for the plugin to appear in the catalog (usually within 24 hours). Once it's there, the configuration you tried will work perfectly:

```yaml
experimental:
  plugins:
    k8s-secret-header:
      moduleName: github.com/ImDevinC/traefik-k8s-secret-header
      version: v1.0.0
```

**Check if your plugin is ready:**
- Visit: https://plugins.traefik.io/plugins
- Search for "kubernetes secret" or "ImDevinC"
- Or check: https://plugins.traefik.io/api/plugins

## Why LocalPlugins is Actually Not That Bad

While local mode seems cumbersome, it's actually the **official way** to test plugins before they're published. The init container approach is a one-time setup:

**Advantages:**
- Works immediately without waiting
- Same experience as production will be
- No external dependencies on catalog availability
- Good for development and testing

**One-time Setup:**
Just add this to your Traefik deployment once:

```yaml
initContainers:
- name: plugin-loader
  image: alpine/git
  command: [sh, -c]
  args:
  - |
    mkdir -p /plugins-local/src/github.com/ImDevinC
    cd /plugins-local/src/github.com/ImDevinC
    git clone --depth 1 --branch v1.0.0 https://github.com/ImDevinC/traefik-k8s-secret-header.git
  volumeMounts:
  - name: plugins
    mountPath: /plugins-local

containers:
- name: traefik
  volumeMounts:
  - name: plugins
    mountPath: /plugins-local
    readOnly: true

volumes:
- name: plugins
  emptyDir: {}
```

And in your config:
```yaml
experimental:
  localPlugins:
    k8s-secret-header:
      moduleName: github.com/ImDevinC/traefik-k8s-secret-header
```

## Alternative: Use Helm

If you're using the official Traefik Helm chart, it's even simpler with local mode:

```yaml
# values.yaml
experimental:
  plugins:
    enabled: true

additionalArguments:
  - --experimental.localPlugins.k8s-secret-header.moduleName=github.com/ImDevinC/traefik-k8s-secret-header

deployment:
  initContainers:
    - name: plugin-loader
      image: alpine/git
      command:
        - sh
        - -c
        - |
          mkdir -p /plugins-local/src/github.com/ImDevinC/traefik-k8s-secret-header
          git clone --depth 1 https://github.com/ImDevinC/traefik-k8s-secret-header.git /plugins-local/src/github.com/ImDevinC/traefik-k8s-secret-header
      volumeMounts:
        - name: plugins
          mountPath: /plugins-local

volumes:
  - name: plugins
    emptyDir: {}

additionalVolumeMounts:
  - name: plugins
    mountPath: /plugins-local
```

Then:
```bash
helm upgrade --install traefik traefik/traefik -f values.yaml -n traefik
```

## Check Plugin Catalog Status

You can manually check if the catalog has picked up your plugin:

```bash
# Check if it's listed
curl -s "https://plugins.traefik.io/api/plugins" | jq '.[] | select(.name | contains("secret"))'

# Or check for your username
curl -s "https://plugins.traefik.io/api/plugins" | jq '.[] | select(.owner == "ImDevinC")'
```

## Monitoring for Catalog Inclusion

The catalog will create an issue in your GitHub repo if there are any problems. Check:
https://github.com/ImDevinC/traefik-k8s-secret-header/issues

If no issues appear and it's been 24+ hours, the plugin should be available.

## Timeline

**Current Status:**
- ✅ Plugin code published to GitHub
- ✅ Tag v1.0.0 created
- ✅ `traefik-plugin` topic added
- ✅ Module available in Go proxy
- ⏳ Waiting for Traefik Plugin Catalog to index (0-48 hours)

**When Catalog Indexes Your Plugin:**
You can then remove the init container and switch to the simple config:
```yaml
experimental:
  plugins:
    k8s-secret-header:
      moduleName: github.com/ImDevinC/traefik-k8s-secret-header
      version: v1.0.0
```

## Recommendation

**For immediate use:** Use local mode with the init container (one-time setup)
**For production:** Wait 24-48 hours for catalog indexing, then use the simple plugin config

The init container approach is actually the standard development workflow and is used by many Traefik plugin developers during testing.
