import { TestRepositoryConfig } from '../../types.js';
import path from 'path';

export const monorepoMicroservicesConfig: TestRepositoryConfig = {
  repository: {
    name: 'monorepo-microservices',
    type: 'monorepo',
    path: path.join(process.cwd(), 'test/fixtures/repositories/complex/monorepo-microservices'),
    language: 'multi-language',
    framework: 'multi-framework',
    complexity: 'complex',
    description: 'Monorepo with multiple microservices using different technologies',
    expectedFeatures: [
      'api-gateway',
      'user-service',
      'order-service', 
      'notification-service',
      'database-migrations',
      'docker-compose',
      'kubernetes-manifests',
      'ci-cd-pipeline'
    ]
  },
  expectation: {
    analysis: {
      language: 'multi-language',
      buildTool: 'multi-tool',
      packageManager: 'multi-manager',
      entryPoints: [
        'services/api-gateway/server.js',
        'services/user-service/main.py',
        'services/order-service/src/main/java/OrderServiceApplication.java',
        'services/notification-service/main.go'
      ],
      dependencies: [
        'express',
        'fastapi',
        'spring-boot-starter-web',
        'gin-gonic/gin',
        'redis',
        'postgresql',
        'kafka'
      ],
      ports: [8080, 8081, 8082, 8083, 5432, 6379, 9092],
      environment: {
        NODE_ENV: 'production',
        PYTHON_ENV: 'production',
        SPRING_PROFILES_ACTIVE: 'production',
        GO_ENV: 'production',
        DATABASE_URL: 'postgresql://postgres:password@postgres:5432/monorepo_db',
        REDIS_URL: 'redis://redis:6379',
        KAFKA_BROKERS: 'kafka:9092'
      }
    },
    dockerfile: {
      baseImage: 'multi-stage',
      workdir: '/app',
      exposedPorts: [8080, 8081, 8082, 8083],
      hasMultiStage: true,
      hasHealthCheck: true,
      hasNonRootUser: true
    },
    k8sManifests: {
      hasDeployment: true,
      hasService: true,
      hasConfigMap: true,
      hasSecret: true,
      hasIngress: true,
      replicas: 3
    },
    buildShouldSucceed: true,
    estimatedBuildTimeMs: 180000 // 3 minutes
  }
};

