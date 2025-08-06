# AI/ML Services Deployment Guide

This guide covers the deployment of the 6 AI/ML services that form the core AI capabilities of the 002AIC platform, building upon the foundation of the 5 core services.

## Overview

The AI/ML services deployment includes:

### AI/ML Services (6)
1. **Agent Orchestration Service** (Python) - AI agent management and coordination
2. **Model Training Service** (Python) - ML model training with GPU support
3. **Model Tuning Service** (Python) - Hyperparameter optimization and model tuning
4. **Model Deployment Service** (Go) - Model serving and deployment automation
5. **Data Integration Service** (Python) - Data pipeline and ETL operations
6. **Pipeline Execution Service** (Python) - ML workflow orchestration

### Supporting Infrastructure
- **MLflow** - Experiment tracking and model registry
- **GPU Support** - NVIDIA GPU integration for training workloads
- **Persistent Storage** - Dedicated storage for models, data, and experiments
- **Enhanced PostgreSQL** - Additional databases for AI/ML services

## Prerequisites

### Core Services Requirement
The AI/ML services depend on the core services being deployed first:
```bash
# Ensure core services are running
./scripts/deploy-core-services.sh verify
```

### GPU Requirements (Optional but Recommended)
For optimal performance, especially for model training:
- **NVIDIA GPU nodes** with CUDA support
- **GPU drivers** installed on nodes
- **NVIDIA Device Plugin** for Kubernetes
- **Node labels** for GPU scheduling

### Resource Requirements
```yaml
Minimum Resources:
  CPU: 8 cores
  Memory: 16GB RAM
  GPU: 1 NVIDIA GPU (optional)
  Storage: 200GB persistent storage

Recommended Resources:
  CPU: 16 cores
  Memory: 32GB RAM
  GPU: 2+ NVIDIA GPUs
  Storage: 500GB persistent storage
```

## Quick Start

### 1. Verify Core Services
```bash
# Check that core services are running
kubectl get pods -n aic-platform
kubectl get service postgresql-service -n aic-platform
```

### 2. Deploy AI/ML Services
```bash
# Full AI/ML deployment
./scripts/deploy-ai-ml-services.sh

# Or deploy components separately
./scripts/deploy-ai-ml-services.sh mlflow
./scripts/deploy-ai-ml-services.sh services
```

### 3. Verify Deployment
```bash
# Check deployment status
./scripts/deploy-ai-ml-services.sh verify

# View all AI/ML resources
kubectl get all -n aic-ai-ml
```

## Detailed Deployment Steps

### Step 1: Namespace and Resource Management

#### AI/ML Namespace Creation
```bash
# Create dedicated namespace with resource quotas
kubectl apply -f infra/k8s/ai-ml-services/namespace.yaml

# Verify namespace and quotas
kubectl describe namespace aic-ai-ml
kubectl describe resourcequota ai-ml-resource-quota -n aic-ai-ml
```

#### Resource Quotas
The AI/ML namespace includes resource quotas to manage consumption:
- **CPU**: 20 cores requested, 40 cores limit
- **Memory**: 40GB requested, 80GB limit
- **GPU**: 4 NVIDIA GPUs
- **Storage**: 10 persistent volume claims

### Step 2: Database Setup

#### PostgreSQL Extension
```bash
# Update PostgreSQL with AI/ML databases
kubectl apply -f infra/k8s/infrastructure/postgresql-ai-ml-init.yaml

# Restart PostgreSQL to apply changes
kubectl rollout restart statefulset/postgresql -n aic-platform
```

#### AI/ML Databases Created
- `agent_db` - Agent orchestration data
- `training_db` - Training job metadata
- `tuning_db` - Hyperparameter optimization results
- `deployment_db` - Model deployment configurations
- `data_integration_db` - Data pipeline metadata
- `pipeline_db` - Workflow execution logs
- `mlflow_db` - MLflow experiment tracking

### Step 3: MLflow Deployment

#### MLflow Tracking Server
```bash
# Deploy MLflow with PostgreSQL backend and S3 artifacts
kubectl apply -f infra/k8s/ai-ml-services/mlflow.yaml

# Wait for MLflow to be ready
kubectl wait --for=condition=available deployment/mlflow-service -n aic-ai-ml --timeout=600s

# Access MLflow UI
kubectl port-forward service/mlflow-ui 5000:5000 -n aic-ai-ml
# Open http://localhost:5000
```

