# Data Services Deployment Guide

This guide covers the deployment of the 3 data services that provide user management, data operations, and analytics capabilities for the 002AIC platform, building upon the core services and AI/ML infrastructure.

## Overview

The data services deployment includes:

### Data Services (3)
1. **User Management Service** (Go) - User authentication, authorization, and profile management
2. **Data Management Service** (Go) - Dataset management, storage, and data lifecycle operations
3. **Analytics Service** (Python) - Business intelligence, reporting, and real-time analytics

### Supporting Infrastructure
- **MongoDB** - Document storage for user profiles and unstructured data
- **ClickHouse** - High-performance analytics database for time-series data
- **Elasticsearch** - Full-text search and log analytics
- **Enhanced PostgreSQL** - Additional databases for data services
- **Multi-tier Storage** - S3, GCS, and Azure Blob Storage integration

## Prerequisites

### Core and AI/ML Services Requirement
The data services depend on both core and AI/ML services:
```bash
# Ensure core services are running
./scripts/deploy-core-services.sh verify

# Ensure AI/ML services are running (optional but recommended)
./scripts/deploy-ai-ml-services.sh verify
```

### Compliance Requirements
Data services handle sensitive user data and require:
- **GDPR Compliance** - European data protection regulations
- **HIPAA Compliance** - Healthcare data protection (if applicable)
- **SOX Compliance** - Financial data protection
- **Data Encryption** - At rest and in transit
- **Audit Logging** - Comprehensive activity tracking

### Resource Requirements
```yaml
Minimum Resources:
  CPU: 6 cores
  Memory: 12GB RAM
  Storage: 300GB persistent storage

Recommended Resources:
  CPU: 12 cores
  Memory: 24GB RAM
  Storage: 1TB persistent storage
```

## Quick Start

### 1. Verify Prerequisites
```bash
# Check that core services are running
kubectl get pods -n aic-platform
kubectl get service postgresql-service -n aic-platform

# Check AI/ML services (optional)
kubectl get pods -n aic-ai-ml
```

### 2. Deploy Data Services
```bash
# Full data services deployment
./scripts/deploy-data-services.sh

# Or deploy components separately
./scripts/deploy-data-services.sh databases
./scripts/deploy-data-services.sh services
```

### 3. Verify Deployment
```bash
# Check deployment status
./scripts/deploy-data-services.sh verify

# View all data resources
kubectl get all -n aic-data
```

## Detailed Deployment Steps

### Step 1: Namespace and Resource Management

#### Data Namespace Creation
```bash
# Create dedicated namespace with resource quotas
kubectl apply -f infra/k8s/data-services/namespace.yaml

# Verify namespace and quotas
kubectl describe namespace aic-data
kubectl describe resourcequota data-resource-quota -n aic-data
```

#### Resource Quotas
The data namespace includes resource quotas:
- **CPU**: 10 cores requested, 20 cores limit
- **Memory**: 20GB requested, 40GB limit
- **Storage**: 6 persistent volume claims
- **Load Balancers**: 2 external services

### Step 2: Database Infrastructure

#### PostgreSQL Extension
```bash
# Update PostgreSQL with data service databases
kubectl apply -f infra/k8s/infrastructure/postgresql-data-init.yaml

# Restart PostgreSQL to apply changes
kubectl rollout restart statefulset/postgresql -n aic-platform
```

#### Data Service Databases Created
- `user_management_db` - User profiles, sessions, audit logs
- `data_management_db` - Dataset metadata, lineage, quality checks
- `analytics_db` - Reports, dashboards, metrics (with TimescaleDB)

#### MongoDB Deployment
```bash
# Deploy MongoDB for document storage
kubectl apply -f infra/k8s/data-services/mongodb.yaml

# Wait for MongoDB to be ready
kubectl wait --for=condition=ready pod -l app=mongodb -n aic-data --timeout=600s

# Access MongoDB
kubectl port-forward service/mongodb-service 27017:27017 -n aic-data
```

#### ClickHouse Deployment
```bash
# Deploy ClickHouse for analytics
kubectl apply -f infra/k8s/data-services/clickhouse.yaml

# Wait for ClickHouse to be ready
kubectl wait --for=condition=ready pod -l app=clickhouse -n aic-data --timeout=600s

# Access ClickHouse
kubectl port-forward service/clickhouse-service 8123:8123 -n aic-data
# Open http://localhost:8123/play for web interface
```

#### Elasticsearch Deployment
```bash
# Deploy Elasticsearch for search and analytics
kubectl apply -f infra/k8s/data-services/elasticsearch.yaml

# Wait for Elasticsearch to be ready
kubectl wait --for=condition=ready pod -l app=elasticsearch -n aic-data --timeout=600s

# Access Elasticsearch
kubectl port-forward service/elasticsearch-service 9200:9200 -n aic-data
# Test: curl http://localhost:9200/_cluster/health
```

