/**
 * Spring Boot Application Test Fixture
 * Provides realistic Spring Boot project structure for testing containerization workflows
 */

export interface ProjectFixture {
  name: string;
  description: string;
  files: Record<string, string>;
  expectedDockerfile?: string;
  expectedKubernetesManifests?: Record<string, string>;
}

export const SPRING_BOOT_REST_API: ProjectFixture = {
  name: 'spring-boot-rest-api',
  description: 'Simple Spring Boot REST API with Maven',
  files: {
    'pom.xml': `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0
         https://maven.apache.org/xsd/maven-4.0.0.xsd">
    <modelVersion>4.0.0</modelVersion>

    <parent>
        <groupId>org.springframework.boot</groupId>
        <artifactId>spring-boot-starter-parent</artifactId>
        <version>3.2.0</version>
        <relativePath/>
    </parent>

    <groupId>com.example</groupId>
    <artifactId>demo-api</artifactId>
    <version>1.0.0</version>
    <name>Demo REST API</name>
    <description>Demo project for Spring Boot containerization testing</description>

    <properties>
        <java.version>17</java.version>
    </properties>

    <dependencies>
        <dependency>
            <groupId>org.springframework.boot</groupId>
            <artifactId>spring-boot-starter-web</artifactId>
        </dependency>
        <dependency>
            <groupId>org.springframework.boot</groupId>
            <artifactId>spring-boot-starter-actuator</artifactId>
        </dependency>
        <dependency>
            <groupId>org.springframework.boot</groupId>
            <artifactId>spring-boot-starter-test</artifactId>
            <scope>test</scope>
        </dependency>
    </dependencies>

    <build>
        <plugins>
            <plugin>
                <groupId>org.springframework.boot</groupId>
                <artifactId>spring-boot-maven-plugin</artifactId>
            </plugin>
        </plugins>
    </build>
</project>`,

    'src/main/java/com/example/demo/DemoApplication.java': `package com.example.demo;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

@SpringBootApplication
public class DemoApplication {
    public static void main(String[] args) {
        SpringApplication.run(DemoApplication.class, args);
    }
}`,

    'src/main/java/com/example/demo/controller/HelloController.java': `package com.example.demo.controller;

import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RestController;

@RestController
public class HelloController {

    @GetMapping("/")
    public String hello() {
        return "Hello, World!";
    }

    @GetMapping("/hello/{name}")
    public String helloName(@PathVariable String name) {
        return "Hello, " + name + "!";
    }

    @GetMapping("/health")
    public String health() {
        return "OK";
    }
}`,

    'src/main/resources/application.yml': `server:
  port: 8080

management:
  endpoints:
    web:
      exposure:
        include: health,info,metrics
  endpoint:
    health:
      show-details: always

logging:
  level:
    com.example.demo: INFO
    org.springframework: INFO`,

    'src/test/java/com/example/demo/DemoApplicationTests.java': `package com.example.demo;

import org.junit.jupiter.api.Test;
import org.springframework.boot.test.context.SpringBootTest;

@SpringBootTest
class DemoApplicationTests {

    @Test
    void contextLoads() {
    }
}`,

    '.gitignore': `target/
!.mvn/wrapper/maven-wrapper.jar
!**/src/main/**/target/
!**/src/test/**/target/

### STS ###
.apt_generated
.classpath
.factorypath
.project
.settings
.springBeans
.sts4-cache

### IntelliJ IDEA ###
.idea
*.iws
*.iml
*.ipr

### NetBeans ###
/nbproject/private/
/nbbuild/
/dist/
/nbdist/
/.nb-gradle/
build/
!**/src/main/**/build/
!**/src/test/**/build/

### VS Code ###
.vscode/`,

    'README.md': `# Demo REST API

A simple Spring Boot REST API for containerization testing.

## Endpoints

- \`GET /\` - Returns "Hello, World!"
- \`GET /hello/{name}\` - Returns personalized greeting
- \`GET /health\` - Health check endpoint

## Build and Run

\`\`\`bash
mvn clean package
java -jar target/demo-api-1.0.0.jar
\`\`\`

## Docker

\`\`\`bash
docker build -t demo-api:latest .
docker run -p 8080:8080 demo-api:latest
\`\`\``,
  },

  expectedDockerfile: `# Build stage
FROM maven:3.9.4-eclipse-temurin-17 AS build
WORKDIR /app
COPY pom.xml .
RUN mvn dependency:go-offline -B
COPY src ./src
RUN mvn clean package -DskipTests

# Runtime stage
FROM eclipse-temurin:17-jre-alpine
WORKDIR /app

# Create non-root user
RUN addgroup -g 1001 -S appgroup && \\
    adduser -u 1001 -S appuser -G appgroup

# Copy jar file
COPY --from=build --chown=appuser:appgroup /app/target/demo-api-*.jar app.jar

# Switch to non-root user
USER appuser

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \\
    CMD curl -f http://localhost:8080/health || exit 1

# Expose port
EXPOSE 8080

# Run application
ENTRYPOINT ["java", "-jar", "app.jar"]`,

  expectedKubernetesManifests: {
    'deployment.yaml': `apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo-api
  labels:
    app: demo-api
spec:
  replicas: 2
  selector:
    matchLabels:
      app: demo-api
  template:
    metadata:
      labels:
        app: demo-api
    spec:
      containers:
      - name: demo-api
        image: demo-api:latest
        imagePullPolicy: Always
        ports:
        - containerPort: 8080
          name: http
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 5
        env:
        - name: SPRING_PROFILES_ACTIVE
          value: "production"`,

    'service.yaml': `apiVersion: v1
kind: Service
metadata:
  name: demo-api-service
  labels:
    app: demo-api
spec:
  type: ClusterIP
  ports:
  - port: 80
    targetPort: 8080
    protocol: TCP
    name: http
  selector:
    app: demo-api`,
  },
};

