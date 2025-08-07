# Nexus Platform - AI/ML Development Guide

This guide provides comprehensive instructions for developing AI/ML applications on the Nexus Platform.

## 🚀 Quick Start

### Start the AI-Native Platform

```bash
./scripts/start-ai-platform.sh
```

This command starts the complete AI/ML stack including:
- MLflow for experiment tracking and model management
- Jupyter Lab for data science development
- Airflow for ML workflow orchestration
- MinIO for artifact storage
- Qdrant for vector embeddings
- TensorFlow Serving and Triton for model serving
- All AI/ML microservices

### Test the Platform

```bash
./scripts/test-ai-platform.sh
```

## 🏗️ AI/ML Architecture

### Core Components

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Data Sources  │    │  ML Pipelines   │    │ Model Serving   │
│   (APIs/Files)  │───►│   (Airflow)     │───►│   (TF/Triton)   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│ Data Processing │    │ Experiment      │    │ Model Registry  │
│  (Jupyter/Py)   │    │ Tracking        │    │   (MLflow)      │
└─────────────────┘    │   (MLflow)      │    └─────────────────┘
                       └─────────────────┘
```

### AI/ML Services

| Service | Port | Purpose | Technology |
|---------|------|---------|------------|
| **MLflow Server** | 5000 | Experiment tracking, model registry | Python/MLflow |
| **Jupyter Lab** | 8888 | Data science development | Jupyter/Python |
| **Airflow** | 8080 | ML workflow orchestration | Apache Airflow |
| **MinIO** | 9000/9001 | Object storage for artifacts | MinIO |
| **Qdrant** | 6333 | Vector database for embeddings | Qdrant |
| **TensorFlow Serving** | 8501 | Model serving (REST) | TensorFlow |
| **Triton Server** | 8000-8002 | Multi-framework serving | NVIDIA Triton |
| **Model Management** | 8086 | Model lifecycle management | FastAPI/Python |
| **Model Training** | 8087 | Training job orchestration | FastAPI/Python |

## 🔬 Development Workflow

### 1. Data Science Development

**Access Jupyter Lab:**
```bash
# Open in browser
http://localhost:8888
# Token: nexus-jupyter-token
```

**Example Notebook:**
```python
import pandas as pd
import numpy as np
from sklearn.ensemble import RandomForestClassifier
from sklearn.model_selection import train_test_split
import mlflow
import mlflow.sklearn

# Set MLflow tracking URI
mlflow.set_tracking_uri("http://mlflow-server:5000")
mlflow.set_experiment("my-experiment")

# Load and prepare data
df = pd.read_csv("data.csv")
X = df.drop('target', axis=1)
y = df['target']

X_train, X_test, y_train, y_test = train_test_split(X, y, test_size=0.2)

# Train model with MLflow tracking
with mlflow.start_run():
    model = RandomForestClassifier(n_estimators=100)
    model.fit(X_train, y_train)
    
    # Log metrics
    accuracy = model.score(X_test, y_test)
    mlflow.log_metric("accuracy", accuracy)
    
    # Log model
    mlflow.sklearn.log_model(model, "model")
```

### 2. Automated Training Jobs

**Create Training Job via API:**
```bash
curl -X POST http://localhost:8000/training/training-jobs \
  -H "Content-Type: application/json" \
  -d '{
    "job_name": "customer-churn-model",
    "model_type": "classifier",
    "algorithm": "random_forest",
    "target_column": "churn",
    "hyperparameter_tuning": true,
    "tuning_method": "optuna",
    "cv_folds": 5
  }'
```

**Upload Dataset:**
```bash
curl -X POST http://localhost:8087/training-jobs \
  -F "dataset_file=@customer_data.csv" \
  -F 'job={"job_name":"churn-model","model_type":"classifier","algorithm":"random_forest","target_column":"churn"}'
```

**Monitor Training Job:**
```bash
# List all jobs
curl http://localhost:8087/training-jobs

