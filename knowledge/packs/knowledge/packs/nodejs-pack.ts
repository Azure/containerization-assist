export default [
  {
    "id": "node-package-cache",
    "category": "dockerfile",
    "pattern": "COPY package\\.json",
    "recommendation": "Copy package.json and package-lock.json first, then install dependencies for better Docker layer caching",
    "example": "COPY package*.json ./\nRUN npm ci --only=production\nCOPY . .",
    "severity": "medium",
    "tags": [
      "caching",
      "fix-dockerfile",
      "generate-dockerfile",
      "node",
      "npm",
      "npm-ci",
      "optimization"
    ],
    "description": "Proper dependency caching improves build times significantly"
  },
  {
    "id": "node-npm-ci",
    "category": "dockerfile",
    "pattern": "npm install",
    "recommendation": "Use 'npm ci' instead of 'npm install' in production builds for faster, reliable installs",
    "example": "RUN npm ci --only=production && npm cache clean --force",
    "severity": "medium",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "node",
      "npm",
      "npm-ci",
      "performance",
      "production"
    ],
    "description": "npm ci installs directly from package-lock.json and is faster than npm install"
  },
  {
    "id": "node-cache-clean",
    "category": "dockerfile",
    "pattern": "npm (ci|install)",
    "recommendation": "Clean npm cache after installation to reduce image size",
    "example": "RUN npm ci --only=production && npm cache clean --force",
    "severity": "low",
    "tags": [
      "cleanup",
      "fix-dockerfile",
      "generate-dockerfile",
      "node",
      "npm",
      "npm-ci",
      "size"
    ],
    "description": "npm cache can add significant size to final image"
  },
  {
    "id": "node-health-check",
    "category": "dockerfile",
    "pattern": "EXPOSE 3000",
    "recommendation": "Add health check for Node.js applications",
    "example": "HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \\\n  CMD curl -f http://localhost:3000/health || exit 1",
    "severity": "medium",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "health",
      "monitoring",
      "node"
    ],
    "description": "Health checks help container orchestrators manage application lifecycle"
  },
  {
    "id": "node-process-signals",
    "category": "dockerfile",
    "pattern": "CMD.*node",
    "recommendation": "Use proper signal handling for graceful shutdowns in Node.js",
    "example": "CMD [\"node\", \"--enable-source-maps\", \"dist/index.js\"]",
    "severity": "medium",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "node",
      "production",
      "signals"
    ],
    "description": "Proper signal handling ensures graceful shutdowns and resource cleanup"
  },
  {
    "id": "node-source-maps",
    "category": "dockerfile",
    "pattern": "node.*\\.js",
    "recommendation": "Enable source maps in production Node.js containers for better debugging",
    "example": "CMD [\"node\", \"--enable-source-maps\", \"dist/index.js\"]",
    "severity": "low",
    "tags": [
      "debugging",
      "fix-dockerfile",
      "generate-dockerfile",
      "node",
      "production"
    ],
    "description": "Source maps help with debugging production issues"
  },
  {
    "id": "node-yarn-cache",
    "category": "dockerfile",
    "pattern": "yarn install",
    "recommendation": "Use yarn install --frozen-lockfile for deterministic builds",
    "example": "COPY yarn.lock package.json ./\nRUN yarn install --frozen-lockfile --production",
    "severity": "medium",
    "tags": [
      "deterministic",
      "fix-dockerfile",
      "generate-dockerfile",
      "node",
      "yarn"
    ],
    "description": "Frozen lockfile prevents unexpected dependency updates"
  },
  {
    "id": "express-port-env",
    "category": "dockerfile",
    "pattern": "EXPOSE 3000",
    "recommendation": "Use environment variable for port configuration in Express apps",
    "example": "ENV PORT=3000\nEXPOSE $PORT",
    "severity": "low",
    "tags": [
      "configuration",
      "express",
      "fix-dockerfile",
      "flexibility",
      "generate-dockerfile"
    ],
    "description": "Environment-based port configuration increases deployment flexibility"
  },
  {
    "id": "node-memory-limit",
    "category": "dockerfile",
    "pattern": "CMD.*node",
    "recommendation": "Set Node.js memory limit to prevent OOM issues in containers",
    "example": "CMD [\"node\", \"--max-old-space-size=512\", \"dist/index.js\"]",
    "severity": "medium",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "memory",
      "node",
      "performance"
    ],
    "description": "Setting memory limits prevents out-of-memory crashes in production"
  },
  {
    "id": "typescript-build-stage",
    "category": "dockerfile",
    "pattern": "FROM.*node.*typescript",
    "recommendation": "Use multi-stage builds for TypeScript applications to exclude dev dependencies",
    "example": "FROM node:18-alpine AS build\nRUN npm ci\nRUN npm run build\n\nFROM node:18-alpine AS production\nCOPY --from=build /app/dist ./dist",
    "severity": "medium",
    "tags": [
      "build-stage",
      "fix-dockerfile",
      "generate-dockerfile",
      "multistage",
      "npm-ci",
      "production",
      "runtime-stage",
      "typescript"
    ],
    "description": "Separating build and runtime stages reduces final image size"
  },
  {
    "id": "nestjs-production",
    "category": "dockerfile",
    "pattern": "nest.*build",
    "recommendation": "Optimize NestJS production builds with proper compilation flags",
    "example": "RUN npm run build\nCMD [\"node\", \"dist/main\"]",
    "severity": "medium",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "nestjs",
      "production",
      "typescript"
    ],
    "description": "NestJS requires transpilation for production deployment"
  },
  {
    "id": "nextjs-standalone",
    "category": "dockerfile",
    "pattern": "next.*build",
    "recommendation": "Use Next.js standalone output for optimal container deployment",
    "example": "# In next.config.js: output: 'standalone'\nCOPY --from=deps /app/.next/standalone ./\nCOPY --from=deps /app/.next/static ./.next/static\nCMD [\"node\", \"server.js\"]",
    "severity": "high",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "nextjs",
      "optimization",
      "standalone"
    ],
    "description": "Standalone output reduces Next.js container size by 90%"
  },
  {
    "id": "node-security-updates",
    "category": "security",
    "pattern": "FROM node:",
    "recommendation": "Use specific Node.js versions and regularly update for security patches",
    "example": "FROM node:18.17.1-alpine",
    "severity": "high",
    "tags": [
      "fix-dockerfile",
      "node",
      "scan-image",
      "security",
      "versions"
    ],
    "description": "Specific versions prevent unexpected changes and security vulnerabilities"
  },
  {
    "id": "node-non-root-user",
    "category": "security",
    "pattern": "FROM node:",
    "recommendation": "Node.js official images include 'node' user - use it instead of root",
    "example": "USER node\nWORKDIR /home/node/app",
    "severity": "high",
    "tags": [
      "fix-dockerfile",
      "node",
      "scan-image",
      "security",
      "user"
    ],
    "description": "Node.js images provide a built-in non-root user"
  },
  {
    "id": "npm-audit-security",
    "category": "security",
    "pattern": "npm (install|ci)",
    "recommendation": "Run npm audit during build to catch security vulnerabilities",
    "example": "RUN npm ci --only=production && npm audit --audit-level high",
    "severity": "medium",
    "tags": [
      "audit",
      "fix-dockerfile",
      "npm",
      "npm-ci",
      "scan-image",
      "security"
    ],
    "description": "npm audit identifies known security vulnerabilities in dependencies"
  },
  {
    "id": "node-environment-production",
    "category": "dockerfile",
    "pattern": "FROM node:",
    "recommendation": "Set NODE_ENV=production for optimal Node.js performance",
    "example": "ENV NODE_ENV=production",
    "severity": "medium",
    "tags": [
      "environment",
      "fix-dockerfile",
      "generate-dockerfile",
      "node",
      "performance"
    ],
    "description": "Production environment enables optimizations and disables dev features"
  },
  {
    "id": "pnpm-cache-optimization",
    "category": "dockerfile",
    "pattern": "pnpm install",
    "recommendation": "Use pnpm with proper caching for faster builds and smaller images",
    "example": "COPY pnpm-lock.yaml package.json ./\nRUN pnpm install --frozen-lockfile --prod",
    "severity": "medium",
    "tags": [
      "caching",
      "fix-dockerfile",
      "generate-dockerfile",
      "performance",
      "pnpm"
    ],
    "description": "pnpm offers better caching and deduplication than npm"
  },
  {
    "id": "node-graceful-shutdown",
    "category": "dockerfile",
    "pattern": "CMD.*node",
    "recommendation": "Implement graceful shutdown handlers in Node.js applications",
    "example": "# In app code: process.on('SIGTERM', gracefulShutdown)\nCMD [\"node\", \"server.js\"]",
    "severity": "medium",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "graceful-shutdown",
      "node",
      "signals"
    ],
    "description": "Graceful shutdowns prevent data loss and connection issues"
  },
  {
    "id": "node-dumb-init",
    "category": "dockerfile",
    "pattern": "CMD.*node",
    "recommendation": "Consider using dumb-init for proper signal handling in containers",
    "example": "RUN apk add --no-cache dumb-init\nENTRYPOINT [\"dumb-init\", \"--\"]\nCMD [\"node\", \"server.js\"]",
    "severity": "low",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "init",
      "node",
      "signals"
    ],
    "description": "dumb-init ensures proper signal propagation to Node.js processes"
  },
  {
    "id": "node-logging-stdout",
    "category": "dockerfile",
    "pattern": "FROM node:",
    "recommendation": "Configure Node.js applications to log to stdout for container best practices",
    "example": "# Use console.log/console.error or winston with console transport",
    "severity": "medium",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "logging",
      "node",
      "stdout"
    ],
    "description": "Container logs should go to stdout/stderr for proper log aggregation"
  },
  {
    "id": "typescript-compilation-optimization",
    "category": "dockerfile",
    "pattern": "tsc.*build",
    "recommendation": "Optimize TypeScript compilation with proper configuration and caching",
    "example": "# Use build cache\nRUN npm run build -- --incremental\n# Or with tsc directly\nRUN npx tsc --build --force",
    "severity": "medium",
    "tags": [
      "caching",
      "compilation",
      "fix-dockerfile",
      "generate-dockerfile",
      "performance",
      "typescript"
    ],
    "description": "TypeScript incremental compilation reduces build times in containers"
  },
  {
    "id": "typescript-type-checking-ci",
    "category": "dockerfile",
    "pattern": "typescript|tsc",
    "recommendation": "Add TypeScript type checking as a separate build step for CI/CD validation",
    "example": "RUN npm run type-check\n# In package.json: \"type-check\": \"tsc --noEmit\"",
    "severity": "medium",
    "tags": [
      "ci",
      "fix-dockerfile",
      "generate-dockerfile",
      "type-checking",
      "typescript",
      "validation"
    ],
    "description": "Separate type checking catches errors early without affecting runtime builds"
  },
  {
    "id": "deno-runtime-patterns",
    "category": "dockerfile",
    "pattern": "FROM.*deno",
    "recommendation": "Use Deno with proper security permissions and dependency caching",
    "example": "FROM denoland/deno:alpine\nWORKDIR /app\nCOPY deps.ts .\nRUN deno cache deps.ts\nCOPY . .\nCMD [\"deno\", \"run\", \"--allow-net\", \"--allow-read\", \"main.ts\"]",
    "severity": "medium",
    "tags": [
      "caching",
      "deno",
      "fix-dockerfile",
      "generate-dockerfile",
      "permissions",
      "security"
    ],
    "description": "Deno requires explicit permissions and benefits from dependency pre-caching"
  },
  {
    "id": "bun-runtime-optimization",
    "category": "dockerfile",
    "pattern": "FROM.*bun|bun install",
    "recommendation": "Optimize Bun runtime for fast startup and efficient dependency management",
    "example": "FROM oven/bun:alpine\nWORKDIR /app\nCOPY package.json bun.lockb ./\nRUN bun install --frozen-lockfile --production\nCOPY . .\nCMD [\"bun\", \"run\", \"start\"]",
    "severity": "medium",
    "tags": [
      "bun",
      "dependencies",
      "fix-dockerfile",
      "generate-dockerfile",
      "performance",
      "startup"
    ],
    "description": "Bun offers faster package installation and JavaScript execution"
  },
  {
    "id": "pnpm-advanced-caching",
    "category": "dockerfile",
    "pattern": "pnpm",
    "recommendation": "Use pnpm with advanced caching strategies for optimal Docker layer efficiency",
    "example": "# Copy pnpm files for dependency caching\nCOPY pnpm-lock.yaml package.json .npmrc ./\n# Use pnpm fetch for better caching\nRUN pnpm fetch\nCOPY . .\nRUN pnpm install --offline --frozen-lockfile --prod",
    "severity": "medium",
    "tags": [
      "caching",
      "fix-dockerfile",
      "generate-dockerfile",
      "layers",
      "optimization",
      "pnpm"
    ],
    "description": "pnpm fetch enables better Docker layer caching for dependencies"
  },
  {
    "id": "npm-audit-security-scanning",
    "category": "security",
    "pattern": "npm (install|ci)",
    "recommendation": "Integrate npm audit and Snyk security scanning in container builds",
    "example": "RUN npm ci --only=production\n# Security scanning\nRUN npm audit --audit-level=moderate\n# Optional: Snyk scanning\n# RUN npx snyk test --severity-threshold=medium",
    "severity": "high",
    "tags": [
      "audit",
      "fix-dockerfile",
      "npm",
      "npm-ci",
      "scan-image",
      "security",
      "snyk",
      "vulnerability"
    ],
    "description": "Security scanning identifies vulnerable dependencies before deployment"
  },
  {
    "id": "esm-module-support",
    "category": "dockerfile",
    "pattern": "\"type\":\\s*\"module\"",
    "recommendation": "Configure proper ESM module support with Node.js flags and package.json settings",
    "example": "# In package.json: \"type\": \"module\"\nCMD [\"node\", \"--experimental-modules\", \"--es-module-specifier-resolution=node\", \"index.js\"]",
    "severity": "medium",
    "tags": [
      "configuration",
      "esm",
      "fix-dockerfile",
      "generate-dockerfile",
      "modules",
      "node"
    ],
    "description": "ESM modules require specific Node.js configuration for proper execution"
  },
  {
    "id": "turborepo-monorepo-optimization",
    "category": "dockerfile",
    "pattern": "turbo|turborepo",
    "recommendation": "Optimize Turborepo monorepo builds with proper pruning and caching",
    "example": "# Use turbo prune for focused builds\nRUN npx turbo prune --scope=@myorg/web --docker\n# Copy pruned package.json files\nCOPY --from=pruned /app/out/json/ .\nRUN npm ci\n# Copy source and build\nCOPY --from=pruned /app/out/full/ .\nRUN npx turbo run build --filter=@myorg/web",
    "severity": "medium",
    "tags": [
      "caching",
      "fix-dockerfile",
      "generate-dockerfile",
      "monorepo",
      "npm-ci",
      "pruning",
      "turborepo"
    ],
    "description": "Turborepo pruning reduces Docker context size for focused builds"
  },
  {
    "id": "nx-monorepo-optimization",
    "category": "dockerfile",
    "pattern": "nx|@nrwl",
    "recommendation": "Use Nx affected commands and caching for efficient monorepo container builds",
    "example": "# Install dependencies\nRUN npm ci\n# Build only affected projects\nRUN npx nx affected:build --prod --base=origin/main\n# Or build specific project\nRUN npx nx build myapp --prod",
    "severity": "medium",
    "tags": [
      "affected",
      "caching",
      "fix-dockerfile",
      "generate-dockerfile",
      "monorepo",
      "npm-ci",
      "nx"
    ],
    "description": "Nx affected builds optimize CI/CD by building only changed projects"
  },
  {
    "id": "node-inspector-debug",
    "category": "dockerfile",
    "pattern": "NODE_ENV.*development",
    "recommendation": "Enable Node.js inspector for debugging in development containers",
    "example": "# Development debugging\nEXPOSE 9229\nCMD [\"node\", \"--inspect=0.0.0.0:9229\", \"index.js\"]",
    "severity": "low",
    "tags": [
      "debugging",
      "development",
      "fix-dockerfile",
      "generate-dockerfile",
      "inspector",
      "node"
    ],
    "description": "Node.js inspector enables remote debugging in development containers"
  },
  {
    "id": "node-heap-snapshot",
    "category": "dockerfile",
    "pattern": "CMD.*node",
    "recommendation": "Configure Node.js heap snapshot generation for memory leak debugging",
    "example": "CMD [\"node\", \"--heapsnapshot-signal=SIGUSR2\", \"--max-old-space-size=${NODE_MEMORY_LIMIT:-512}\", \"index.js\"]",
    "severity": "low",
    "tags": [
      "debugging",
      "fix-dockerfile",
      "generate-dockerfile",
      "heap",
      "memory",
      "node"
    ],
    "description": "Heap snapshots help diagnose memory leaks in production"
  },
  {
    "id": "node-cluster-mode",
    "category": "dockerfile",
    "pattern": "CMD.*node",
    "recommendation": "Consider cluster mode for CPU-intensive Node.js applications",
    "example": "# Use PM2 for cluster mode\nRUN npm install -g pm2\nCOPY ecosystem.config.js .\nCMD [\"pm2-runtime\", \"ecosystem.config.js\"]",
    "severity": "low",
    "tags": [
      "cluster",
      "fix-dockerfile",
      "generate-dockerfile",
      "node",
      "performance",
      "pm2"
    ],
    "description": "Cluster mode utilizes multiple CPU cores for better performance"
  },
  {
    "id": "node-experimental-features",
    "category": "dockerfile",
    "pattern": "CMD.*node",
    "recommendation": "Use stable Node.js features and avoid experimental flags in production",
    "example": "# Avoid in production:\n# CMD [\"node\", \"--experimental-*\", \"index.js\"]\n# Prefer stable features:\nCMD [\"node\", \"--enable-source-maps\", \"index.js\"]",
    "severity": "medium",
    "tags": [
      "experimental",
      "fix-dockerfile",
      "generate-dockerfile",
      "node",
      "production",
      "stability"
    ],
    "description": "Experimental Node.js features may be unstable in production environments"
  },
  {
    "id": "node-worker-threads",
    "category": "dockerfile",
    "pattern": "worker_threads",
    "recommendation": "Configure proper resource limits for Node.js worker threads",
    "example": "ENV UV_THREADPOOL_SIZE=${THREAD_POOL_SIZE:-4}\nCMD [\"node\", \"--max-old-space-size=${NODE_MEMORY_LIMIT:-512}\", \"index.js\"]",
    "severity": "medium",
    "tags": [
      "concurrency",
      "fix-dockerfile",
      "generate-dockerfile",
      "node",
      "performance",
      "worker-threads"
    ],
    "description": "Worker threads require proper resource configuration for optimal performance"
  }
] as const;
