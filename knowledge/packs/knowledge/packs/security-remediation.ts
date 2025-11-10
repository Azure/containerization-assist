export default [
  {
    "id": "cve-remediation-base-image-upgrade",
    "category": "security",
    "pattern": "FROM",
    "recommendation": "Update base images to latest patch version to remediate known CVEs in base layers",
    "example": "# Vulnerable base image with CVEs\n# FROM node:18.12.0-alpine\n\n# Remediated: Update to latest patch version\nFROM node:18.19.0-alpine3.19\n\n# Or use digest for reproducibility\nFROM node:18.19.0-alpine3.19@sha256:435dcad253bb5b7f347ebc69c8cc52de7c912eb7241098b920f2fc2d7843183d\n\nWORKDIR /app\nCOPY package*.json ./\nRUN npm ci --only=production\nCOPY . .\nUSER node\nCMD [\"node\", \"server.js\"]",
    "severity": "high",
    "tags": [
      "base-image",
      "cve",
      "fix-dockerfile",
      "npm-ci",
      "remediation",
      "scan-image",
      "security",
      "upgrade"
    ],
    "description": "Regular base image updates patch OS-level vulnerabilities before they're exploited"
  },
  {
    "id": "dependency-vulnerability-npm-audit",
    "category": "security",
    "pattern": "npm install",
    "recommendation": "Use npm audit fix to automatically remediate dependency vulnerabilities",
    "example": "FROM node:20-alpine AS builder\n\nWORKDIR /app\nCOPY package*.json ./\n\n# Audit and fix vulnerabilities\nRUN npm audit --audit-level=high && \\\n    npm audit fix --audit-level=high && \\\n    npm ci --only=production\n\n# Or fail build if high/critical vulnerabilities remain\nRUN npm audit --audit-level=high --production || exit 1\n\nCOPY . .\nRUN npm run build\n\nFROM node:20-alpine\nWORKDIR /app\nCOPY --from=builder /app/dist ./dist\nCOPY --from=builder /app/node_modules ./node_modules\nUSER node\nCMD [\"node\", \"dist/index.js\"]",
    "severity": "high",
    "tags": [
      "audit",
      "dependencies",
      "fix-dockerfile",
      "nodejs",
      "npm",
      "npm-ci",
      "remediation",
      "scan-image"
    ],
    "description": "npm audit fix automatically updates dependencies to non-vulnerable versions"
  },
  {
    "id": "python-safety-check-dependencies",
    "category": "security",
    "pattern": "pip install",
    "recommendation": "Use safety to check and remediate Python dependency vulnerabilities",
    "example": "FROM python:3.11-slim AS builder\n\nWORKDIR /app\n\n# Install safety for vulnerability checking\nRUN pip install --no-cache-dir safety\n\nCOPY requirements.txt .\n\n# Check for vulnerabilities before installing\nRUN safety check -r requirements.txt --continue-on-error || \\\n    (safety check -r requirements.txt --json > /tmp/vulnerabilities.json && exit 1)\n\n# Install dependencies\nRUN pip install --no-cache-dir -r requirements.txt\n\nCOPY . .\n\nFROM python:3.11-slim\nWORKDIR /app\nCOPY --from=builder /usr/local/lib/python3.11/site-packages /usr/local/lib/python3.11/site-packages\nCOPY . .\n\nRUN adduser --disabled-password --gecos '' appuser\nUSER appuser\nCMD [\"python\", \"app.py\"]",
    "severity": "high",
    "tags": [
      "dependencies",
      "fix-dockerfile",
      "pip-install",
      "python",
      "remediation",
      "safety",
      "scan-image",
      "vulnerabilities"
    ],
    "description": "Safety checks Python packages against known vulnerability databases"
  },
  {
    "id": "alpine-apk-upgrade-security",
    "category": "security",
    "pattern": "FROM.*alpine",
    "recommendation": "Run apk upgrade to patch Alpine package vulnerabilities after FROM",
    "example": "FROM node:20-alpine\n\n# Upgrade all packages to latest secure versions\nRUN apk upgrade --no-cache && \\\n    apk add --no-cache \\\n    # Security: Use specific versions when needed\n    ca-certificates=20230506-r0 \\\n    && rm -rf /var/cache/apk/*\n\nWORKDIR /app\nCOPY package*.json ./\nRUN npm ci --only=production\nCOPY . .\nUSER node\nCMD [\"node\", \"server.js\"]",
    "severity": "high",
    "tags": [
      "alpine",
      "apk",
      "fix-dockerfile",
      "npm-ci",
      "remediation",
      "scan-image",
      "security",
      "upgrade"
    ],
    "description": "apk upgrade patches system packages with latest security fixes"
  },
  {
    "id": "debian-apt-security-updates",
    "category": "security",
    "pattern": "FROM.*(debian|ubuntu)",
    "recommendation": "Install security updates from Debian/Ubuntu security repositories",
    "example": "FROM ubuntu:22.04\n\n# Install security updates only\nRUN apt-get update && \\\n    apt-get upgrade -y && \\\n    apt-get install -y --no-install-recommends \\\n    ca-certificates \\\n    curl \\\n    && rm -rf /var/lib/apt/lists/*\n\n# Or install from security repo explicitly\nRUN apt-get update && \\\n    apt-get install -y -t $(lsb_release -cs)-security openssl libssl3 && \\\n    rm -rf /var/lib/apt/lists/*\n\nWORKDIR /app\nCOPY . .\nCMD [\"./app\"]",
    "severity": "high",
    "tags": [
      "apt",
      "canonical",
      "debian",
      "fix-dockerfile",
      "remediation",
      "scan-image",
      "security-updates",
      "ubuntu"
    ],
    "description": "Security repositories provide patches for critical vulnerabilities quickly"
  },
  {
    "id": "openssl-vulnerability-mitigation",
    "category": "security",
    "pattern": "openssl",
    "recommendation": "Update OpenSSL to latest version to remediate critical vulnerabilities like Heartbleed",
    "example": "FROM alpine:latest\n\n# Update OpenSSL to latest secure version\nRUN apk update && \\\n    apk upgrade --no-cache openssl libcrypto3 libssl3 && \\\n    apk add --no-cache ca-certificates && \\\n    rm -rf /var/cache/apk/*\n\n# Verify OpenSSL version\nRUN openssl version | grep -q \"3.1\" || exit 1\n\nWORKDIR /app\nCOPY . .\nCMD [\"./app\"]",
    "severity": "high",
    "tags": [
      "fix-dockerfile",
      "openssl",
      "remediation",
      "scan-image",
      "ssl",
      "tls",
      "vulnerability"
    ],
    "description": "OpenSSL vulnerabilities are frequently discovered and must be patched promptly"
  },
  {
    "id": "log4j-vulnerability-remediation",
    "category": "security",
    "pattern": "log4j",
    "recommendation": "Upgrade Log4j to 2.17.1+ to remediate Log4Shell and related vulnerabilities",
    "example": "# In pom.xml or build.gradle, ensure Log4j version >= 2.17.1\n# Dockerfile verification\nFROM maven:3.9-eclipse-temurin-17 AS builder\n\nWORKDIR /app\nCOPY pom.xml .\n\n# Force Log4j version upgrade\nRUN mvn versions:use-dep-version \\\n    -Dincludes=org.apache.logging.log4j:* \\\n    -DdepVersion=2.21.1 \\\n    -DforceVersion=true\n\nCOPY src ./src\nRUN mvn package -DskipTests\n\n# Verify no vulnerable Log4j versions\nRUN find target -name \"*.jar\" -exec unzip -l {} \\; | \\\n    grep -i \"log4j\" | \\\n    grep -qv \"2.2[1-9]\\|2.[3-9]\" || exit 1\n\nFROM eclipse-temurin:17-jre-alpine\nCOPY --from=builder /app/target/app.jar /app.jar\nUSER 1001\nENTRYPOINT [\"java\", \"-jar\", \"/app.jar\"]",
    "severity": "high",
    "tags": [
      "cve",
      "fix-dockerfile",
      "java",
      "log4j",
      "log4shell",
      "remediation",
      "scan-image"
    ],
    "description": "Log4Shell was one of the most critical vulnerabilities requiring immediate patching"
  },
  {
    "id": "docker-socket-removal",
    "category": "security",
    "pattern": "/var/run/docker.sock",
    "recommendation": "Remove Docker socket mount and use Kaniko or BuildKit for builds instead",
    "example": "# Bad: Mounting Docker socket (security risk)\n# docker run -v /var/run/docker.sock:/var/run/docker.sock myapp\n\n# Good: Use Kaniko for builds without Docker socket\nFROM gcr.io/kaniko-project/executor:latest AS builder\n\nCOPY . /workspace\nWORKDIR /workspace\n\n# Build without Docker daemon\nRUN /kaniko/executor \\\n    --dockerfile=Dockerfile \\\n    --context=dir:///workspace \\\n    --destination=myapp:latest \\\n    --no-push\n\n# Or use BuildKit\n# docker buildx build --load -t myapp:latest .",
    "severity": "high",
    "tags": [
      "buildkit",
      "docker-socket",
      "fix-dockerfile",
      "google",
      "kaniko",
      "remediation",
      "scan-image",
      "security"
    ],
    "description": "Docker socket access grants root-equivalent privileges and should be avoided"
  },
  {
    "id": "secrets-removal-from-env",
    "category": "security",
    "pattern": "ENV.*(PASSWORD|SECRET|KEY|TOKEN)",
    "recommendation": "Move secrets from ENV to Kubernetes Secrets or external secret managers",
    "example": "# Bad: Hardcoded secrets in Dockerfile\n# ENV DATABASE_PASSWORD=mysecret123\n# ENV API_KEY=abc123xyz\n\n# Good: Use Kubernetes Secrets\n# Dockerfile\nFROM node:20-alpine\nWORKDIR /app\nCOPY package*.json ./\nRUN npm ci --only=production\nCOPY . .\nUSER node\nCMD [\"node\", \"server.js\"]\n\n# Kubernetes Deployment\n# apiVersion: apps/v1\n# kind: Deployment\n# spec:\n#   template:\n#     spec:\n#       containers:\n#       - name: app\n#         env:\n#         - name: DATABASE_PASSWORD\n#           valueFrom:\n#             secretKeyRef:\n#               name: db-secrets\n#               key: password\n#         - name: API_KEY\n#           valueFrom:\n#             secretKeyRef:\n#               name: api-secrets\n#               key: key",
    "severity": "high",
    "tags": [
      "aws",
      "env",
      "fix-dockerfile",
      "kubernetes",
      "npm-ci",
      "remediation",
      "scan-image",
      "secrets",
      "security"
    ],
    "description": "Secrets in ENV are visible in image layers and docker inspect output"
  },
  {
    "id": "privilege-escalation-prevention",
    "category": "security",
    "pattern": "allowPrivilegeEscalation",
    "recommendation": "Set allowPrivilegeEscalation to false to prevent privilege escalation attacks",
    "example": "# Kubernetes Deployment with security hardening\napiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: myapp\nspec:\n  template:\n    spec:\n      securityContext:\n        runAsNonRoot: true\n        runAsUser: 1001\n        fsGroup: 1001\n        seccompProfile:\n          type: RuntimeDefault\n      containers:\n      - name: app\n        image: myapp:latest\n        securityContext:\n          # Prevent privilege escalation\n          allowPrivilegeEscalation: false\n          readOnlyRootFilesystem: true\n          runAsNonRoot: true\n          runAsUser: 1001\n          capabilities:\n            drop:\n            - ALL\n            add: []",
    "severity": "high",
    "tags": [
      "fix-dockerfile",
      "kubernetes",
      "privilege-escalation",
      "remediation",
      "scan-image",
      "security-context"
    ],
    "description": "Preventing privilege escalation blocks common container breakout techniques"
  },
  {
    "id": "capability-drop-all",
    "category": "security",
    "pattern": "capabilities:",
    "recommendation": "Drop all Linux capabilities and add only required ones for least privilege",
    "example": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: myapp\nspec:\n  template:\n    spec:\n      containers:\n      - name: app\n        image: myapp:latest\n        securityContext:\n          capabilities:\n            # Drop all capabilities\n            drop:\n            - ALL\n            # Add only required capabilities\n            add:\n            # NET_BIND_SERVICE: Bind to ports < 1024\n            - NET_BIND_SERVICE\n            # Or run on port > 1024 and don't add any capabilities\n        ports:\n        - containerPort: 8080  # > 1024, no capabilities needed",
    "severity": "high",
    "tags": [
      "capabilities",
      "fix-dockerfile",
      "least-privilege",
      "linux",
      "remediation",
      "scan-image",
      "security"
    ],
    "description": "Dropping all capabilities minimizes attack surface and follows least privilege principle"
  },
  {
    "id": "readonly-root-filesystem",
    "category": "security",
    "pattern": "readOnlyRootFilesystem",
    "recommendation": "Enable read-only root filesystem and use emptyDir for writable directories",
    "example": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: myapp\nspec:\n  template:\n    spec:\n      containers:\n      - name: app\n        image: myapp:latest\n        securityContext:\n          readOnlyRootFilesystem: true\n        volumeMounts:\n        # Mount writable directories as needed\n        - name: tmp\n          mountPath: /tmp\n        - name: cache\n          mountPath: /app/.cache\n        - name: logs\n          mountPath: /var/log/app\n      volumes:\n      - name: tmp\n        emptyDir: {}\n      - name: cache\n        emptyDir: {}\n      - name: logs\n        emptyDir: {}",
    "severity": "high",
    "tags": [
      "fix-dockerfile",
      "immutable",
      "readonly-filesystem",
      "remediation",
      "scan-image",
      "security"
    ],
    "description": "Read-only filesystem prevents runtime file modification and malware persistence"
  },
  {
    "id": "seccomp-profile-runtime-default",
    "category": "security",
    "pattern": "seccompProfile:",
    "recommendation": "Apply RuntimeDefault seccomp profile to restrict syscalls and reduce attack surface",
    "example": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: myapp\nspec:\n  template:\n    spec:\n      securityContext:\n        # Pod-level seccomp\n        seccompProfile:\n          type: RuntimeDefault\n      containers:\n      - name: app\n        image: myapp:latest\n        securityContext:\n          # Container-level seccomp\n          seccompProfile:\n            type: RuntimeDefault\n          # Or use custom profile\n          # seccompProfile:\n          #   type: Localhost\n          #   localhostProfile: profiles/myapp-seccomp.json",
    "severity": "medium",
    "tags": [
      "fix-dockerfile",
      "remediation",
      "runtime",
      "scan-image",
      "seccomp",
      "security",
      "syscalls"
    ],
    "description": "Seccomp profiles block dangerous syscalls used in container breakout attacks"
  },
  {
    "id": "apparmor-profile-enforcement",
    "category": "security",
    "pattern": "apparmor",
    "recommendation": "Apply AppArmor profiles to enforce mandatory access control on containers",
    "example": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: myapp\nspec:\n  template:\n    metadata:\n      annotations:\n        # Apply AppArmor profile\n        container.apparmor.security.beta.kubernetes.io/app: runtime/default\n        # Or custom profile\n        # container.apparmor.security.beta.kubernetes.io/app: localhost/myapp-profile\n    spec:\n      containers:\n      - name: app\n        image: myapp:latest\n        securityContext:\n          runAsNonRoot: true\n          runAsUser: 1001\n          allowPrivilegeEscalation: false",
    "severity": "medium",
    "tags": [
      "apparmor",
      "fix-dockerfile",
      "mac",
      "remediation",
      "runtime",
      "scan-image",
      "security"
    ],
    "description": "AppArmor provides mandatory access control limiting file and network access"
  },
  {
    "id": "network-policy-zero-trust",
    "category": "security",
    "pattern": "kind: NetworkPolicy",
    "recommendation": "Implement deny-all default network policy and whitelist only required traffic",
    "example": "# Default deny all ingress and egress\napiVersion: networking.k8s.io/v1\nkind: NetworkPolicy\nmetadata:\n  name: default-deny-all\n  namespace: production\nspec:\n  podSelector: {}  # Applies to all pods\n  policyTypes:\n  - Ingress\n  - Egress\n---\n# Allow specific ingress\napiVersion: networking.k8s.io/v1\nkind: NetworkPolicy\nmetadata:\n  name: myapp-allow-ingress\n  namespace: production\nspec:\n  podSelector:\n    matchLabels:\n      app: myapp\n  policyTypes:\n  - Ingress\n  ingress:\n  # Allow from frontend pods only\n  - from:\n    - podSelector:\n        matchLabels:\n          role: frontend\n    ports:\n    - protocol: TCP\n      port: 8080\n---\n# Allow specific egress\napiVersion: networking.k8s.io/v1\nkind: NetworkPolicy\nmetadata:\n  name: myapp-allow-egress\n  namespace: production\nspec:\n  podSelector:\n    matchLabels:\n      app: myapp\n  policyTypes:\n  - Egress\n  egress:\n  # Allow to database\n  - to:\n    - podSelector:\n        matchLabels:\n          app: postgres\n    ports:\n    - protocol: TCP\n      port: 5432\n  # Allow DNS\n  - to:\n    - namespaceSelector:\n        matchLabels:\n          name: kube-system\n    ports:\n    - protocol: UDP\n      port: 53",
    "severity": "high",
    "tags": [
      "fix-dockerfile",
      "micro-segmentation",
      "network-policy",
      "scan-image",
      "security",
      "zero-trust"
    ],
    "description": "Zero-trust networking prevents lateral movement after container compromise"
  },
  {
    "id": "image-pull-secrets-private-registry",
    "category": "security",
    "pattern": "imagePullSecrets:",
    "recommendation": "Use imagePullSecrets to authenticate with private registries securely",
    "example": "# Create docker registry secret\n# kubectl create secret docker-registry regcred \\\n#   --docker-server=myregistry.azurecr.io \\\n#   --docker-username=myuser \\\n#   --docker-password=mypassword \\\n#   --docker-email=myemail@example.com\n\napiVersion: v1\nkind: ServiceAccount\nmetadata:\n  name: myapp-sa\nimagePullSecrets:\n- name: regcred\n---\napiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: myapp\nspec:\n  template:\n    spec:\n      serviceAccountName: myapp-sa\n      # Or specify directly\n      imagePullSecrets:\n      - name: regcred\n      containers:\n      - name: app\n        image: myregistry.azurecr.io/myapp:latest",
    "severity": "high",
    "tags": [
      "authentication",
      "aws",
      "azure",
      "fix-dockerfile",
      "image-pull-secrets",
      "private-registry",
      "scan-image",
      "security"
    ],
    "description": "imagePullSecrets enable secure access to private container registries"
  },
  {
    "id": "pod-security-admission-restricted",
    "category": "security",
    "pattern": "pod-security.kubernetes.io",
    "recommendation": "Enforce restricted Pod Security Standards at namespace level",
    "example": "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: production\n  labels:\n    # Enforce restricted security standard\n    pod-security.kubernetes.io/enforce: restricted\n    pod-security.kubernetes.io/enforce-version: latest\n    # Audit and warn for violations\n    pod-security.kubernetes.io/audit: restricted\n    pod-security.kubernetes.io/warn: restricted\n---\napiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: myapp\n  namespace: production\nspec:\n  template:\n    spec:\n      # Must comply with restricted standard\n      securityContext:\n        runAsNonRoot: true\n        runAsUser: 1001\n        seccompProfile:\n          type: RuntimeDefault\n      containers:\n      - name: app\n        image: myapp:latest\n        securityContext:\n          allowPrivilegeEscalation: false\n          capabilities:\n            drop:\n            - ALL\n          readOnlyRootFilesystem: true\n          runAsNonRoot: true\n          runAsUser: 1001",
    "severity": "high",
    "tags": [
      "fix-dockerfile",
      "pod-security-admission",
      "pss",
      "remediation",
      "restricted",
      "scan-image",
      "security"
    ],
    "description": "Pod Security Standards enforce security best practices at admission time"
  },
  {
    "id": "service-account-token-projection",
    "category": "security",
    "pattern": "serviceAccountToken:",
    "recommendation": "Use bound service account tokens with limited audience and expiration",
    "example": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: myapp\nspec:\n  template:\n    spec:\n      serviceAccountName: myapp-sa\n      # Disable auto-mounting default token\n      automountServiceAccountToken: false\n      containers:\n      - name: app\n        image: myapp:latest\n        volumeMounts:\n        # Mount projected token with limited scope\n        - name: sa-token\n          mountPath: /var/run/secrets/kubernetes.io/serviceaccount\n          readOnly: true\n      volumes:\n      - name: sa-token\n        projected:\n          sources:\n          - serviceAccountToken:\n              # Token expires after 1 hour\n              expirationSeconds: 3600\n              # Limit token audience\n              audience: api\n              path: token\n          - configMap:\n              name: kube-root-ca.crt\n              items:\n              - key: ca.crt\n                path: ca.crt\n          - downwardAPI:\n              items:\n              - path: namespace\n                fieldRef:\n                  fieldPath: metadata.namespace",
    "severity": "medium",
    "tags": [
      "aws",
      "fix-dockerfile",
      "rbac",
      "remediation",
      "scan-image",
      "security",
      "service-account",
      "token"
    ],
    "description": "Bound tokens with expiration reduce impact of token compromise"
  },
  {
    "id": "rbac-least-privilege",
    "category": "security",
    "pattern": "kind: Role|kind: ClusterRole",
    "recommendation": "Grant minimum RBAC permissions required for application functionality",
    "example": "apiVersion: v1\nkind: ServiceAccount\nmetadata:\n  name: myapp-sa\n  namespace: production\n---\napiVersion: rbac.authorization.k8s.io/v1\nkind: Role\nmetadata:\n  name: myapp-role\n  namespace: production\nrules:\n# Grant only required permissions\n- apiGroups: [\"\"]\n  resources: [\"configmaps\"]\n  verbs: [\"get\", \"list\", \"watch\"]\n  # Limit to specific resources\n  resourceNames: [\"myapp-config\"]\n- apiGroups: [\"\"]\n  resources: [\"secrets\"]\n  verbs: [\"get\"]\n  resourceNames: [\"myapp-secrets\"]\n# Avoid wildcards: \"*\" in apiGroups, resources, or verbs\n---\napiVersion: rbac.authorization.k8s.io/v1\nkind: RoleBinding\nmetadata:\n  name: myapp-binding\n  namespace: production\nroleRef:\n  apiGroup: rbac.authorization.k8s.io\n  kind: Role\n  name: myapp-role\nsubjects:\n- kind: ServiceAccount\n  name: myapp-sa\n  namespace: production",
    "severity": "high",
    "tags": [
      "aws",
      "fix-dockerfile",
      "least-privilege",
      "permissions",
      "rbac",
      "remediation",
      "scan-image",
      "security"
    ],
    "description": "Least privilege RBAC prevents unauthorized access to cluster resources"
  },
  {
    "id": "admission-webhook-policy-enforcement",
    "category": "security",
    "pattern": "ValidatingWebhookConfiguration",
    "recommendation": "Implement validating webhooks to enforce custom security policies",
    "example": "apiVersion: admissionregistration.k8s.io/v1\nkind: ValidatingWebhookConfiguration\nmetadata:\n  name: security-policy-validator\nwebhooks:\n- name: validate.security.example.com\n  clientConfig:\n    service:\n      name: security-validator\n      namespace: security-system\n      path: /validate\n    caBundle: <base64-encoded-ca-cert>\n  rules:\n  - operations: [\"CREATE\", \"UPDATE\"]\n    apiGroups: [\"apps\", \"\"]\n    apiVersions: [\"v1\"]\n    resources: [\"deployments\", \"pods\"]\n  admissionReviewVersions: [\"v1\"]\n  sideEffects: None\n  failurePolicy: Fail  # Reject on webhook failure\n  namespaceSelector:\n    matchLabels:\n      security-enforcement: enabled\n  # Enforce policies:\n  # - Require non-root user\n  # - Require resource limits\n  # - Require security context\n  # - Reject privileged containers\n  # - Enforce image registry whitelist",
    "severity": "high",
    "tags": [
      "admission-webhook",
      "enforcement",
      "fix-dockerfile",
      "policy",
      "scan-image",
      "security",
      "validation"
    ],
    "description": "Admission webhooks enforce organizational security policies at deployment time"
  }
] as const;
