# Deployment Guide

Container Kit supports multiple deployment modes including standalone, containerized, and Kubernetes deployments. This guide covers deployment strategies, configuration, and best practices.

## Deployment Modes

### 1. Standalone Deployment
**Use Case**: Development, testing, and single-server deployments

```bash
# Build Container Kit
make mcp

# Run standalone server
./container-kit-mcp --mode=standalone --port=8080

# Or with configuration file
./container-kit-mcp --config=config.yaml
```

### 2. Containerized Deployment
**Use Case**: Production deployments with Docker

```dockerfile
# Dockerfile for Container Kit
FROM golang:1.24.1-alpine AS builder

WORKDIR /app
COPY . .
RUN make mcp

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/container-kit-mcp .
COPY --from=builder /app/config/production.yaml ./config.yaml

EXPOSE 8080
CMD ["./container-kit-mcp", "--config=config.yaml"]
```

### 3. Kubernetes Deployment
**Use Case**: Scalable production deployments

```yaml
# k8s/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: container-kit
  namespace: container-kit
spec:
  replicas: 3
  selector:
    matchLabels:
      app: container-kit
  template:
    metadata:
      labels:
        app: container-kit
    spec:
      containers:
      - name: container-kit
        image: container-kit:latest
        ports:
        - containerPort: 8080
        env:
        - name: CONTAINER_KIT_MODE
          value: "server"
        - name: CONTAINER_KIT_DB_PATH
          value: "/data/sessions.db"
        volumeMounts:
        - name: data
          mountPath: /data
        - name: workspace
          mountPath: /workspace
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: container-kit-data
      - name: workspace
        emptyDir: {}
```

## Configuration Management

### Configuration File Structure
```yaml
# config/production.yaml
server:
  host: "0.0.0.0"
  port: 8080
  mode: "server"  # "chat", "workflow", "dual"
  
database:
  type: "boltdb"
  path: "/data/sessions.db"
  
workspace:
  root: "/workspace"
  max_size: "10GB"
  cleanup_interval: "1h"
  
security:
  file_access:
    max_file_size: "10MB"
    blocked_extensions: [".exe", ".bat", ".ps1"]
    workspace_isolation: true
  
  scanning:
    enabled: true
    scanners: ["trivy"]
    fail_on_critical: true
  
logging:
  level: "info"
  format: "json"
  output: "stdout"
  
metrics:
  enabled: true
  port: 9090
  path: "/metrics"
  
tracing:
  enabled: true
  endpoint: "http://jaeger:14268/api/traces"
  
docker:
  host: "unix:///var/run/docker.sock"
  api_version: "1.41"
  
kubernetes:
  config_path: "/etc/kubeconfig"
  namespace: "default"
```

### Environment Variables
```bash
# Core Configuration
export CONTAINER_KIT_HOST=0.0.0.0
export CONTAINER_KIT_PORT=8080
export CONTAINER_KIT_MODE=server

# Database Configuration
export CONTAINER_KIT_DB_TYPE=boltdb
export CONTAINER_KIT_DB_PATH=/data/sessions.db

# Workspace Configuration
export CONTAINER_KIT_WORKSPACE_ROOT=/workspace
export CONTAINER_KIT_WORKSPACE_MAX_SIZE=10GB

# Security Configuration
export CONTAINER_KIT_SECURITY_ENABLED=true
export CONTAINER_KIT_SCANNING_ENABLED=true

# Logging Configuration
export CONTAINER_KIT_LOG_LEVEL=info
export CONTAINER_KIT_LOG_FORMAT=json

# Docker Configuration
export DOCKER_HOST=unix:///var/run/docker.sock

# Kubernetes Configuration
export KUBECONFIG=/etc/kubeconfig
```

## Production Deployment

### Docker Compose Setup
```yaml
# docker-compose.yaml
version: '3.8'

services:
  container-kit:
    build: .
    ports:
      - "8080:8080"
      - "9090:9090"  # Metrics
    volumes:
      - ./data:/data
      - ./workspace:/workspace
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      - CONTAINER_KIT_DB_PATH=/data/sessions.db
      - CONTAINER_KIT_WORKSPACE_ROOT=/workspace
      - CONTAINER_KIT_LOG_LEVEL=info
    depends_on:
      - prometheus
      - jaeger
    restart: unless-stopped
    
  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9091:9090"
    volumes:
      - ./monitoring/prometheus.yml:/etc/prometheus/prometheus.yml
    restart: unless-stopped
    
  jaeger:
    image: jaegertracing/all-in-one:latest
    ports:
      - "16686:16686"
      - "14268:14268"
    restart: unless-stopped
    
  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    volumes:
      - ./monitoring/grafana:/etc/grafana/provisioning
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
    restart: unless-stopped

volumes:
  data:
  workspace:
```