#### MLflow Configuration
- **Backend Store**: PostgreSQL for metadata
- **Artifact Store**: S3-compatible storage
- **Authentication**: Basic auth with configurable credentials
- **High Availability**: Single replica (can be scaled)

### Step 4: AI/ML Services Deployment

#### Service Deployment Order
Services are deployed in dependency order:

1. **Agent Orchestration Service** - Foundation for AI agents
2. **Data Integration Service** - Data pipeline capabilities
3. **Pipeline Execution Service** - Workflow orchestration
4. **Model Training Service** - GPU-accelerated training
5. **Model Tuning Service** - Hyperparameter optimization
6. **Model Deployment Service** - Model serving automation

#### Individual Service Details

##### Agent Orchestration Service
```yaml
Features:
  - AI agent lifecycle management
  - Multi-agent coordination
  - Resource allocation and scheduling
  - Agent communication protocols

Resources:
  - 2 replicas for high availability
  - 1GB memory, 500m CPU per replica
  - Persistent storage for agent data
```

##### Model Training Service
```yaml
Features:
  - TensorFlow, PyTorch, Scikit-learn support
  - GPU acceleration with CUDA
  - Distributed training capabilities
  - MLflow integration for experiment tracking

Resources:
  - 1 replica (GPU-bound workload)
  - 4GB memory, 2 CPU cores, 1 GPU
  - Large persistent storage for models and data
  - Shared memory for efficient data loading
```

##### Model Tuning Service
```yaml
Features:
  - Bayesian optimization
  - Grid search and random search
  - Early stopping and pruning
  - Integration with Optuna and Hyperopt

Resources:
  - 1 replica with GPU support
  - 2GB memory, 1 CPU core, 1 GPU
  - Storage for tuning experiments
```

##### Model Deployment Service
```yaml
Features:
  - Kubernetes-native model serving
  - Auto-scaling based on load
  - A/B testing and canary deployments
  - Multi-framework support

Resources:
  - 2 replicas for high availability
  - 1GB memory, 500m CPU per replica
  - RBAC permissions for Kubernetes API
```

##### Data Integration Service
```yaml
Features:
  - Multi-source data connectors
  - Apache Spark integration
  - Real-time and batch processing
  - Data quality validation

Resources:
  - 2 replicas for parallel processing
  - 2GB memory, 1 CPU core per replica
  - Large storage for data processing
```

##### Pipeline Execution Service
```yaml
Features:
  - Airflow-based workflow orchestration
  - Kubernetes job execution
  - Pipeline monitoring and logging
  - Retry and error handling

Resources:
  - 2 replicas for reliability
  - 1GB memory, 500m CPU per replica
  - Storage for pipeline definitions and logs
```

## GPU Configuration

### Node Setup
```bash
# Label GPU nodes
kubectl label nodes <gpu-node-name> accelerator=nvidia-tesla-gpu

# Verify GPU availability
kubectl describe nodes -l accelerator=nvidia-tesla-gpu
```

### GPU Scheduling
```yaml
# GPU node selector and tolerations
nodeSelector:
  accelerator: nvidia-tesla-gpu
tolerations:
- key: nvidia.com/gpu
  operator: Exists
  effect: NoSchedule
```

### GPU Resource Requests
```yaml
resources:
  requests:
    nvidia.com/gpu: 1
  limits:
    nvidia.com/gpu: 1
```

## Storage Configuration

### Persistent Volume Claims
Each AI/ML service has dedicated storage:

```bash
# View all PVCs
kubectl get pvc -n aic-ai-ml

# Storage allocations
agent-orchestration-pvc: 10Gi
model-training-pvc: 50Gi
training-data-pvc: 100Gi
model-tuning-pvc: 30Gi
tuning-data-pvc: 20Gi
model-deployment-pvc: 20Gi
data-integration-pvc: 100Gi
pipeline-execution-pvc: 20Gi
pipeline-logs-pvc: 50Gi
mlflow-pvc: 10Gi
```

