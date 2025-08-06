# Core Services Deployment Guide

This guide covers the deployment of the 5 core services that form the foundation of the 002AIC platform, along with monitoring infrastructure and CI/CD pipelines.

## Overview

The core services deployment includes:

### Core Services (5)
1. **Auth Service** (Go) - Authentication and authorization
2. **Configuration Service** (Go) - Centralized configuration management
3. **Service Discovery** (Go) - Service registration and discovery
4. **Health Check Service** (Node.js) - System health monitoring
5. **API Gateway Service** (Go) - Request routing and API management

### Infrastructure Components
- **PostgreSQL** - Primary database for all services
- **Redis** - Caching and session storage
- **Prometheus** - Metrics collection and monitoring
- **Grafana** - Visualization and dashboards

### CI/CD Pipeline
- **GitHub Actions** - Automated testing, building, and deployment
- **Container Registry** - Docker image storage (GitHub Container Registry)
- **Multi-environment** - Staging and production deployments

## Prerequisites

### Required Tools
- **kubectl** v1.28+ - Kubernetes command-line tool
- **Docker** v20+ - Container runtime (for local builds)
- **Helm** v3+ - Kubernetes package manager (optional)
- **Git** - Version control

### Kubernetes Cluster Requirements
- **Kubernetes** v1.25+ cluster
- **Storage Class** - For persistent volumes
- **Load Balancer** - For external service access
- **RBAC** - Role-based access control enabled

### Minimum Resource Requirements
```yaml
CPU: 4 cores
Memory: 8GB RAM
Storage: 50GB persistent storage
Nodes: 3 (for high availability)
```

## Quick Start

### 1. Clone Repository
```bash
git clone https://github.com/thomas-carter-aic/002AIC.git
cd 002AIC
```

### 2. Configure Kubernetes Context
```bash
# Verify cluster connection
kubectl cluster-info

# Check available storage classes
kubectl get storageclass
```

### 3. Deploy Core Services
```bash
# Full deployment (infrastructure + monitoring + services)
./scripts/deploy-core-services.sh

# Or deploy components separately
./scripts/deploy-core-services.sh infrastructure
./scripts/deploy-core-services.sh monitoring
./scripts/deploy-core-services.sh services
```

### 4. Verify Deployment
```bash
# Check deployment status
./scripts/deploy-core-services.sh verify

# View all resources
kubectl get all -n aic-platform
kubectl get all -n aic-monitoring
```

## Detailed Deployment Steps

### Step 1: Infrastructure Setup

#### PostgreSQL Database
```bash
# Deploy PostgreSQL with persistent storage
kubectl apply -f infra/k8s/infrastructure/postgresql.yaml

# Wait for PostgreSQL to be ready
kubectl wait --for=condition=ready pod -l app=postgresql -n aic-platform --timeout=300s

# Verify database connectivity
kubectl exec -it postgresql-0 -n aic-platform -- psql -U postgres -c "\l"
```

#### Redis Cache
```bash
# Deploy Redis with persistent storage
kubectl apply -f infra/k8s/infrastructure/redis.yaml

# Wait for Redis to be ready
kubectl wait --for=condition=ready pod -l app=redis -n aic-platform --timeout=300s

# Test Redis connectivity
kubectl exec -it deployment/redis -n aic-platform -- redis-cli ping
```

### Step 2: Monitoring Stack

#### Prometheus
```bash
# Deploy Prometheus with RBAC
kubectl apply -f infra/k8s/monitoring/prometheus.yaml

# Wait for Prometheus to be ready
kubectl wait --for=condition=ready pod -l app=prometheus -n aic-monitoring --timeout=300s

# Access Prometheus UI (port-forward)
kubectl port-forward service/prometheus-service 9090:9090 -n aic-monitoring
# Open http://localhost:9090
```

#### Grafana
```bash
# Deploy Grafana with dashboards
kubectl apply -f infra/k8s/monitoring/grafana.yaml

# Wait for Grafana to be ready
kubectl wait --for=condition=ready pod -l app=grafana -n aic-monitoring --timeout=300s

# Get Grafana admin password
kubectl get secret grafana-secret -n aic-monitoring -o jsonpath='{.data.admin-password}' | base64 -d

# Access Grafana UI (port-forward)
kubectl port-forward service/grafana-service 3000:3000 -n aic-monitoring
# Open http://localhost:3000 (admin/admin123)
```

### Step 3: Core Services Deployment

#### Service Deployment Order
Services are deployed in dependency order:

1. **Configuration Service** - Provides config for other services
2. **Service Discovery** - Enables service-to-service communication
3. **Auth Service** - Provides authentication for other services
4. **Health Check Service** - Monitors all services
5. **API Gateway Service** - Routes external requests

#### Individual Service Deployment
```bash
# Deploy each service
kubectl apply -f infra/k8s/core-services/configuration-service.yaml
kubectl apply -f infra/k8s/core-services/service-discovery.yaml
kubectl apply -f infra/k8s/core-services/auth-service.yaml
kubectl apply -f infra/k8s/core-services/health-check-service.yaml
kubectl apply -f infra/k8s/core-services/api-gateway-service.yaml

# Wait for all deployments to be ready
kubectl wait --for=condition=available deployment --all -n aic-platform --timeout=300s
```

