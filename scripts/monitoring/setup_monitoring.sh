#!/bin/bash

# Container Kit monitoring setup script
echo "=== Container Kit Monitoring Setup ==="

MONITORING_DIR="tools/monitoring"
DOCKER_COMPOSE_FILE="docker-compose.monitoring.yml"
CONFIG_DIR="monitoring/config"

# Configuration
PROMETHEUS_PORT="${PROMETHEUS_PORT:-9090}"
GRAFANA_PORT="${GRAFANA_PORT:-3000}"
JAEGER_PORT="${JAEGER_PORT:-16686}"
ALERTMANAGER_PORT="${ALERTMANAGER_PORT:-9093}"

echo "Setting up monitoring infrastructure..."
echo "Prometheus: http://localhost:$PROMETHEUS_PORT"
echo "Grafana: http://localhost:$GRAFANA_PORT"
echo "Jaeger: http://localhost:$JAEGER_PORT"
echo "Alertmanager: http://localhost:$ALERTMANAGER_PORT"
echo ""

# Create monitoring configuration directory
mkdir -p "$CONFIG_DIR"/{prometheus,grafana,alertmanager}

# Create Prometheus configuration
cat > "$CONFIG_DIR/prometheus/prometheus.yml" << EOF
global:
  scrape_interval: 15s
  evaluation_interval: 15s

rule_files:
  - "/etc/prometheus/rules/*.yml"

alerting:
  alertmanagers:
    - static_configs:
        - targets:
          - alertmanager:9093

scrape_configs:
  - job_name: 'container-kit'
    static_configs:
      - targets: ['host.docker.internal:8080']
    metrics_path: '/metrics'
    scrape_interval: 5s
    scrape_timeout: 5s

  - job_name: 'prometheus'
    static_configs:
      - targets: ['localhost:9090']

  - job_name: 'node-exporter'
    static_configs:
      - targets: ['node-exporter:9100']
EOF

# Create Alertmanager configuration
cat > "$CONFIG_DIR/alertmanager/alertmanager.yml" << EOF
global:
  smtp_smarthost: 'localhost:587'
  smtp_from: 'alertmanager@container-kit.dev'

route:
  group_by: ['alertname']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 1h
  receiver: 'web.hook'

receivers:
  - name: 'web.hook'
    webhook_configs:
      - url: 'http://localhost:5001/'
        send_resolved: true

  - name: 'slack'
    slack_configs:
      - api_url: '\${SLACK_WEBHOOK_URL}'
        channel: '#alerts'
        title: 'Container Kit Alert'
        text: '{{ range .Alerts }}{{ .Annotations.summary }}{{ end }}'

inhibit_rules:
  - source_match:
      severity: 'critical'
    target_match:
      severity: 'warning'
    equal: ['alertname', 'component']
EOF

# Create Grafana datasource configuration
cat > "$CONFIG_DIR/grafana/datasources.yml" << EOF
apiVersion: 1

datasources:
  - name: Prometheus
    type: prometheus
    url: http://prometheus:9090
    isDefault: true
    access: proxy
    editable: true

  - name: Jaeger
    type: jaeger
    url: http://jaeger:16686
    access: proxy
    editable: true
EOF

# Create Grafana dashboard provisioning
cat > "$CONFIG_DIR/grafana/dashboards.yml" << EOF
apiVersion: 1

providers:
  - name: 'Container Kit'
    orgId: 1
    folder: ''
    type: file
    disableDeletion: false
    updateIntervalSeconds: 10
    allowUiUpdates: true
    options:
      path: /var/lib/grafana/dashboards
EOF

# Copy alert rules
cp "$MONITORING_DIR/prometheus_alerts.yml" "$CONFIG_DIR/prometheus/rules/"

# Create Docker Compose file for monitoring stack
cat > "$DOCKER_COMPOSE_FILE" << EOF
version: '3.8'

