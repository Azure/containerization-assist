# BuildSecBot Best Practices Guide

## Overview

BuildSecBot is responsible for secure container building operations within the Container Kit MCP server. This guide provides best practices for using BuildSecBot's atomic tools effectively.

## Core Responsibilities

- **Build Operations**: Atomic image building with comprehensive error recovery
- **Security Scanning**: Vulnerability assessment and compliance validation
- **Performance Optimization**: Layer analysis and build optimization
- **Error Recovery**: AI-powered build failure analysis and automatic fixes

## Atomic Tools

### 1. atomic_build_image

**Purpose**: Build Docker images with progress tracking and error recovery

**Best Practices**:
- Always validate Dockerfiles before building
- Use build arguments for dynamic configuration
- Enable caching for faster builds
- Monitor build performance metrics

**Example Usage**:
```go
args := AtomicBuildImageArgs{
    SessionID:      "session-123",
    DockerfilePath: "./Dockerfile",
    ImageName:      "myapp:latest",
    BuildArgs: map[string]string{
        "NODE_VERSION": "18",
    },
}
result, err := tool.ExecuteBuild(ctx, args)
```

### 2. atomic_push_image

**Purpose**: Push Docker images to registries with retry logic

**Best Practices**:
- Authenticate with registries before pushing
- Use specific tags instead of 'latest' for production
- Implement retry logic for network failures
- Monitor push metrics (layers cached, transfer size)

**Example Usage**:
```go
args := AtomicPushImageArgs{
    SessionID:  "session-123",
    ImageRef:   "myregistry.azurecr.io/myapp:v1.0.0",
    RetryCount: 3,
    Timeout:    600,
}
result, err := tool.ExecutePush(ctx, args)
```

### 3. atomic_tag_image

**Purpose**: Tag Docker images for versioning and organization

**Best Practices**:
- Use semantic versioning for tags
- Tag images before pushing to registries
- Maintain consistent naming conventions
- Document tag meanings

**Example Usage**:
```go
args := AtomicTagImageArgs{
    SessionID:    "session-123",
    SourceImage:  "myapp:latest",
    TargetImage:  "myapp:v1.0.0",
    Force:        false,
}
result, err := tool.ExecuteTag(ctx, args)
```

### 4. atomic_scan_image_security

**Purpose**: Scan images for vulnerabilities and compliance

**Best Practices**:
- Scan all images before deployment
- Set appropriate severity thresholds
- Generate remediation plans
- Track security metrics over time

**Example Usage**:
```go
args := AtomicScanImageSecurityArgs{
    SessionID:           "session-123",
    ImageName:           "myapp:latest",
    SeverityThreshold:   "HIGH",
    IncludeRemediations: true,
    GenerateReport:      true,
}
result, err := tool.ExecuteScan(ctx, args)
```

## Security Best Practices

### 1. Base Image Security
- Use minimal base images (alpine, distroless)
- Regularly update base images
- Scan base images for vulnerabilities
- Pin base image versions

### 2. Build-Time Security
- Don't include secrets in Dockerfiles
- Use multi-stage builds to reduce attack surface
- Run containers as non-root users
- Set appropriate file permissions

### 3. Vulnerability Management
- Set strict severity thresholds
- Address critical vulnerabilities immediately
- Track fixable vs non-fixable vulnerabilities
- Implement regular scanning schedules

### 4. Compliance
- Validate against frameworks (CIS Docker, NIST 800-190)
- Document compliance status
- Implement policy-as-code
- Automate compliance checks

## Performance Optimization

### 1. Layer Optimization
- Order Dockerfile instructions for optimal caching
- Minimize layer count
- Combine RUN commands where appropriate
- Clean up package manager caches

### 2. Build Caching
- Use BuildKit for advanced caching
- Leverage multi-stage builds
- Cache dependencies separately
- Monitor cache hit rates

### 3. Build Performance
- Track build duration metrics
- Identify slow build steps
- Parallelize independent operations
- Use appropriate build resources

## Error Recovery Strategies

### 1. Network Errors
- Implement exponential backoff
- Use alternative registries/mirrors
- Cache base images locally
- Monitor network reliability

### 2. Dockerfile Errors
- Validate syntax before building
- Use linting tools
- Provide clear error messages
- Suggest fixes for common issues

### 3. Permission Errors
- Check Docker daemon access
- Verify registry credentials
- Validate file permissions
- Use appropriate user contexts

### 4. Resource Errors
- Monitor disk space
- Set appropriate resource limits
- Clean up old images/containers
- Implement resource quotas

## Integration with Other Teams

### 1. AnalyzeBot Integration
- Use analysis results for Dockerfile generation
- Validate generated Dockerfiles
- Provide feedback on build issues

### 2. DeployBot Integration
- Provide secure, optimized images
- Share security scan results
- Document image metadata
- Coordinate versioning

### 3. OrchBot Integration
- Participate in orchestrated workflows
- Report build status and metrics
- Handle workflow errors gracefully
- Support rollback operations

## Monitoring and Metrics

### Key Metrics to Track:
- Build success/failure rates
- Build duration by stage
- Image size trends
- Vulnerability counts by severity
- Cache hit rates
- Registry push/pull performance

### Prometheus Metrics:
```
container_kit_build_duration_seconds
container_kit_build_errors_total
container_kit_vulnerabilities_total
container_kit_compliance_score
container_kit_image_size_bytes
```

## Troubleshooting Guide

### Common Issues:

1. **Build Failures**
   - Check Dockerfile syntax
   - Verify base image availability
   - Review build logs
   - Check network connectivity

2. **Push Failures**
   - Verify registry authentication
   - Check image size limits
   - Review network timeouts
   - Validate registry URL

3. **Security Scan Failures**
   - Ensure scanner (Trivy) is installed
   - Check scanner database updates
   - Review scan timeout settings
   - Validate image format

4. **Performance Issues**
   - Review layer ordering
   - Check cache configuration
   - Monitor resource usage
   - Optimize Dockerfile instructions

## Configuration

### Environment Variables:
```bash
# Build Configuration
BUILD_TIMEOUT=1800
BUILD_RETRY_COUNT=3
BUILD_CACHE_ENABLED=true

# Security Configuration
SECURITY_SCAN_ENABLED=true
SECURITY_SEVERITY_THRESHOLD=HIGH
SECURITY_COMPLIANCE_FRAMEWORKS=cis-docker,nist-800-190

# Performance Configuration
PERFORMANCE_MONITORING_ENABLED=true
PERFORMANCE_METRICS_INTERVAL=60
```

### Tool Configuration:
```yaml
build_tools:
  atomic_build_image:
    max_attempts: 3
    timeout: 1800
    enable_progress: true
    
  security_scanning:
    scanner: trivy
    severity_threshold: HIGH
    compliance_frameworks:
      - cis-docker
      - nist-800-190
      
  performance:
    track_metrics: true
    optimization_enabled: true
```

## Conclusion

BuildSecBot provides comprehensive build and security capabilities for containerized applications. By following these best practices, you can ensure secure, efficient, and reliable container builds that integrate seamlessly with the broader Container Kit ecosystem.

For more information, see the [Container Kit documentation](../README.md) and [API reference](./api-reference.md).