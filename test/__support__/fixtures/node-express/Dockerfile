# Generated Dockerfile for javascript (express)
# Module root: .
FROM node:18-alpine

# Metadata
LABEL maintainer="generated"
LABEL language="javascript"
LABEL framework="express"
LABEL module.root="."

WORKDIR /app

# Copy package files
COPY package*.json ./
RUN npm install --omit=dev

# Copy application files
COPY . .

# Security hardening
RUN addgroup -g 1001 -S appgroup && adduser -u 1001 -S appuser -G appgroup
USER appuser

# Expose application port
EXPOSE 3000

# Start application
CMD ["node", "server.js"]
