export default [
  {
    "id": "ruby-bundler-cache",
    "category": "dockerfile",
    "pattern": "COPY Gemfile",
    "recommendation": "Copy Gemfile and Gemfile.lock first, then bundle install for better caching",
    "example": "COPY Gemfile Gemfile.lock ./\nRUN bundle config set --local deployment 'true' && bundle install\nCOPY . .",
    "severity": "medium",
    "tags": [
      "bundler",
      "caching",
      "fix-dockerfile",
      "generate-dockerfile",
      "optimization",
      "ruby"
    ],
    "description": "Separate gem installation improves Docker build cache efficiency"
  },
  {
    "id": "ruby-alpine-dependencies",
    "category": "dockerfile",
    "pattern": "FROM ruby.*alpine",
    "recommendation": "Install necessary build dependencies for native extensions in Alpine",
    "example": "RUN apk add --no-cache build-base postgresql-dev nodejs yarn tzdata",
    "severity": "high",
    "tags": [
      "alpine",
      "dependencies",
      "fix-dockerfile",
      "generate-dockerfile",
      "native-extensions",
      "ruby"
    ],
    "description": "Alpine requires specific packages for building native gem extensions"
  },
  {
    "id": "rails-assets-precompile",
    "category": "dockerfile",
    "pattern": "rails.*assets:precompile",
    "recommendation": "Precompile Rails assets during build with proper environment",
    "example": "RUN SECRET_KEY_BASE=dummy RAILS_ENV=production bundle exec rails assets:precompile",
    "severity": "medium",
    "tags": [
      "assets",
      "aws",
      "build",
      "build-tool",
      "fix-dockerfile",
      "generate-dockerfile",
      "production",
      "rails"
    ],
    "description": "Asset precompilation during build improves runtime performance"
  },
  {
    "id": "ruby-multi-stage-build",
    "category": "dockerfile",
    "pattern": "FROM ruby:",
    "recommendation": "Use multi-stage builds to separate build dependencies from runtime",
    "example": "FROM ruby:3.2-alpine AS build\nRUN bundle install\nFROM ruby:3.2-alpine AS runtime\nCOPY --from=build /usr/local/bundle /usr/local/bundle",
    "severity": "high",
    "tags": [
      "build-stage",
      "fix-dockerfile",
      "generate-dockerfile",
      "multistage",
      "optimization",
      "ruby",
      "runtime-stage",
      "size"
    ],
    "description": "Multi-stage builds reduce final image size by excluding build tools"
  },
  {
    "id": "rails-database-migration",
    "category": "dockerfile",
    "pattern": "rails db:migrate",
    "recommendation": "Handle database migrations separately from application startup",
    "example": "# In entrypoint script or K8s init container\nbundle exec rails db:migrate || bundle exec rails db:setup",
    "severity": "medium",
    "tags": [
      "database",
      "fix-dockerfile",
      "generate-dockerfile",
      "migrations",
      "rails",
      "startup"
    ],
    "description": "Separate migration handling prevents startup failures and race conditions"
  },
  {
    "id": "ruby-jemalloc-memory",
    "category": "dockerfile",
    "pattern": "FROM ruby:",
    "recommendation": "Use jemalloc for better Ruby memory management",
    "example": "RUN apt-get install -y libjemalloc2\nENV LD_PRELOAD=/usr/lib/x86_64-linux-gnu/libjemalloc.so.2",
    "severity": "medium",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "jemalloc",
      "memory",
      "performance",
      "ruby"
    ],
    "description": "jemalloc reduces memory fragmentation in long-running Ruby processes"
  },
  {
    "id": "puma-configuration",
    "category": "dockerfile",
    "pattern": "puma",
    "recommendation": "Configure Puma workers and threads for container environment",
    "example": "ENV WEB_CONCURRENCY=2\nENV RAILS_MAX_THREADS=5\nCMD [\"bundle\", \"exec\", \"puma\", \"-C\", \"config/puma.rb\"]",
    "severity": "medium",
    "tags": [
      "concurrency",
      "fix-dockerfile",
      "generate-dockerfile",
      "performance",
      "puma",
      "rails"
    ],
    "description": "Proper Puma configuration optimizes resource usage in containers"
  },
  {
    "id": "ruby-bundler-deployment",
    "category": "dockerfile",
    "pattern": "bundle install",
    "recommendation": "Use bundle install with deployment flag for production",
    "example": "RUN bundle config set --local deployment 'true' \\\n    && bundle config set --local without 'development test' \\\n    && bundle install --jobs 4",
    "severity": "medium",
    "tags": [
      "bundler",
      "deployment",
      "fix-dockerfile",
      "generate-dockerfile",
      "optimization",
      "production"
    ],
    "description": "Deployment mode ensures consistent gem versions and excludes dev dependencies"
  },
  {
    "id": "ruby-health-endpoint",
    "category": "dockerfile",
    "pattern": "EXPOSE 3000",
    "recommendation": "Add health check endpoint for Ruby/Rails applications",
    "example": "HEALTHCHECK --interval=30s --timeout=3s --start-period=60s --retries=3 \\\n  CMD curl -f http://localhost:3000/health || exit 1",
    "severity": "medium",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "health",
      "monitoring",
      "rails",
      "ruby"
    ],
    "description": "Health checks enable better container orchestration"
  },
  {
    "id": "sidekiq-worker-config",
    "category": "dockerfile",
    "pattern": "sidekiq",
    "recommendation": "Configure Sidekiq workers with appropriate concurrency settings",
    "example": "CMD [\"bundle\", \"exec\", \"sidekiq\", \"-C\", \"config/sidekiq.yml\", \"-c\", \"10\"]",
    "severity": "medium",
    "tags": [
      "background-jobs",
      "concurrency",
      "fix-dockerfile",
      "generate-dockerfile",
      "sidekiq",
      "workers"
    ],
    "description": "Proper Sidekiq configuration optimizes background job processing"
  },
  {
    "id": "ruby-security-user",
    "category": "security",
    "pattern": "FROM ruby:",
    "recommendation": "Create and use a non-root user for Ruby applications",
    "example": "RUN groupadd -r ruby && useradd -r -g ruby -d /app ruby\nUSER ruby\nWORKDIR /app",
    "severity": "high",
    "tags": [
      "fix-dockerfile",
      "non-root",
      "ruby",
      "scan-image",
      "security",
      "user"
    ],
    "description": "Running as non-root reduces security attack surface"
  },
  {
    "id": "rails-credentials-security",
    "category": "security",
    "pattern": "credentials.yml.enc",
    "recommendation": "Handle Rails credentials securely using environment variables",
    "example": "ENV RAILS_MASTER_KEY=${RAILS_MASTER_KEY}\n# Never commit master.key to repository",
    "severity": "high",
    "tags": [
      "credentials",
      "fix-dockerfile",
      "rails",
      "scan-image",
      "secrets",
      "security"
    ],
    "description": "Rails credentials should be decrypted using environment variables"
  },
  {
    "id": "ruby-slim-base",
    "category": "dockerfile",
    "pattern": "FROM ruby:(?!.*slim|.*alpine)",
    "recommendation": "Use ruby:slim images for smaller size with glibc compatibility",
    "example": "FROM ruby:3.2-slim",
    "severity": "medium",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "optimization",
      "ruby",
      "size",
      "slim"
    ],
    "description": "Slim images provide good balance between size and compatibility"
  },
  {
    "id": "rails-log-stdout",
    "category": "dockerfile",
    "pattern": "RAILS_LOG_TO_STDOUT",
    "recommendation": "Configure Rails to log to stdout for container best practices",
    "example": "ENV RAILS_LOG_TO_STDOUT=true",
    "severity": "medium",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "logging",
      "observability",
      "rails",
      "stdout"
    ],
    "description": "Logging to stdout enables proper log aggregation in container environments"
  },
  {
    "id": "ruby-bundler-version",
    "category": "dockerfile",
    "pattern": "gem install bundler",
    "recommendation": "Pin Bundler version to match Gemfile.lock",
    "example": "RUN gem install bundler -v $(grep -A 1 'BUNDLED WITH' Gemfile.lock | tail -n 1)",
    "severity": "medium",
    "tags": [
      "bundler",
      "fix-dockerfile",
      "generate-dockerfile",
      "reproducible",
      "versioning"
    ],
    "description": "Matching Bundler versions prevents compatibility issues"
  }
] as const;