# Get specific job
curl http://localhost:8087/training-jobs/{job_id}
```

### 3. Model Management

**List Models:**
```bash
curl http://localhost:8086/models
```

**Get Model Details:**
```bash
curl http://localhost:8086/models/{model_id}
```

**Update Model Status:**
```bash
curl -X PUT http://localhost:8086/models/{model_id} \
  -H "Content-Type: application/json" \
  -d '{"status": "ready"}'
```

### 4. Model Deployment

**Deploy Model:**
```bash
curl -X POST http://localhost:8088/deployments \
  -H "Content-Type: application/json" \
  -d '{
    "model_id": "model-uuid",
    "deployment_name": "churn-predictor",
    "environment": "production",
    "replicas": 3
  }'
```

**Make Predictions:**
```bash
curl -X POST http://localhost:8086/predict \
  -H "Content-Type: application/json" \
  -d '{
    "model_id": "model-uuid",
    "input_data": {
      "feature1": 0.5,
      "feature2": 1.2,
      "feature3": "category_a"
    }
  }'
```

## 🔄 ML Pipeline Development

### Airflow DAG Example

Create a DAG in `/infra/docker-compose/airflow/dags/`:

```python
from datetime import datetime, timedelta
from airflow import DAG
from airflow.operators.python import PythonOperator
import requests

default_args = {
    'owner': 'data-team',
    'depends_on_past': False,
    'start_date': datetime(2024, 1, 1),
    'retries': 1,
    'retry_delay': timedelta(minutes=5),
}

dag = DAG(
    'daily_model_training',
    default_args=default_args,
    description='Daily model retraining pipeline',
    schedule_interval=timedelta(days=1),
    catchup=False,
)

def trigger_training_job(**context):
    """Trigger a training job via API"""
    response = requests.post(
        'http://model-training-service:8087/training-jobs',
        json={
            'job_name': f'daily_retrain_{context["ds"]}',
            'model_type': 'classifier',
            'algorithm': 'random_forest',
            'target_column': 'target',
            'hyperparameter_tuning': True
        }
    )
    return response.json()

def check_model_performance(**context):
    """Check if model meets performance criteria"""
    # Implementation here
    pass

def deploy_if_better(**context):
    """Deploy model if it performs better than current"""
    # Implementation here
    pass

# Define tasks
train_task = PythonOperator(
    task_id='trigger_training',
    python_callable=trigger_training_job,
    dag=dag,
)

evaluate_task = PythonOperator(
    task_id='evaluate_model',
    python_callable=check_model_performance,
    dag=dag,
)

deploy_task = PythonOperator(
    task_id='deploy_model',
    python_callable=deploy_if_better,
    dag=dag,
)

# Set dependencies
train_task >> evaluate_task >> deploy_task
```

## 📊 Experiment Tracking

### MLflow Integration

**Access MLflow UI:**
```bash
http://localhost:5000
```

**Track Experiments Programmatically:**
```python
import mlflow
import mlflow.sklearn
from sklearn.metrics import accuracy_score

# Set tracking server
mlflow.set_tracking_uri("http://localhost:5000")

# Create or set experiment
mlflow.set_experiment("customer-segmentation")

# Start run
with mlflow.start_run(run_name="rf-baseline"):
    # Log parameters
    mlflow.log_param("n_estimators", 100)
    mlflow.log_param("max_depth", 10)
    
    # Train model
    model = RandomForestClassifier(n_estimators=100, max_depth=10)
    model.fit(X_train, y_train)
    
    # Log metrics
    predictions = model.predict(X_test)
    accuracy = accuracy_score(y_test, predictions)
    mlflow.log_metric("accuracy", accuracy)
    
    # Log artifacts
    mlflow.log_artifact("feature_importance.png")
    
    # Log model
    mlflow.sklearn.log_model(
        model, 
        "model",
        registered_model_name="customer-segmentation-rf"
    )