export const NODE_EXPRESS_API: ProjectFixture = {
  name: 'node-express-api',
  description: 'Node.js Express API with TypeScript',
  files: {
    'package.json': `{
  "name": "node-express-api",
  "version": "1.0.0",
  "description": "Express API for containerization testing",
  "main": "dist/index.js",
  "scripts": {
    "build": "tsc",
    "start": "node dist/index.js",
    "dev": "ts-node src/index.ts",
    "test": "jest"
  },
  "dependencies": {
    "express": "^4.18.2",
    "helmet": "^7.1.0",
    "cors": "^2.8.5"
  },
  "devDependencies": {
    "@types/express": "^4.17.21",
    "@types/cors": "^2.8.17",
    "@types/node": "^20.10.0",
    "typescript": "^5.3.3",
    "ts-node": "^10.9.1",
    "jest": "^29.7.0",
    "@types/jest": "^29.5.8"
  }
}`,

    'tsconfig.json': `{
  "compilerOptions": {
    "target": "ES2020",
    "module": "commonjs",
    "outDir": "./dist",
    "rootDir": "./src",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true,
    "resolveJsonModule": true
  },
  "include": ["src/**/*"],
  "exclude": ["node_modules", "dist"]
}`,

    'src/index.ts': `import express from 'express';
import helmet from 'helmet';
import cors from 'cors';

const app = express();
const PORT = process.env.PORT || 3000;

// Middleware
app.use(helmet());
app.use(cors());
app.use(express.json());

// Routes
app.get('/', (req, res) => {
  res.json({ message: 'Hello, World!', timestamp: new Date().toISOString() });
});

app.get('/hello/:name', (req, res) => {
  const { name } = req.params;
  res.json({ message: \`Hello, \${name}!\`, timestamp: new Date().toISOString() });
});

app.get('/health', (req, res) => {
  res.json({ status: 'OK', timestamp: new Date().toISOString() });
});

// Error handling
app.use((err: Error, req: express.Request, res: express.Response, next: express.NextFunction) => {
  console.error(err.stack);
  res.status(500).json({ error: 'Something went wrong!' });
});

// 404 handler
app.use('*', (req, res) => {
  res.status(404).json({ error: 'Route not found' });
});

app.listen(PORT, () => {
  console.log(\`Server running on port \${PORT}\`);
});

export default app;`,

    '.gitignore': `node_modules/
dist/
npm-debug.log*
yarn-debug.log*
yarn-error.log*
.env
.env.local
.env.development.local
.env.test.local
.env.production.local`,

    'README.md': `# Node Express API

A simple Express API for containerization testing.

## Endpoints

- \`GET /\` - Returns hello message with timestamp
- \`GET /hello/:name\` - Returns personalized greeting
- \`GET /health\` - Health check endpoint

## Development

\`\`\`bash
npm install
npm run dev
\`\`\`

## Build and Run

\`\`\`bash
npm run build
npm start
\`\`\``,
  },

  expectedDockerfile: `# Build stage
FROM node:18-alpine AS build
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production

# Runtime stage
FROM node:18-alpine
WORKDIR /app

# Create non-root user
RUN addgroup -g 1001 -S nodegroup && \\
    adduser -u 1001 -S nodeuser -G nodegroup

# Copy application files
COPY --from=build --chown=nodeuser:nodegroup /app/node_modules ./node_modules
COPY --chown=nodeuser:nodegroup package*.json ./
COPY --chown=nodeuser:nodegroup dist ./dist

# Switch to non-root user
USER nodeuser

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \\
    CMD wget --no-verbose --tries=1 --spider http://localhost:3000/health || exit 1

# Expose port
EXPOSE 3000

# Run application
CMD ["npm", "start"]`,
};