### Step 3: Data Services Deployment

#### Service Deployment Order
Services are deployed in dependency order:

1. **User Management Service** - Foundation for user operations
2. **Data Management Service** - Dataset and storage operations
3. **Analytics Service** - Business intelligence and reporting

#### Individual Service Details

##### User Management Service
```yaml
Features:
  - User registration and authentication
  - Organization and team management
  - Role-based access control (RBAC)
  - GDPR compliance and data privacy
  - Session management and security
  - Audit logging and compliance reporting

Resources:
  - 3 replicas for high availability
  - 1GB memory, 500m CPU per replica
  - Persistent storage for user data
  - Encryption at rest and in transit

Compliance:
  - GDPR data protection
  - Password policy enforcement
  - Account lockout protection
  - Data retention policies
  - Consent tracking
```

##### Data Management Service
```yaml
Features:
  - Dataset lifecycle management
  - Multi-cloud storage integration (S3, GCS, Azure)
  - Data versioning and lineage tracking
  - Data quality validation
  - Compression and encryption
  - Metadata management

Resources:
  - 2 replicas for reliability
  - 2GB memory, 1 CPU core per replica
  - Large persistent storage for data operations
  - Temporary storage for processing

Storage Backends:
  - Primary: S3-compatible storage
  - Backup: Local persistent volumes
  - Supported formats: JSON, CSV, Parquet, Avro, ORC
```

##### Analytics Service
```yaml
Features:
  - Real-time and batch analytics
  - Interactive dashboards and reports
  - Custom metrics and KPIs
  - Data visualization and charting
  - Scheduled report generation
  - Alert and notification system

Resources:
  - 2 replicas for parallel processing
  - 2GB memory, 1 CPU core per replica
  - Analytics storage and cache volumes
  - Integration with Spark for big data processing

Analytics Engines:
  - Apache Spark for distributed processing
  - Pandas for data manipulation
  - Dask for parallel computing
  - ClickHouse for OLAP queries
```

## Security and Compliance

### Data Protection
```yaml
Encryption:
  - TLS 1.3 for data in transit
  - AES-256 for data at rest
  - Field-level encryption for sensitive data
  - Key rotation and management

Access Control:
  - JWT-based authentication
  - Role-based access control (RBAC)
  - Multi-factor authentication (MFA)
  - API rate limiting and throttling
```

### GDPR Compliance
```yaml
Features:
  - Right to be forgotten (data deletion)
  - Data portability and export
  - Consent management and tracking
  - Data processing transparency
  - Privacy by design principles
  - Data breach notification

Implementation:
  - User data anonymization
  - Audit trail for all data operations
  - Configurable data retention policies
  - Automated compliance reporting
```

### Audit and Monitoring
```yaml
Audit Logging:
  - All user actions logged
  - Administrative operations tracked
  - Data access and modifications recorded
  - Compliance events monitored

Retention:
  - 7 years for compliance data
  - Configurable retention policies
  - Automated data archival
  - Secure data disposal
```

## Storage Configuration

### Persistent Volume Claims
Each data service has dedicated storage:

```bash
# View all PVCs
kubectl get pvc -n aic-data

# Storage allocations
user-management-pvc: 20Gi        # User profiles and sessions
data-management-pvc: 100Gi       # Dataset metadata and processing
analytics-pvc: 50Gi              # Reports and dashboards
analytics-cache-pvc: 20Gi        # Analytics cache
mongodb-storage: 50Gi            # Document storage
clickhouse-storage: 100Gi        # Analytics database
elasticsearch-storage: 50Gi      # Search indexes
```

### Multi-Cloud Storage
```yaml
Supported Backends:
  - Amazon S3 and S3-compatible storage
  - Google Cloud Storage (GCS)
  - Azure Blob Storage
  - Local persistent volumes

Configuration:
  - Primary backend for production data
  - Backup backend for redundancy
  - Cross-region replication
  - Lifecycle management policies
```

## Monitoring and Observability

### Prometheus Metrics
Data services expose comprehensive metrics:
- `user_registrations_total` - Total user registrations
- `user_sessions_active` - Active user sessions
- `datasets_total` - Total datasets managed
- `data_processing_jobs_duration` - Data processing times
- `analytics_queries_total` - Analytics query count
- `analytics_dashboard_views` - Dashboard usage metrics

