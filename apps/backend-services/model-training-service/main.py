"""
Nexus Platform - Model Training Service
Handles ML model training, hyperparameter tuning, and experiment tracking
"""

import os
import asyncio
import logging
from datetime import datetime
from typing import List, Optional, Dict, Any, Union
from contextlib import asynccontextmanager
import json
import uuid

import uvicorn
from fastapi import FastAPI, HTTPException, BackgroundTasks, UploadFile, File
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel, Field
import mlflow
import mlflow.sklearn
import mlflow.tensorflow
import mlflow.pytorch
import pandas as pd
import numpy as np
from sklearn.model_selection import train_test_split, GridSearchCV, RandomizedSearchCV
from sklearn.ensemble import RandomForestClassifier, RandomForestRegressor
from sklearn.linear_model import LogisticRegression, LinearRegression
from sklearn.svm import SVC, SVR
from sklearn.metrics import accuracy_score, precision_score, recall_score, f1_score, mean_squared_error, r2_score
import optuna
import redis
import httpx

# Configuration
REDIS_URL = os.getenv("REDIS_URL", "redis://:nexus-password@redis:6379/0")
MLFLOW_TRACKING_URI = os.getenv("MLFLOW_TRACKING_URI", "http://mlflow-server:5000")
MODEL_MANAGEMENT_URL = os.getenv("MODEL_MANAGEMENT_URL", "http://model-management-service:8086")
SERVICE_PORT = int(os.getenv("PORT", "8087"))

# Setup logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Redis setup
redis_client = redis.from_url(REDIS_URL, decode_responses=True)

# MLflow setup
mlflow.set_tracking_uri(MLFLOW_TRACKING_URI)

# Pydantic Models
class TrainingJobCreate(BaseModel):
    job_name: str = Field(..., description="Training job name")
    model_type: str = Field(..., description="Model type (classifier, regressor)")
    algorithm: str = Field(..., description="Algorithm (random_forest, logistic_regression, svm)")
    dataset_path: Optional[str] = None
    target_column: str = Field(..., description="Target column name")
    feature_columns: Optional[List[str]] = None
    hyperparameters: Optional[Dict[str, Any]] = None
    hyperparameter_tuning: bool = False
    tuning_method: str = "grid_search"  # grid_search, random_search, optuna
    cv_folds: int = 5
    test_size: float = 0.2
    random_state: int = 42

class TrainingJobResponse(BaseModel):
    job_id: str
    job_name: str
    status: str
    model_type: str
    algorithm: str
    created_at: datetime
    started_at: Optional[datetime] = None
    completed_at: Optional[datetime] = None
    metrics: Optional[Dict[str, float]] = None
    best_parameters: Optional[Dict[str, Any]] = None
    model_id: Optional[str] = None
    error_message: Optional[str] = None

class HyperparameterTuningConfig(BaseModel):
    method: str = "optuna"  # optuna, grid_search, random_search
    n_trials: int = 100
    timeout: Optional[int] = None  # seconds
    parameter_space: Dict[str, Any]

class ExperimentCreate(BaseModel):
    experiment_name: str
    description: Optional[str] = None
    tags: Optional[Dict[str, str]] = None

class ExperimentResponse(BaseModel):
    experiment_id: str
    name: str
    description: Optional[str]
    tags: Optional[Dict[str, str]]
    created_at: datetime

# Training job storage
training_jobs = {}

# Lifespan context manager
@asynccontextmanager
async def lifespan(app: FastAPI):
    # Startup
    logger.info("🚀 Model Training Service starting up...")
    
    try:
        # Test Redis connection
        redis_client.ping()
        logger.info("✅ Redis connection successful")
        
        # Test MLflow connection
        mlflow.set_tracking_uri(MLFLOW_TRACKING_URI)
        logger.info("✅ MLflow connection configured")
        
    except Exception as e:
        logger.error(f"❌ Startup connection test failed: {e}")
    
    yield
    
    # Shutdown
    logger.info("🛑 Model Training Service shutting down...")

# Create FastAPI app
app = FastAPI(
    title="Nexus Model Training Service",
    description="ML Model Training and Hyperparameter Tuning for Nexus Platform",
    version="1.0.0",
    lifespan=lifespan
)

# Add CORS middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Health check endpoint
@app.get("/health")
async def health_check():
    """Health check endpoint"""
    try:
        redis_client.ping()
        return {
            "status": "healthy",
            "service": "model-training-service",
            "timestamp": datetime.utcnow().isoformat(),
            "version": "1.0.0"
        }
    except Exception as e:
        raise HTTPException(status_code=503, detail=f"Service unhealthy: {str(e)}")

