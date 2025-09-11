#!/usr/bin/env tsx
/**
 * Test Dockerfile Generation Fix
 * 
 * This script tests that the generate-dockerfile tool properly extracts
 * clean Dockerfile content from verbose AI responses.
 */

import { stripFencesAndNoise, isValidDockerfileContent } from '../../src/lib/text-processing';

// Test cases with verbose AI responses
const testCases = [
  {
    name: 'Verbose response with markdown fences',
    input: `Certainly! Below is a **production-ready, optimized Dockerfile** for a Spring (Java) application, following best practices for containerization, performance, security, and minimal image size.

**Assumptions:**
- Your application is packaged as a single executable JAR (e.g., \`app.jar\`).
- You use Java 17 (update as needed).
- You want to run as a non-root user.

---

\`\`\`dockerfile
# ---- Build Stage ----
FROM eclipse-temurin:17-jdk-alpine AS builder

WORKDIR /app

# Copy source code and build with Maven (if you want to build inside Docker)
# COPY . .
# RUN ./mvnw clean package -DskipTests

# If you build outside Docker, just copy the JAR
COPY target/app.jar /app/app.jar

# ---- Production Stage ----
FROM eclipse-temurin:17-jre-alpine AS runtime

# Create a non-root user and group
RUN addgroup --system spring && adduser --system --ingroup spring spring

WORKDIR /app

# Copy only the built JAR from builder stage
COPY --from=builder /app/app.jar /app/app.jar

# Set permissions
RUN chown spring:spring /app/app.jar

USER spring:spring

# JVM optimizations for containers
ENV JAVA_OPTS="-XX:+UseContainerSupport -XX:MaxRAMPercentage=75.0"

# Expose application port (update as needed)
EXPOSE 8080

# Healthcheck (optional, update endpoint as needed)
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \\
  CMD wget --spider --quiet http://localhost:8080/actuator/health || exit 1

# Start the application
ENTRYPOINT ["sh", "-c", "java $JAVA_OPTS -jar /app/app.jar"]
\`\`\`

---

## **Best Practices Applied**

- **Multi-stage build:** Keeps final image minimal and free of build tools.
- **Alpine base image:** Small, secure, and fast.
- **Non-root user:** Improves security.
- **Minimal layers:** Only necessary files are copied.
- **JVM container optimizations:** Ensures Java respects container memory limits.
- **Healthcheck:** For container orchestration readiness.
- **No unnecessary files:** Only the JAR is included.
- **Explicit port exposure:** For clarity.
- **No hardcoded secrets:** Keep secrets out of the image.

---

### **Tips**

- If you use Maven/Gradle, you can uncomment the build lines to build inside Docker.
- Adjust \`JAVA_OPTS\` for your app's needs.
- Update the healthcheck endpoint to match your app.
- For even smaller images, consider [distroless Java images](https://github.com/GoogleContainerTools/distroless) if you don't need shell access.

---

**Let me know if you need a Gradle/Maven build inside Docker or further customization!**`,
    expectedStart: '# ---- Build Stage ----\nFROM eclipse-temurin:17-jdk-alpine AS builder'
  },
  {
    name: 'Simple response without fences',
    input: `FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
CMD ["node", "index.js"]`,
    expectedStart: 'FROM node:18-alpine'
  },
  {
    name: 'Response with explanatory text but no fences',
    input: `Here's a Dockerfile for your Node.js application:

FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
USER node
EXPOSE 3000
CMD ["node", "index.js"]

This Dockerfile uses Alpine Linux for a smaller image size.`,
    expectedStart: 'FROM node:18-alpine'
  },
  {
    name: 'Response with comments before FROM',
    input: `\`\`\`dockerfile
# Production Dockerfile for Node.js
# Optimized for size and security

FROM node:18-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:18-alpine
WORKDIR /app
COPY --from=builder /app/dist ./dist
COPY --from=builder /app/node_modules ./node_modules
CMD ["node", "dist/index.js"]
\`\`\``,
    expectedStart: '# Production Dockerfile for Node.js'
  }
];

// Color helpers
const green = '\x1b[32m';
const red = '\x1b[31m';
const yellow = '\x1b[33m';
const reset = '\x1b[0m';

console.log(`${yellow}Testing Dockerfile extraction from verbose AI responses...${reset}\n`);

let passed = 0;
let failed = 0;

for (const testCase of testCases) {
  console.log(`Test: ${testCase.name}`);
  
  const cleaned = stripFencesAndNoise(testCase.input);
  const isValid = isValidDockerfileContent(cleaned);
  
  if (cleaned.startsWith(testCase.expectedStart) && isValid) {
    console.log(`${green}✓ PASSED${reset}`);
    console.log(`  Extracted ${cleaned.split('\n').length} lines`);
    console.log(`  First line: ${cleaned.split('\n')[0]}`);
    passed++;
  } else {
    console.log(`${red}✗ FAILED${reset}`);
    console.log(`  Expected to start with: ${testCase.expectedStart}`);
    console.log(`  Actually starts with: ${cleaned.substring(0, 50)}...`);
    console.log(`  Is valid Dockerfile: ${isValid}`);
    failed++;
  }
  console.log();
}

console.log('---');
console.log(`Results: ${green}${passed} passed${reset}, ${failed > 0 ? red : ''}${failed} failed${reset}`);

if (failed > 0) {
  process.exit(1);
}