### Database Monitoring
- **PostgreSQL**: Connection pool usage, query performance
- **MongoDB**: Document operations, index usage
- **ClickHouse**: Query performance, compression ratios
- **Elasticsearch**: Index size, search performance

### Grafana Dashboards
Pre-configured dashboards for data services:
- **User Management Overview** - User activity and authentication metrics
- **Data Operations Dashboard** - Dataset management and processing
- **Analytics Performance** - Query performance and dashboard usage
- **Database Health** - All database systems health and performance
- **Compliance Monitoring** - GDPR and audit compliance metrics

## API Integration

### User Management API
```bash
# User registration
POST /v1/users/register
{
  "email": "user@example.com",
  "password": "secure_password",
  "first_name": "John",
  "last_name": "Doe",
  "organization_id": "uuid"
}

# User authentication
POST /v1/users/login
{
  "email": "user@example.com",
  "password": "secure_password"
}

# Get user profile
GET /v1/users/profile
Authorization: Bearer <jwt_token>
```

### Data Management API
```bash
# Create dataset
POST /v1/datasets
{
  "name": "customer_data",
  "description": "Customer information dataset",
  "schema": {...},
  "storage_type": "s3"
}

# Upload data
POST /v1/datasets/{id}/upload
Content-Type: multipart/form-data

# Get dataset metadata
GET /v1/datasets/{id}
```

### Analytics API
```bash
# Create report
POST /v1/analytics/reports
{
  "name": "Monthly Sales Report",
  "query": "SELECT * FROM sales WHERE month = ?",
  "visualization": "chart"
}

# Execute query
POST /v1/analytics/query
{
  "query": "SELECT COUNT(*) FROM users",
  "database": "postgresql"
}

# Get dashboard
GET /v1/analytics/dashboards/{id}
```

## Scaling and Performance

### Horizontal Pod Autoscaling
```yaml
# Example HPA for User Management Service
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: user-management-hpa
  namespace: aic-data
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: user-management-service
  minReplicas: 3
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

### Database Scaling
```bash
# Scale MongoDB replica set
kubectl scale statefulset mongodb --replicas=3 -n aic-data

# Scale ClickHouse cluster
kubectl scale statefulset clickhouse --replicas=3 -n aic-data

# Scale Elasticsearch cluster
kubectl scale statefulset elasticsearch --replicas=3 -n aic-data
```

### Performance Optimization
- **Connection Pooling**: Optimize database connections
- **Caching Strategy**: Multi-tier caching with Redis
- **Query Optimization**: Index optimization and query tuning
- **Data Partitioning**: Time-based and hash partitioning
- **Compression**: Data compression for storage efficiency

## Troubleshooting

### Common Issues

#### User Authentication Failures
```bash
# Check user management service logs
kubectl logs -f deployment/user-management-service -n aic-data

# Verify JWT secret configuration
kubectl get secret user-management-secret -n aic-data -o yaml

# Test authentication endpoint
curl -X POST http://user-management-service.aic-data.svc.cluster.local/v1/users/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password"}'
```

#### Database Connection Issues
```bash
# Test PostgreSQL connectivity
kubectl exec -it postgresql-0 -n aic-platform -- \
  psql -U postgres -d user_management_db -c "SELECT version();"

# Test MongoDB connectivity
kubectl exec -it mongodb-0 -n aic-data -- \
  mongosh --eval "db.adminCommand('ping')"

# Test ClickHouse connectivity
kubectl exec -it clickhouse-0 -n aic-data -- \
  clickhouse-client --query "SELECT version()"

# Test Elasticsearch connectivity
kubectl run es-test --image=curlimages/curl --rm -i --restart=Never -- \
  curl -f http://elasticsearch-service.aic-data.svc.cluster.local:9200/_cluster/health
```

#### Data Processing Failures
```bash
# Check data management service logs
kubectl logs -f deployment/data-management-service -n aic-data

# Verify storage credentials
kubectl get secret data-storage-secret -n aic-data -o yaml

# Check dataset processing status
curl http://data-management-service.aic-data.svc.cluster.local/v1/datasets/status
```

#### Analytics Query Performance
```bash
# Check analytics service logs
kubectl logs -f deployment/analytics-service -n aic-data

# Monitor ClickHouse query performance
kubectl exec -it clickhouse-0 -n aic-data -- \
  clickhouse-client --query "SELECT * FROM system.query_log ORDER BY event_time DESC LIMIT 10"

# Check Elasticsearch index health
curl http://elasticsearch-service.aic-data.svc.cluster.local:9200/_cat/indices?v
```

### Debugging Commands
```bash
# Get all data resources
kubectl get all -n aic-data

# Check resource usage
kubectl top pods -n aic-data

# View events
kubectl get events -n aic-data --sort-by='.lastTimestamp'