# Experiment Management
@app.post("/experiments", response_model=ExperimentResponse)
async def create_experiment(experiment: ExperimentCreate):
    """Create a new MLflow experiment"""
    try:
        # Create experiment in MLflow
        experiment_id = mlflow.create_experiment(
            name=experiment.experiment_name,
            tags=experiment.tags
        )
        
        logger.info(f"✅ Created experiment: {experiment.experiment_name}")
        
        return ExperimentResponse(
            experiment_id=experiment_id,
            name=experiment.experiment_name,
            description=experiment.description,
            tags=experiment.tags,
            created_at=datetime.utcnow()
        )
        
    except Exception as e:
        logger.error(f"❌ Error creating experiment: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/experiments", response_model=List[ExperimentResponse])
async def list_experiments():
    """List all MLflow experiments"""
    try:
        experiments = mlflow.search_experiments()
        
        return [
            ExperimentResponse(
                experiment_id=exp.experiment_id,
                name=exp.name,
                description=exp.tags.get("description"),
                tags=exp.tags,
                created_at=datetime.fromtimestamp(exp.creation_time / 1000)
            )
            for exp in experiments
        ]
        
    except Exception as e:
        logger.error(f"❌ Error listing experiments: {e}")
        raise HTTPException(status_code=500, detail=str(e))

# Training Job Management
@app.post("/training-jobs", response_model=TrainingJobResponse)
async def create_training_job(
    job: TrainingJobCreate, 
    background_tasks: BackgroundTasks,
    dataset_file: Optional[UploadFile] = File(None)
):
    """Create a new training job"""
    try:
        job_id = str(uuid.uuid4())
        
        # Handle dataset upload
        dataset_path = job.dataset_path
        if dataset_file:
            # Save uploaded file
            dataset_path = f"/tmp/dataset_{job_id}.csv"
            with open(dataset_path, "wb") as f:
                content = await dataset_file.read()
                f.write(content)
        
        # Create training job record
        training_job = {
            "job_id": job_id,
            "job_name": job.job_name,
            "status": "queued",
            "model_type": job.model_type,
            "algorithm": job.algorithm,
            "dataset_path": dataset_path,
            "target_column": job.target_column,
            "feature_columns": job.feature_columns,
            "hyperparameters": job.hyperparameters,
            "hyperparameter_tuning": job.hyperparameter_tuning,
            "tuning_method": job.tuning_method,
            "cv_folds": job.cv_folds,
            "test_size": job.test_size,
            "random_state": job.random_state,
            "created_at": datetime.utcnow(),
            "started_at": None,
            "completed_at": None,
            "metrics": None,
            "best_parameters": None,
            "model_id": None,
            "error_message": None
        }
        
        # Store in memory and Redis
        training_jobs[job_id] = training_job
        redis_client.setex(f"training_job:{job_id}", 86400, json.dumps(training_job, default=str))
        
        # Start training in background
        background_tasks.add_task(run_training_job, job_id)
        
        logger.info(f"✅ Created training job: {job.job_name}")
        
        return TrainingJobResponse(**training_job)
        
    except Exception as e:
        logger.error(f"❌ Error creating training job: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/training-jobs", response_model=List[TrainingJobResponse])
async def list_training_jobs(status: Optional[str] = None):
    """List all training jobs"""
    try:
        jobs = []
        for job_id, job_data in training_jobs.items():
            if status is None or job_data["status"] == status:
                jobs.append(TrainingJobResponse(**job_data))
        
        return jobs
        
    except Exception as e:
        logger.error(f"❌ Error listing training jobs: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/training-jobs/{job_id}", response_model=TrainingJobResponse)
async def get_training_job(job_id: str):
    """Get a specific training job"""
    try:
        if job_id not in training_jobs:
            # Try to load from Redis
            cached_job = redis_client.get(f"training_job:{job_id}")
            if cached_job:
                job_data = json.loads(cached_job)
                # Convert string dates back to datetime
                for date_field in ["created_at", "started_at", "completed_at"]:
                    if job_data[date_field]:
                        job_data[date_field] = datetime.fromisoformat(job_data[date_field])
                training_jobs[job_id] = job_data
            else:
                raise HTTPException(status_code=404, detail="Training job not found")
        
        return TrainingJobResponse(**training_jobs[job_id])
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"❌ Error getting training job: {e}")
        raise HTTPException(status_code=500, detail=str(e))

