# Generated Dockerfile for javascript (express)
FROM node:18-alpine

# Metadata
LABEL maintainer="generated"
LABEL language="javascript"
LABEL framework="express"

WORKDIR /app

# Copy package files
COPY package*.json ./
RUN npm ci --only=production

# Copy application files
COPY . .

# Security hardening
RUN addgroup -g 1001 -S appgroup && adduser -u 1001 -S appuser -G appgroup
USER appuser

# Expose application port
EXPOSE 3000

# Start application
CMD ["node", "index.js"]