export const PROBLEMATIC_DOCKERFILE: ProjectFixture = {
  name: 'problematic-app',
  description: 'Application with security and optimization issues in Dockerfile',
  files: {
    'Dockerfile': `FROM ubuntu:latest

# Running as root - security issue
RUN apt-get update && apt-get install -y \\
    curl \\
    wget \\
    python3 \\
    python3-pip \\
    && rm -rf /var/lib/apt/lists/*

# No version pinning - reproducibility issue
RUN pip3 install flask requests

WORKDIR /app
COPY . .

# Hardcoded secrets - security issue
ENV API_KEY=super-secret-key-123
ENV DATABASE_URL=postgresql://admin:password@localhost/db

# Running on privileged port as root
EXPOSE 80

# No health check
CMD ["python3", "app.py"]`,

    'app.py': `from flask import Flask, jsonify
import os
import requests

app = Flask(__name__)

@app.route('/')
def hello():
    return jsonify({'message': 'Hello World'})

@app.route('/external')
def external():
    # Potential security issue - no timeout
    response = requests.get('https://api.example.com/data')
    return response.json()

if __name__ == '__main__':
    # Security issue - debug mode in production
    app.run(host='0.0.0.0', port=80, debug=True)`,

    'requirements.txt': `flask
requests`,
  },

  expectedDockerfile: `# Use specific version for reproducibility
FROM python:3.11-slim

# Create non-root user for security
RUN groupadd -r appgroup && useradd -r -g appgroup appuser

# Set working directory
WORKDIR /app

# Install system dependencies with specific versions
RUN apt-get update && apt-get install -y --no-install-recommends \\
    curl=7.* \\
    && rm -rf /var/lib/apt/lists/* \\
    && apt-get clean

# Copy requirements first for better caching
COPY requirements.txt .

# Install Python dependencies with pinned versions
RUN pip install --no-cache-dir --upgrade pip==23.* \\
    && pip install --no-cache-dir -r requirements.txt

# Copy application code
COPY --chown=appuser:appgroup app.py .

# Switch to non-root user
USER appuser

# Use non-privileged port
EXPOSE 8080

# Add health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \\
    CMD curl -f http://localhost:8080/health || exit 1

# Run with production settings
CMD ["python", "app.py"]`,
};

/**
 * Create project files in a temporary directory
 */
export async function createProjectFixture(
  fixture: ProjectFixture,
  baseDir: string
): Promise<string> {
  const projectDir = join(baseDir, fixture.name);
  await fs.mkdir(projectDir, { recursive: true });

  for (const [filePath, content] of Object.entries(fixture.files)) {
    const fullPath = join(projectDir, filePath);
    const dirPath = dirname(fullPath);

    await fs.mkdir(dirPath, { recursive: true });
    await fs.writeFile(fullPath, content.trim() + '\\n');
  }

  return projectDir;
}

import { dirname } from 'node:path';
import { promises as fs } from 'node:fs';
import { join } from 'node:path';

/**
 * Get all available project fixtures
 */
export function getAllFixtures(): ProjectFixture[] {
  return [
    SPRING_BOOT_REST_API,
    NODE_EXPRESS_API,
    PROBLEMATIC_DOCKERFILE,
  ];
}

/**
 * Get fixture by name
 */
export function getFixture(name: string): ProjectFixture | undefined {
  return getAllFixtures().find(fixture => fixture.name === name);
}