export const monorepoMicroservicesStructure = {
  'package.json': JSON.stringify({
    name: 'monorepo-microservices',
    private: true,
    workspaces: [
      'services/api-gateway',
      'services/user-service',
      'shared/common'
    ],
    scripts: {
      'build:all': 'npm run build --workspaces',
      'test:all': 'npm run test --workspaces',
      'docker:build': 'docker-compose build',
      'docker:up': 'docker-compose up',
      'k8s:deploy': 'kubectl apply -f k8s/',
      'migrate': 'npm run migrate -w services/user-service'
    },
    devDependencies: {
      '@types/node': '^18.0.0',
      'typescript': '^4.9.0',
      'jest': '^29.0.0',
      'eslint': '^8.0.0',
      'prettier': '^2.8.0'
    }
  }, null, 2),
  
  'docker-compose.yml': `version: '3.8'
services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_DB: monorepo_db
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: password
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./database/init.sql:/docker-entrypoint-initdb.d/init.sql

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data

  kafka:
    image: confluentinc/cp-kafka:latest
    environment:
      KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka:9092
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
    ports:
      - "9092:9092"
    depends_on:
      - zookeeper

  zookeeper:
    image: confluentinc/cp-zookeeper:latest
    environment:
      ZOOKEEPER_CLIENT_PORT: 2181
    ports:
      - "2181:2181"

  api-gateway:
    build: ./services/api-gateway
    ports:
      - "8080:8080"
    depends_on:
      - postgres
      - redis
    environment:
      NODE_ENV: production
      DATABASE_URL: postgresql://postgres:password@postgres:5432/monorepo_db
      REDIS_URL: redis://redis:6379
      USER_SERVICE_URL: http://user-service:8081
      ORDER_SERVICE_URL: http://order-service:8082
      NOTIFICATION_SERVICE_URL: http://notification-service:8083

  user-service:
    build: ./services/user-service
    ports:
      - "8081:8081"
    depends_on:
      - postgres
      - redis
      - kafka
    environment:
      PYTHON_ENV: production
      DATABASE_URL: postgresql://postgres:password@postgres:5432/monorepo_db
      REDIS_URL: redis://redis:6379
      KAFKA_BROKERS: kafka:9092

  order-service:
    build: ./services/order-service
    ports:
      - "8082:8082"
    depends_on:
      - postgres
      - kafka
    environment:
      SPRING_PROFILES_ACTIVE: production
      DATABASE_URL: postgresql://postgres:password@postgres:5432/monorepo_db
      KAFKA_BROKERS: kafka:9092

  notification-service:
    build: ./services/notification-service
    ports:
      - "8083:8083"
    depends_on:
      - kafka
    environment:
      GO_ENV: production
      KAFKA_BROKERS: kafka:9092

volumes:
  postgres_data:
  redis_data:
`,

  'services/api-gateway/package.json': JSON.stringify({
    name: 'api-gateway',
    version: '1.0.0',
    main: 'server.js',
    scripts: {
      start: 'node server.js',
      dev: 'nodemon server.js',
      test: 'jest',
      build: 'tsc'
    },
    dependencies: {
      express: '^4.18.0',
      'http-proxy-middleware': '^2.0.0',
      'express-rate-limit': '^6.0.0',
      helmet: '^6.0.0',
      cors: '^2.8.5',
      'express-validator': '^6.14.0',
      redis: '^4.0.0',
      pg: '^8.8.0',
      winston: '^3.8.0',
      'node-fetch': '^3.3.0'
    },
    devDependencies: {
      '@types/node': '^18.0.0',
      'typescript': '^4.9.0',
      'nodemon': '^2.0.20',
      'jest': '^29.0.0',
      'supertest': '^6.3.0'
    }
  }, null, 2),

  'services/api-gateway/server.js': `const express = require('express');
const { createProxyMiddleware } = require('http-proxy-middleware');
const rateLimit = require('express-rate-limit');
const helmet = require('helmet');
const cors = require('cors');
const winston = require('winston');

const app = express();
const PORT = process.env.PORT || 8080;

// Configure logger
const logger = winston.createLogger({
  level: 'info',
  format: winston.format.combine(
    winston.format.timestamp(),
    winston.format.json()
  ),
  transports: [
    new winston.transports.Console(),
    new winston.transports.File({ filename: 'logs/gateway.log' })
  ]
});

// Security middleware
app.use(helmet());
app.use(cors());
app.use(express.json({ limit: '10mb' }));

// Rate limiting
const limiter = rateLimit({
  windowMs: 15 * 60 * 1000, // 15 minutes
  max: 1000, // limit each IP to 1000 requests per windowMs
  message: 'Too many requests from this IP'
});
app.use(limiter);

// Health check
app.get('/health', (req, res) => {
  res.json({ status: 'healthy', service: 'api-gateway', timestamp: new Date().toISOString() });
});

// API routing
app.use('/api/users', createProxyMiddleware({
  target: process.env.USER_SERVICE_URL || 'http://localhost:8081',
  changeOrigin: true,
  pathRewrite: {
    '^/api/users': '/users'
  },
  onError: (err, req, res) => {
    logger.error('User service proxy error:', err);
    res.status(503).json({ error: 'User service unavailable' });
  }
}));

app.use('/api/orders', createProxyMiddleware({
  target: process.env.ORDER_SERVICE_URL || 'http://localhost:8082',
  changeOrigin: true,
  pathRewrite: {
    '^/api/orders': '/orders'
  },
  onError: (err, req, res) => {
    logger.error('Order service proxy error:', err);
    res.status(503).json({ error: 'Order service unavailable' });
  }
}));

app.use('/api/notifications', createProxyMiddleware({
  target: process.env.NOTIFICATION_SERVICE_URL || 'http://localhost:8083',
  changeOrigin: true,
  pathRewrite: {
    '^/api/notifications': '/notifications'
  },
  onError: (err, req, res) => {
    logger.error('Notification service proxy error:', err);
    res.status(503).json({ error: 'Notification service unavailable' });
  }
}));

// Default route
app.get('/', (req, res) => {
  res.json({
    name: 'Monorepo Microservices API Gateway',
    version: '1.0.0',
    services: [
      { name: 'user-service', path: '/api/users' },
      { name: 'order-service', path: '/api/orders' },
      { name: 'notification-service', path: '/api/notifications' }
    ]
  });
});

// Error handling
app.use((err, req, res, next) => {
  logger.error('Unhandled error:', err);
  res.status(500).json({ error: 'Internal server error' });
});

app.listen(PORT, () => {
  logger.info(\`API Gateway running on port \${PORT}\`);
});

module.exports = app;
`,

  'services/api-gateway/Dockerfile': `FROM node:18-alpine

WORKDIR /app

# Install dependencies
COPY package*.json ./
RUN npm ci --only=production

# Copy source code
COPY . .

# Create logs directory
RUN mkdir -p logs

# Create non-root user
RUN addgroup -g 1001 -S nodejs && \\
    adduser -S nextjs -u 1001

RUN chown -R nextjs:nodejs /app
USER nextjs

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \\
  CMD node healthcheck.js

CMD ["npm", "start"]
`,

  'services/user-service/requirements.txt': `fastapi==0.104.0
uvicorn[standard]==0.24.0
sqlalchemy==2.0.23
psycopg2-binary==2.9.9
redis==5.0.1
kafka-python==2.0.2
pydantic==2.5.0
pydantic-settings==2.1.0
alembic==1.13.0
bcrypt==4.1.1
python-jose[cryptography]==3.3.0
passlib[bcrypt]==1.7.4
python-multipart==0.0.6
`,

  'services/user-service/main.py': `from fastapi import FastAPI, Depends, HTTPException, status
from fastapi.security import HTTPBearer
from sqlalchemy.orm import Session
from contextlib import asynccontextmanager
import uvicorn
import os
import logging
from datetime import datetime
from typing import List, Optional

from database import get_db, engine
from models import User, UserCreate, UserResponse
from auth import get_current_user
from kafka_producer import KafkaProducer
import redis

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Initialize Redis
redis_client = redis.from_url(os.getenv('REDIS_URL', 'redis://localhost:6379'))

# Initialize Kafka producer
kafka_producer = KafkaProducer()

@asynccontextmanager
async def lifespan(app: FastAPI):
    # Startup
    logger.info("User service starting up...")
    yield
    # Shutdown
    logger.info("User service shutting down...")
    kafka_producer.close()

app = FastAPI(
    title="User Service",
    description="Microservice for user management",
    version="1.0.0",
    lifespan=lifespan
)

security = HTTPBearer()

@app.get("/health")
async def health_check():
    return {
        "status": "healthy",
        "service": "user-service",
        "timestamp": datetime.utcnow().isoformat()
    }

@app.post("/users", response_model=UserResponse)
async def create_user(user: UserCreate, db: Session = Depends(get_db)):
    try:
        # Check if user already exists
        existing_user = db.query(User).filter(User.email == user.email).first()
        if existing_user:
            raise HTTPException(status_code=400, detail="Email already registered")
        
        # Create new user
        db_user = User(**user.dict())
        db.add(db_user)
        db.commit()
        db.refresh(db_user)
        
        # Cache user data
        redis_client.setex(f"user:{db_user.id}", 3600, db_user.json())
        
        # Publish event
        kafka_producer.send_event("user.created", {
            "user_id": db_user.id,
            "email": db_user.email,
            "created_at": db_user.created_at.isoformat()
        })
        
        return UserResponse.from_orm(db_user)
    except Exception as e:
        logger.error(f"Error creating user: {e}")
        db.rollback()
        raise HTTPException(status_code=500, detail="Internal server error")

@app.get("/users/{user_id}", response_model=UserResponse)
async def get_user(user_id: int, db: Session = Depends(get_db)):
    # Try cache first
    cached_user = redis_client.get(f"user:{user_id}")
    if cached_user:
        return UserResponse.parse_raw(cached_user)
    
    # Fallback to database
    user = db.query(User).filter(User.id == user_id).first()
    if not user:
        raise HTTPException(status_code=404, detail="User not found")
    
    # Update cache
    redis_client.setex(f"user:{user_id}", 3600, user.json())
    
    return UserResponse.from_orm(user)

@app.get("/users", response_model=List[UserResponse])
async def list_users(
    skip: int = 0, 
    limit: int = 100, 
    db: Session = Depends(get_db)
):
    users = db.query(User).offset(skip).limit(limit).all()
    return [UserResponse.from_orm(user) for user in users]

@app.put("/users/{user_id}", response_model=UserResponse)
async def update_user(
    user_id: int, 
    user_update: UserCreate, 
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user)
):
    user = db.query(User).filter(User.id == user_id).first()
    if not user:
        raise HTTPException(status_code=404, detail="User not found")
    
    # Update user fields
    for field, value in user_update.dict(exclude_unset=True).items():
        setattr(user, field, value)
    
    db.commit()
    db.refresh(user)
    
    # Update cache
    redis_client.setex(f"user:{user_id}", 3600, user.json())
    
    # Publish event
    kafka_producer.send_event("user.updated", {
        "user_id": user.id,
        "updated_at": datetime.utcnow().isoformat()
    })
    
    return UserResponse.from_orm(user)

@app.delete("/users/{user_id}")
async def delete_user(
    user_id: int, 
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user)
):
    user = db.query(User).filter(User.id == user_id).first()
    if not user:
        raise HTTPException(status_code=404, detail="User not found")
    
    db.delete(user)
    db.commit()
    
    # Remove from cache
    redis_client.delete(f"user:{user_id}")
    
    # Publish event
    kafka_producer.send_event("user.deleted", {
        "user_id": user_id,
        "deleted_at": datetime.utcnow().isoformat()
    })
    
    return {"message": "User deleted successfully"}

if __name__ == "__main__":
    uvicorn.run(
        "main:app",
        host="0.0.0.0",
        port=int(os.getenv("PORT", 8081)),
        reload=os.getenv("PYTHON_ENV") != "production"
    )
`,

  'services/user-service/Dockerfile': `FROM python:3.11-slim

WORKDIR /app

# Install system dependencies
RUN apt-get update && apt-get install -y \\
    gcc \\
    libpq-dev \\
    && rm -rf /var/lib/apt/lists/*

# Copy requirements and install Python dependencies
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy source code
COPY . .

# Create non-root user
RUN useradd --create-home --shell /bin/bash app \\
    && chown -R app:app /app
USER app

EXPOSE 8081

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \\
  CMD python healthcheck.py

CMD ["python", "main.py"]
`
};