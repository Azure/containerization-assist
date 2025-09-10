import { TestRepositoryConfig } from '../../types.js';
import path from 'path';

export const securityHardenedAppConfig: TestRepositoryConfig = {
  repository: {
    name: 'security-hardened-app',
    type: 'security-focused',
    path: path.join(process.cwd(), 'test/fixtures/repositories/complex/security-hardened-app'),
    language: 'javascript',
    framework: 'fastify',
    complexity: 'complex',
    description: 'Security-hardened Node.js application with comprehensive security features',
    expectedFeatures: [
      'security-headers',
      'rate-limiting',
      'input-validation',
      'authentication',
      'authorization',
      'audit-logging',
      'secrets-management',
      'vulnerability-scanning'
    ],
    securityIssues: [
      'exposed-secrets',
      'insecure-dependencies',
      'weak-authentication',
      'missing-security-headers'
    ]
  },
  expectation: {
    analysis: {
      language: 'javascript',
      framework: 'fastify',
      buildTool: 'npm',
      packageManager: 'npm',
      entryPoints: ['src/server.js'],
      dependencies: [
        'fastify',
        '@fastify/helmet',
        '@fastify/rate-limit',
        '@fastify/jwt',
        '@fastify/sensible',
        'bcrypt',
        'joi',
        'pino'
      ],
      ports: [3000, 3443],
      environment: {
        NODE_ENV: 'production',
        JWT_SECRET: 'CHANGE_ME',
        DATABASE_URL: 'postgresql://localhost:5432/secure_app',
        LOG_LEVEL: 'info'
      }
    },
    dockerfile: {
      baseImage: 'node:18-alpine',
      workdir: '/app',
      exposedPorts: [3000, 3443],
      hasMultiStage: true,
      hasHealthCheck: true,
      hasNonRootUser: true
    },
    k8sManifests: {
      hasDeployment: true,
      hasService: true,
      hasConfigMap: true,
      hasSecret: true,
      hasIngress: false, // Security-focused apps might not expose ingress
      replicas: 2
    },
    buildShouldSucceed: false, // Should fail due to security issues
    estimatedBuildTimeMs: 45000
  }
};