async def run_training_job(job_id: str):
    """Run the training job asynchronously"""
    try:
        job_data = training_jobs[job_id]
        job_data["status"] = "running"
        job_data["started_at"] = datetime.utcnow()
        
        logger.info(f"🏃 Starting training job: {job_id}")
        
        # Load dataset
        if not job_data["dataset_path"]:
            raise ValueError("No dataset provided")
        
        df = pd.read_csv(job_data["dataset_path"])
        
        # Prepare features and target
        target_col = job_data["target_column"]
        if target_col not in df.columns:
            raise ValueError(f"Target column '{target_col}' not found in dataset")
        
        feature_cols = job_data["feature_columns"]
        if feature_cols is None:
            feature_cols = [col for col in df.columns if col != target_col]
        
        X = df[feature_cols]
        y = df[target_col]
        
        # Train-test split
        X_train, X_test, y_train, y_test = train_test_split(
            X, y, 
            test_size=job_data["test_size"], 
            random_state=job_data["random_state"],
            stratify=y if job_data["model_type"] == "classifier" else None
        )
        
        # Start MLflow run
        with mlflow.start_run(run_name=f"{job_data['job_name']}_{job_id}"):
            # Log parameters
            mlflow.log_param("algorithm", job_data["algorithm"])
            mlflow.log_param("model_type", job_data["model_type"])
            mlflow.log_param("dataset_shape", df.shape)
            mlflow.log_param("test_size", job_data["test_size"])
            mlflow.log_param("cv_folds", job_data["cv_folds"])
            
            # Get model and hyperparameters
            model, param_grid = get_model_and_params(
                job_data["algorithm"], 
                job_data["model_type"],
                job_data["hyperparameters"]
            )
            
            best_model = model
            best_params = {}
            
            # Hyperparameter tuning
            if job_data["hyperparameter_tuning"] and param_grid:
                logger.info(f"🔧 Starting hyperparameter tuning: {job_data['tuning_method']}")
                
                if job_data["tuning_method"] == "grid_search":
                    search = GridSearchCV(
                        model, param_grid, 
                        cv=job_data["cv_folds"], 
                        scoring='accuracy' if job_data["model_type"] == "classifier" else 'r2',
                        n_jobs=-1
                    )
                elif job_data["tuning_method"] == "random_search":
                    search = RandomizedSearchCV(
                        model, param_grid, 
                        cv=job_data["cv_folds"],
                        scoring='accuracy' if job_data["model_type"] == "classifier" else 'r2',
                        n_iter=50,
                        n_jobs=-1,
                        random_state=job_data["random_state"]
                    )
                else:  # optuna
                    best_model, best_params = await run_optuna_optimization(
                        model, X_train, y_train, param_grid, job_data
                    )
                
                if job_data["tuning_method"] in ["grid_search", "random_search"]:
                    search.fit(X_train, y_train)
                    best_model = search.best_estimator_
                    best_params = search.best_params_
                    
                    # Log best parameters
                    mlflow.log_params(best_params)
                    
            else:
                # Train with default parameters
                best_model.fit(X_train, y_train)
            
            # Make predictions
            y_pred = best_model.predict(X_test)
            
            # Calculate metrics
            metrics = {}
            if job_data["model_type"] == "classifier":
                metrics = {
                    "accuracy": accuracy_score(y_test, y_pred),
                    "precision": precision_score(y_test, y_pred, average='weighted'),
                    "recall": recall_score(y_test, y_pred, average='weighted'),
                    "f1_score": f1_score(y_test, y_pred, average='weighted')
                }
            else:  # regressor
                metrics = {
                    "mse": mean_squared_error(y_test, y_pred),
                    "rmse": np.sqrt(mean_squared_error(y_test, y_pred)),
                    "r2_score": r2_score(y_test, y_pred)
                }
            
            # Log metrics
            mlflow.log_metrics(metrics)
            
            # Log model
            if job_data["algorithm"] in ["random_forest", "logistic_regression", "linear_regression", "svm"]:
                mlflow.sklearn.log_model(best_model, "model")
            
            # Get MLflow run info
            run = mlflow.active_run()
            mlflow_run_id = run.info.run_id
            
            # Register model with Model Management Service
            model_id = await register_model_with_management_service(
                job_data, best_model, metrics, best_params, mlflow_run_id
            )
            
            # Update job status
            job_data["status"] = "completed"
            job_data["completed_at"] = datetime.utcnow()
            job_data["metrics"] = metrics
            job_data["best_parameters"] = best_params
            job_data["model_id"] = model_id
            
            logger.info(f"✅ Training job completed: {job_id}")
            
    except Exception as e:
        logger.error(f"❌ Training job failed: {job_id} - {e}")
        job_data["status"] = "failed"
        job_data["completed_at"] = datetime.utcnow()
        job_data["error_message"] = str(e)
    
    finally:
        # Update Redis cache
        redis_client.setex(f"training_job:{job_id}", 86400, json.dumps(job_data, default=str))