## Configuration Management

### Secrets Management
```bash
# View current secrets
kubectl get secrets -n aic-platform

# Update database connection string
kubectl create secret generic database-secret \
  --from-literal=auth-db-url="postgresql://user:pass@host:5432/auth_db" \
  --dry-run=client -o yaml | kubectl apply -f -

# Update JWT secret
kubectl create secret generic auth-secret \
  --from-literal=jwt-secret="your-super-secure-jwt-secret" \
  --dry-run=client -o yaml | kubectl apply -f -
```

### ConfigMaps
```bash
# View service configurations
kubectl get configmaps -n aic-platform

# Update service configuration
kubectl edit configmap auth-service-config -n aic-platform
```

### Environment-Specific Configurations
```bash
# Development environment
export ENVIRONMENT=development
sed -i 's/production/development/g' infra/k8s/core-services/*.yaml

# Staging environment
export ENVIRONMENT=staging
# Use staging-specific secrets and configs

# Production environment
export ENVIRONMENT=production
# Use production-specific secrets and configs
```

## Monitoring and Observability

### Prometheus Metrics
Key metrics to monitor:
- `up` - Service availability
- `http_requests_total` - Request count
- `http_request_duration_seconds` - Response time
- `process_resident_memory_bytes` - Memory usage
- `process_cpu_seconds_total` - CPU usage

### Grafana Dashboards
Pre-configured dashboards:
- **002AIC Platform Overview** - High-level system metrics
- **Service Health** - Individual service health
- **Infrastructure** - Database and cache metrics
- **Performance** - Response times and throughput

### Alerting Rules
Configured alerts:
- Service down for > 1 minute
- High memory usage (> 80%)
- High CPU usage (> 80%)
- Pod crash looping

### Log Aggregation
```bash
# View service logs
kubectl logs -f deployment/auth-service -n aic-platform
kubectl logs -f deployment/api-gateway-service -n aic-platform

# View logs from all pods
kubectl logs -f -l tier=core -n aic-platform

# Stream logs with stern (if installed)
stern -n aic-platform auth-service
```

## CI/CD Pipeline

### GitHub Actions Workflow
The CI/CD pipeline includes:

1. **Change Detection** - Only build/deploy changed services
2. **Security Scanning** - Trivy vulnerability scanning
3. **Testing** - Unit tests, integration tests
4. **Building** - Docker image building and pushing
5. **Deployment** - Automated deployment to staging/production
6. **Verification** - Health checks and smoke tests

### Pipeline Configuration
```yaml
# Required GitHub Secrets
KUBE_CONFIG_STAGING    # Base64 encoded kubeconfig for staging
KUBE_CONFIG_PRODUCTION # Base64 encoded kubeconfig for production
SLACK_WEBHOOK         # Slack notification webhook
```

### Manual Deployment
```bash
# Build and push images manually
docker build -t ghcr.io/002aic/auth-service:latest apps/backend-services/auth-service/
docker push ghcr.io/002aic/auth-service:latest

# Update deployment with new image
kubectl set image deployment/auth-service auth-service=ghcr.io/002aic/auth-service:latest -n aic-platform
```

## Scaling and Performance

### Horizontal Pod Autoscaling
```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: auth-service-hpa
  namespace: aic-platform
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: auth-service
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

### Vertical Pod Autoscaling
```yaml
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: auth-service-vpa
  namespace: aic-platform
spec:
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: auth-service
  updatePolicy:
    updateMode: "Auto"
```

### Manual Scaling
```bash
# Scale individual services
kubectl scale deployment auth-service --replicas=5 -n aic-platform
kubectl scale deployment api-gateway-service --replicas=3 -n aic-platform

# Scale all core services
kubectl scale deployment --all --replicas=3 -n aic-platform
```

## Security Considerations

### Network Policies
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: core-services-network-policy
  namespace: aic-platform
spec:
  podSelector:
    matchLabels:
      tier: core
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: aic-platform
    ports:
    - protocol: TCP
      port: 8080
```

### Pod Security Standards
```yaml
apiVersion: v1
kind: Pod
spec:
  securityContext:
    runAsNonRoot: true
    runAsUser: 1001
    fsGroup: 1001
    seccompProfile:
      type: RuntimeDefault
  containers:
  - name: auth-service
    securityContext:
      allowPrivilegeEscalation: false
      readOnlyRootFilesystem: true
      capabilities:
        drop:
        - ALL
```

### Secret Rotation
```bash
# Rotate JWT secret
kubectl create secret generic auth-secret-new \
  --from-literal=jwt-secret="new-super-secure-jwt-secret"

# Update deployment to use new secret
kubectl patch deployment auth-service -n aic-platform \
  -p '{"spec":{"template":{"spec":{"containers":[{"name":"auth-service","env":[{"name":"JWT_SECRET","valueFrom":{"secretKeyRef":{"name":"auth-secret-new","key":"jwt-secret"}}}]}]}}}}'

# Delete old secret
kubectl delete secret auth-secret -n aic-platform
kubectl rename secret auth-secret-new auth-secret -n aic-platform
```

