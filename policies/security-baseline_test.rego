package containerization.security

# ==============================================================================
# Security Baseline Policy Tests
# ==============================================================================
#
# Run with: opa test policies/
#
# These tests verify that the security baseline policy correctly identifies
# security violations, warnings, and allows compliant configurations.
#
# ==============================================================================

# ==============================================================================
# DOCKERFILE TESTS - block-root-user
# ==============================================================================

test_block_root_user_explicit if {
	some v in result.violations with input as {"content": `
FROM node:20-alpine
USER root
CMD ["node", "app.js"]
`}
	v.rule == "block-root-user"
}

test_block_root_user_uid_zero if {
	some v in result.violations with input as {"content": `
FROM node:20-alpine
USER 0
CMD ["node", "app.js"]
`}
	v.rule == "block-root-user"
}

test_allow_nonroot_user if {
	result.allow with input as {"content": `
FROM node:20-alpine
USER node
HEALTHCHECK CMD curl --fail http://localhost:8080/health || exit 1
CMD ["node", "app.js"]
`}
}

# ==============================================================================
# DOCKERFILE TESTS - require-user-directive
# ==============================================================================

test_require_user_directive_missing if {
	count(result.warnings) > 0 with input as {"content": `
FROM node:20-alpine
CMD ["node", "app.js"]
`}
}

test_require_user_directive_present if {
	r := result with input as {"content": `
FROM node:20-alpine
USER node
CMD ["node", "app.js"]
`}
	count([w | some w in r.warnings; w.rule == "require-user-directive"]) == 0
}

# ==============================================================================
# DOCKERFILE TESTS - block-secrets-in-env
# ==============================================================================

test_block_secrets_password if {
	some v in result.violations with input as {"content": `
FROM node:20-alpine
ENV PASSWORD=mysecretpassword
CMD ["node", "app.js"]
`}
	v.rule == "block-secrets-in-env"
}

test_block_secrets_api_key if {
	some v in result.violations with input as {"content": `
FROM node:20-alpine
ENV API_KEY=sk_test_123456789
CMD ["node", "app.js"]
`}
	v.rule == "block-secrets-in-env"
}

test_block_secrets_token if {
	some v in result.violations with input as {"content": `
FROM node:20-alpine
ENV AUTH_TOKEN=bearer_token_here
CMD ["node", "app.js"]
`}
	v.rule == "block-secrets-in-env"
}

test_allow_non_secret_env if {
	r := result with input as {"content": `
FROM node:20-alpine
ENV NODE_ENV=production
ENV PORT=8080
CMD ["node", "app.js"]
`}
	count([v | some v in r.violations; v.rule == "block-secrets-in-env"]) == 0
}

# ==============================================================================
# KUBERNETES TESTS - block-privileged
# ==============================================================================

test_block_privileged_container if {
	some v in result.violations with input as {"content": `
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  containers:
  - name: app
    image: nginx:latest
    securityContext:
      privileged: true
`}
	v.rule == "block-privileged"
}

test_allow_non_privileged_container if {
	result.allow with input as {"content": `
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  securityContext:
    runAsNonRoot: true
  containers:
  - name: app
    image: nginx:latest
    securityContext:
      allowPrivilegeEscalation: false
`}
}

# ==============================================================================
# KUBERNETES TESTS - block-host-network
# ==============================================================================

test_block_host_network if {
	some v in result.violations with input as {"content": `
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  hostNetwork: true
  containers:
  - name: app
    image: nginx:latest
`}
	v.rule == "block-host-network"
}

test_allow_pod_network if {
	result.allow with input as {"content": `
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  containers:
  - name: app
    image: nginx:latest
    securityContext:
      runAsNonRoot: true
`}
}

# ==============================================================================
# QUALITY TESTS - require-healthcheck
# ==============================================================================

test_warn_missing_healthcheck if {
	count(result.warnings) > 0 with input as {"content": `
FROM node:20-alpine
USER node
CMD ["node", "app.js"]
`}
}

test_allow_with_healthcheck if {
	result.allow with input as {"content": `
FROM node:20-alpine
USER node
HEALTHCHECK CMD curl --fail http://localhost:8080/health || exit 1
CMD ["node", "app.js"]
`}
}

# ==============================================================================
# QUALITY TESTS - avoid-apt-upgrade
# ==============================================================================

test_warn_apt_upgrade if {
	count(result.warnings) > 0 with input as {"content": `
FROM ubuntu:22.04
RUN apt-get update && apt-get upgrade -y
USER node
`}
}

test_warn_apt_dist_upgrade if {
	count(result.warnings) > 0 with input as {"content": `
FROM ubuntu:22.04
RUN apt-get update && apt-get dist-upgrade -y
USER node
`}
}

test_allow_apt_install_without_upgrade if {
	result.allow with input as {"content": `
FROM ubuntu:22.04
RUN apt-get update && apt-get install -y curl
USER node
HEALTHCHECK CMD curl --fail http://localhost:8080/health || exit 1
`}
}

# ==============================================================================
# INTEGRATION TESTS
# ==============================================================================

test_multiple_violations if {
	count(result.violations) >= 2 with input as {"content": `
FROM node:20-alpine
USER root
ENV PASSWORD=secret123
CMD ["node", "app.js"]
`}
}

test_compliant_dockerfile if {
	result.allow with input as {"content": `
FROM node:20-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
USER node
HEALTHCHECK CMD curl --fail http://localhost:8080/health || exit 1
EXPOSE 8080
CMD ["node", "app.js"]
`}
}

test_compliant_kubernetes_manifest if {
	result.allow with input as {"content": `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
spec:
  replicas: 3
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
    spec:
      securityContext:
        runAsNonRoot: true
      containers:
      - name: app
        image: myapp:1.0.0
        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
        resources:
          limits:
            cpu: 500m
            memory: 512Mi
          requests:
            cpu: 250m
            memory: 256Mi
`}
}
