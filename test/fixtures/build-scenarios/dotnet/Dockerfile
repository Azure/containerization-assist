# Multi-stage build for .NET application
# Demonstrates best practices: multi-stage, non-root user, health check

# Build stage - compile the application
FROM mcr.microsoft.com/dotnet/sdk:8.0-alpine AS builder

WORKDIR /app

# Copy project file first for better layer caching
COPY *.csproj ./

# Restore dependencies (separate layer for caching)
RUN dotnet restore

# Copy source files
COPY . .

# Build release version
RUN dotnet publish -c Release -o /app/publish --no-restore

# Runtime stage - use ASP.NET runtime only
FROM mcr.microsoft.com/dotnet/aspnet:8.0-alpine AS runtime

# Create non-root user
RUN addgroup -g 1001 -S dotnetuser && \
    adduser -S dotnetuser -u 1001 -G dotnetuser

WORKDIR /app

# Copy published output from builder
COPY --from=builder --chown=dotnetuser:dotnetuser /app/publish .

# Switch to non-root user
USER dotnetuser

# Expose port
EXPOSE 8080

# Set ASP.NET environment
ENV ASPNETCORE_URLS=http://+:8080
ENV ASPNETCORE_ENVIRONMENT=Production

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Build args for metadata
ARG VERSION=1.0.0
LABEL version=${VERSION}

# Start application
ENTRYPOINT ["dotnet", "BuildTestApp.dll"]
