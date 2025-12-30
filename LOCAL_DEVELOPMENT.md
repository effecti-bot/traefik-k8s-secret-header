# Local Development and Testing

Since the Traefik Plugin Catalog polls GitHub only once per day, you can use local mode to test the plugin immediately.

## Local Mode Setup

### Step 1: Prepare Local Plugin Directory

Traefik expects plugins in a specific directory structure. Create this in your Traefik working directory:

```bash
mkdir -p plugins-local/src/github.com/ImDevinC
```

### Step 2: Clone the Plugin

```bash
cd plugins-local/src/github.com/ImDevinC
git clone https://github.com/ImDevinC/traefik-k8s-secret-header.git
```

Your directory structure should now look like:
```
./plugins-local/
    └── src/
        └── github.com/
            └── ImDevinC/
                └── traefik-k8s-secret-header/
                    ├── k8s_secret_header.go
                    ├── .traefik.yml
                    ├── go.mod
                    ├── vendor/
                    └── ...
```

### Step 3: Update Traefik Static Configuration

Change from `plugins` to `localPlugins` in your Traefik configuration:

#### For Helm Values:
```yaml
# Static configuration
experimental:
  localPlugins:
    k8s-secret-header:
      moduleName: github.com/ImDevinC/traefik-k8s-secret-header

# Ensure the plugins-local directory is mounted
volumes:
  - name: plugins-local
    hostPath:
      path: /path/to/plugins-local
      type: Directory

volumeMounts:
  - name: plugins-local
    mountPath: /plugins-local
```

#### For Static Configuration File (traefik.yml):
```yaml
experimental:
  localPlugins:
    k8s-secret-header:
      moduleName: github.com/ImDevinC/traefik-k8s-secret-header
```

#### For Command Line:
```bash
--experimental.localPlugins.k8s-secret-header.moduleName=github.com/ImDevinC/traefik-k8s-secret-header
```

### Step 4: Kubernetes Deployment

If running Traefik in Kubernetes, you need to make the plugins-local directory available to the pod.

#### Option A: ConfigMap (for development)

Create a ConfigMap with your plugin code:
```bash
kubectl create configmap traefik-plugin-k8s-secret-header \
  --from-file=plugins-local/src/github.com/ImDevinC/traefik-k8s-secret-header \
  -n traefik
```

Then mount it in your Traefik deployment:
```yaml
spec:
  template:
    spec:
      volumes:
      - name: plugins
        configMap:
          name: traefik-plugin-k8s-secret-header
      containers:
      - name: traefik
        volumeMounts:
        - name: plugins
          mountPath: /plugins-local/src/github.com/ImDevinC/traefik-k8s-secret-header
```

#### Option B: Init Container (recommended)

Add an init container to clone the plugin at startup:
```yaml
spec:
  template:
    spec:
      volumes:
      - name: plugins
        emptyDir: {}
      
      initContainers:
      - name: plugin-installer
        image: alpine/git:latest
        command:
        - sh
        - -c
        - |
          mkdir -p /plugins-local/src/github.com/ImDevinC
          cd /plugins-local/src/github.com/ImDevinC
          git clone https://github.com/ImDevinC/traefik-k8s-secret-header.git
        volumeMounts:
        - name: plugins
          mountPath: /plugins-local
      
      containers:
      - name: traefik
        volumeMounts:
        - name: plugins
          mountPath: /plugins-local
          readOnly: true
```

### Step 5: Dynamic Configuration (Unchanged)

The middleware configuration remains the same:
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

## Complete Kubernetes Example with Init Container

Here's a complete example of a Traefik deployment with the plugin loaded via init container:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: traefik-config
  namespace: traefik
data:
  traefik.yml: |
    experimental:
      localPlugins:
        k8s-secret-header:
          moduleName: github.com/ImDevinC/traefik-k8s-secret-header
    
    entryPoints:
      web:
        address: ":80"
    
    providers:
      kubernetesCRD: {}

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: traefik
  namespace: traefik
spec:
  replicas: 1
  selector:
    matchLabels:
      app: traefik
  template:
    metadata:
      labels:
        app: traefik
    spec:
      serviceAccountName: traefik
      
      volumes:
      - name: config
        configMap:
          name: traefik-config
      - name: plugins
        emptyDir: {}
      
      initContainers:
      - name: plugin-installer
        image: alpine/git:latest
        command:
        - sh
        - -c
        - |
          echo "Installing plugin..."
          mkdir -p /plugins-local/src/github.com/ImDevinC
          cd /plugins-local/src/github.com/ImDevinC
          git clone --depth 1 --branch v1.0.0 https://github.com/ImDevinC/traefik-k8s-secret-header.git
          echo "Plugin installed successfully"
          ls -la /plugins-local/src/github.com/ImDevinC/traefik-k8s-secret-header/
        volumeMounts:
        - name: plugins
          mountPath: /plugins-local
      
      containers:
      - name: traefik
        image: traefik:v3.0
        args:
        - --configFile=/config/traefik.yml
        ports:
        - name: web
          containerPort: 80
        volumeMounts:
        - name: config
          mountPath: /config
        - name: plugins
          mountPath: /plugins-local
          readOnly: true
```

## Verification

After deploying, check Traefik logs to verify the plugin loaded:

```bash
kubectl logs -n traefik deployment/traefik | grep -i plugin
```

You should see:
```
[k8s-secret-header] Plugin 'secret-auth-header' initialized: secret=default/basic-auth-header key=token header=Authorization ttl=300s
```

## Switching to Plugin Catalog Later

Once the plugin appears in the Traefik Plugin Catalog (within 24 hours), you can switch back to the standard configuration:

```yaml
experimental:
  plugins:  # Changed from localPlugins
    k8s-secret-header:
      moduleName: github.com/ImDevinC/traefik-k8s-secret-header
      version: v1.0.0
```

And remove the init container and volume mounts.

## Troubleshooting

### Plugin not loading
- Check that the directory structure matches exactly: `plugins-local/src/github.com/ImDevinC/traefik-k8s-secret-header/`
- Verify the working directory is where Traefik expects it (usually `/`)
- Check Traefik logs for detailed error messages

### Init container fails
- Check init container logs: `kubectl logs -n traefik deployment/traefik -c plugin-installer`
- Verify network connectivity to GitHub
- Ensure the volume mount path is correct

### Permission errors
- Ensure the RBAC is properly configured (see examples/rbac/rbac.yaml)
- Verify the ServiceAccount is attached to the Traefik pod
- Check that the namespace in RoleBinding matches your setup
