# Template Injection Examples

This guide demonstrates how to use policy-driven template injection to automatically add organizational standards to generated artifacts.

## Overview

Template injection allows you to:
- Add CA certificates to all Docker images
- Inject observability agents (New Relic, DataDog) in production
- Add security hardening (non-root users, read-only filesystems)
- Inject sidecars (log forwarding, proxies) into Kubernetes manifests
- Add init containers (database migrations, secret fetching)

## Example 1: CA Certificate Installation

### Policy File: `policies/org-ca-certs.rego`

```rego
package containerization.templates

import rego.v1

# CA certificate template (applies to all environments and languages)
ca_cert_template := {
  "id": "org-ca-certificates",
  "section": "security",
  "description": "Install organization CA certificates for internal TLS",
  "content": `# Install organization CA certificates
COPY certs/org-ca.crt /usr/local/share/ca-certificates/org-ca.crt
RUN update-ca-certificates`,
  "priority": 100
}

dockerfile_templates contains ca_cert_template

templates := {
  "dockerfile": [template | template := dockerfile_templates[_]],
  "kubernetes": []
}
```

### Usage

```bash
# Generate Dockerfile with CA cert template
containerization-assist generate-dockerfile \
  --repository-path ./my-app \
  --language python \
  --environment production
```

### Output

The generated Dockerfile plan will include:

```json
{
  "recommendations": {
    "securityConsiderations": [
      {
        "id": "org-ca-certificates",
        "category": "dockerfile-template-security",
        "recommendation": "Install organization CA certificates for internal TLS\n\n# Install organization CA certificates\nCOPY certs/org-ca.crt /usr/local/share/ca-certificates/org-ca.crt\nRUN update-ca-certificates",
        "policyDriven": true,
        "matchScore": 100
      }
    ]
  }
}
```

## Example 2: Environment-Specific Observability

### Policy File: `policies/observability.rego`

```rego
package containerization.templates

import rego.v1

# Java observability (production/staging only)
java_apm_template := {
  "id": "java-new-relic",
  "section": "observability",
  "description": "Install New Relic Java agent for APM monitoring",
  "content": `# Install New Relic Java agent
ENV NEW_RELIC_APP_NAME="{{APP_NAME}}"
ENV NEW_RELIC_LICENSE_KEY="{{LICENSE_KEY}}"
ADD https://download.newrelic.com/newrelic/java-agent/newrelic-agent/current/newrelic-java.zip /tmp/newrelic.zip
RUN unzip /tmp/newrelic.zip -d /opt && rm /tmp/newrelic.zip`,
  "priority": 90,
  "conditions": {
    "languages": ["java"],
    "environments": ["production", "staging"]
  }
}

# Node.js observability (production/staging only)
node_apm_template := {
  "id": "node-datadog",
  "section": "observability",
  "description": "Install DataDog APM for Node.js monitoring",
  "content": `# Install DataDog APM
ENV DD_AGENT_HOST="datadog-agent"
ENV DD_SERVICE="{{APP_NAME}}"
ENV DD_ENV="{{ENVIRONMENT}}"
RUN npm install --save dd-trace`,
  "priority": 90,
  "conditions": {
    "languages": ["node", "javascript", "typescript"],
    "environments": ["production", "staging"]
  }
}

dockerfile_templates contains java_apm_template if {
  input.language == "java"
  input.environment in {"production", "staging"}
}

dockerfile_templates contains node_apm_template if {
  input.language in {"node", "javascript", "typescript"}
  input.environment in {"production", "staging"}
}

templates := {
  "dockerfile": [template | template := dockerfile_templates[_]],
  "kubernetes": []
}
```

### Usage

```bash
# Production Java app - includes New Relic
containerization-assist generate-dockerfile \
  --repository-path ./java-api \
  --language java \
  --environment production

# Development Java app - no observability agent
containerization-assist generate-dockerfile \
  --repository-path ./java-api \
  --language java \
  --environment development

# Production Node.js app - includes DataDog
containerization-assist generate-dockerfile \
  --repository-path ./node-api \
  --language node \
  --environment production
```

## Example 3: Kubernetes Sidecar Injection

### Policy File: `policies/k8s-sidecars.rego`

```rego
package containerization.templates

import rego.v1

# Log forwarding sidecar (production/staging only)
log_forwarder_sidecar := {
  "id": "log-forwarder",
  "type": "sidecar",
  "description": "Fluentd sidecar for centralized log aggregation",
  "spec": {
    "name": "log-forwarder",
    "image": "fluent/fluentd:v1.16-1",
    "volumeMounts": [
      {
        "name": "app-logs",
        "mountPath": "/var/log/app"
      }
    ],
    "env": [
      {
        "name": "FLUENT_ELASTICSEARCH_HOST",
        "value": "elasticsearch.logging.svc.cluster.local"
      },
      {
        "name": "FLUENT_ELASTICSEARCH_PORT",
        "value": "9200"
      }
    ]
  },
  "priority": 90,
  "conditions": {
    "environments": ["production", "staging"]
  }
}

# Shared log volume
log_volume := {
  "id": "app-logs-volume",
  "type": "volume",
  "description": "Shared volume for application logs",
  "spec": {
    "name": "app-logs",
    "emptyDir": {}
  },
  "priority": 90,
  "conditions": {
    "environments": ["production", "staging"]
  }
}

kubernetes_templates contains log_forwarder_sidecar if {
  input.environment in {"production", "staging"}
}

kubernetes_templates contains log_volume if {
  input.environment in {"production", "staging"}
}

templates := {
  "dockerfile": [],
  "kubernetes": [template | template := kubernetes_templates[_]]
}
```