def get_model_and_params(algorithm: str, model_type: str, custom_params: Optional[Dict] = None):
    """Get model instance and parameter grid for hyperparameter tuning"""
    
    if algorithm == "random_forest":
        if model_type == "classifier":
            model = RandomForestClassifier(random_state=42)
            param_grid = {
                "n_estimators": [50, 100, 200],
                "max_depth": [None, 10, 20, 30],
                "min_samples_split": [2, 5, 10],
                "min_samples_leaf": [1, 2, 4]
            }
        else:
            model = RandomForestRegressor(random_state=42)
            param_grid = {
                "n_estimators": [50, 100, 200],
                "max_depth": [None, 10, 20, 30],
                "min_samples_split": [2, 5, 10],
                "min_samples_leaf": [1, 2, 4]
            }
    
    elif algorithm == "logistic_regression":
        model = LogisticRegression(random_state=42, max_iter=1000)
        param_grid = {
            "C": [0.01, 0.1, 1, 10, 100],
            "penalty": ["l1", "l2"],
            "solver": ["liblinear", "saga"]
        }
    
    elif algorithm == "linear_regression":
        model = LinearRegression()
        param_grid = {
            "fit_intercept": [True, False],
            "normalize": [True, False]
        }
    
    elif algorithm == "svm":
        if model_type == "classifier":
            model = SVC(random_state=42)
            param_grid = {
                "C": [0.1, 1, 10, 100],
                "kernel": ["linear", "rbf", "poly"],
                "gamma": ["scale", "auto", 0.001, 0.01, 0.1, 1]
            }
        else:
            model = SVR()
            param_grid = {
                "C": [0.1, 1, 10, 100],
                "kernel": ["linear", "rbf", "poly"],
                "gamma": ["scale", "auto", 0.001, 0.01, 0.1, 1]
            }
    
    else:
        raise ValueError(f"Unsupported algorithm: {algorithm}")
    
    # Override with custom parameters if provided
    if custom_params:
        model.set_params(**custom_params)
    
    return model, param_grid

async def run_optuna_optimization(model, X_train, y_train, param_grid, job_data):
    """Run Optuna hyperparameter optimization"""
    
    def objective(trial):
        # Suggest parameters based on param_grid
        params = {}
        for param, values in param_grid.items():
            if isinstance(values[0], int):
                params[param] = trial.suggest_int(param, min(values), max(values))
            elif isinstance(values[0], float):
                params[param] = trial.suggest_float(param, min(values), max(values))
            else:
                params[param] = trial.suggest_categorical(param, values)
        
        # Set model parameters
        model.set_params(**params)
        
        # Cross-validation
        from sklearn.model_selection import cross_val_score
        scores = cross_val_score(
            model, X_train, y_train, 
            cv=job_data["cv_folds"],
            scoring='accuracy' if job_data["model_type"] == "classifier" else 'r2'
        )
        
        return scores.mean()
    
    # Create study
    study = optuna.create_study(direction='maximize')
    study.optimize(objective, n_trials=100, timeout=600)  # 10 minutes timeout
    
    # Get best parameters and retrain model
    best_params = study.best_params
    model.set_params(**best_params)
    model.fit(X_train, y_train)
    
    return model, best_params

async def register_model_with_management_service(job_data, model, metrics, best_params, mlflow_run_id):
    """Register the trained model with the Model Management Service"""
    try:
        async with httpx.AsyncClient() as client:
            model_data = {
                "name": job_data["job_name"],
                "framework": "sklearn",
                "model_type": job_data["model_type"],
                "description": f"Model trained with {job_data['algorithm']}",
                "tags": {
                    "algorithm": job_data["algorithm"],
                    "training_job_id": job_data["job_id"]
                },
                "mlflow_run_id": mlflow_run_id
            }
            
            response = await client.post(
                f"{MODEL_MANAGEMENT_URL}/models",
                json=model_data
            )
            
            if response.status_code == 200:
                model_info = response.json()
                logger.info(f"✅ Model registered: {model_info['id']}")
                return model_info["id"]
            else:
                logger.error(f"❌ Failed to register model: {response.text}")
                return None
                
    except Exception as e:
        logger.error(f"❌ Error registering model: {e}")
        return None

# Metrics endpoint
@app.get("/metrics")
async def get_metrics():
    """Get service metrics"""
    try:
        # Count jobs by status
        status_counts = {}
        for status in ["queued", "running", "completed", "failed"]:
            count = sum(1 for job in training_jobs.values() if job["status"] == status)
            status_counts[f"jobs_{status}"] = count
        
        return {
            "service": "model-training-service",
            "timestamp": datetime.utcnow().isoformat(),
            "total_jobs": len(training_jobs),
            **status_counts
        }
        
    except Exception as e:
        logger.error(f"❌ Error getting metrics: {e}")
        raise HTTPException(status_code=500, detail=str(e))

if __name__ == "__main__":
    uvicorn.run(
        "main:app",
        host="0.0.0.0",
        port=SERVICE_PORT,
        reload=True,
        log_level="info"
    )