### Storage Classes
Ensure appropriate storage classes are available:
```bash
# Check available storage classes
kubectl get storageclass

# Recommended storage classes
# - SSD for databases and frequently accessed data
# - Standard for model storage and archives
# - High-performance for training data
```

## Monitoring and Observability

### Prometheus Metrics
AI/ML services expose custom metrics:
- `ml_training_jobs_total` - Total training jobs
- `ml_model_deployments_active` - Active model deployments
- `ml_pipeline_executions_duration` - Pipeline execution times
- `ml_agent_orchestration_requests` - Agent orchestration requests
- `ml_data_integration_records_processed` - Data processing volume

### MLflow Tracking
- **Experiments**: Organized by project and team
- **Runs**: Individual training or tuning runs
- **Models**: Versioned model artifacts
- **Metrics**: Training metrics and validation scores
- **Parameters**: Hyperparameters and configurations

### Grafana Dashboards
Pre-configured dashboards for AI/ML monitoring:
- **ML Training Overview** - Training job status and performance
- **Model Deployment Status** - Deployed model health and usage
- **Data Pipeline Monitoring** - Data processing metrics
- **GPU Utilization** - GPU usage across training workloads
- **MLflow Metrics** - Experiment tracking statistics

## Security Considerations

### RBAC Permissions
AI/ML services have specific RBAC requirements:

```yaml
# Model Deployment Service needs Kubernetes API access
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: model-deployment-service
rules:
- apiGroups: ["apps"]
  resources: ["deployments"]
  verbs: ["get", "list", "create", "update", "delete"]
- apiGroups: [""]
  resources: ["services", "configmaps"]
  verbs: ["get", "list", "create", "update", "delete"]
```

### Secret Management
AI/ML services use multiple secret types:
- **Database credentials** for each service
- **Cloud storage credentials** for model artifacts
- **API tokens** for external services (Hugging Face, etc.)
- **MLflow authentication** credentials

### Network Policies
```yaml
# Restrict AI/ML service communication
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: ai-ml-network-policy
  namespace: aic-ai-ml
spec:
  podSelector:
    matchLabels:
      tier: ai-ml
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: aic-platform
    - namespaceSelector:
        matchLabels:
          name: aic-ai-ml
```

## Scaling and Performance

### Horizontal Pod Autoscaling
```yaml
# Example HPA for Agent Orchestration Service
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: agent-orchestration-hpa
  namespace: aic-ai-ml
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: agent-orchestration-service
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

### GPU Scaling
```bash
# Scale GPU-enabled services carefully
kubectl scale deployment model-training-service --replicas=2 -n aic-ai-ml

# Monitor GPU usage
kubectl describe nodes -l accelerator=nvidia-tesla-gpu
```

### Performance Optimization
- **Model Caching**: Cache frequently used models in memory
- **Data Preprocessing**: Optimize data loading and preprocessing
- **Batch Processing**: Use appropriate batch sizes for training
- **Connection Pooling**: Optimize database connections
- **Async Processing**: Use async operations where possible

## Troubleshooting

### Common Issues

#### GPU Not Available
```bash
# Check GPU node labels
kubectl get nodes -l accelerator=nvidia-tesla-gpu

# Verify GPU device plugin
kubectl get daemonset nvidia-device-plugin-daemonset -n kube-system

# Check GPU resources
kubectl describe nodes -l accelerator=nvidia-tesla-gpu | grep nvidia.com/gpu
```

#### Training Jobs Failing
```bash
# Check training service logs
kubectl logs -f deployment/model-training-service -n aic-ai-ml

# Check GPU memory usage
kubectl exec -it deployment/model-training-service -n aic-ai-ml -- nvidia-smi

# Verify data access
kubectl exec -it deployment/model-training-service -n aic-ai-ml -- ls -la /app/data
```

#### MLflow Connection Issues
```bash
# Check MLflow service status
kubectl get service mlflow-service -n aic-ai-ml

# Test MLflow connectivity
kubectl run mlflow-test --image=python:3.9-slim --rm -i --restart=Never -- \
  sh -c "pip install mlflow && python -c 'import mlflow; print(mlflow.get_tracking_uri())'"

# Check MLflow logs
kubectl logs -f deployment/mlflow-service -n aic-ai-ml
```

#### Storage Issues
```bash
# Check PVC status
kubectl get pvc -n aic-ai-ml