```

## 🗄️ Data Management

### MinIO Object Storage

**Access MinIO Console:**
```bash
http://localhost:9001
# Username: nexus-minio
# Password: nexus-minio-password
```

**Python SDK Usage:**
```python
from minio import Minio

# Initialize client
client = Minio(
    "localhost:9000",
    access_key="nexus-minio",
    secret_key="nexus-minio-password",
    secure=False
)

# Upload dataset
client.fput_object("datasets", "customer_data.csv", "local_file.csv")

# Download model artifacts
client.fget_object("models", "model.pkl", "downloaded_model.pkl")
```

### Vector Database (Qdrant)

**Store Embeddings:**
```python
from qdrant_client import QdrantClient
from qdrant_client.models import Distance, VectorParams, PointStruct

# Initialize client
client = QdrantClient("localhost", port=6333)

# Create collection
client.create_collection(
    collection_name="document_embeddings",
    vectors_config=VectorParams(size=384, distance=Distance.COSINE),
)

# Insert vectors
points = [
    PointStruct(
        id=1,
        vector=[0.1, 0.2, 0.3, ...],  # 384-dimensional vector
        payload={"text": "Document content", "category": "technical"}
    )
]

client.upsert(
    collection_name="document_embeddings",
    points=points
)

# Search similar vectors
results = client.search(
    collection_name="document_embeddings",
    query_vector=[0.1, 0.2, 0.3, ...],
    limit=5
)
```

## 🚀 Model Serving

### TensorFlow Serving

**Prepare Model:**
```python
import tensorflow as tf

# Save model in SavedModel format
model.save("/models/my_model/1")  # Version 1
```

**Make Predictions:**
```bash
curl -X POST http://localhost:8501/v1/models/my_model:predict \
  -H "Content-Type: application/json" \
  -d '{
    "instances": [
      {"feature1": 1.0, "feature2": 2.0}
    ]
  }'
```

### NVIDIA Triton Inference Server

**Model Repository Structure:**
```
/models/
├── my_model/
│   ├── config.pbtxt
│   └── 1/
│       └── model.savedmodel/
```

**Configuration (config.pbtxt):**
```
name: "my_model"
platform: "tensorflow_savedmodel"
max_batch_size: 8
input [
  {
    name: "input"
    data_type: TYPE_FP32
    dims: [ -1, 10 ]
  }
]
output [
  {
    name: "output"
    data_type: TYPE_FP32
    dims: [ -1, 1 ]
  }
]
```

## 📈 Monitoring and Observability

### Model Performance Monitoring

**Custom Metrics:**
```python
import prometheus_client
from prometheus_client import Counter, Histogram, Gauge

# Define metrics
prediction_counter = Counter('model_predictions_total', 'Total predictions made')
prediction_latency = Histogram('model_prediction_duration_seconds', 'Prediction latency')
model_accuracy = Gauge('model_accuracy', 'Current model accuracy')

# Use in prediction endpoint
@prediction_latency.time()
def make_prediction(data):
    prediction_counter.inc()
    # Make prediction
    result = model.predict(data)
    return result
```

### Grafana Dashboards

**Access Grafana:**
```bash
http://localhost:3000
# Username: admin
# Password: admin
```

**Example Dashboard Queries:**
```promql
# Prediction rate
rate(model_predictions_total[5m])

# Average prediction latency
rate(model_prediction_duration_seconds_sum[5m]) / rate(model_prediction_duration_seconds_count[5m])

# Model accuracy over time
model_accuracy
```

## 🔧 Advanced Features

### Hyperparameter Optimization with Optuna

```python
import optuna
from sklearn.model_selection import cross_val_score

def objective(trial):
    # Suggest hyperparameters
    n_estimators = trial.suggest_int('n_estimators', 10, 100)
    max_depth = trial.suggest_int('max_depth', 1, 32)
    
    # Create model
    model = RandomForestClassifier(
        n_estimators=n_estimators,
        max_depth=max_depth,
        random_state=42
    )
    
    # Cross-validation score
    scores = cross_val_score(model, X_train, y_train, cv=5)
    return scores.mean()