### Kubernetes Production Setup
```yaml
# k8s/namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: container-kit

---
# k8s/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: container-kit-config
  namespace: container-kit
data:
  config.yaml: |
    server:
      host: "0.0.0.0"
      port: 8080
      mode: "server"
    database:
      type: "boltdb"
      path: "/data/sessions.db"
    workspace:
      root: "/workspace"
      max_size: "50GB"
    security:
      file_access:
        max_file_size: "100MB"
        workspace_isolation: true
      scanning:
        enabled: true
        scanners: ["trivy"]

---
# k8s/pvc.yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: container-kit-data
  namespace: container-kit
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 20Gi

---
# k8s/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: container-kit-service
  namespace: container-kit
spec:
  selector:
    app: container-kit
  ports:
    - name: http
      port: 80
      targetPort: 8080
    - name: metrics
      port: 9090
      targetPort: 9090
  type: ClusterIP

---
# k8s/ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: container-kit-ingress
  namespace: container-kit
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
spec:
  tls:
  - hosts:
    - container-kit.example.com
    secretName: container-kit-tls
  rules:
  - host: container-kit.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: container-kit-service
            port:
              number: 80
```

## High Availability Setup

### Multi-Instance Deployment
```yaml
# k8s/deployment-ha.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: container-kit
  namespace: container-kit
spec:
  replicas: 5
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 1
  selector:
    matchLabels:
      app: container-kit
  template:
    metadata:
      labels:
        app: container-kit
    spec:
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            podAffinityTerm:
              labelSelector:
                matchExpressions:
                - key: app
                  operator: In
                  values:
                  - container-kit
              topologyKey: kubernetes.io/hostname
      containers:
      - name: container-kit
        image: container-kit:latest
        ports:
        - containerPort: 8080
        - containerPort: 9090
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 60
          periodSeconds: 20
        resources:
          requests:
            memory: "512Mi"
            cpu: "500m"
          limits:
            memory: "1Gi"
            cpu: "1000m"
```

### Load Balancer Configuration
```yaml
# k8s/hpa.yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: container-kit-hpa
  namespace: container-kit
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: container-kit
  minReplicas: 3
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

## Security Hardening

### Pod Security Standards
```yaml
# k8s/pod-security.yaml
apiVersion: v1
kind: Pod
metadata:
  name: container-kit
  namespace: container-kit
spec:
  securityContext:
    runAsNonRoot: true
    runAsUser: 1000
    runAsGroup: 1000
    fsGroup: 1000
  containers:
  - name: container-kit
    image: container-kit:latest
    securityContext:
      allowPrivilegeEscalation: false
      readOnlyRootFilesystem: true
      capabilities:
        drop:
        - ALL
    volumeMounts:
    - name: tmp
      mountPath: /tmp
    - name: data
      mountPath: /data
  volumes:
  - name: tmp
    emptyDir: {}
  - name: data
    persistentVolumeClaim:
      claimName: container-kit-data
```

### Network Policies
```yaml
# k8s/network-policy.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: container-kit-network-policy
  namespace: container-kit
spec:
  podSelector:
    matchLabels:
      app: container-kit
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: ingress-nginx
    ports:
    - protocol: TCP
      port: 8080
  egress:
  - to: []
    ports:
    - protocol: TCP
      port: 443  # HTTPS
    - protocol: TCP
      port: 53   # DNS
    - protocol: UDP
      port: 53   # DNS
```

## Monitoring and Observability

### Health Checks
```go
func (s *Server) setupHealthChecks() {
    http.HandleFunc("/health", s.healthHandler)
    http.HandleFunc("/ready", s.readinessHandler)
    http.HandleFunc("/metrics", promhttp.Handler())
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
    status := map[string]interface{}{
        "status": "healthy",
        "timestamp": time.Now(),
        "version": s.version,
        "uptime": time.Since(s.startTime),
    }
    
    // Check service health
    if err := s.serviceContainer.HealthCheck(); err != nil {
        status["status"] = "unhealthy"
        status["error"] = err.Error()
        w.WriteHeader(http.StatusServiceUnavailable)
    }
    
    json.NewEncoder(w).Encode(status)
}
```

### Prometheus Metrics
```yaml
# monitoring/prometheus.yml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'container-kit'
    static_configs:
      - targets: ['container-kit:9090']
    metrics_path: /metrics
    scrape_interval: 5s
    
  - job_name: 'kubernetes-pods'
    kubernetes_sd_configs:
      - role: pod
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
        action: keep
        regex: true
