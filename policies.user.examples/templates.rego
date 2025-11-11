package containerization.templates

# Template Injection Policy
# Sprint 3: Demonstrates organizational template injection for Dockerfiles and K8s manifests
#
# This policy injects standard organizational components:
# - Dockerfile: CA certificates, observability agents, security hardening
# - Kubernetes: Log forwarding sidecars, secret volumes, init containers
#
# Templates are conditionally applied based on language, environment, and framework.

import rego.v1

# ===== DOCKERFILE TEMPLATES =====

# CA Certificate Installation (all languages, all environments)
ca_cert_template := {
	"id": "org-ca-certificates",
	"section": "security",
	"description": "Install organization CA certificates for internal TLS connections",
	"content": `# Install organization CA certificates
COPY certs/org-ca.crt /usr/local/share/ca-certificates/org-ca.crt
RUN update-ca-certificates`,
	"priority": 100,
}

# Observability Agent for Java (production only)
java_observability_template := {
	"id": "org-java-observability",
	"section": "observability",
	"description": "Install New Relic Java agent for APM monitoring",
	"content": `# Install New Relic Java agent
ENV NEW_RELIC_APP_NAME="myapp"
ENV NEW_RELIC_LICENSE_KEY="placeholder"
ADD https://download.newrelic.com/newrelic/java-agent/newrelic-agent/current/newrelic-java.zip /tmp/newrelic.zip
RUN unzip /tmp/newrelic.zip -d /opt && rm /tmp/newrelic.zip`,
	"priority": 90,
	"conditions": {
		"languages": ["java"],
		"environments": ["production", "staging"],
	},
}

# Security Hardening Template (production environments)
security_hardening_template := {
	"id": "org-security-hardening",
	"section": "security",
	"description": "Apply security hardening: non-root user, read-only filesystem",
	"content": `# Security hardening
RUN useradd -r -u 1001 -g root appuser
USER appuser
# Note: Add --read-only flag when running container`,
	"priority": 80,
	"conditions": {
		"environments": ["production"],
	},
}

# Node.js Observability (production only)
node_observability_template := {
	"id": "org-node-observability",
	"section": "observability",
	"description": "Install DataDog APM agent for Node.js monitoring",
	"content": `# Install DataDog APM
ENV DD_AGENT_HOST="datadog-agent"
ENV DD_SERVICE="myapp"
ENV DD_ENV="production"
RUN npm install --save dd-trace`,
	"priority": 90,
	"conditions": {
		"languages": ["node", "javascript", "typescript"],
		"environments": ["production", "staging"],
	},
}

# Collect all applicable Dockerfile templates based on input context
dockerfile_templates contains template if {
	# Always include CA certificates
	template := ca_cert_template
}

dockerfile_templates contains template if {
	# Java observability agent (production/staging only)
	input.language == "java"
	input.environment in {"production", "staging"}
	template := java_observability_template
}

dockerfile_templates contains template if {
	# Node observability agent (production/staging only)
	input.language in {"node", "javascript", "typescript"}
	input.environment in {"production", "staging"}
	template := node_observability_template
}

dockerfile_templates contains template if {
	# Security hardening (production only)
	input.environment == "production"
	template := security_hardening_template
}

# ===== KUBERNETES TEMPLATES =====

# Log Forwarding Sidecar (all apps in production)
log_forwarder_sidecar := {
	"id": "org-log-forwarder",
	"type": "sidecar",
	"description": "Fluentd sidecar for centralized log aggregation",
	"spec": {
		"name": "log-forwarder",
		"image": "fluent/fluentd:v1.16-1",
		"volumeMounts": [{
			"name": "app-logs",
			"mountPath": "/var/log/app",
		}],
		"env": [
			{
				"name": "FLUENT_ELASTICSEARCH_HOST",
				"value": "elasticsearch.logging.svc.cluster.local",
			},
			{
				"name": "FLUENT_ELASTICSEARCH_PORT",
				"value": "9200",
			},
		],
	},
	"priority": 100,
	"conditions": {
		"environments": ["production", "staging"],
	},
}

# Secrets Volume Mount (all apps with secrets)
secrets_volume := {
	"id": "org-secrets-volume",
	"type": "volume",
	"description": "Mount organization secrets from Kubernetes Secret",
	"spec": {
		"name": "org-secrets",
		"secret": {
			"secretName": "org-secrets",
		},
	},
	"priority": 90,
}

secrets_volume_mount := {
	"id": "org-secrets-volume-mount",
	"type": "volumeMount",
	"description": "Mount org-secrets volume to /etc/secrets",
	"spec": {
		"name": "org-secrets",
		"mountPath": "/etc/secrets",
		"readOnly": true,
	},
	"priority": 90,
}

# Database Migration Init Container (Java apps with database)
db_migration_init_container := {
	"id": "org-db-migration",
	"type": "initContainer",
	"description": "Run Flyway database migrations before app starts",
	"spec": {
		"name": "db-migrate",
		"image": "flyway/flyway:latest",
		"command": ["flyway", "migrate"],
		"env": [
			{
				"name": "FLYWAY_URL",
				"valueFrom": {
					"secretKeyRef": {
						"name": "db-credentials",
						"key": "url",
					},
				},
			},
			{
				"name": "FLYWAY_USER",
				"valueFrom": {
					"secretKeyRef": {
						"name": "db-credentials",
						"key": "username",
					},
				},
			},
			{
				"name": "FLYWAY_PASSWORD",
				"valueFrom": {
					"secretKeyRef": {
						"name": "db-credentials",
						"key": "password",
					},
				},
			},
		],
	},
	"priority": 80,
	"conditions": {
		"languages": ["java"],
	},
}

# Collect all applicable Kubernetes templates based on input context
kubernetes_templates contains template if {
	# Log forwarder sidecar (production/staging only)
	input.environment in {"production", "staging"}
	template := log_forwarder_sidecar
}

kubernetes_templates contains template if {
	# Always include org secrets volume
	template := secrets_volume
}

kubernetes_templates contains template if {
	# Always include org secrets volume mount
	template := secrets_volume_mount
}

kubernetes_templates contains template if {
	# DB migration init container (Java apps only)
	input.language == "java"
	template := db_migration_init_container
}

# ===== TEMPLATE QUERY ENTRYPOINT =====

# Query all templates based on input context
# This is called by tools via ctx.queryConfig('containerization.templates', input)
default templates := {}

templates := result if {
	result := {
		"dockerfile": array.concat([], [template | template := dockerfile_templates[_]]),
		"kubernetes": array.concat([], [template | template := kubernetes_templates[_]]),
	}
}