# Optimize
study = optuna.create_study(direction='maximize')
study.optimize(objective, n_trials=100)

print(f"Best parameters: {study.best_params}")
print(f"Best score: {study.best_value}")
```

### A/B Testing for Models

```python
import random

def route_prediction(user_id, model_a, model_b):
    """Route users to different models for A/B testing"""
    
    # Consistent routing based on user ID
    if hash(user_id) % 2 == 0:
        model_version = "A"
        prediction = model_a.predict(features)
    else:
        model_version = "B"
        prediction = model_b.predict(features)
    
    # Log for analysis
    log_prediction(user_id, model_version, prediction)
    
    return prediction
```

### Feature Store Integration

```python
class FeatureStore:
    def __init__(self, redis_client):
        self.redis = redis_client
    
    def get_features(self, entity_id, feature_names):
        """Get features for an entity"""
        features = {}
        for feature_name in feature_names:
            key = f"feature:{entity_id}:{feature_name}"
            value = self.redis.get(key)
            if value:
                features[feature_name] = float(value)
        return features
    
    def store_features(self, entity_id, features):
        """Store features for an entity"""
        for feature_name, value in features.items():
            key = f"feature:{entity_id}:{feature_name}"
            self.redis.setex(key, 3600, str(value))  # 1 hour TTL
```

## 🚨 Troubleshooting

### Common Issues

1. **MLflow Server Not Starting:**
   ```bash
   # Check logs
   docker-compose logs mlflow-server
   
   # Restart service
   docker-compose restart mlflow-server
   ```

2. **Training Job Stuck:**
   ```bash
   # Check training service logs
   docker-compose logs model-training-service
   
   # Check job status
   curl http://localhost:8087/training-jobs/{job_id}
   ```

3. **Model Serving Issues:**
   ```bash
   # Check TensorFlow Serving
   curl http://localhost:8501/v1/models
   
   # Check Triton Server
   curl http://localhost:8000/v2/health/ready
   ```

4. **Resource Issues:**
   ```bash
   # Check Docker resources
   docker system df
   docker stats
   
   # Clean up unused resources
   docker system prune
   ```

### Performance Optimization

1. **GPU Support:**
   ```yaml
   # Add to docker-compose.yml
   deploy:
     resources:
       reservations:
         devices:
           - driver: nvidia
             count: 1
             capabilities: [gpu]
   ```

2. **Memory Optimization:**
   ```yaml
   # Increase memory limits
   mem_limit: 4g
   memswap_limit: 4g
   ```

3. **Parallel Processing:**
   ```python
   # Use joblib for parallel training
   from joblib import Parallel, delayed
   
   results = Parallel(n_jobs=-1)(
       delayed(train_model)(params) for params in param_grid
   )
   ```

## 📚 Best Practices

### Model Development

1. **Version Control:**
   - Use Git for code versioning
   - Use MLflow for model versioning
   - Tag models with semantic versions

2. **Experiment Tracking:**
   - Log all hyperparameters
   - Track metrics consistently
   - Save model artifacts

3. **Data Validation:**
   - Validate input data schemas
   - Check for data drift
   - Monitor feature distributions

### Production Deployment

1. **Model Validation:**
   - Test models before deployment
   - Use shadow deployments
   - Implement rollback mechanisms

2. **Monitoring:**
   - Track prediction latency
   - Monitor model accuracy
   - Set up alerts for anomalies

3. **Security:**
   - Secure API endpoints
   - Validate input data
   - Use proper authentication

---

**Ready to build AI-native applications!** 🚀

The Nexus Platform provides a complete AI/ML development environment with enterprise-grade 
capabilities. Start experimenting with the provided examples and scale to production-ready 
AI applications.