# Debug specific service
kubectl describe deployment user-management-service -n aic-data
kubectl logs -f deployment/user-management-service -n aic-data

# Port forward for debugging
kubectl port-forward deployment/user-management-service 8080:8080 -n aic-data
```

## Integration with Other Services

### Core Services Integration
```bash
# Authentication through core auth service
curl -H "Authorization: Bearer <token>" \
  http://user-management-service.aic-data.svc.cluster.local/v1/users/profile

# Configuration from core config service
curl http://configuration-service.aic-platform.svc.cluster.local/v1/config/data-services

# Health monitoring by core health check service
curl http://health-check-service.aic-platform.svc.cluster.local/v1/services | grep data
```

### AI/ML Services Integration
```bash
# Data integration with AI/ML pipeline
curl -X POST http://data-integration-service.aic-ai-ml.svc.cluster.local/v1/pipelines \
  -H "Content-Type: application/json" \
  -d '{"source":"data-management","destination":"training-data"}'

# Analytics for ML model performance
curl http://analytics-service.aic-data.svc.cluster.local/v1/analytics/ml-metrics
```

## Maintenance

### Regular Maintenance Tasks

#### Daily
- Monitor user authentication success rates
- Check data processing job status
- Review analytics query performance
- Monitor database health and performance

#### Weekly
- Review user management audit logs
- Check data quality validation results
- Analyze analytics dashboard usage
- Review and rotate access credentials

#### Monthly
- Update user management policies
- Archive old analytics data
- Review data retention compliance
- Update database indexes and statistics

#### Quarterly
- Conduct GDPR compliance audit
- Review and update data retention policies
- Performance optimization and tuning
- Security vulnerability assessment

### Backup and Recovery
```bash
# Backup PostgreSQL data services databases
kubectl exec postgresql-0 -n aic-platform -- \
  pg_dump -U postgres user_management_db > user-mgmt-backup-$(date +%Y%m%d).sql

kubectl exec postgresql-0 -n aic-platform -- \
  pg_dump -U postgres data_management_db > data-mgmt-backup-$(date +%Y%m%d).sql

kubectl exec postgresql-0 -n aic-platform -- \
  pg_dump -U postgres analytics_db > analytics-backup-$(date +%Y%m%d).sql

# Backup MongoDB
kubectl exec mongodb-0 -n aic-data -- \
  mongodump --out /tmp/mongodb-backup-$(date +%Y%m%d)

# Backup ClickHouse
kubectl exec clickhouse-0 -n aic-data -- \
  clickhouse-client --query "BACKUP DATABASE analytics TO S3('s3://backup-bucket/clickhouse/$(date +%Y%m%d)')"

# Backup Elasticsearch indexes
kubectl exec elasticsearch-0 -n aic-data -- \
  curl -X PUT "localhost:9200/_snapshot/backup_repo/snapshot_$(date +%Y%m%d)"
```

## Next Steps

After successfully deploying the data services:

1. **DevOps Services Group** - Deploy monitoring, deployment, and operational services
2. **Business Services Group** - Add billing, support, and marketplace services
3. **Advanced Data Features** - Implement data mesh and advanced analytics
4. **Multi-Region Data** - Expand data services across regions
5. **Data Governance** - Implement advanced data governance and cataloging
6. **Real-time Analytics** - Add streaming analytics capabilities

## Support

For data services specific issues and questions:
- **Documentation**: `/documentation/ops/data-services-deployment.md`
- **Database Access**: Use port-forwarding for local development
- **GitHub Issues**: Tag with `data-services` label
- **Slack**: #data-platform channel
- **Email**: data-team@appliedinnovationcorp.com

## Performance Benchmarks

### Expected Performance Metrics
- **User Authentication**: <100ms response time for login
- **Data Upload**: 100MB/s throughput for dataset uploads
- **Analytics Queries**: <5s for standard dashboard queries
- **Database Operations**: <50ms for CRUD operations
- **Search Performance**: <200ms for full-text search queries

### Monitoring Thresholds
- **CPU Usage**: Alert if >80% for >5 minutes
- **Memory Usage**: Alert if >85% for >5 minutes
- **Database Connections**: Alert if >80% of pool capacity
- **Storage Usage**: Alert if >90% capacity
- **Authentication Failures**: Alert if >5% failure rate

## Compliance Reporting

### GDPR Compliance Dashboard
- Data processing activities log
- User consent tracking
- Data retention policy compliance
- Right to be forgotten requests
- Data breach incident tracking

### Audit Reports
- User access and activity logs
- Administrative actions audit
- Data modification tracking
- Compliance violation alerts
- Security incident reports