services:
  prometheus:
    image: prom/prometheus:latest
    container_name: container-kit-prometheus
    ports:
      - "$PROMETHEUS_PORT:9090"
    volumes:
      - ./monitoring/config/prometheus:/etc/prometheus
      - prometheus_data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--web.console.libraries=/etc/prometheus/console_libraries'
      - '--web.console.templates=/etc/prometheus/consoles'
      - '--storage.tsdb.retention.time=200h'
      - '--web.enable-lifecycle'
      - '--web.enable-admin-api'
    restart: unless-stopped

  grafana:
    image: grafana/grafana:latest
    container_name: container-kit-grafana
    ports:
      - "$GRAFANA_PORT:3000"
    volumes:
      - grafana_data:/var/lib/grafana
      - ./monitoring/config/grafana/dashboards.yml:/etc/grafana/provisioning/dashboards/dashboards.yml
      - ./monitoring/config/grafana/datasources.yml:/etc/grafana/provisioning/datasources/datasources.yml
      - ./tools/monitoring:/var/lib/grafana/dashboards
    environment:
      - GF_SECURITY_ADMIN_USER=admin
      - GF_SECURITY_ADMIN_PASSWORD=admin
      - GF_USERS_ALLOW_SIGN_UP=false
    restart: unless-stopped

  jaeger:
    image: jaegertracing/all-in-one:latest
    container_name: container-kit-jaeger
    ports:
      - "$JAEGER_PORT:16686"
      - "14268:14268"
      - "6831:6831/udp"
      - "6832:6832/udp"
    environment:
      - COLLECTOR_ZIPKIN_HOST_PORT=:9411
    restart: unless-stopped

  alertmanager:
    image: prom/alertmanager:latest
    container_name: container-kit-alertmanager
    ports:
      - "$ALERTMANAGER_PORT:9093"
    volumes:
      - ./monitoring/config/alertmanager:/etc/alertmanager
      - alertmanager_data:/alertmanager
    command:
      - '--config.file=/etc/alertmanager/alertmanager.yml'
      - '--storage.path=/alertmanager'
      - '--web.external-url=http://localhost:$ALERTMANAGER_PORT'
    restart: unless-stopped

  node-exporter:
    image: prom/node-exporter:latest
    container_name: container-kit-node-exporter
    ports:
      - "9100:9100"
    volumes:
      - /proc:/host/proc:ro
      - /sys:/host/sys:ro
      - /:/rootfs:ro
    command:
      - '--path.procfs=/host/proc'
      - '--path.rootfs=/rootfs'
      - '--path.sysfs=/host/sys'
      - '--collector.filesystem.mount-points-exclude=^/(sys|proc|dev|host|etc)($$|/)'
    restart: unless-stopped

volumes:
  prometheus_data:
  grafana_data:
  alertmanager_data:

networks:
  default:
    name: container-kit-monitoring
EOF

# Create startup script
cat > "scripts/monitoring/start_monitoring.sh" << 'EOF'
#!/bin/bash

echo "🚀 Starting Container Kit monitoring stack..."

# Check if Docker is running
if ! docker info >/dev/null 2>&1; then
    echo "❌ Docker is not running. Please start Docker first."
    exit 1
fi

# Check if Docker Compose is available
if ! command -v docker-compose >/dev/null 2>&1; then
    echo "❌ docker-compose not found. Please install Docker Compose."
    exit 1
fi

# Start monitoring stack
echo "Starting monitoring services..."
docker-compose -f docker-compose.monitoring.yml up -d

# Wait for services to start
echo "Waiting for services to be ready..."
sleep 10

# Check service health
echo ""
echo "📊 Monitoring Stack Status:"

# Check Prometheus
if curl -s http://localhost:9090/-/healthy >/dev/null 2>&1; then
    echo "✅ Prometheus: http://localhost:9090"
else
    echo "❌ Prometheus: Not responding"
fi

# Check Grafana
if curl -s http://localhost:3000/api/health >/dev/null 2>&1; then
    echo "✅ Grafana: http://localhost:3000 (admin/admin)"
else
    echo "❌ Grafana: Not responding"
fi

# Check Jaeger
if curl -s http://localhost:16686/ >/dev/null 2>&1; then
    echo "✅ Jaeger: http://localhost:16686"
else
    echo "❌ Jaeger: Not responding"
fi

# Check Alertmanager
if curl -s http://localhost:9093/-/healthy >/dev/null 2>&1; then
    echo "✅ Alertmanager: http://localhost:9093"
else
    echo "❌ Alertmanager: Not responding"
fi

