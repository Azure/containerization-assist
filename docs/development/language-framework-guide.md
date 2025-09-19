# Language & Framework Expansion Guide

This document provides a comprehensive framework for adding new languages and improving existing language coverage in the containerization knowledge base.

## Table of Contents

1. [Overview](#overview)
2. [Assessment Framework](#assessment-framework)
3. [Implementation Phases](#implementation-phases)
4. [Knowledge Pack Structure](#knowledge-pack-structure)
5. [Language-Specific Guidelines](#language-specific-guidelines)
6. [Testing & Validation](#testing--validation)
7. [Best Practices](#best-practices)
8. [Examples](#examples)

## Overview

The containerization assistant uses a knowledge-driven approach to provide intelligent Dockerfile generation and Kubernetes deployment recommendations. Each language requires:

- **Framework Detection**: Pattern recognition for identifying project types
- **Knowledge Packs**: JSON-based rule sets for containerization patterns
- **Base Image Recommendations**: Optimized image selections per language/framework
- **Test Coverage**: Validation of generated containerization artifacts

## Assessment Framework

### Current Coverage Analysis

Before adding or improving language support, assess the current state:

#### 1. Coverage Audit
```bash
# Count existing rules per language
find src/knowledge/data/ -name "*.json" | xargs grep -l "language-name" | wc -l

# Analyze rule distribution
grep -r "\"language\":" src/knowledge/data/ | cut -d: -f3 | sort | uniq -c | sort -nr
```

#### 2. Gap Analysis Template

| Aspect | Current State | Target State | Priority |
|--------|---------------|--------------|----------|
| **Basic Detection** | ✅/❌ | ✅ | High |
| **Framework Patterns** | Count: X | Count: Y | High |
| **Advanced Patterns** | Count: X | Count: Y | Medium |
| **Deployment Models** | Count: X | Count: Y | Medium |
| **Performance Optimization** | ✅/❌ | ✅ | Low |
| **Security Hardening** | ✅/❌ | ✅ | High |

#### 3. Priority Matrix

Use this matrix to prioritize language expansion efforts:

```
High Impact, Low Effort    | High Impact, High Effort
---------------------------|---------------------------
• Basic framework detection| • Comprehensive microservice patterns
• Core containerization    | • Advanced deployment strategies
• Security fundamentals    | • Performance optimization suites

Low Impact, Low Effort     | Low Impact, High Effort
---------------------------|---------------------------
• Nice-to-have patterns    | • Highly specialized use cases
• Edge case scenarios      | • Legacy framework support
```

## Implementation Phases

### Phase 1: Foundation (Essential)
**Goal**: Establish basic containerization capability

#### 1.1 Framework Detection
Add detection patterns to `src/tools/analyze-repo/tool.ts`:

```typescript
// Add to FRAMEWORK_PATTERNS
{
  name: 'your-framework',
  language: 'your-language',
  indicators: {
    files: ['framework.config', 'project.toml'],
    dependencies: ['framework-core', 'framework-web'],
    patterns: [/framework\s+version/i],
    directories: ['framework/', 'modules/']
  },
  confidence: 0.9
}
```

#### 1.2 Base Image Recommendations
Update `src/lib/base-images.ts`:

```typescript
// Add to BASE_IMAGE_MAP
'your-language': {
  primary: 'your-lang:latest-alpine',
  alternatives: ['your-lang:latest-slim', 'your-lang:latest'],
  security: ['your-lang:latest-alpine', 'distroless/your-lang'],
  performance: ['your-lang:latest-slim', 'your-lang:latest']
}
```

#### 1.3 Basic Knowledge Pack
Create `src/knowledge/data/your-language-basic-pack.json`:

```json
{
  "name": "YourLanguage Basic Pack",
  "description": "Essential containerization patterns for YourLanguage applications",
  "version": "1.0.0",
  "triggers": {
    "frameworks": ["your-language"],
    "packages": ["core-package"],
    "files": ["main.ext"],
    "patterns": ["YourLanguageClass", "YourLanguageInterface"]
  },
  "rules": [
    {
      "id": "basic-setup",
      "description": "Basic YourLanguage application containerization",
      "conditions": {
        "packages": ["core-package"]
      },
      "containerization": {
        "dockerfile": {
          "base_image": "your-lang:latest-alpine",
          "layers": [
            "# YourLanguage Application",
            "FROM your-lang:latest-alpine",
            "WORKDIR /app",
            "COPY . ./",
            "RUN your-lang build",
            "EXPOSE 8080",
            "CMD [\"your-lang\", \"run\"]"
          ]
        }
      }
    }
  ]
}
```

### Phase 2: Framework Specialization (Important)
**Goal**: Add framework-specific optimizations

#### 2.1 Popular Frameworks
For each major framework, create specialized knowledge packs:

- `your-language-web-framework-pack.json`
- `your-language-api-framework-pack.json`
- `your-language-microservices-pack.json`

#### 2.2 Framework-Specific Base Images
Add framework context to base image selection:

```typescript
// In getBaseImageRecommendations()
if (langKey === 'your-language' && options.framework) {
  const framework = options.framework.toLowerCase();

  if (framework.includes('web-framework')) {
    return {
      primary: 'your-lang:web-optimized',
      alternatives: ['your-lang:latest-alpine'],
      // ...
    };
  }
}
```

### Phase 3: Advanced Patterns (Nice-to-have)
**Goal**: Comprehensive enterprise-ready patterns

#### 3.1 Advanced Use Cases
- Native compilation patterns
- Serverless deployment models
- Multi-stage optimization
- Security hardening

#### 3.2 Performance Optimization
- JIT compilation strategies
- Memory optimization
- Startup time improvements

#### 3.3 Observability Integration
- Monitoring and metrics
- Distributed tracing
- Health checks

## Knowledge Pack Structure

### Required Fields

```json
{
  "name": "Descriptive Pack Name",
  "description": "Clear description of use cases covered",
  "version": "1.0.0",
  "triggers": {
    "frameworks": ["framework-names"],
    "packages": ["package-names"],
    "files": ["file-patterns"],
    "patterns": ["code-patterns"]
  },
  "rules": [
    {
      "id": "unique-rule-id",
      "description": "What this rule accomplishes",
      "conditions": {
        "packages": ["required-packages"],
        "patterns": ["code-patterns"]
      },
      "containerization": {
        "dockerfile": {
          "base_image": "recommended-base",
          "layers": ["dockerfile-content"]
        },
        "kubernetes": {
          "deployment": {
            "replicas": 3,
            "resources": {
              "requests": {"cpu": "100m", "memory": "128Mi"},
              "limits": {"cpu": "500m", "memory": "512Mi"}
            }
          }
        }
      }
    }
  ]
}
```

### Rule Categories

#### 1. Basic Application Rules
- Simple containerization
- Development environments
- Basic security

#### 2. Framework-Specific Rules
- Framework optimizations
- Specific build processes
- Framework security patterns

#### 3. Deployment Model Rules
- Microservices patterns
- Serverless containers
- Multi-environment configs

#### 4. Advanced Optimization Rules
- Performance tuning
- Resource optimization
- Security hardening

### Dockerfile Pattern Templates

#### Multi-Stage Build Template
```dockerfile
# Build stage
FROM your-lang:sdk AS build
WORKDIR /src
COPY *.config ./
RUN your-lang restore
COPY . ./
RUN your-lang build -c Release

# Runtime stage
FROM your-lang:runtime-alpine
WORKDIR /app
COPY --from=build /src/out ./
EXPOSE 8080
ENTRYPOINT ["your-lang", "app.dll"]
```

#### Security Hardened Template
```dockerfile
FROM your-lang:alpine
RUN addgroup -g 1001 -S appgroup && adduser -u 1001 -S appuser -G appgroup
WORKDIR /app
COPY --chown=appuser:appgroup . ./
USER appuser
EXPOSE 8080
CMD ["your-lang", "run"]
```

## Language-Specific Guidelines

### Web Languages (JavaScript, TypeScript, Python, Ruby)

#### Focus Areas:
- Package manager optimization (npm, yarn, pip, bundler)
- Build tool integration (webpack, vite, rollup)
- Framework-specific patterns (React, Vue, Django, Rails)
- Runtime optimization (Node.js versions, Python versions)

#### Common Patterns:
```json
{
  "triggers": {
    "files": ["package.json", "requirements.txt", "Gemfile"],
    "packages": ["express", "django", "rails"],
    "patterns": ["import.*from", "require\\(", "from.*import"]
  }
}
```

### Compiled Languages (Java, C#, Go, Rust)

#### Focus Areas:
- Build system integration (Maven, Gradle, MSBuild, Cargo)
- Multi-stage builds for size optimization
- Native compilation strategies
- Runtime selection (JRE vs JDK, .NET Runtime vs SDK)

#### Common Patterns:
```json
{
  "triggers": {
    "files": ["pom.xml", "build.gradle", "*.csproj", "Cargo.toml"],
    "packages": ["spring-boot", "Microsoft.AspNetCore", "tokio"],
    "patterns": ["@SpringBootApplication", "\\[ApiController\\]", "fn main"]
  }
}
```

### Functional Languages (Elixir, Haskell, Clojure)

#### Focus Areas:
- Runtime environment setup
- Dependency management (hex, cabal, leiningen)
- Concurrent processing patterns
- Memory optimization

#### Common Patterns:
```json
{
  "triggers": {
    "files": ["mix.exs", "*.cabal", "project.clj"],
    "patterns": ["defmodule", "module.*where", "defproject"]
  }
}
```

## Testing & Validation

### 1. Unit Tests
Create tests in `test/unit/tools/` to validate framework detection:

```typescript
describe('YourLanguage Framework Detection', () => {
  it('should detect your-framework projects', async () => {
    const result = await analyzeRepo({
      path: 'test/fixtures/your-language/your-framework'
    });
    expect(result.framework).toBe('your-framework');
    expect(result.confidence).toBeGreaterThan(0.8);
  });
});
```

### 2. Integration Tests
Add test fixtures in `test/fixtures/your-language/`:

```
test/fixtures/your-language/
├── basic-app/
│   ├── main.ext
│   └── config.ext
├── web-framework/
│   ├── app.ext
│   ├── routes.ext
│   └── framework.config
└── microservice/
    ├── service.ext
    ├── api.ext
    └── deployment.yaml
```

### 3. Knowledge Pack Validation
```bash
# Validate JSON syntax
npm run validate:knowledge

# Test rule matching
npm test -- --grep "knowledge.*your-language"

# Integration test
npm run test:integration
```

### 4. Manual Testing
```bash
# Test framework detection
npm run mcp:inspect
# Use: analyze-repo with your test project

# Test Dockerfile generation
# Use: generate-dockerfile with detected results

# Validate generated Dockerfile
docker build -t test-image .
docker run --rm test-image
```

## Best Practices

### 1. Rule Design Principles

#### Specificity Over Generality
- Create specific rules for common use cases
- Prefer multiple targeted rules over one generic rule
- Use precise condition matching

#### Example:
```json
// Good: Specific
{
  "id": "spring-boot-web-mvc",
  "conditions": {
    "packages": ["spring-boot-starter-web"],
    "patterns": ["@RestController", "@RequestMapping"]
  }
}

// Poor: Generic
{
  "id": "java-web-app",
  "conditions": {
    "packages": ["any-web-library"]
  }
}
```

#### Layered Complexity
- Start with basic patterns
- Add complexity incrementally
- Maintain backwards compatibility

#### Progressive Enhancement
```json
// Layer 1: Basic
{"packages": ["core-framework"]}

// Layer 2: Web-specific
{"packages": ["core-framework", "web-extension"]}

// Layer 3: Advanced
{"packages": ["core-framework", "web-extension", "advanced-features"]}
```

### 2. Performance Considerations

#### Trigger Efficiency
- Use specific file patterns over broad regex
- Prefer exact package names over patterns
- Optimize for common cases

#### Rule Ordering
- Place most specific rules first
- Order by likelihood of match
- Group related rules together

### 3. Maintainability

#### Documentation
- Document each rule's purpose
- Include example use cases
- Reference source documentation

#### Versioning
- Use semantic versioning for knowledge packs
- Track breaking changes
- Maintain migration guides

#### Testing Coverage
- Test positive and negative cases
- Validate edge cases
- Include performance benchmarks

### 4. Security Guidelines

#### Base Image Selection
- Prefer official images
- Use specific version tags
- Include security-focused alternatives

#### Security Patterns
```json
{
  "containerization": {
    "dockerfile": {
      "layers": [
        "# Security: Create non-root user",
        "RUN addgroup -g 1001 -S appgroup && adduser -u 1001 -S appuser -G appgroup",
        "USER appuser",
        "# Security: Remove package manager caches",
        "RUN rm -rf /var/lib/apt/lists/* /var/cache/apt/*"
      ]
    }
  }
}
```

## Examples

### Complete Language Addition: Kotlin

#### 1. Framework Detection
```typescript
// In analyze-repo/tool.ts
{
  name: 'kotlin-spring',
  language: 'kotlin',
  indicators: {
    files: ['build.gradle.kts', 'pom.xml'],
    dependencies: ['org.springframework.boot', 'kotlin-stdlib'],
    patterns: [/@SpringBootApplication/, /class.*Application/],
    directories: ['src/main/kotlin/']
  },
  confidence: 0.9
}
```

#### 2. Base Images
```typescript
// In base-images.ts
kotlin: {
  primary: 'openjdk:17-alpine',
  alternatives: ['openjdk:17-slim', 'eclipse-temurin:17'],
  security: ['openjdk:17-alpine', 'eclipse-temurin:17-alpine'],
  performance: ['openjdk:17-slim', 'eclipse-temurin:17-jre-slim']
}
```

#### 3. Knowledge Pack
```json
{
  "name": "Kotlin Spring Boot Pack",
  "description": "Containerization patterns for Kotlin Spring Boot applications",
  "version": "1.0.0",
  "triggers": {
    "frameworks": ["kotlin-spring"],
    "packages": ["org.springframework.boot", "kotlin-stdlib"],
    "files": ["*.kt", "build.gradle.kts"],
    "patterns": ["@SpringBootApplication", "class.*Application"]
  },
  "rules": [
    {
      "id": "kotlin-spring-basic",
      "description": "Basic Kotlin Spring Boot application",
      "conditions": {
        "packages": ["org.springframework.boot"]
      },
      "containerization": {
        "dockerfile": {
          "base_image": "openjdk:17-alpine",
          "layers": [
            "# Kotlin Spring Boot Application",
            "FROM gradle:8-jdk17 AS build",
            "WORKDIR /build",
            "COPY build.gradle.kts settings.gradle.kts ./",
            "COPY gradle/ gradle/",
            "RUN gradle dependencies --no-daemon",
            "COPY src ./src",
            "RUN gradle bootJar --no-daemon",
            "",
            "FROM openjdk:17-alpine",
            "WORKDIR /app",
            "COPY --from=build /build/build/libs/*.jar app.jar",
            "EXPOSE 8080",
            "ENTRYPOINT [\"java\", \"-jar\", \"app.jar\"]"
          ]
        }
      }
    }
  ]
}
```

### Improving Existing Language: Python Enhancement

#### Current State Assessment
```bash
# Analyze current Python coverage
grep -r "python" src/knowledge/data/ | wc -l
# Result: 15 rules across 3 files
```

#### Gap Analysis
- ✅ Basic Flask/Django support
- ❌ FastAPI patterns missing
- ❌ Data science frameworks (pandas, numpy)
- ❌ Async patterns (asyncio, aiohttp)
- ❌ ML/AI deployment patterns

#### Enhancement Plan
1. Create `python-fastapi-pack.json`
2. Create `python-data-science-pack.json`
3. Create `python-async-pack.json`
4. Create `python-ml-deployment-pack.json`

#### FastAPI Example
```json
{
  "name": "Python FastAPI Pack",
  "description": "Modern async Python web API patterns with FastAPI",
  "version": "1.0.0",
  "triggers": {
    "frameworks": ["fastapi"],
    "packages": ["fastapi", "uvicorn"],
    "patterns": ["from fastapi import", "@/app\\."]
  },
  "rules": [
    {
      "id": "fastapi-async-api",
      "description": "FastAPI async web API with uvicorn",
      "conditions": {
        "packages": ["fastapi", "uvicorn"]
      },
      "containerization": {
        "dockerfile": {
          "base_image": "python:3.11-slim",
          "layers": [
            "# FastAPI Async Application",
            "FROM python:3.11-slim",
            "WORKDIR /app",
            "",
            "# Install system dependencies",
            "RUN apt-get update && apt-get install -y \\",
            "    gcc \\",
            "    && rm -rf /var/lib/apt/lists/*",
            "",
            "# Install Python dependencies",
            "COPY requirements.txt .",
            "RUN pip install --no-cache-dir -r requirements.txt",
            "",
            "# Copy application",
            "COPY . .",
            "",
            "# Create non-root user",
            "RUN adduser --disabled-password --gecos '' apiuser",
            "USER apiuser",
            "",
            "EXPOSE 8000",
            "CMD [\"uvicorn\", \"main:app\", \"--host\", \"0.0.0.0\", \"--port\", \"8000\"]"
          ]
        },
        "kubernetes": {
          "deployment": {
            "replicas": 3,
            "resources": {
              "requests": {"cpu": "100m", "memory": "128Mi"},
              "limits": {"cpu": "500m", "memory": "512Mi"}
            },
            "readinessProbe": {
              "httpGet": {"path": "/health", "port": 8000},
              "initialDelaySeconds": 10
            }
          }
        }
      }
    }
  ]
}
```

## Conclusion

This framework provides a systematic approach to expanding language coverage in the containerization knowledge base. Follow the phases sequentially, prioritize based on impact and effort, and maintain high quality through comprehensive testing.

For questions or contributions, refer to the main project documentation or create an issue in the repository.

---

**Next Steps After Reading This Guide:**
1. Assess current language coverage using the audit tools
2. Identify gaps using the priority matrix
3. Start with Phase 1 (Foundation) for new languages
4. Enhance existing languages following the improvement examples
5. Test thoroughly using the validation framework
6. Document your additions for future maintainers