```

## Backup and Recovery

### Database Backup
```bash
#!/bin/bash
# backup-sessions.sh

DB_PATH="/data/sessions.db"
BACKUP_DIR="/backups"
DATE=$(date +%Y%m%d_%H%M%S)

# Create backup
cp "$DB_PATH" "$BACKUP_DIR/sessions_$DATE.db"

# Compress backup
gzip "$BACKUP_DIR/sessions_$DATE.db"

# Clean old backups (keep 30 days)
find "$BACKUP_DIR" -name "sessions_*.db.gz" -mtime +30 -delete
```

### Workspace Backup
```bash
#!/bin/bash
# backup-workspace.sh

WORKSPACE_DIR="/workspace"
BACKUP_DIR="/backups/workspace"
DATE=$(date +%Y%m%d_%H%M%S)

# Create compressed backup
tar -czf "$BACKUP_DIR/workspace_$DATE.tar.gz" -C "$WORKSPACE_DIR" .

# Clean old backups
find "$BACKUP_DIR" -name "workspace_*.tar.gz" -mtime +7 -delete
```

### Kubernetes Backup
```yaml
# k8s/backup-cronjob.yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: container-kit-backup
  namespace: container-kit
spec:
  schedule: "0 2 * * *"  # Daily at 2 AM
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: backup
            image: alpine:latest
            command:
            - /bin/sh
            - -c
            - |
              cp /data/sessions.db /backups/sessions_$(date +%Y%m%d_%H%M%S).db
              gzip /backups/sessions_*.db
              find /backups -name "sessions_*.db.gz" -mtime +30 -delete
            volumeMounts:
            - name: data
              mountPath: /data
            - name: backups
              mountPath: /backups
          volumes:
          - name: data
            persistentVolumeClaim:
              claimName: container-kit-data
          - name: backups
            persistentVolumeClaim:
              claimName: container-kit-backups
          restartPolicy: OnFailure
```

## Troubleshooting

### Common Issues

1. **Permission Denied on Docker Socket**
   ```bash
   # Add user to docker group
   sudo usermod -aG docker $USER
   
   # Or run with elevated privileges
   sudo ./container-kit-mcp
   ```

2. **Database Lock Error**
   ```bash
   # Check for multiple instances
   ps aux | grep container-kit-mcp
   
   # Stop conflicting instances
   pkill container-kit-mcp
   ```

3. **Workspace Permission Issues**
   ```bash
   # Fix workspace permissions
   sudo chown -R 1000:1000 /workspace
   sudo chmod -R 755 /workspace
   ```

### Debug Mode
```bash
# Enable debug logging
export CONTAINER_KIT_LOG_LEVEL=debug

# Run with verbose output
./container-kit-mcp --debug --verbose
```

### Performance Tuning
```yaml
# Production optimizations
server:
  max_connections: 1000
  read_timeout: "30s"
  write_timeout: "30s"
  
workspace:
  max_concurrent_operations: 100
  cache_size: "1GB"
  
database:
  max_batch_size: 1000
  max_batch_delay: "10ms"
```

## Migration Guide

### Version Upgrades
```bash
# Backup before upgrade
./backup-sessions.sh

# Stop service
systemctl stop container-kit

# Update binary
cp container-kit-mcp /usr/local/bin/

# Migrate database if needed
./container-kit-mcp migrate --from=v1.0 --to=v2.0

# Start service
systemctl start container-kit
```

### Configuration Migration
```bash
# Convert old config format
./container-kit-mcp config migrate \
  --from=config-v1.yaml \
  --to=config-v2.yaml
```

## Related Documentation

- [Architecture Overview](../../architecture/overview.md)
- [Security Guide](security.md)
- [Monitoring Guide](monitoring.md)
- [Performance Guide](performance.md)
- [Configuration Reference](../../reference/configuration.md)

Container Kit's deployment options provide flexibility for various environments while maintaining security, reliability, and performance standards.