export const securityHardenedAppStructure = {
  'package.json': JSON.stringify({
    name: 'security-hardened-app',
    version: '1.0.0',
    description: 'Security-hardened Node.js application',
    main: 'src/server.js',
    scripts: {
      start: 'node src/server.js',
      dev: 'nodemon src/server.js',
      test: 'jest',
      'test:security': 'npm audit && snyk test',
      'security:scan': 'snyk test',
      'security:fix': 'snyk wizard',
      lint: 'eslint src/',
      'lint:security': 'eslint src/ --config .eslintrc.security.js'
    },
    dependencies: {
      fastify: '^4.24.0',
      '@fastify/helmet': '^11.1.1',
      '@fastify/rate-limit': '^8.0.3',
      '@fastify/jwt': '^7.2.4',
      '@fastify/sensible': '^5.5.0',
      '@fastify/cookie': '^9.2.0',
      '@fastify/session': '^10.7.0',
      bcrypt: '^5.1.0',
      joi: '^17.11.0',
      pino: '^8.16.0',
      'pino-pretty': '^10.2.3',
      helmet: '^7.1.0',
      'express-rate-limit': '^7.1.0', // Vulnerable package (intentional)
      lodash: '^4.17.20', // Vulnerable version (intentional)
      'node-forge': '^1.0.0' // Potentially vulnerable (intentional)
    },
    devDependencies: {
      '@types/node': '^18.0.0',
      'nodemon': '^3.0.1',
      'jest': '^29.7.0',
      'eslint': '^8.50.0',
      'eslint-plugin-security': '^1.7.1',
      'snyk': '^1.1233.0',
      'supertest': '^6.3.3'
    }
  }, null, 2),

  'src/server.js': `const fastify = require('fastify')({
  logger: {
    level: process.env.LOG_LEVEL || 'info',
    transport: process.env.NODE_ENV !== 'production' ? {
      target: 'pino-pretty',
      options: {
        colorize: true
      }
    } : undefined
  }
});

const bcrypt = require('bcrypt');
const Joi = require('joi');

// Security plugins
fastify.register(require('@fastify/helmet'), {
  contentSecurityPolicy: {
    directives: {
      defaultSrc: ["'self'"],
      scriptSrc: ["'self'"],
      styleSrc: ["'self'", "'unsafe-inline'"],
      imgSrc: ["'self'", 'data:', 'https:']
    }
  },
  hsts: {
    maxAge: 31536000,
    includeSubDomains: true,
    preload: true
  }
});

fastify.register(require('@fastify/rate-limit'), {
  max: 100,
  timeWindow: '1 minute',
  skipOnError: false
});

fastify.register(require('@fastify/jwt'), {
  secret: process.env.JWT_SECRET || 'INSECURE_DEFAULT_SECRET' // Security issue
});

fastify.register(require('@fastify/sensible'));
fastify.register(require('@fastify/cookie'));

// Security vulnerability: Hardcoded credentials
const ADMIN_USERNAME = 'admin';
const ADMIN_PASSWORD = 'password123'; // Weak password

// Security vulnerability: SQL injection possibility
const users = [
  { id: 1, username: 'admin', password: '$2b$10$...' },
  { id: 2, username: 'user', password: '$2b$10$...' }
];

// Input validation schemas
const loginSchema = Joi.object({
  username: Joi.string().alphanum().min(3).max(30).required(),
  password: Joi.string().min(6).required()
});

const userSchema = Joi.object({
  username: Joi.string().alphanum().min(3).max(30).required(),
  email: Joi.string().email().required(),
  password: Joi.string().min(8).pattern(/^(?=.*[a-z])(?=.*[A-Z])(?=.*\\d)/)
});

// Routes
fastify.get('/health', async (request, reply) => {
  return { 
    status: 'healthy', 
    timestamp: new Date().toISOString(),
    version: process.env.npm_package_version
  };
});

fastify.post('/api/login', async (request, reply) => {
  try {
    const { error, value } = loginSchema.validate(request.body);
    if (error) {
      return reply.code(400).send({ error: error.details[0].message });
    }

    const { username, password } = value;
    
    // Security vulnerability: Direct string comparison without constant time
    const user = users.find(u => u.username === username);
    if (!user) {
      return reply.code(401).send({ error: 'Invalid credentials' });
    }

    // Security vulnerability: Timing attack possible
    const isValid = await bcrypt.compare(password, user.password);
    if (!isValid) {
      // Security vulnerability: Different response time reveals user existence
      return reply.code(401).send({ error: 'Invalid credentials' });
    }

    const token = fastify.jwt.sign({ 
      userId: user.id, 
      username: user.username 
    });

    // Security vulnerability: Token in response body
    return { 
      message: 'Login successful', 
      token,
      user: { id: user.id, username: user.username }
    };
  } catch (error) {
    fastify.log.error(error);
    return reply.code(500).send({ error: 'Internal server error' });
  }
});

fastify.get('/api/users', {
  preValidation: [fastify.authenticate]
}, async (request, reply) => {
  // Security vulnerability: No authorization check
  return users.map(u => ({ id: u.id, username: u.username }));
});

// Security vulnerability: Unprotected admin endpoint
fastify.get('/api/admin/config', async (request, reply) => {
  return {
    database_url: process.env.DATABASE_URL,
    jwt_secret: process.env.JWT_SECRET, // Exposing secrets
    admin_credentials: {
      username: ADMIN_USERNAME,
      password: ADMIN_PASSWORD
    }
  };
});

// Security vulnerability: Path traversal
fastify.get('/api/files/:filename', async (request, reply) => {
  const { filename } = request.params;
  const fs = require('fs');
  
  try {
    // No path validation - allows ../../../etc/passwd
    const content = fs.readFileSync(\`./uploads/\${filename}\`, 'utf8');
    return { content };
  } catch (error) {
    return reply.code(404).send({ error: 'File not found' });
  }
});

// Security middleware
fastify.decorate('authenticate', async function(request, reply) {
  try {
    await request.jwtVerify();
  } catch (err) {
    reply.code(401).send({ error: 'Authentication required' });
  }
});

// Error handling
fastify.setErrorHandler((error, request, reply) => {
  fastify.log.error(error);
  
  // Security vulnerability: Stack trace exposure
  if (process.env.NODE_ENV !== 'production') {
    reply.code(500).send({ 
      error: error.message,
      stack: error.stack // Information disclosure
    });
  } else {
    reply.code(500).send({ error: 'Internal server error' });
  }
});

// Start server
const start = async () => {
  try {
    const port = process.env.PORT || 3000;
    const host = process.env.HOST || '0.0.0.0'; // Security: Binding to all interfaces
    
    await fastify.listen({ port, host });
    fastify.log.info(\`Server listening on \${host}:\${port}\`);
  } catch (err) {
    fastify.log.error(err);
    process.exit(1);
  }
};

start();

module.exports = fastify;
`,

  '.env': `# Security vulnerability: Exposed secrets in repository
NODE_ENV=production
JWT_SECRET=super_secret_key_that_should_not_be_here
DATABASE_URL=postgresql://admin:password123@localhost:5432/secure_app
ADMIN_API_KEY=sk-1234567890abcdef
STRIPE_SECRET_KEY=sk_test_1234567890
AWS_ACCESS_KEY_ID=AKIAI1234567890
AWS_SECRET_ACCESS_KEY=abcdefghijklmnopqrstuvwxyz1234567890
`,

  'Dockerfile': `FROM node:18-alpine

# Security vulnerability: Running as root user
WORKDIR /app

# Copy package files
COPY package*.json ./

# Security vulnerability: No integrity checking
RUN npm install

# Security vulnerability: Copying sensitive files
COPY . .

# Security vulnerability: Exposing internal port
EXPOSE 3000

# Security vulnerability: No health check
CMD ["npm", "start"]
`,

  'docker-compose.yml': `version: '3.8'
services:
  app:
    build: .
    ports:
      - "3000:3000"
    environment:
      - NODE_ENV=production
      - JWT_SECRET=insecure_secret
      - DATABASE_URL=postgresql://postgres:password@db:5432/secure_app
    # Security vulnerability: No resource limits
    
  db:
    image: postgres:13
    environment:
      - POSTGRES_DB=secure_app
      - POSTGRES_USER=postgres
      - POSTGRES_PASSWORD=password # Weak password in plain text
    ports:
      - "5432:5432" # Security vulnerability: Exposing database port
    # Security vulnerability: No volume for data persistence
    # Security vulnerability: Using default postgres image (no hardening)
`,

  'k8s/deployment.yml': `apiVersion: apps/v1
kind: Deployment
metadata:
  name: security-hardened-app
  labels:
    app: security-hardened-app
spec:
  replicas: 2
  selector:
    matchLabels:
      app: security-hardened-app
  template:
    metadata:
      labels:
        app: security-hardened-app
    spec:
      containers:
      - name: app
        image: security-hardened-app:latest
        ports:
        - containerPort: 3000
        env:
        - name: NODE_ENV
          value: "production"
        - name: JWT_SECRET
          value: "insecure_secret" # Security vulnerability: Hardcoded secret
        # Security vulnerability: No resource limits
        # Security vulnerability: No security context
        # Security vulnerability: No probes
`,

  'k8s/service.yml': `apiVersion: v1
kind: Service
metadata:
  name: security-hardened-app-service
spec:
  selector:
    app: security-hardened-app
  ports:
    - protocol: TCP
      port: 80
      targetPort: 3000
  type: ClusterIP
`,

  '.eslintrc.security.js': `module.exports = {
  extends: ['eslint:recommended'],
  plugins: ['security'],
  rules: {
    'security/detect-hardcoded-secrets': 'error',
    'security/detect-sql-injection': 'error',
    'security/detect-unsafe-regex': 'error',
    'security/detect-buffer-noassert': 'error',
    'security/detect-child-process': 'error',
    'security/detect-disable-mustache-escape': 'error',
    'security/detect-eval-with-expression': 'error',
    'security/detect-no-csrf-before-method-override': 'error',
    'security/detect-non-literal-fs-filename': 'error',
    'security/detect-non-literal-regexp': 'error',
    'security/detect-non-literal-require': 'error',
    'security/detect-object-injection': 'error',
    'security/detect-possible-timing-attacks': 'error',
    'security/detect-pseudoRandomBytes': 'error'
  },
  env: {
    node: true,
    es6: true
  },
  parserOptions: {
    ecmaVersion: 2022
  }
};
`,

  'snyk.json': `{
  "vulnerabilities": [
    {
      "id": "SNYK-JS-LODASH-567746",
      "package": "lodash@4.17.20",
      "severity": "high",
      "title": "Prototype Pollution",
      "description": "This affects the package lodash before 4.17.21.",
      "patches": []
    },
    {
      "id": "SNYK-JS-EXPRESSRATELIMIT-2331901",
      "package": "express-rate-limit@7.1.0",
      "severity": "medium",
      "title": "Memory Leak",
      "description": "Memory leak in express-rate-limit",
      "patches": []
    }
  ]
}
`,

  'security-scan-report.json': `{
  "timestamp": "2024-01-15T10:30:00Z",
  "scanner": "multiple",
  "findings": [
    {
      "type": "hardcoded-secret",
      "severity": "high",
      "file": ".env",
      "line": 3,
      "description": "JWT secret exposed in environment file"
    },
    {
      "type": "hardcoded-secret",
      "severity": "critical",
      "file": "src/server.js",
      "line": 45,
      "description": "Hardcoded admin credentials"
    },
    {
      "type": "path-traversal",
      "severity": "high",
      "file": "src/server.js",
      "line": 142,
      "description": "Unvalidated file path allows directory traversal"
    },
    {
      "type": "information-disclosure",
      "severity": "medium",
      "file": "src/server.js",
      "line": 174,
      "description": "Stack trace exposed in error responses"
    },
    {
      "type": "weak-authentication",
      "severity": "medium",
      "file": "src/server.js",
      "line": 95,
      "description": "Timing attack possible in authentication"
    },
    {
      "type": "insecure-container",
      "severity": "high",
      "file": "Dockerfile",
      "line": 3,
      "description": "Container runs as root user"
    },
    {
      "type": "exposed-port",
      "severity": "low",
      "file": "docker-compose.yml",
      "line": 12,
      "description": "Database port exposed unnecessarily"
    }
  ],
  "summary": {
    "total": 7,
    "critical": 1,
    "high": 3,
    "medium": 2,
    "low": 1
  }
}
`
};