### Usage

```bash
# Production manifest - includes log sidecar
containerization-assist generate-k8s-manifests \
  --app-name my-api \
  --language node \
  --environment production

# Development manifest - no sidecar
containerization-assist generate-k8s-manifests \
  --app-name my-api \
  --language node \
  --environment development
```

## Example 4: Complete Organization Template Policy

### Policy File: `policies/org-templates.rego`

```rego
package containerization.templates

import rego.v1

# ===== DOCKERFILE TEMPLATES =====

# Always include CA certificates
ca_cert_template := {
  "id": "org-ca-certs",
  "section": "security",
  "description": "Install organization CA certificates",
  "content": `COPY certs/org-ca.crt /usr/local/share/ca-certificates/org-ca.crt
RUN update-ca-certificates`,
  "priority": 100
}

dockerfile_templates contains ca_cert_template

# Production security hardening
security_hardening := {
  "id": "org-security-hardening",
  "section": "security",
  "description": "Apply security hardening: non-root user, read-only filesystem",
  "content": `RUN useradd -r -u 1001 -g root appuser
USER appuser`,
  "priority": 80,
  "conditions": {
    "environments": ["production"]
  }
}

dockerfile_templates contains security_hardening if {
  input.environment == "production"
}

# ===== KUBERNETES TEMPLATES =====

# Organization secrets (all environments)
org_secrets_volume := {
  "id": "org-secrets-volume",
  "type": "volume",
  "description": "Mount organization secrets",
  "spec": {
    "name": "org-secrets",
    "secret": {
      "secretName": "org-secrets"
    }
  },
  "priority": 90
}

kubernetes_templates contains org_secrets_volume

org_secrets_mount := {
  "id": "org-secrets-mount",
  "type": "volumeMount",
  "description": "Mount org-secrets to /etc/secrets",
  "spec": {
    "name": "org-secrets",
    "mountPath": "/etc/secrets",
    "readOnly": true
  },
  "priority": 90
}

kubernetes_templates contains org_secrets_mount

# ===== EXPORT =====

templates := {
  "dockerfile": [template | template := dockerfile_templates[_]],
  "kubernetes": [template | template := kubernetes_templates[_]]
}
```

## Testing Templates

### Unit Test Your Policy

```bash
# Create test file: policies/org-templates_test.rego
cat > policies/org-templates_test.rego << 'EOF'
package containerization.templates

import rego.v1

test_ca_cert_always_included if {
  result := templates with input as {"language": "python", "environment": "development"}
  count(result.dockerfile) > 0
  some template in result.dockerfile
  template.id == "org-ca-certs"
}

test_security_hardening_production_only if {
  # Production should have hardening
  prod_result := templates with input as {"language": "python", "environment": "production"}
  some template in prod_result.dockerfile
  template.id == "org-security-hardening"

  # Development should not
  dev_result := templates with input as {"language": "python", "environment": "development"}
  not some template in dev_result.dockerfile
    template.id == "org-security-hardening"
}
EOF

# Run OPA tests
opa test policies/org-templates.rego policies/org-templates_test.rego
```

## Debugging Templates

### Enable Debug Logging

```bash
export LOG_LEVEL=debug
containerization-assist generate-dockerfile --repository-path ./my-app --language node
```

### Inspect Policy Query Result

```bash
# Query the policy directly
echo '{"language": "node", "environment": "production"}' | \
  opa eval \
  -d policies/org-templates.rego \
  -f json \
  'data.containerization.templates.templates'
```

## Best Practices

1. **Keep templates focused**: One concern per template (security, observability, compliance)
2. **Use conditions wisely**: Target templates to specific languages/environments
3. **Set appropriate priorities**: Higher priority = appears first in recommendations
4. **Test thoroughly**: Write OPA unit tests for all conditional logic
5. **Document placeholders**: Use `{{PLACEHOLDER}}` syntax for values that need replacement
6. **Version your templates**: Include version info in template descriptions
7. **Validate generated output**: Always review the generated recommendations before applying

## Common Use Cases

- **Compliance**: Inject regulatory requirements (HIPAA, PCI-DSS, SOC2)
- **Observability**: Add APM agents, metrics exporters, distributed tracing
- **Security**: Harden images, add vulnerability scanners, enforce least privilege
- **Networking**: Add service mesh sidecars, API gateways, proxies
- **Data**: Mount secrets, configure databases, inject credentials
- **Operations**: Add health checks, readiness probes, graceful shutdown handlers

## Related Documentation

- [Policy Authoring Guide](../guides/policy-authoring.md)
- [Dynamic Defaults Guide](./dynamic-defaults-example.md)
- [Sprint 3 Overview](../sprints/sprint-3.md)
