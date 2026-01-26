# Multi-stage build for Java application
# Demonstrates best practices: multi-stage, non-root user, health check

# Build stage - compile the application
FROM eclipse-temurin:21-jdk-alpine AS builder

WORKDIR /app

# Copy source files
COPY App.java .

# Compile the application with package structure
# -d . creates com/example/App.class directory structure
RUN javac -d . App.java && \
    jar cfe app.jar com.example.App com/

# Runtime stage - use JRE only
FROM eclipse-temurin:21-jre-alpine AS runtime

# Create non-root user
RUN addgroup -g 1001 -S javauser && \
    adduser -S javauser -u 1001 -G javauser

WORKDIR /app

# Copy JAR from builder
COPY --from=builder --chown=javauser:javauser /app/app.jar .

# Switch to non-root user
USER javauser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD java -cp app.jar com.example.App health || exit 1

# Build args for metadata
ARG VERSION=1.0.0
LABEL version=${VERSION}

# Start application
CMD ["java", "-jar", "app.jar"]