# Check storage usage
kubectl exec -it deployment/model-training-service -n aic-ai-ml -- df -h

# Verify storage class
kubectl describe storageclass
```

### Debugging Commands
```bash
# Get all AI/ML resources
kubectl get all -n aic-ai-ml

# Check resource usage
kubectl top pods -n aic-ai-ml

# View events
kubectl get events -n aic-ai-ml --sort-by='.lastTimestamp'

# Debug specific service
kubectl describe deployment model-training-service -n aic-ai-ml
kubectl logs -f deployment/model-training-service -n aic-ai-ml

# Port forward for debugging
kubectl port-forward deployment/agent-orchestration-service 8080:8080 -n aic-ai-ml
```

## Integration with Core Services

### Service Discovery
AI/ML services register with the core service discovery:
```bash
# Check service registration
curl http://service-discovery.aic-platform.svc.cluster.local/v1/services
```

### Authentication
AI/ML services authenticate through the core auth service:
```bash
# Test authentication
curl -H "Authorization: Bearer <token>" \
  http://agent-orchestration-service.aic-ai-ml.svc.cluster.local/v1/agents
```

### Configuration Management
AI/ML services retrieve configuration from the core configuration service:
```bash
# Check configuration
curl http://configuration-service.aic-platform.svc.cluster.local/v1/config/ai-ml
```

### Health Monitoring
The core health check service monitors all AI/ML services:
```bash
# View AI/ML service health
curl http://health-check-service.aic-platform.svc.cluster.local/v1/services
```

## Maintenance

### Regular Maintenance Tasks

#### Weekly
- Review MLflow experiments and clean up old runs
- Monitor GPU utilization and optimize resource allocation
- Check model deployment performance and scaling
- Review training job success rates and failures

#### Monthly
- Update ML framework versions (TensorFlow, PyTorch)
- Rotate cloud storage credentials
- Archive old model versions and experiments
- Review and optimize resource requests/limits

#### Quarterly
- Major ML framework upgrades
- GPU driver and CUDA updates
- Model performance audits
- Storage optimization and cleanup

### Backup and Recovery
```bash
# Backup MLflow database
kubectl exec postgresql-0 -n aic-platform -- \
  pg_dump -U postgres mlflow_db > mlflow-backup-$(date +%Y%m%d).sql

# Backup model artifacts (if using local storage)
kubectl exec deployment/mlflow-service -n aic-ai-ml -- \
  tar -czf /tmp/mlflow-artifacts-$(date +%Y%m%d).tar.gz /mlflow

# Backup AI/ML configurations
kubectl get configmaps -n aic-ai-ml -o yaml > ai-ml-configs-$(date +%Y%m%d).yaml
```

## Next Steps

After successfully deploying the AI/ML services:

1. **Data Services Group** - Deploy data management and analytics services
2. **DevOps Services Group** - Add monitoring, deployment, and operational services
3. **Business Services Group** - Implement billing, support, and marketplace services
4. **Advanced AI Features** - Add specialized AI services (NLP, computer vision, etc.)
5. **Model Serving Optimization** - Implement advanced serving patterns
6. **Multi-Region AI/ML** - Expand AI/ML capabilities across regions

## Support

For AI/ML specific issues and questions:
- **Documentation**: `/documentation/ops/ai-ml-services-deployment.md`
- **MLflow UI**: Access through port-forward or LoadBalancer
- **GitHub Issues**: Tag with `ai-ml` label
- **Slack**: #ai-ml-platform channel
- **Email**: ai-ml-team@appliedinnovationcorp.com

## Performance Benchmarks

### Expected Performance Metrics
- **Model Training**: 80% GPU utilization during active training
- **Model Deployment**: <100ms inference latency for standard models
- **Data Integration**: 1M+ records processed per hour
- **Pipeline Execution**: 95% success rate for automated pipelines
- **Agent Orchestration**: <50ms response time for agent coordination

### Monitoring Thresholds
- **CPU Usage**: Alert if >80% for >5 minutes
- **Memory Usage**: Alert if >85% for >5 minutes
- **GPU Usage**: Alert if <20% during training hours
- **Storage Usage**: Alert if >90% capacity
- **Training Job Failures**: Alert if >10% failure rate