## Troubleshooting

### Common Issues

#### Pod Stuck in Pending State
```bash
# Check node resources
kubectl describe nodes

# Check pod events
kubectl describe pod <pod-name> -n aic-platform

# Check storage class
kubectl get storageclass
```

#### Service Not Accessible
```bash
# Check service endpoints
kubectl get endpoints -n aic-platform

# Check service configuration
kubectl describe service api-gateway-service -n aic-platform

# Test internal connectivity
kubectl run debug-pod --image=curlimages/curl --rm -i --restart=Never -- \
  curl -f http://auth-service.aic-platform.svc.cluster.local/health
```

#### Database Connection Issues
```bash
# Check PostgreSQL logs
kubectl logs -f statefulset/postgresql -n aic-platform

# Test database connectivity
kubectl exec -it postgresql-0 -n aic-platform -- \
  psql -U postgres -c "SELECT version();"

# Check database secrets
kubectl get secret database-secret -n aic-platform -o yaml
```

#### High Memory Usage
```bash
# Check memory usage
kubectl top pods -n aic-platform

# Check resource limits
kubectl describe deployment auth-service -n aic-platform

# Analyze memory leaks
kubectl exec -it deployment/auth-service -n aic-platform -- \
  curl http://localhost:8080/debug/pprof/heap
```

### Debugging Commands
```bash
# Get all resources
kubectl get all -n aic-platform

# Describe problematic resources
kubectl describe deployment auth-service -n aic-platform
kubectl describe pod <pod-name> -n aic-platform

# Check events
kubectl get events -n aic-platform --sort-by='.lastTimestamp'

# Port forward for debugging
kubectl port-forward deployment/auth-service 8080:8080 -n aic-platform

# Execute commands in pods
kubectl exec -it deployment/auth-service -n aic-platform -- /bin/sh
```

## Maintenance

### Regular Maintenance Tasks

#### Weekly
- Review monitoring dashboards and alerts
- Check resource usage and scaling needs
- Review security scan results
- Update documentation

#### Monthly
- Rotate secrets and certificates
- Update container images
- Review and optimize resource requests/limits
- Backup configuration and data

#### Quarterly
- Kubernetes cluster updates
- Major version upgrades
- Security audit and penetration testing
- Disaster recovery testing

### Backup and Recovery
```bash
# Backup PostgreSQL data
kubectl exec postgresql-0 -n aic-platform -- \
  pg_dumpall -U postgres > backup-$(date +%Y%m%d).sql

# Backup Kubernetes manifests
kubectl get all -n aic-platform -o yaml > k8s-backup-$(date +%Y%m%d).yaml

# Backup secrets (encrypted)
kubectl get secrets -n aic-platform -o yaml > secrets-backup-$(date +%Y%m%d).yaml
```

### Disaster Recovery
```bash
# Restore from backup
kubectl apply -f k8s-backup-YYYYMMDD.yaml

# Restore database
kubectl exec -i postgresql-0 -n aic-platform -- \
  psql -U postgres < backup-YYYYMMDD.sql

# Verify recovery
./scripts/deploy-core-services.sh verify
```

## Performance Tuning

### Database Optimization
```sql
-- PostgreSQL performance tuning
ALTER SYSTEM SET shared_buffers = '256MB';
ALTER SYSTEM SET effective_cache_size = '1GB';
ALTER SYSTEM SET maintenance_work_mem = '64MB';
ALTER SYSTEM SET checkpoint_completion_target = 0.9;
ALTER SYSTEM SET wal_buffers = '16MB';
ALTER SYSTEM SET default_statistics_target = 100;
SELECT pg_reload_conf();
```

### Redis Optimization
```bash
# Redis performance tuning
kubectl exec -it deployment/redis -n aic-platform -- redis-cli CONFIG SET maxmemory-policy allkeys-lru
kubectl exec -it deployment/redis -n aic-platform -- redis-cli CONFIG SET tcp-keepalive 300
```

### Application Optimization
- Enable connection pooling
- Implement caching strategies
- Optimize database queries
- Use async processing where possible
- Implement circuit breakers

## Next Steps

After successfully deploying the core services:

1. **Add Remaining Services** - Deploy the remaining 31 microservices in logical groups
2. **Service Mesh** - Implement Istio or Kuma for advanced traffic management
3. **Advanced Monitoring** - Add distributed tracing with Jaeger
4. **Security Hardening** - Implement mTLS and advanced security policies
5. **Multi-Region** - Expand to multiple regions for high availability
6. **Edge Computing** - Deploy edge nodes for global performance

## Support

For issues and questions:
- **Documentation**: `/documentation/ops/`
- **GitHub Issues**: https://github.com/thomas-carter-aic/002AIC/issues
- **Slack**: #platform-support
- **Email**: platform-team@appliedinnovationcorp.com