echo ""
echo "🎉 Monitoring stack is ready!"
echo ""
echo "📈 Next steps:"
echo "1. Start Container Kit with telemetry enabled:"
echo "   export CONTAINER_KIT_TRACING_ENABLED=true"
echo "   export CONTAINER_KIT_METRICS_ENABLED=true"
echo "   ./container-kit-mcp"
echo ""
echo "2. Generate some traffic:"
echo "   # Run some tool operations to see metrics"
echo ""
echo "3. View metrics:"
echo "   - Grafana Dashboard: http://localhost:3000"
echo "   - Prometheus Metrics: http://localhost:9090"
echo "   - Jaeger Traces: http://localhost:16686"
echo "   - Alertmanager: http://localhost:9093"
EOF

chmod +x scripts/monitoring/start_monitoring.sh

# Create stop script
cat > "scripts/monitoring/stop_monitoring.sh" << 'EOF'
#!/bin/bash

echo "🛑 Stopping Container Kit monitoring stack..."

# Stop and remove containers
docker-compose -f docker-compose.monitoring.yml down

echo "✅ Monitoring stack stopped"
EOF

chmod +x scripts/monitoring/stop_monitoring.sh

# Create monitoring health check script
cat > "scripts/monitoring/check_monitoring.sh" << 'EOF'
#!/bin/bash

echo "🔍 Container Kit Monitoring Health Check"
echo "======================================="

# Check if monitoring containers are running
CONTAINERS=("container-kit-prometheus" "container-kit-grafana" "container-kit-jaeger" "container-kit-alertmanager")

for container in "${CONTAINERS[@]}"; do
    if docker ps --format "table {{.Names}}" | grep -q "$container"; then
        status=$(docker inspect --format='{{.State.Health.Status}}' "$container" 2>/dev/null || echo "running")
        echo "✅ $container: $status"
    else
        echo "❌ $container: not running"
    fi
done

echo ""
echo "🌐 Service Endpoints:"

# Check service health
SERVICES=("Prometheus:9090" "Grafana:3000" "Jaeger:16686" "Alertmanager:9093")

for service in "${SERVICES[@]}"; do
    name=$(echo "$service" | cut -d: -f1)
    port=$(echo "$service" | cut -d: -f2)

    if curl -s "http://localhost:$port" >/dev/null 2>&1; then
        echo "✅ $name: http://localhost:$port"
    else
        echo "❌ $name: http://localhost:$port (not responding)"
    fi
done

echo ""
echo "📊 Metrics Status:"

# Check if Container Kit is exposing metrics
if curl -s http://localhost:8080/metrics >/dev/null 2>&1; then
    metric_count=$(curl -s http://localhost:8080/metrics | grep -c "^container_kit_")
    echo "✅ Container Kit metrics: $metric_count metrics exposed"
else
    echo "❌ Container Kit metrics: not available (is Container Kit running with telemetry enabled?)"
fi

# Check Prometheus targets
if curl -s http://localhost:9090/api/v1/targets >/dev/null 2>&1; then
    targets=$(curl -s http://localhost:9090/api/v1/targets | jq -r '.data.activeTargets | length' 2>/dev/null || echo "unknown")
    echo "✅ Prometheus targets: $targets active"
else
    echo "❌ Prometheus targets: cannot retrieve"
fi

echo ""
echo "🔔 Recent Alerts:"
if curl -s http://localhost:9093/api/v1/alerts >/dev/null 2>&1; then
    alert_count=$(curl -s http://localhost:9093/api/v1/alerts | jq -r '. | length' 2>/dev/null || echo "0")
    echo "📢 Active alerts: $alert_count"
else
    echo "❌ Cannot retrieve alerts"
fi
EOF

chmod +x scripts/monitoring/check_monitoring.sh

echo "✅ Monitoring setup complete!"
echo ""
echo "📁 Files created:"
echo "  - $DOCKER_COMPOSE_FILE"
echo "  - $CONFIG_DIR/ (configuration files)"
echo "  - scripts/monitoring/ (management scripts)"
echo ""
echo "🚀 To start monitoring:"
echo "  ./scripts/monitoring/start_monitoring.sh"
echo ""
echo "🔍 To check monitoring health:"
echo "  ./scripts/monitoring/check_monitoring.sh"
echo ""
echo "🛑 To stop monitoring:"
echo "  ./scripts/monitoring/stop_monitoring.sh"
