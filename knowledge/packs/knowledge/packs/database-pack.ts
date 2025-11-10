export default [
  {
    "id": "postgres-data-persistence",
    "category": "dockerfile",
    "pattern": "postgres:",
    "recommendation": "Use named volumes for PostgreSQL data persistence",
    "example": "volumes:\n  - postgres_data:/var/lib/postgresql/data\nenvironment:\n  PGDATA: /var/lib/postgresql/data/pgdata",
    "severity": "high",
    "tags": [
      "data",
      "fix-dockerfile",
      "generate-dockerfile",
      "persistence",
      "postgres",
      "volumes"
    ],
    "description": "Named volumes ensure data persistence across container restarts"
  },
  {
    "id": "mysql-init-scripts",
    "category": "dockerfile",
    "pattern": "mysql:",
    "recommendation": "Use initialization scripts for database setup",
    "example": "volumes:\n  - ./init-scripts:/docker-entrypoint-initdb.d\n# Scripts run in alphabetical order on first startup",
    "severity": "medium",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "initialization",
      "mysql",
      "scripts",
      "setup"
    ],
    "description": "Init scripts automate database schema and user creation"
  },
  {
    "id": "mongodb-replica-set",
    "category": "dockerfile",
    "pattern": "mongo:",
    "recommendation": "Configure MongoDB replica set for high availability",
    "example": "command: mongod --replSet rs0 --bind_ip_all --wiredTigerCacheSizeGB 2\nenvironment:\n  MONGO_INITDB_ROOT_USERNAME: ${MONGO_ROOT_USER}\n  MONGO_INITDB_ROOT_PASSWORD: ${MONGO_ROOT_PASS}\nvolumes:\n  - mongo_data:/data/db\n  - mongo_config:/data/configdb",
    "severity": "high",
    "tags": [
      "availability",
      "clustering",
      "fix-dockerfile",
      "generate-dockerfile",
      "mongodb",
      "replica-set"
    ],
    "description": "Replica sets provide redundancy and automatic failover"
  },
  {
    "id": "redis-persistence-config",
    "category": "dockerfile",
    "pattern": "redis:",
    "recommendation": "Configure Redis persistence appropriately for use case",
    "example": "command: redis-server --appendonly yes --appendfsync everysec --maxmemory 2gb --maxmemory-policy allkeys-lru\nvolumes:\n  - redis_data:/data",
    "severity": "medium",
    "tags": [
      "aof",
      "configuration",
      "fix-dockerfile",
      "generate-dockerfile",
      "persistence",
      "redis"
    ],
    "description": "AOF persistence provides better durability than RDB snapshots"
  },
  {
    "id": "database-health-checks",
    "category": "dockerfile",
    "pattern": "(postgres|mysql|mongo):",
    "recommendation": "Implement proper health checks for database containers",
    "example": "healthcheck:\n  test: [\"CMD\", \"pg_isready\", \"-U\", \"${POSTGRES_USER}\"]\n  interval: 10s\n  timeout: 5s\n  retries: 5\n  start_period: 30s",
    "severity": "high",
    "tags": [
      "availability",
      "database",
      "database-server",
      "fix-dockerfile",
      "generate-dockerfile",
      "health",
      "monitoring"
    ],
    "description": "Health checks ensure database readiness before accepting connections"
  },
  {
    "id": "database-resource-limits",
    "category": "dockerfile",
    "pattern": "(postgres|mysql|mongo):",
    "recommendation": "Set appropriate resource limits for database containers",
    "example": "deploy:\n  resources:\n    limits:\n      memory: 4G\n      cpus: '2.0'\n    reservations:\n      memory: 2G\n      cpus: '1.0'",
    "severity": "high",
    "tags": [
      "database",
      "database-server",
      "fix-dockerfile",
      "generate-dockerfile",
      "limits",
      "performance",
      "resources"
    ],
    "description": "Resource limits prevent database containers from exhausting host resources"
  },
  {
    "id": "postgres-configuration",
    "category": "dockerfile",
    "pattern": "postgres:",
    "recommendation": "Optimize PostgreSQL configuration for containers",
    "example": "command: postgres -c shared_buffers=256MB -c max_connections=200 -c effective_cache_size=1GB -c maintenance_work_mem=128MB -c checkpoint_completion_target=0.9 -c wal_buffers=16MB",
    "severity": "medium",
    "tags": [
      "configuration",
      "fix-dockerfile",
      "generate-dockerfile",
      "performance",
      "postgres",
      "tuning"
    ],
    "description": "Container-specific tuning improves database performance"
  },
  {
    "id": "mysql-slow-query-log",
    "category": "dockerfile",
    "pattern": "mysql:",
    "recommendation": "Enable slow query logging for performance monitoring",
    "example": "command: --slow-query-log=1 --slow-query-log-file=/var/log/mysql/slow.log --long-query-time=2 --log-queries-not-using-indexes=1",
    "severity": "low",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "logging",
      "monitoring",
      "mysql",
      "performance"
    ],
    "description": "Slow query logs help identify performance bottlenecks"
  },
  {
    "id": "database-backup-strategy",
    "category": "dockerfile",
    "pattern": "(postgres|mysql):",
    "recommendation": "Implement automated backup strategy for databases",
    "example": "# Backup container with cron\nservices:\n  backup:\n    image: postgres:15-alpine\n    environment:\n      POSTGRES_USER: ${POSTGRES_USER}\n      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}\n    volumes:\n      - ./backups:/backups\n      - ./scripts/backup.sh:/backup.sh\n    entrypoint: |\n      sh -c 'echo \"0 2 * * * /backup.sh\" | crontab - && crond -f'",
    "severity": "high",
    "tags": [
      "backup",
      "data-protection",
      "database",
      "database-server",
      "disaster-recovery",
      "fix-dockerfile",
      "generate-dockerfile"
    ],
    "description": "Regular backups are essential for disaster recovery"
  },
  {
    "id": "database-secrets-management",
    "category": "dockerfile",
    "pattern": "POSTGRES_PASSWORD|MYSQL_ROOT_PASSWORD",
    "recommendation": "Use Docker secrets or external secret management for database credentials",
    "example": "secrets:\n  db_password:\n    external: true\n  db_user:\n    external: true\nenvironment:\n  POSTGRES_PASSWORD_FILE: /run/secrets/db_password\n  POSTGRES_USER_FILE: /run/secrets/db_user",
    "severity": "high",
    "tags": [
      "aws",
      "credentials",
      "database",
      "database-server",
      "fix-dockerfile",
      "generate-dockerfile",
      "secrets",
      "security"
    ],
    "description": "Secure credential management prevents password exposure"
  },
  {
    "id": "elasticsearch-cluster",
    "category": "dockerfile",
    "pattern": "elasticsearch:",
    "recommendation": "Configure Elasticsearch cluster settings for production",
    "example": "environment:\n  - cluster.name=docker-cluster\n  - discovery.type=single-node\n  - ES_JAVA_OPTS=-Xms2g -Xmx2g\n  - xpack.security.enabled=true\n  - xpack.security.http.ssl.enabled=true\n  - bootstrap.memory_lock=true\nulimits:\n  memlock:\n    soft: -1\n    hard: -1",
    "severity": "high",
    "tags": [
      "cluster",
      "configuration",
      "elasticsearch",
      "fix-dockerfile",
      "generate-dockerfile",
      "search"
    ],
    "description": "Proper cluster configuration ensures stability and performance"
  },
  {
    "id": "database-network-isolation",
    "category": "dockerfile",
    "pattern": "(postgres|mysql|mongo):",
    "recommendation": "Isolate database containers in dedicated networks",
    "example": "networks:\n  backend:\n    internal: true\n    driver: bridge\n    ipam:\n      config:\n        - subnet: 172.28.0.0/16\n  frontend:\n    external: true",
    "severity": "high",
    "tags": [
      "database",
      "database-server",
      "fix-dockerfile",
      "generate-dockerfile",
      "isolation",
      "networking",
      "security"
    ],
    "description": "Network isolation prevents unauthorized database access"
  },
  {
    "id": "postgres-wal-configuration",
    "category": "dockerfile",
    "pattern": "postgres:",
    "recommendation": "Configure PostgreSQL WAL for replication and recovery",
    "example": "command: postgres -c wal_level=replica -c max_wal_senders=3 -c max_replication_slots=3 -c hot_standby=on -c archive_mode=on -c archive_command='test ! -f /archive/%f && cp %p /archive/%f'",
    "severity": "medium",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "postgres",
      "recovery",
      "replication",
      "wal"
    ],
    "description": "WAL configuration enables point-in-time recovery and replication"
  },
  {
    "id": "database-connection-pooling",
    "category": "dockerfile",
    "pattern": "(postgres|mysql):",
    "recommendation": "Implement connection pooling for database access",
    "example": "# PgBouncer for PostgreSQL\nservices:\n  pgbouncer:\n    image: pgbouncer/pgbouncer:latest\n    environment:\n      DATABASES_HOST: postgres\n      DATABASES_PORT: 5432\n      DATABASES_DBNAME: ${DB_NAME}\n      POOL_MODE: transaction\n      MAX_CLIENT_CONN: 1000\n      DEFAULT_POOL_SIZE: 25",
    "severity": "high",
    "tags": [
      "connections",
      "database",
      "database-server",
      "fix-dockerfile",
      "generate-dockerfile",
      "performance",
      "pooling"
    ],
    "description": "Connection pooling reduces database load and improves performance"
  },
  {
    "id": "cassandra-cluster-config",
    "category": "dockerfile",
    "pattern": "cassandra:",
    "recommendation": "Configure Cassandra for distributed deployment",
    "example": "environment:\n  - CASSANDRA_CLUSTER_NAME=docker-cluster\n  - CASSANDRA_SEEDS=cassandra1,cassandra2\n  - CASSANDRA_ENDPOINT_SNITCH=GossipingPropertyFileSnitch\n  - CASSANDRA_DC=dc1\n  - CASSANDRA_RACK=rack1\n  - MAX_HEAP_SIZE=2G\n  - HEAP_NEWSIZE=800M",
    "severity": "high",
    "tags": [
      "cassandra",
      "cluster",
      "distributed",
      "fix-dockerfile",
      "generate-dockerfile",
      "nosql"
    ],
    "description": "Proper cluster configuration ensures data distribution and availability"
  },
  {
    "id": "database-migration-handling",
    "category": "dockerfile",
    "pattern": "(flyway|liquibase|migrate):",
    "recommendation": "Use database migration tools for schema management",
    "example": "services:\n  migrate:\n    image: flyway/flyway:latest\n    command: -url=jdbc:postgresql://db:5432/${DB_NAME} -user=${DB_USER} -password=${DB_PASS} -connectRetries=60 migrate\n    volumes:\n      - ./migrations:/flyway/sql\n    depends_on:\n      db:\n        condition: service_healthy",
    "severity": "high",
    "tags": [
      "database",
      "fix-dockerfile",
      "generate-dockerfile",
      "migrations",
      "schema",
      "versioning"
    ],
    "description": "Migration tools ensure consistent schema across environments"
  },
  {
    "id": "timescaledb-optimization",
    "category": "dockerfile",
    "pattern": "timescale:",
    "recommendation": "Optimize TimescaleDB for time-series workloads",
    "example": "command: postgres -c shared_preload_libraries=timescaledb -c timescaledb.max_background_workers=8 -c timescaledb.last_tuned='2024-01-01' -c timescaledb.telemetry_level=off",
    "severity": "medium",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "optimization",
      "postgres",
      "timescaledb",
      "timeseries"
    ],
    "description": "TimescaleDB-specific settings optimize time-series performance"
  },
  {
    "id": "database-monitoring-export",
    "category": "dockerfile",
    "pattern": "(postgres|mysql)_exporter:",
    "recommendation": "Deploy database exporters for Prometheus monitoring",
    "example": "services:\n  postgres_exporter:\n    image: prometheuscommunity/postgres-exporter:latest\n    environment:\n      DATA_SOURCE_NAME: postgresql://${DB_USER}:${DB_PASS}@postgres:5432/${DB_NAME}?sslmode=disable\n      PG_EXPORTER_DISABLE_DEFAULT_METRICS: false\n    ports:\n      - '9187:9187'",
    "severity": "medium",
    "tags": [
      "database",
      "fix-dockerfile",
      "generate-dockerfile",
      "metrics",
      "monitoring",
      "prometheus"
    ],
    "description": "Exporters provide database metrics for monitoring systems"
  },
  {
    "id": "mongodb-sharding",
    "category": "dockerfile",
    "pattern": "mongo.*shard",
    "recommendation": "Configure MongoDB sharding for horizontal scaling",
    "example": "# Config server\nservices:\n  configsvr:\n    image: mongo:7\n    command: mongod --configsvr --replSet configrs --port 27019 --bind_ip_all\n  mongos:\n    image: mongo:7\n    command: mongos --configdb configrs/configsvr:27019 --bind_ip_all --port 27017",
    "severity": "high",
    "tags": [
      "distributed",
      "fix-dockerfile",
      "generate-dockerfile",
      "mongodb",
      "scaling",
      "sharding"
    ],
    "description": "Sharding enables horizontal scaling for large datasets"
  },
  {
    "id": "redis-cluster-mode",
    "category": "dockerfile",
    "pattern": "redis.*cluster",
    "recommendation": "Configure Redis cluster for high availability and scaling",
    "example": "command: redis-server --cluster-enabled yes --cluster-config-file nodes.conf --cluster-node-timeout 5000 --appendonly yes --protected-mode no\nvolumes:\n  - redis-cluster-data:/data",
    "severity": "high",
    "tags": [
      "availability",
      "cluster",
      "fix-dockerfile",
      "generate-dockerfile",
      "redis",
      "scaling"
    ],
    "description": "Redis cluster provides automatic sharding and failover"
  },
  {
    "id": "redis-sentinel",
    "category": "dockerfile",
    "pattern": "redis.*sentinel",
    "recommendation": "Use Redis Sentinel for high availability",
    "example": "services:\n  sentinel:\n    image: redis:7-alpine\n    command: redis-sentinel /etc/redis/sentinel.conf\n    volumes:\n      - ./sentinel.conf:/etc/redis/sentinel.conf\n    environment:\n      - REDIS_MASTER_HOST=${REDIS_MASTER}",
    "severity": "high",
    "tags": [
      "availability",
      "failover",
      "fix-dockerfile",
      "generate-dockerfile",
      "redis",
      "sentinel"
    ],
    "description": "Sentinel provides automatic failover for Redis primary-replica setups"
  },
  {
    "id": "database-security-scanning",
    "category": "security",
    "pattern": "(postgres|mysql|mongo|redis):",
    "recommendation": "Implement security scanning for database containers",
    "example": "# Security scanning stage\nFROM aquasec/trivy:latest AS security\nCOPY --from=database /app /app\nRUN trivy fs --exit-code 1 --severity HIGH,CRITICAL --no-progress /app\n\n# Scan for known CVEs\nRUN trivy image --exit-code 1 --severity HIGH,CRITICAL postgres:15",
    "severity": "high",
    "tags": [
      "database",
      "fix-dockerfile",
      "scan-image",
      "scanning",
      "security",
      "vulnerabilities"
    ],
    "description": "Security scanning identifies vulnerabilities in database images"
  },
  {
    "id": "mysql-innodb-optimization",
    "category": "dockerfile",
    "pattern": "mysql:",
    "recommendation": "Optimize InnoDB settings for containerized MySQL",
    "example": "command: --innodb-buffer-pool-size=2G --innodb-log-file-size=256M --innodb-flush-log-at-trx-commit=2 --innodb-flush-method=O_DIRECT --innodb-file-per-table=1",
    "severity": "medium",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "innodb",
      "mysql",
      "optimization",
      "performance"
    ],
    "description": "InnoDB optimization improves MySQL performance in containers"
  },
  {
    "id": "postgres-pgvector",
    "category": "dockerfile",
    "pattern": "postgres.*vector",
    "recommendation": "Configure PostgreSQL with pgvector for AI/ML workloads",
    "example": "FROM pgvector/pgvector:pg16\nRUN apt-get update && apt-get install -y postgresql-16-pgvector\nCMD postgres -c shared_preload_libraries=vector -c vector.max_vector_dimension=2000",
    "severity": "medium",
    "tags": [
      "ai",
      "embeddings",
      "fix-dockerfile",
      "generate-dockerfile",
      "ml",
      "pgvector",
      "postgres"
    ],
    "description": "pgvector enables efficient vector similarity search for AI applications"
  },
  {
    "id": "couchdb-cluster",
    "category": "dockerfile",
    "pattern": "couchdb:",
    "recommendation": "Configure CouchDB cluster for distributed document storage",
    "example": "environment:\n  - COUCHDB_USER=${COUCH_USER}\n  - COUCHDB_PASSWORD=${COUCH_PASS}\n  - COUCHDB_SECRET=${COUCH_SECRET}\n  - NODENAME=couchdb@${HOSTNAME}\n  - ERL_FLAGS=-setcookie ${ERLANG_COOKIE}",
    "severity": "medium",
    "tags": [
      "aws",
      "cluster",
      "couchdb",
      "document",
      "fix-dockerfile",
      "generate-dockerfile",
      "nosql"
    ],
    "description": "CouchDB clustering provides distributed document storage with replication"
  },
  {
    "id": "influxdb-timeseries",
    "category": "dockerfile",
    "pattern": "influxdb:",
    "recommendation": "Optimize InfluxDB for time-series data",
    "example": "environment:\n  - INFLUXDB_DB=${DB_NAME}\n  - INFLUXDB_ADMIN_USER=${ADMIN_USER}\n  - INFLUXDB_ADMIN_PASSWORD=${ADMIN_PASS}\n  - INFLUXDB_DATA_CACHE_MAX_MEMORY_SIZE=2g\n  - INFLUXDB_DATA_WAL_FSYNC_DELAY=10ms\nvolumes:\n  - influxdb_data:/var/lib/influxdb2",
    "severity": "medium",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "influxdb",
      "metrics",
      "monitoring",
      "timeseries"
    ],
    "description": "InfluxDB optimization for high-performance time-series storage"
  },
  {
    "id": "neo4j-graph-database",
    "category": "dockerfile",
    "pattern": "neo4j:",
    "recommendation": "Configure Neo4j graph database for production",
    "example": "environment:\n  - NEO4J_AUTH=${NEO4J_USER}/${NEO4J_PASS}\n  - NEO4J_dbms_memory_heap_max__size=2G\n  - NEO4J_dbms_memory_pagecache_size=1G\n  - NEO4J_dbms_connector_bolt_enabled=true\n  - NEO4J_dbms_security_procedures_unrestricted=algo.*,apoc.*",
    "severity": "medium",
    "tags": [
      "database",
      "fix-dockerfile",
      "generate-dockerfile",
      "graph",
      "neo4j",
      "nosql"
    ],
    "description": "Neo4j configuration for graph database workloads"
  },
  {
    "id": "dynamodb-local",
    "category": "dockerfile",
    "pattern": "dynamodb-local:",
    "recommendation": "Configure DynamoDB Local for development and testing",
    "example": "services:\n  dynamodb:\n    image: amazon/dynamodb-local:latest\n    command: -jar DynamoDBLocal.jar -sharedDb -dbPath /data\n    ports:\n      - '8000:8000'\n    volumes:\n      - dynamodb_data:/data",
    "severity": "low",
    "tags": [
      "aws",
      "development",
      "dynamodb",
      "fix-dockerfile",
      "generate-dockerfile",
      "nosql"
    ],
    "description": "DynamoDB Local provides AWS-compatible NoSQL for development"
  },
  {
    "id": "database-encryption-at-rest",
    "category": "security",
    "pattern": "(postgres|mysql|mongo):",
    "recommendation": "Enable encryption at rest for database storage",
    "example": "# PostgreSQL with encrypted volume\nvolumes:\n  db_data:\n    driver: local\n    driver_opts:\n      type: 'none'\n      o: 'bind,encrypt=aes-xts-plain64,key-size=512'\n      device: '/encrypted/postgresql'",
    "severity": "high",
    "tags": [
      "compliance",
      "database",
      "encryption",
      "fix-dockerfile",
      "scan-image",
      "security"
    ],
    "description": "Encryption at rest protects data on disk"
  },
  {
    "id": "database-tls-configuration",
    "category": "security",
    "pattern": "(postgres|mysql|mongo):",
    "recommendation": "Enable TLS/SSL for database connections",
    "example": "command: postgres -c ssl=on -c ssl_cert_file=/certs/server.crt -c ssl_key_file=/certs/server.key -c ssl_ca_file=/certs/ca.crt\nvolumes:\n  - ./certs:/certs:ro\nenvironment:\n  - POSTGRES_HOST_AUTH_METHOD=scram-sha-256",
    "severity": "high",
    "tags": [
      "database",
      "encryption",
      "fix-dockerfile",
      "scan-image",
      "security",
      "ssl",
      "tls"
    ],
    "description": "TLS encryption protects data in transit"
  },
  {
    "id": "database-audit-logging",
    "category": "security",
    "pattern": "(postgres|mysql):",
    "recommendation": "Enable audit logging for compliance and security",
    "example": "# PostgreSQL with pgAudit\ncommand: postgres -c shared_preload_libraries=pgaudit -c pgaudit.log=ALL -c log_destination=csvlog -c logging_collector=on",
    "severity": "high",
    "tags": [
      "audit",
      "compliance",
      "database",
      "fix-dockerfile",
      "logging",
      "scan-image",
      "security"
    ],
    "description": "Audit logging tracks database access and modifications"
  },
  {
    "id": "mongodb-authentication",
    "category": "security",
    "pattern": "mongo:",
    "recommendation": "Enable MongoDB authentication and authorization",
    "example": "command: mongod --auth --keyFile /data/keyfile --replSet rs0\nenvironment:\n  - MONGO_INITDB_ROOT_USERNAME=${MONGO_ROOT_USER}\n  - MONGO_INITDB_ROOT_PASSWORD=${MONGO_ROOT_PASS}\n  - MONGO_INITDB_DATABASE=admin",
    "severity": "high",
    "tags": [
      "authentication",
      "authorization",
      "fix-dockerfile",
      "mongodb",
      "scan-image",
      "security"
    ],
    "description": "Authentication prevents unauthorized database access"
  },
  {
    "id": "redis-acl-configuration",
    "category": "security",
    "pattern": "redis:",
    "recommendation": "Configure Redis ACL for fine-grained access control",
    "example": "command: redis-server --requirepass ${REDIS_PASS} --aclfile /etc/redis/users.acl\nvolumes:\n  - ./redis-acl.conf:/etc/redis/users.acl:ro",
    "severity": "high",
    "tags": [
      "access-control",
      "acl",
      "fix-dockerfile",
      "redis",
      "scan-image",
      "security"
    ],
    "description": "ACLs provide user-level access control in Redis"
  },
  {
    "id": "database-performance-tuning",
    "category": "generic",
    "pattern": "(postgres|mysql):",
    "recommendation": "Apply performance tuning based on workload",
    "example": "# OLTP workload tuning\ncommand: postgres \\\n  -c max_connections=400 \\\n  -c shared_buffers=4GB \\\n  -c effective_cache_size=12GB \\\n  -c work_mem=10MB \\\n  -c random_page_cost=1.1 \\\n  -c effective_io_concurrency=200",
    "severity": "medium",
    "tags": [
      "database",
      "performance",
      "tuning",
      "optimization"
    ],
    "description": "Workload-specific tuning maximizes database performance"
  },
  {
    "id": "clickhouse-analytics",
    "category": "dockerfile",
    "pattern": "clickhouse:",
    "recommendation": "Configure ClickHouse for analytical workloads",
    "example": "environment:\n  - CLICKHOUSE_USER=${CH_USER}\n  - CLICKHOUSE_PASSWORD=${CH_PASS}\n  - CLICKHOUSE_DEFAULT_ACCESS_MANAGEMENT=1\n  - CLICKHOUSE_MAX_MEMORY_USAGE=10000000000\nvolumes:\n  - clickhouse_data:/var/lib/clickhouse",
    "severity": "medium",
    "tags": [
      "analytics",
      "clickhouse",
      "columnar",
      "fix-dockerfile",
      "generate-dockerfile",
      "olap"
    ],
    "description": "ClickHouse configuration for high-performance analytics"
  },
  {
    "id": "database-zero-downtime-migration",
    "category": "kubernetes",
    "pattern": "(flyway|liquibase):",
    "recommendation": "Implement zero-downtime database migrations",
    "example": "# Blue-green migration strategy\nservices:\n  migrate-pre:\n    image: flyway/flyway\n    command: migrate -placeholders.deployment.type=blue\n  app-blue:\n    depends_on:\n      migrate-pre:\n        condition: service_completed_successfully",
    "severity": "high",
    "tags": [
      "database",
      "deploy",
      "deployment",
      "generate-k8s-manifests",
      "migration",
      "zero-downtime"
    ],
    "description": "Zero-downtime migrations prevent service interruptions"
  },
  {
    "id": "database-disaster-recovery",
    "category": "generic",
    "pattern": "(postgres|mysql):",
    "recommendation": "Implement comprehensive disaster recovery strategy",
    "example": "# WAL-E for PostgreSQL continuous archiving\nservices:\n  wal-e:\n    image: postgres:15\n    command: |\n      sh -c 'wal-e backup-push /var/lib/postgresql/data'\n    environment:\n      - AWS_ACCESS_KEY_ID=${AWS_KEY}\n      - AWS_SECRET_ACCESS_KEY=${AWS_SECRET}\n      - WALE_S3_PREFIX=s3://backup-bucket/postgres",
    "severity": "high",
    "tags": [
      "archiving",
      "aws",
      "backup",
      "database",
      "disaster-recovery"
    ],
    "description": "Continuous archiving enables point-in-time recovery"
  },
  {
    "id": "scylladb-configuration",
    "category": "dockerfile",
    "pattern": "scylladb:",
    "recommendation": "Configure ScyllaDB for high-performance NoSQL",
    "example": "command: --smp 4 --memory 4G --overprovisioned 1 --developer-mode 0\nenvironment:\n  - SCYLLA_CLUSTER_NAME=scylla-cluster\n  - SCYLLA_SEEDS=scylla-node1,scylla-node2\n  - SCYLLA_ENDPOINT_SNITCH=GossipingPropertyFileSnitch",
    "severity": "medium",
    "tags": [
      "cassandra-compatible",
      "fix-dockerfile",
      "generate-dockerfile",
      "nosql",
      "performance",
      "scylladb"
    ],
    "description": "ScyllaDB provides C++ reimplementation of Cassandra with better performance"
  },
  {
    "id": "database-observability-stack",
    "category": "generic",
    "pattern": "(postgres|mysql|mongo):",
    "recommendation": "Deploy comprehensive observability stack",
    "example": "# Grafana + Prometheus + Loki stack\nservices:\n  prometheus:\n    image: prom/prometheus\n    volumes:\n      - ./prometheus.yml:/etc/prometheus/prometheus.yml\n  grafana:\n    image: grafana/grafana\n    environment:\n      - GF_SECURITY_ADMIN_PASSWORD=${GRAFANA_PASS}\n  loki:\n    image: grafana/loki\n    command: -config.file=/etc/loki/config.yml",
    "severity": "high",
    "tags": [
      "database",
      "observability",
      "monitoring",
      "metrics",
      "logging"
    ],
    "description": "Full observability stack for database monitoring and troubleshooting"
  },
  {
    "id": "etcd-configuration",
    "category": "dockerfile",
    "pattern": "etcd:",
    "recommendation": "Configure etcd for distributed key-value storage",
    "example": "command: |\n  etcd \\\n  --name etcd0 \\\n  --initial-advertise-peer-urls http://etcd0:2380 \\\n  --listen-peer-urls http://0.0.0.0:2380 \\\n  --advertise-client-urls http://etcd0:2379 \\\n  --listen-client-urls http://0.0.0.0:2379 \\\n  --initial-cluster etcd0=http://etcd0:2380,etcd1=http://etcd1:2380",
    "severity": "medium",
    "tags": [
      "consensus",
      "distributed",
      "etcd",
      "fix-dockerfile",
      "generate-dockerfile",
      "key-value"
    ],
    "description": "etcd provides distributed key-value storage with strong consistency"
  },
  {
    "id": "cockroachdb-cluster",
    "category": "dockerfile",
    "pattern": "cockroachdb:",
    "recommendation": "Configure CockroachDB for distributed SQL",
    "example": "command: start --insecure --join=roach1,roach2,roach3 --cache=.25 --max-sql-memory=.25\nvolumes:\n  - cockroach_data:/cockroach/cockroach-data\nenvironment:\n  - COCKROACH_CHANNEL=kubernetes-multiregion",
    "severity": "high",
    "tags": [
      "cockroachdb",
      "distributed-sql",
      "fix-dockerfile",
      "generate-dockerfile",
      "resilient",
      "scaling"
    ],
    "description": "CockroachDB provides distributed SQL with strong consistency"
  },
  {
    "id": "database-cdc-setup",
    "category": "generic",
    "pattern": "(postgres|mysql):",
    "recommendation": "Implement Change Data Capture (CDC) for real-time data streaming",
    "example": "# Debezium for CDC\nservices:\n  debezium:\n    image: debezium/connect\n    environment:\n      - BOOTSTRAP_SERVERS=kafka:9092\n      - GROUP_ID=1\n      - CONFIG_STORAGE_TOPIC=connect-configs\n      - OFFSET_STORAGE_TOPIC=connect-offsets\n      - STATUS_STORAGE_TOPIC=connect-status",
    "severity": "medium",
    "tags": [
      "database",
      "cdc",
      "streaming",
      "debezium",
      "kafka"
    ],
    "description": "CDC enables real-time data synchronization and event streaming"
  },
  {
    "id": "vitess-sharding",
    "category": "dockerfile",
    "pattern": "vitess:",
    "recommendation": "Use Vitess for MySQL horizontal scaling",
    "example": "services:\n  vtgate:\n    image: vitess/lite\n    command: vtgate -mysql_server_port 3306 -grpc_port 15991 -service_map 'grpc-vtgateservice'\n  vttablet:\n    image: vitess/lite\n    command: vttablet -tablet-path 'zone1-100' -init_keyspace test_keyspace",
    "severity": "high",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "mysql",
      "proxy",
      "scaling",
      "sharding",
      "vitess"
    ],
    "description": "Vitess provides horizontal scaling for MySQL databases"
  },
  {
    "id": "arangodb-multimodel",
    "category": "dockerfile",
    "pattern": "arangodb:",
    "recommendation": "Configure ArangoDB for multi-model database workloads",
    "example": "environment:\n  - ARANGO_ROOT_PASSWORD=${ARANGO_PASS}\n  - ARANGO_STORAGE_ENGINE=rocksdb\n  - ARANGO_CLUSTER=true\ncommand: arangod --server.endpoint tcp://0.0.0.0:8529",
    "severity": "medium",
    "tags": [
      "arangodb",
      "document",
      "fix-dockerfile",
      "generate-dockerfile",
      "graph",
      "multimodel",
      "nosql"
    ],
    "description": "ArangoDB supports document, graph, and key-value data models"
  },
  {
    "id": "database-compliance-controls",
    "category": "security",
    "pattern": "(postgres|mysql):",
    "recommendation": "Implement compliance controls for regulated industries",
    "example": "# HIPAA/PCI compliance settings\ncommand: postgres \\\n  -c log_connections=on \\\n  -c log_disconnections=on \\\n  -c log_statement=all \\\n  -c log_min_duration_statement=0 \\\n  -c ssl=on \\\n  -c ssl_ciphers='HIGH:!aNULL:!MD5'",
    "severity": "high",
    "tags": [
      "compliance",
      "database",
      "fix-dockerfile",
      "hipaa",
      "pci",
      "scan-image",
      "security"
    ],
    "description": "Compliance controls meet regulatory requirements"
  }
] as const;
