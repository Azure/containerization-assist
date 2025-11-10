export default [
  {
    "id": "java-official-temurin",
    "category": "dockerfile",
    "pattern": "java|openjdk|jdk|jre",
    "recommendation": "Eclipse Temurin as alternative for Java applications",
    "example": "# Use specific major version tags, not patch versions\nFROM eclipse-temurin:17-jre-alpine\n# Or for build stage:\nFROM eclipse-temurin:21-jdk AS builder",
    "severity": "medium",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "java",
      "official",
      "temurin"
    ],
    "description": "Eclipse Temurin is the official OpenJDK distribution. Use major version tags (17, 21) not patch versions (17.0.8)"
  },
  {
    "id": "dotnet-microsoft-azure-linux",
    "category": "dockerfile",
    "pattern": "dotnet|csharp|aspnet|netcore",
    "recommendation": "Use Microsoft Azure Linux for .NET applications",
    "example": "FROM mcr.microsoft.com/dotnet/sdk:8.0-azurelinux AS build\nFROM mcr.microsoft.com/dotnet/aspnet:8.0-azurelinux AS runtime",
    "severity": "high",
    "tags": [
      "azure",
      "dotnet",
      "enterprise",
      "fix-dockerfile",
      "generate-dockerfile",
      "microsoft"
    ],
    "description": "Azure Linux optimized for .NET workloads with Microsoft support"
  },
  {
    "id": "node-microsoft-azurelinux",
    "category": "dockerfile",
    "pattern": "node|nodejs|javascript|typescript",
    "recommendation": "Use Microsoft Azure Linux 3.0 for Node.js in enterprise environments.",
    "example": "FROM mcr.microsoft.com/azurelinux/base/nodejs:18 AS builder\nWORKDIR /app\nCOPY package*.json ./\nRUN npm ci\nCOPY . .\nRUN npm run build\n\nFROM mcr.microsoft.com/azurelinux/distroless/nodejs:18\nWORKDIR /app\nCOPY package*.json ./\nCOPY --from=builder /app/dist ./dist\nRUN npm ci --only=production\nCMD [\"node\", \"dist/index.js\"]",
    "severity": "medium",
    "tags": [
      "enterprise",
      "fix-dockerfile",
      "generate-dockerfile",
      "microsoft",
      "node",
      "distroless"
    ],
    "description": "Azure Linux 3.0 provides a secure, enterprise-supported base for Node.js applications"
  },
  {
    "id": "python-microsoft-azurelinux",
    "category": "dockerfile",
    "pattern": "python|pip|django|flask",
    "recommendation": "Use Microsoft Azure Linux 3.0 for Python in enterprise environments. Provides enterprise support and security updates. Microsoft publishes both base and distroless variants.",
    "example": "FROM mcr.microsoft.com/azurelinux/base/python:3.11 AS builder\nWORKDIR /app\nCOPY requirements.txt .\nRUN pip install --no-cache-dir -r requirements.txt\nCOPY . .\n\nFROM mcr.microsoft.com/azurelinux/distroless/python:3.11\nWORKDIR /app\nCOPY --from=builder /app/app.py ./app.py\nCOPY --from=builder /usr/local/lib/python3.11/site-packages /usr/local/lib/python3.11/site-packages\nCMD [\"python\", \"app.py\"]",
    "severity": "medium",
    "tags": [
      "azure",
      "fix-dockerfile",
      "generate-dockerfile",
      "microsoft",
      "python"
    ],
    "description": "Azure Linux 3.0 provides a secure, enterprise-supported base for Python applications"
  },
  {
    "id": "distroless-security",
    "category": "security",
    "pattern": "runtime|production|secure",
    "recommendation": "For enhanced security in production runtime stages, consider using distroless images when Azure/vendor-specific images are not available. Distroless images contain only runtime dependencies with no package managers or shells.",
    "example": "# Use in runtime stage of multi-stage builds\n# Distroless images have no shell, so use exec form for CMD/ENTRYPOINT\nFROM gcr.io/distroless/base-debian12:nonroot\nCOPY --from=build /app/binary /app/binary\nCMD [\"/app/binary\"]",
    "severity": "medium",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "google",
      "minimal",
      "production",
      "runtime-stage",
      "security"
    ],
    "description": "Security guidance for using distroless images as a fallback when vendor-specific hardened images are not available"
  },
  {
    "id": "maven-wrapper-setup",
    "category": "dockerfile",
    "pattern": "maven|mvn|pom.xml",
    "recommendation": "Use Maven wrapper for reproducible builds",
    "example": "# Install tar if needed (Azure Linux 3.0)\nRUN tdnf install -y tar || yum install -y tar || apt-get update && apt-get install -y tar || apk add tar\n\n# Copy wrapper files first\nCOPY mvnw pom.xml ./\nCOPY .mvn .mvn\n\n# Make wrapper executable and download dependencies\nRUN chmod +x mvnw && ./mvnw dependency:go-offline -B\n\n# Then copy source\nCOPY src ./src\nRUN ./mvnw clean package -DskipTests -B",
    "severity": "high",
    "tags": [
      "azure",
      "build",
      "build-tool",
      "fix-dockerfile",
      "generate-dockerfile",
      "maven",
      "maven-wrapper",
      "microsoft",
      "reproducible",
      "wrapper"
    ],
    "description": "Maven wrapper ensures consistent Maven version and offline dependency resolution"
  },
  {
    "id": "gradle-wrapper-setup",
    "category": "dockerfile",
    "pattern": "gradle|gradlew|build.gradle",
    "recommendation": "Use Gradle wrapper for consistent builds",
    "example": "# Install tar if needed\nRUN tdnf install -y tar || yum install -y tar || apt-get update && apt-get install -y tar || apk add tar\n\n# Copy wrapper files\nCOPY gradlew build.gradle settings.gradle ./\nCOPY gradle gradle\n\n# Download dependencies\nRUN chmod +x gradlew && ./gradlew dependencies --no-daemon\n\n# Copy source and build\nCOPY src ./src\nRUN ./gradlew build --no-daemon",
    "severity": "high",
    "tags": [
      "build",
      "build-tool",
      "fix-dockerfile",
      "generate-dockerfile",
      "gradle",
      "gradle-wrapper",
      "reproducible",
      "wrapper"
    ],
    "description": "Gradle wrapper provides version consistency and offline builds"
  },
  {
    "id": "package-manager-compatibility",
    "category": "dockerfile",
    "pattern": "RUN.*install",
    "recommendation": "Use correct package manager for base image",
    "example": "# Azure Linux 3.0: tdnf\nRUN tdnf install -y tar curl\n\n# Alpine: apk\nRUN apk add --no-cache tar curl\n\n# Debian/Ubuntu: apt\nRUN apt-get update && apt-get install -y tar curl\n\n# RHEL/CentOS: yum/dnf\nRUN yum install -y tar curl",
    "severity": "high",
    "tags": [
      "azure",
      "canonical",
      "compatibility",
      "fix-dockerfile",
      "generate-dockerfile",
      "microsoft",
      "package-manager",
      "redhat"
    ],
    "description": "Different base images use different package managers - use the correct one"
  },
  {
    "id": "base-image-preference-order",
    "category": "dockerfile",
    "pattern": "FROM",
    "recommendation": "When selecting base images, follow this preference order: (1) Microsoft Azure Linux 3.0 for enterprise support, (2) Google distroless for security-focused deployments, (3) Official language images for general purpose, (4) Alpine for size optimization, (5) Ubuntu/Debian for maximum compatibility.",
    "example": "# Selection criteria:\n# - Enterprise + Microsoft ecosystem → Azure Linux 3.0\n# - Security-focused + minimal attack surface → Distroless\n# - General purpose + broad compatibility → Official images\n# - Size-constrained environments → Alpine\n# - Legacy compatibility needs → Ubuntu/Debian",
    "severity": "low",
    "tags": [
      "azure",
      "canonical",
      "enterprise",
      "fix-dockerfile",
      "generate-dockerfile",
      "google",
      "microsoft",
      "selection"
    ],
    "description": "Best practice guidance for base image selection based on organizational requirements"
  }
] as const;
