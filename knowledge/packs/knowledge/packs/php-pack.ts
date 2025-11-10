export default [
  {
    "id": "php-composer-cache",
    "category": "dockerfile",
    "pattern": "COPY composer\\.json",
    "recommendation": "Copy composer files first and install dependencies for better caching",
    "example": "COPY composer.json composer.lock ./\nRUN composer install --no-scripts --no-dev --optimize-autoloader\nCOPY . .\nRUN composer dump-autoload --optimize",
    "severity": "medium",
    "tags": [
      "caching",
      "composer",
      "fix-dockerfile",
      "generate-dockerfile",
      "optimization",
      "php"
    ],
    "description": "Separate dependency installation improves Docker build cache"
  },
  {
    "id": "php-fpm-optimization",
    "category": "dockerfile",
    "pattern": "php-fpm",
    "recommendation": "Configure PHP-FPM for container environments",
    "example": "RUN sed -i 's/;pm.max_children = 5/pm.max_children = 20/' /usr/local/etc/php-fpm.d/www.conf\nEXPOSE 9000\nCMD [\"php-fpm\"]",
    "severity": "medium",
    "tags": [
      "configuration",
      "fix-dockerfile",
      "fpm",
      "generate-dockerfile",
      "performance",
      "php"
    ],
    "description": "Proper PHP-FPM configuration optimizes request handling"
  },
  {
    "id": "php-opcache-config",
    "category": "dockerfile",
    "pattern": "FROM php:",
    "recommendation": "Enable and configure OPcache for production performance",
    "example": "RUN docker-php-ext-install opcache\nCOPY opcache.ini /usr/local/etc/php/conf.d/\n# opcache.ini: opcache.enable=1, opcache.memory_consumption=128",
    "severity": "high",
    "tags": [
      "caching",
      "fix-dockerfile",
      "generate-dockerfile",
      "opcache",
      "performance",
      "php"
    ],
    "description": "OPcache significantly improves PHP performance in production"
  },
  {
    "id": "php-alpine-extensions",
    "category": "dockerfile",
    "pattern": "FROM php.*alpine",
    "recommendation": "Install necessary dependencies for PHP extensions in Alpine",
    "example": "RUN apk add --no-cache $PHPIZE_DEPS postgresql-dev libzip-dev \\\n    && docker-php-ext-install pdo_pgsql zip \\\n    && apk del $PHPIZE_DEPS",
    "severity": "high",
    "tags": [
      "alpine",
      "dependencies",
      "extensions",
      "fix-dockerfile",
      "generate-dockerfile",
      "php"
    ],
    "description": "Alpine requires specific packages for PHP extension compilation"
  },
  {
    "id": "laravel-optimization",
    "category": "dockerfile",
    "pattern": "laravel",
    "recommendation": "Optimize Laravel for production containers",
    "example": "RUN php artisan config:cache \\\n    && php artisan route:cache \\\n    && php artisan view:cache",
    "severity": "medium",
    "tags": [
      "caching",
      "fix-dockerfile",
      "generate-dockerfile",
      "laravel",
      "optimization",
      "production"
    ],
    "description": "Laravel caching commands improve application performance"
  },
  {
    "id": "symfony-cache-warmup",
    "category": "dockerfile",
    "pattern": "symfony",
    "recommendation": "Warm up Symfony cache during build",
    "example": "ENV APP_ENV=prod\nRUN php bin/console cache:warmup --env=prod",
    "severity": "medium",
    "tags": [
      "cache",
      "fix-dockerfile",
      "generate-dockerfile",
      "production",
      "symfony",
      "warmup"
    ],
    "description": "Cache warmup prevents first-request performance penalties"
  },
  {
    "id": "php-multi-stage-build",
    "category": "dockerfile",
    "pattern": "composer install",
    "recommendation": "Use multi-stage builds to exclude development dependencies",
    "example": "FROM composer:2 AS build\nCOPY . /app\nRUN composer install --no-dev --optimize-autoloader\n\nFROM php:8.2-fpm-alpine\nCOPY --from=build /app /var/www/html",
    "severity": "high",
    "tags": [
      "build-stage",
      "composer",
      "fix-dockerfile",
      "generate-dockerfile",
      "multistage",
      "optimization",
      "php"
    ],
    "description": "Multi-stage builds reduce image size by excluding build tools"
  },
  {
    "id": "php-security-user",
    "category": "security",
    "pattern": "FROM php:",
    "recommendation": "Create and use non-root user for PHP applications",
    "example": "RUN addgroup -g 1000 php && adduser -D -u 1000 -G php php\nUSER php\nWORKDIR /var/www/html",
    "severity": "high",
    "tags": [
      "fix-dockerfile",
      "non-root",
      "php",
      "scan-image",
      "security",
      "user"
    ],
    "description": "Running as non-root reduces security attack surface"
  },
  {
    "id": "php-env-security",
    "category": "security",
    "pattern": "\\.env",
    "recommendation": "Never include .env files in images, use environment variables",
    "example": "# Add to .dockerignore: .env\n# Use K8s secrets or Docker secrets for sensitive data",
    "severity": "high",
    "tags": [
      "aws",
      "environment",
      "fix-dockerfile",
      "php",
      "scan-image",
      "secrets",
      "security"
    ],
    "description": ".env files often contain sensitive credentials"
  },
  {
    "id": "php-nginx-sidecar",
    "category": "dockerfile",
    "pattern": "php-fpm",
    "recommendation": "Consider nginx as sidecar for serving static files",
    "example": "# In docker-compose or K8s: separate nginx and php-fpm containers\n# Share volume for static assets",
    "severity": "low",
    "tags": [
      "architecture",
      "fix-dockerfile",
      "generate-dockerfile",
      "nginx",
      "php",
      "static-files"
    ],
    "description": "Nginx handles static files more efficiently than PHP"
  },
  {
    "id": "php-health-check",
    "category": "dockerfile",
    "pattern": "EXPOSE 9000",
    "recommendation": "Add health check for PHP-FPM applications",
    "example": "HEALTHCHECK --interval=30s --timeout=3s --start-period=30s --retries=3 \\\n  CMD php-fpm -t || exit 1",
    "severity": "medium",
    "tags": [
      "fix-dockerfile",
      "fpm",
      "generate-dockerfile",
      "health",
      "monitoring",
      "php"
    ],
    "description": "Health checks enable better container orchestration"
  },
  {
    "id": "wordpress-optimization",
    "category": "dockerfile",
    "pattern": "wordpress",
    "recommendation": "Optimize WordPress containers with proper configuration",
    "example": "RUN wp config set WP_CACHE true --raw\nRUN chown -R www-data:www-data /var/www/html",
    "severity": "medium",
    "tags": [
      "configuration",
      "fix-dockerfile",
      "generate-dockerfile",
      "optimization",
      "wordpress"
    ],
    "description": "WordPress-specific optimizations improve performance"
  },
  {
    "id": "php-memory-limit",
    "category": "dockerfile",
    "pattern": "memory_limit",
    "recommendation": "Set appropriate PHP memory limits for containers",
    "example": "RUN echo 'memory_limit = 256M' > /usr/local/etc/php/conf.d/memory.ini",
    "severity": "medium",
    "tags": [
      "configuration",
      "fix-dockerfile",
      "generate-dockerfile",
      "limits",
      "memory",
      "php"
    ],
    "description": "Memory limits prevent container OOM issues"
  },
  {
    "id": "php-session-redis",
    "category": "dockerfile",
    "pattern": "session",
    "recommendation": "Use Redis or Memcached for PHP sessions in containers",
    "example": "RUN pecl install redis && docker-php-ext-enable redis\n# Configure session.save_handler = redis",
    "severity": "medium",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "php",
      "redis",
      "sessions",
      "stateless"
    ],
    "description": "External session storage enables horizontal scaling"
  },
  {
    "id": "php-pecl-cleanup",
    "category": "dockerfile",
    "pattern": "pecl install",
    "recommendation": "Clean up after PECL installations to reduce image size",
    "example": "RUN pecl install xdebug \\\n    && docker-php-ext-enable xdebug \\\n    && rm -rf /tmp/pear",
    "severity": "low",
    "tags": [
      "cleanup",
      "fix-dockerfile",
      "generate-dockerfile",
      "pecl",
      "php",
      "size"
    ],
    "description": "PECL cleanup reduces image size"
  },
  {
    "id": "php-composer-security",
    "category": "security",
    "pattern": "composer install",
    "recommendation": "Run composer audit to check for vulnerable dependencies",
    "example": "RUN composer audit --no-dev",
    "severity": "high",
    "tags": [
      "audit",
      "composer",
      "fix-dockerfile",
      "php",
      "scan-image",
      "security"
    ],
    "description": "Composer audit identifies known vulnerabilities"
  },
  {
    "id": "php-ini-production",
    "category": "dockerfile",
    "pattern": "php\\.ini",
    "recommendation": "Use production php.ini configuration",
    "example": "RUN mv \"$PHP_INI_DIR/php.ini-production\" \"$PHP_INI_DIR/php.ini\"",
    "severity": "medium",
    "tags": [
      "configuration",
      "fix-dockerfile",
      "generate-dockerfile",
      "php",
      "production",
      "security"
    ],
    "description": "Production php.ini has security-focused defaults"
  },
  {
    "id": "drupal-containerization",
    "category": "dockerfile",
    "pattern": "drupal",
    "recommendation": "Configure Drupal for container environments",
    "example": "RUN drush config:set system.performance css.preprocess 1 -y\nRUN drush config:set system.performance js.preprocess 1 -y",
    "severity": "medium",
    "tags": [
      "cms",
      "configuration",
      "drupal",
      "fix-dockerfile",
      "generate-dockerfile",
      "optimization"
    ],
    "description": "Drupal-specific optimizations for containers"
  },
  {
    "id": "php8-jit-optimization",
    "category": "dockerfile",
    "pattern": "FROM php:8",
    "recommendation": "Enable PHP 8+ JIT compilation for performance-critical applications",
    "example": "RUN echo 'opcache.enable=1' >> /usr/local/etc/php/conf.d/opcache.ini \\\n    && echo 'opcache.jit_buffer_size=256M' >> /usr/local/etc/php/conf.d/opcache.ini \\\n    && echo 'opcache.jit=1235' >> /usr/local/etc/php/conf.d/opcache.ini",
    "severity": "high",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "jit",
      "opcache",
      "performance",
      "php8"
    ],
    "description": "PHP 8 JIT compilation provides significant performance improvements for CPU-intensive tasks"
  },
  {
    "id": "php8-preloading",
    "category": "dockerfile",
    "pattern": "FROM php:8",
    "recommendation": "Use PHP 8 preloading for faster application startup",
    "example": "COPY preload.php /var/www/html/\nRUN echo 'opcache.preload=/var/www/html/preload.php' >> /usr/local/etc/php/conf.d/opcache.ini \\\n    && echo 'opcache.preload_user=www-data' >> /usr/local/etc/php/conf.d/opcache.ini",
    "severity": "medium",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "performance",
      "php8",
      "preloading",
      "startup"
    ],
    "description": "Preloading reduces application startup time by loading classes into OPcache"
  },
  {
    "id": "php8-attributes-optimization",
    "category": "dockerfile",
    "pattern": "attributes|#\\[.*\\]",
    "recommendation": "Optimize PHP 8 attributes usage with proper OPcache configuration",
    "example": "# Ensure OPcache saves comments for attributes\nRUN echo 'opcache.save_comments=1' >> /usr/local/etc/php/conf.d/opcache.ini",
    "severity": "medium",
    "tags": [
      "attributes",
      "fix-dockerfile",
      "generate-dockerfile",
      "metadata",
      "opcache",
      "php8"
    ],
    "description": "PHP 8 attributes require OPcache to save comments for proper functionality"
  },
  {
    "id": "composer2-optimization",
    "category": "dockerfile",
    "pattern": "composer",
    "recommendation": "Use Composer 2 with advanced caching and parallel downloads",
    "example": "COPY --from=composer:2 /usr/bin/composer /usr/bin/composer\nRUN composer config --global cache-files-maxsize 512MiB \\\n    && composer config --global process-timeout 600 \\\n    && composer install --no-dev --optimize-autoloader --no-scripts --prefer-dist",
    "severity": "high",
    "tags": [
      "caching",
      "composer",
      "fix-dockerfile",
      "generate-dockerfile",
      "optimization",
      "parallel"
    ],
    "description": "Composer 2 provides faster installs with parallel downloads and better caching"
  },
  {
    "id": "php-security-scanning-comprehensive",
    "category": "security",
    "pattern": "composer install",
    "recommendation": "Implement comprehensive PHP security scanning with psalm, phpstan, and security-checker",
    "example": "RUN composer require --dev psalm/plugin-phpunit phpstan/phpstan \\\n    && vendor/bin/psalm --no-cache --threads=4 \\\n    && vendor/bin/phpstan analyse src --level=max \\\n    && composer audit",
    "severity": "high",
    "tags": [
      "fix-dockerfile",
      "phpstan",
      "psalm",
      "scan-image",
      "security",
      "static-analysis"
    ],
    "description": "Multi-tool static analysis provides comprehensive security and quality checks"
  },
  {
    "id": "swoole-async-php",
    "category": "dockerfile",
    "pattern": "swoole",
    "recommendation": "Configure Swoole for high-performance async PHP applications",
    "example": "RUN pecl install swoole \\\n    && docker-php-ext-enable swoole\nEXPOSE 9501\nCMD [\"php\", \"swoole-server.php\"]",
    "severity": "medium",
    "tags": [
      "async",
      "coroutines",
      "fix-dockerfile",
      "generate-dockerfile",
      "performance",
      "swoole"
    ],
    "description": "Swoole enables async/coroutine programming for high-performance PHP applications"
  },
  {
    "id": "reactphp-async-patterns",
    "category": "dockerfile",
    "pattern": "react/.*|reactphp",
    "recommendation": "Optimize ReactPHP for async event-driven applications",
    "example": "RUN composer require react/socket react/http\n# Configure for production\nENV REACT_SOCKET_BUFFER_SIZE=65536\nCMD [\"php\", \"server.php\"]",
    "severity": "medium",
    "tags": [
      "async",
      "event-driven",
      "fix-dockerfile",
      "generate-dockerfile",
      "performance",
      "reactphp"
    ],
    "description": "ReactPHP provides event-driven, non-blocking I/O for PHP applications"
  },
  {
    "id": "frankenphp-modern-deployment",
    "category": "dockerfile",
    "pattern": "frankenphp",
    "recommendation": "Use FrankenPHP for modern PHP deployment with built-in HTTP/2 and HTTP/3 support",
    "example": "FROM dunglas/frankenphp\nCOPY . /app\nWORKDIR /app\nRUN composer install --no-dev --optimize-autoloader\nEXPOSE 80 443\nCMD [\"frankenphp\", \"run\", \"--config\", \"/etc/caddy/Caddyfile\"]",
    "severity": "medium",
    "tags": [
      "fix-dockerfile",
      "frankenphp",
      "generate-dockerfile",
      "http2",
      "http3",
      "modern"
    ],
    "description": "FrankenPHP provides modern HTTP protocols and better performance than traditional setups"
  },
  {
    "id": "opcache-container-optimization",
    "category": "dockerfile",
    "pattern": "opcache",
    "recommendation": "Optimize OPcache configuration specifically for container environments",
    "example": "RUN echo 'opcache.enable=1' > /usr/local/etc/php/conf.d/opcache.ini \\\n    && echo 'opcache.memory_consumption=${OPCACHE_MEMORY:-128}' >> /usr/local/etc/php/conf.d/opcache.ini \\\n    && echo 'opcache.interned_strings_buffer=16' >> /usr/local/etc/php/conf.d/opcache.ini \\\n    && echo 'opcache.max_accelerated_files=20000' >> /usr/local/etc/php/conf.d/opcache.ini \\\n    && echo 'opcache.validate_timestamps=0' >> /usr/local/etc/php/conf.d/opcache.ini",
    "severity": "high",
    "tags": [
      "configuration",
      "containers",
      "fix-dockerfile",
      "generate-dockerfile",
      "opcache",
      "performance"
    ],
    "description": "Container-optimized OPcache settings maximize performance and memory efficiency"
  },
  {
    "id": "php-jit-warmup",
    "category": "dockerfile",
    "pattern": "opcache.*jit",
    "recommendation": "Implement JIT warmup for consistent performance from container start",
    "example": "# Create JIT warmup script\nCOPY jit-warmup.php /var/www/html/\nRUN php /var/www/html/jit-warmup.php\n# Or use opcache preloading with JIT",
    "severity": "medium",
    "tags": [
      "consistency",
      "fix-dockerfile",
      "generate-dockerfile",
      "jit",
      "performance",
      "warmup"
    ],
    "description": "JIT warmup ensures consistent performance from the first request"
  },
  {
    "id": "php-xdebug-conditional",
    "category": "dockerfile",
    "pattern": "xdebug",
    "recommendation": "Configure Xdebug conditionally for development containers only",
    "example": "ARG INSTALL_XDEBUG=false\nRUN if [ \"$INSTALL_XDEBUG\" = \"true\" ] ; then pecl install xdebug && docker-php-ext-enable xdebug ; fi\n# Use build arg: --build-arg INSTALL_XDEBUG=true",
    "severity": "medium",
    "tags": [
      "conditional",
      "development",
      "fix-dockerfile",
      "generate-dockerfile",
      "performance",
      "xdebug"
    ],
    "description": "Conditional Xdebug installation prevents performance impact in production"
  },
  {
    "id": "php-fiber-support",
    "category": "dockerfile",
    "pattern": "Fiber|fiber",
    "recommendation": "Configure PHP 8.1+ Fibers for cooperative multitasking",
    "example": "# Ensure PHP 8.1+ for Fiber support\nFROM php:8.1-fpm\n# Fibers work out of the box in PHP 8.1+\n# Configure memory limits appropriately\nRUN echo 'memory_limit=${PHP_MEMORY_LIMIT:-256M}' > /usr/local/etc/php/conf.d/memory.ini",
    "severity": "low",
    "tags": [
      "concurrency",
      "fibers",
      "fix-dockerfile",
      "generate-dockerfile",
      "multitasking",
      "php81"
    ],
    "description": "PHP Fibers enable cooperative multitasking for better resource utilization"
  },
  {
    "id": "php-enum-optimization",
    "category": "dockerfile",
    "pattern": "enum|BackedEnum",
    "recommendation": "Optimize PHP 8.1+ enum usage with proper OPcache configuration",
    "example": "# Ensure OPcache is configured for enum performance\nRUN echo 'opcache.enable=1' > /usr/local/etc/php/conf.d/opcache.ini \\\n    && echo 'opcache.save_comments=1' >> /usr/local/etc/php/conf.d/opcache.ini",
    "severity": "low",
    "tags": [
      "enums",
      "fix-dockerfile",
      "generate-dockerfile",
      "opcache",
      "performance",
      "php81"
    ],
    "description": "Proper OPcache configuration ensures optimal enum performance"
  },
  {
    "id": "php-readonly-properties",
    "category": "dockerfile",
    "pattern": "readonly",
    "recommendation": "Leverage PHP 8.1+ readonly properties for immutable data structures",
    "example": "# Readonly properties work out of the box in PHP 8.1+\n# Ensure static analysis tools support readonly\nRUN composer require --dev phpstan/phpstan psalm/plugin-phpunit",
    "severity": "low",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "immutability",
      "php81",
      "readonly",
      "static-analysis"
    ],
    "description": "Readonly properties improve code safety and enable compiler optimizations"
  },
  {
    "id": "php-security-headers",
    "category": "security",
    "pattern": "FROM php:.*fpm",
    "recommendation": "Configure security headers and PHP security settings for production",
    "example": "RUN echo 'expose_php=Off' > /usr/local/etc/php/conf.d/security.ini \\\n    && echo 'display_errors=Off' >> /usr/local/etc/php/conf.d/security.ini \\\n    && echo 'log_errors=On' >> /usr/local/etc/php/conf.d/security.ini \\\n    && echo 'session.cookie_secure=1' >> /usr/local/etc/php/conf.d/security.ini",
    "severity": "high",
    "tags": [
      "fix-dockerfile",
      "hardening",
      "headers",
      "production",
      "scan-image",
      "security"
    ],
    "description": "Security headers and settings reduce attack surface and information disclosure"
  }
] as const;
