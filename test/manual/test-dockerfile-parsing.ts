#!/usr/bin/env node

/**
 * Manual test for Dockerfile parsing with new libraries
 */

import { isValidDockerfileContent, extractBaseImage } from '../../src/lib/text-processing';

const testDockerfiles = [
  {
    name: 'Simple Node.js Dockerfile',
    content: `FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
CMD ["node", "index.js"]`,
    expectedValid: true,
    expectedBase: 'node:18-alpine'
  },
  {
    name: 'Multi-stage Dockerfile',
    content: `FROM maven:3-amazoncorretto-17 AS builder
WORKDIR /build
COPY pom.xml .
RUN mvn dependency:go-offline
COPY src ./src
RUN mvn package -DskipTests

FROM amazoncorretto:17-alpine
WORKDIR /app
COPY --from=builder /build/target/*.jar app.jar
CMD ["java", "-jar", "app.jar"]`,
    expectedValid: true,
    expectedBase: 'maven:3-amazoncorretto-17'
  },
  {
    name: 'Invalid - no FROM',
    content: `WORKDIR /app
COPY . .
CMD ["node", "index.js"]`,
    expectedValid: false,
    expectedBase: null
  },
  {
    name: 'Invalid - empty',
    content: '',
    expectedValid: false,
    expectedBase: null
  }
];

console.log('Testing Dockerfile parsing with docker-file-parser and validate-dockerfile\n');

for (const test of testDockerfiles) {
  console.log(`Testing: ${test.name}`);
  console.log('-'.repeat(50));
  
  // Test validation
  const isValid = isValidDockerfileContent(test.content);
  const validationPassed = isValid === test.expectedValid;
  console.log(`Validation: ${isValid ? '✅ Valid' : '❌ Invalid'} ${validationPassed ? '(PASS)' : '(FAIL)'}`);
  
  // Test base image extraction
  const baseImage = extractBaseImage(test.content);
  const extractionPassed = baseImage === test.expectedBase;
  console.log(`Base Image: ${baseImage || 'null'} ${extractionPassed ? '(PASS)' : '(FAIL)'}`);
  
  if (!validationPassed || !extractionPassed) {
    console.log(`⚠️  Test failed!`);
    console.log(`   Expected valid: ${test.expectedValid}, got: ${isValid}`);
    console.log(`   Expected base: ${test.expectedBase}, got: ${baseImage}`);
  }
  
  console.log();
}

console.log('✨ All tests completed');