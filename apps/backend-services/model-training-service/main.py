"""
Model Training Service - Python
ML model training pipeline for the 002AIC platform
Handles distributed training, hyperparameter tuning, and experiment tracking
"""

from fastapi import FastAPI, HTTPException, Depends, BackgroundTasks, UploadFile, File
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel, Field, validator
from typing import List, Optional, Dict, Any, Union
import uvicorn
import os
import logging
from datetime import datetime, timedelta
import asyncio
import uuid
import json
import numpy as np
import pandas as pd
from enum import Enum
import mlflow
import mlflow.sklearn
import mlflow.tensorflow
import mlflow.pytorch
from sklearn.model_selection import train_test_split
from sklearn.ensemble import RandomForestClassifier
from sklearn.metrics import accuracy_score, precision_score, recall_score, f1_score
import joblib
import tempfile
from pathlib import Path

# Import our auth middleware
import sys
sys.path.append('/home/oss/002AIC/libs/auth-middleware/python')
from auth_middleware import FastAPIAuthMiddleware, AuthConfig

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Initialize FastAPI app
app = FastAPI(
    title="002AIC Model Training Service",
    description="ML model training and experiment management",
    version="1.0.0",
    docs_url="/docs",
    redoc_url="/redoc"
)

# CORS middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Initialize auth middleware
auth_config = AuthConfig(
    authorization_service_url=os.getenv("AUTHORIZATION_SERVICE_URL", "http://authorization-service:8080"),
    jwt_public_key_url=os.getenv("JWT_PUBLIC_KEY_URL", "http://keycloak:8080/realms/002aic/protocol/openid-connect/certs"),
    jwt_issuer=os.getenv("JWT_ISSUER", "http://keycloak:8080/realms/002aic"),
    jwt_audience=os.getenv("JWT_AUDIENCE", "002aic-api"),
    service_name="model-training-service"
)

auth = FastAPIAuthMiddleware(auth_config)

# Initialize MLflow
mlflow.set_tracking_uri(os.getenv("MLFLOW_TRACKING_URI", "http://mlflow:5000"))

# Enums
class TrainingStatus(str, Enum):
    QUEUED = "queued"
    RUNNING = "running"
    COMPLETED = "completed"
    FAILED = "failed"
    CANCELLED = "cancelled"

class ModelType(str, Enum):
    CLASSIFICATION = "classification"
    REGRESSION = "regression"
    CLUSTERING = "clustering"
    NLP = "nlp"
    COMPUTER_VISION = "computer_vision"
    TIME_SERIES = "time_series"

class Framework(str, Enum):
    SKLEARN = "sklearn"
    TENSORFLOW = "tensorflow"
    PYTORCH = "pytorch"
    XGBOOST = "xgboost"
    LIGHTGBM = "lightgbm"

# Pydantic models
class TrainingJobRequest(BaseModel):
    name: str = Field(..., description="Training job name")
    model_type: ModelType = Field(..., description="Type of model to train")
    framework: Framework = Field(..., description="ML framework to use")
    dataset_id: str = Field(..., description="Dataset ID for training")
    algorithm: str = Field(..., description="Algorithm to use")
    hyperparameters: Dict[str, Any] = Field(default_factory=dict, description="Hyperparameters")
    training_config: Dict[str, Any] = Field(default_factory=dict, description="Training configuration")
    validation_split: float = Field(default=0.2, description="Validation split ratio")
    test_split: float = Field(default=0.1, description="Test split ratio")
    cross_validation: bool = Field(default=False, description="Use cross-validation")
    early_stopping: bool = Field(default=True, description="Enable early stopping")
    max_epochs: int = Field(default=100, description="Maximum training epochs")
    batch_size: int = Field(default=32, description="Training batch size")
    learning_rate: float = Field(default=0.001, description="Learning rate")
    tags: List[str] = Field(default_factory=list, description="Training job tags")

class HyperparameterTuningRequest(BaseModel):
    base_job_id: str = Field(..., description="Base training job ID")
    tuning_algorithm: str = Field(default="random", description="Tuning algorithm (random, grid, bayesian)")
    parameter_space: Dict[str, Any] = Field(..., description="Parameter search space")
    max_trials: int = Field(default=50, description="Maximum number of trials")
    objective_metric: str = Field(default="accuracy", description="Metric to optimize")
    objective_direction: str = Field(default="maximize", description="Optimization direction")
    parallel_trials: int = Field(default=4, description="Number of parallel trials")

class TrainingJobResponse(BaseModel):
    id: str
    name: str
    model_type: ModelType
    framework: Framework
    status: TrainingStatus
    progress: float
    current_epoch: Optional[int]
    total_epochs: int
    metrics: Dict[str, float]
    best_metrics: Dict[str, float]
    model_artifacts: List[str]
    logs_url: Optional[str]
    created_at: datetime
    started_at: Optional[datetime]
    completed_at: Optional[datetime]
    duration_seconds: Optional[int]
    created_by: str

class ExperimentResponse(BaseModel):
    id: str
    name: str
    description: Optional[str]
    runs_count: int
    best_run_id: Optional[str]
    best_metrics: Dict[str, float]
    created_at: datetime
    last_run_at: Optional[datetime]

class ModelMetrics(BaseModel):
    accuracy: Optional[float]
    precision: Optional[float]
    recall: Optional[float]
    f1_score: Optional[float]
    loss: Optional[float]
    val_loss: Optional[float]
    custom_metrics: Dict[str, float] = Field(default_factory=dict)

# Training job storage (in production, use proper database)
training_jobs = {}
experiments = {}

# Health check endpoint
@app.get("/health")
async def health_check():
    """Health check endpoint"""
    mlflow_healthy = True
    
    try:
        mlflow.get_tracking_uri()
    except Exception:
        mlflow_healthy = False
    
    return {
        "status": "healthy" if mlflow_healthy else "degraded",
        "service": "model-training-service",
        "timestamp": datetime.utcnow().isoformat(),
        "version": "1.0.0",
        "dependencies": {
            "mlflow": "healthy" if mlflow_healthy else "unhealthy"
        }
    }

# Training job endpoints
@app.get("/v1/training/jobs", response_model=List[TrainingJobResponse])
@auth.require_auth("model", "read")
async def list_training_jobs(
    status: Optional[TrainingStatus] = None,
    model_type: Optional[ModelType] = None,
    framework: Optional[Framework] = None,
    skip: int = 0,
    limit: int = 100
):
    """List training jobs"""
    try:
        jobs = list(training_jobs.values())
        
        # Apply filters
        if status:
            jobs = [job for job in jobs if job["status"] == status.value]
        if model_type:
            jobs = [job for job in jobs if job["model_type"] == model_type.value]
        if framework:
            jobs = [job for job in jobs if job["framework"] == framework.value]
        
        # Apply pagination
        jobs = jobs[skip:skip + limit]
        
        return [TrainingJobResponse(**job) for job in jobs]
    except Exception as e:
        logger.error(f"Error listing training jobs: {e}")
        raise HTTPException(status_code=500, detail="Failed to list training jobs")

@app.post("/v1/training/jobs", response_model=TrainingJobResponse)
@auth.require_auth("model", "train")
async def create_training_job(
    job_request: TrainingJobRequest,
    background_tasks: BackgroundTasks
):
    """Create a new training job"""
    try:
        job_id = str(uuid.uuid4())
        
        # Create training job record
        job_data = {
            "id": job_id,
            "name": job_request.name,
            "model_type": job_request.model_type.value,
            "framework": job_request.framework.value,
            "status": TrainingStatus.QUEUED.value,
            "progress": 0.0,
            "current_epoch": None,
            "total_epochs": job_request.max_epochs,
            "metrics": {},
            "best_metrics": {},
            "model_artifacts": [],
            "logs_url": None,
            "created_at": datetime.utcnow(),
            "started_at": None,
            "completed_at": None,
            "duration_seconds": None,
            "created_by": "user",  # Get from auth context
            "config": job_request.dict()
        }
        
        training_jobs[job_id] = job_data
        
        # Queue training job
        background_tasks.add_task(execute_training_job, job_id, job_request)
        
        return TrainingJobResponse(**job_data)
    except Exception as e:
        logger.error(f"Error creating training job: {e}")
        raise HTTPException(status_code=500, detail="Failed to create training job")

@app.get("/v1/training/jobs/{job_id}", response_model=TrainingJobResponse)
@auth.require_auth("model", "read")
async def get_training_job(job_id: str):
    """Get training job details"""
    try:
        if job_id not in training_jobs:
            raise HTTPException(status_code=404, detail="Training job not found")
        
        job_data = training_jobs[job_id]
        return TrainingJobResponse(**job_data)
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error getting training job {job_id}: {e}")
        raise HTTPException(status_code=500, detail="Failed to get training job")

@app.post("/v1/training/jobs/{job_id}/cancel")
@auth.require_auth("model", "train")
async def cancel_training_job(job_id: str):
    """Cancel a training job"""
    try:
        if job_id not in training_jobs:
            raise HTTPException(status_code=404, detail="Training job not found")
        
        job_data = training_jobs[job_id]
        
        if job_data["status"] not in [TrainingStatus.QUEUED.value, TrainingStatus.RUNNING.value]:
            raise HTTPException(status_code=400, detail="Cannot cancel job in current status")
        
        job_data["status"] = TrainingStatus.CANCELLED.value
        job_data["completed_at"] = datetime.utcnow()
        
        return {"message": "Training job cancelled successfully"}
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error cancelling training job {job_id}: {e}")
        raise HTTPException(status_code=500, detail="Failed to cancel training job")

# Hyperparameter tuning endpoints
@app.post("/v1/training/hyperparameter-tuning")
@auth.require_auth("model", "train")
async def start_hyperparameter_tuning(
    tuning_request: HyperparameterTuningRequest,
    background_tasks: BackgroundTasks
):
    """Start hyperparameter tuning"""
    try:
        if tuning_request.base_job_id not in training_jobs:
            raise HTTPException(status_code=404, detail="Base training job not found")
        
        tuning_job_id = str(uuid.uuid4())
        
        # Create tuning job record
        tuning_data = {
            "id": tuning_job_id,
            "base_job_id": tuning_request.base_job_id,
            "status": "running",
            "trials_completed": 0,
            "max_trials": tuning_request.max_trials,
            "best_trial": None,
            "best_metrics": {},
            "created_at": datetime.utcnow(),
            "config": tuning_request.dict()
        }
        
        # Queue hyperparameter tuning
        background_tasks.add_task(execute_hyperparameter_tuning, tuning_job_id, tuning_request)
        
        return {
            "tuning_job_id": tuning_job_id,
            "message": "Hyperparameter tuning started",
            "max_trials": tuning_request.max_trials
        }
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error starting hyperparameter tuning: {e}")
        raise HTTPException(status_code=500, detail="Failed to start hyperparameter tuning")

# Experiment management endpoints
@app.get("/v1/training/experiments", response_model=List[ExperimentResponse])
@auth.require_auth("model", "read")
async def list_experiments():
    """List ML experiments"""
    try:
        mlflow_experiments = mlflow.search_experiments()
        
        experiment_responses = []
        for exp in mlflow_experiments:
            runs = mlflow.search_runs(experiment_ids=[exp.experiment_id])
            
            best_run = None
            best_metrics = {}
            if not runs.empty:
                # Find best run based on accuracy (or other metric)
                if 'metrics.accuracy' in runs.columns:
                    best_run = runs.loc[runs['metrics.accuracy'].idxmax()]
                    best_metrics = {
                        col.replace('metrics.', ''): best_run[col] 
                        for col in runs.columns 
                        if col.startswith('metrics.') and pd.notna(best_run[col])
                    }
            
            experiment_responses.append(ExperimentResponse(
                id=exp.experiment_id,
                name=exp.name,
                description=exp.tags.get('description', '') if exp.tags else None,
                runs_count=len(runs),
                best_run_id=best_run['run_id'] if best_run is not None else None,
                best_metrics=best_metrics,
                created_at=datetime.fromtimestamp(exp.creation_time / 1000),
                last_run_at=datetime.fromtimestamp(runs['start_time'].max() / 1000) if not runs.empty else None
            ))
        
        return experiment_responses
    except Exception as e:
        logger.error(f"Error listing experiments: {e}")
        raise HTTPException(status_code=500, detail="Failed to list experiments")

@app.post("/v1/training/experiments")
@auth.require_auth("model", "create")
async def create_experiment(name: str, description: Optional[str] = None):
    """Create a new ML experiment"""
    try:
        experiment_id = mlflow.create_experiment(
            name=name,
            tags={"description": description} if description else None
        )
        
        return {
            "experiment_id": experiment_id,
            "name": name,
            "description": description,
            "created_at": datetime.utcnow().isoformat()
        }
    except Exception as e:
        logger.error(f"Error creating experiment: {e}")
        raise HTTPException(status_code=500, detail="Failed to create experiment")

# Model evaluation endpoints
@app.post("/v1/training/evaluate")
@auth.require_auth("model", "read")
async def evaluate_model(
    model_id: str,
    dataset_id: str,
    metrics: List[str] = ["accuracy", "precision", "recall", "f1_score"]
):
    """Evaluate a trained model"""
    try:
        # This is a simplified implementation
        # In production, load actual model and dataset
        
        # Generate mock evaluation results
        evaluation_results = {
            "model_id": model_id,
            "dataset_id": dataset_id,
            "metrics": {
                "accuracy": np.random.uniform(0.8, 0.95),
                "precision": np.random.uniform(0.75, 0.9),
                "recall": np.random.uniform(0.7, 0.88),
                "f1_score": np.random.uniform(0.72, 0.89)
            },
            "confusion_matrix": [[85, 5], [10, 90]],
            "evaluation_time": datetime.utcnow().isoformat(),
            "sample_predictions": [
                {"input": "sample_1", "predicted": "class_a", "actual": "class_a", "confidence": 0.92},
                {"input": "sample_2", "predicted": "class_b", "actual": "class_b", "confidence": 0.87}
            ]
        }
        
        return evaluation_results
    except Exception as e:
        logger.error(f"Error evaluating model: {e}")
        raise HTTPException(status_code=500, detail="Failed to evaluate model")

# Background tasks
async def execute_training_job(job_id: str, job_request: TrainingJobRequest):
    """Background task to execute training job"""
    try:
        logger.info(f"Starting training job {job_id}")
        
        # Update job status
        job_data = training_jobs[job_id]
        job_data["status"] = TrainingStatus.RUNNING.value
        job_data["started_at"] = datetime.utcnow()
        
        # Start MLflow run
        with mlflow.start_run(run_name=job_request.name) as run:
            # Log parameters
            mlflow.log_params(job_request.hyperparameters)
            mlflow.log_param("algorithm", job_request.algorithm)
            mlflow.log_param("framework", job_request.framework.value)
            mlflow.log_param("model_type", job_request.model_type.value)
            
            # Simulate training process
            for epoch in range(job_request.max_epochs):
                # Check if job was cancelled
                if training_jobs[job_id]["status"] == TrainingStatus.CANCELLED.value:
                    logger.info(f"Training job {job_id} was cancelled")
                    return
                
                # Simulate training step
                await asyncio.sleep(1)  # Simulate training time
                
                # Generate mock metrics
                accuracy = 0.5 + (epoch / job_request.max_epochs) * 0.4 + np.random.normal(0, 0.02)
                loss = 2.0 - (epoch / job_request.max_epochs) * 1.5 + np.random.normal(0, 0.1)
                val_accuracy = accuracy - np.random.uniform(0, 0.05)
                val_loss = loss + np.random.uniform(0, 0.1)
                
                # Update job progress
                progress = ((epoch + 1) / job_request.max_epochs) * 100
                job_data["progress"] = progress
                job_data["current_epoch"] = epoch + 1
                job_data["metrics"] = {
                    "accuracy": accuracy,
                    "loss": loss,
                    "val_accuracy": val_accuracy,
                    "val_loss": val_loss
                }
                
                # Log metrics to MLflow
                mlflow.log_metrics({
                    "accuracy": accuracy,
                    "loss": loss,
                    "val_accuracy": val_accuracy,
                    "val_loss": val_loss
                }, step=epoch)
                
                # Update best metrics
                if not job_data["best_metrics"] or accuracy > job_data["best_metrics"].get("accuracy", 0):
                    job_data["best_metrics"] = job_data["metrics"].copy()
                
                logger.info(f"Training job {job_id} - Epoch {epoch + 1}/{job_request.max_epochs}, Accuracy: {accuracy:.4f}")
            
            # Create and save a simple model (for demonstration)
            if job_request.framework == Framework.SKLEARN:
                # Generate dummy data for training
                X = np.random.randn(1000, 10)
                y = np.random.randint(0, 2, 1000)
                
                # Train a simple model
                model = RandomForestClassifier(n_estimators=100, random_state=42)
                model.fit(X, y)
                
                # Log model
                mlflow.sklearn.log_model(model, "model")
                
                # Save model artifact info
                job_data["model_artifacts"] = [f"runs:/{run.info.run_id}/model"]
            
            # Complete the job
            job_data["status"] = TrainingStatus.COMPLETED.value
            job_data["completed_at"] = datetime.utcnow()
            job_data["duration_seconds"] = int((job_data["completed_at"] - job_data["started_at"]).total_seconds())
            
            logger.info(f"Training job {job_id} completed successfully")
    
    except Exception as e:
        logger.error(f"Error in training job {job_id}: {e}")
        
        # Update job status to failed
        job_data = training_jobs[job_id]
        job_data["status"] = TrainingStatus.FAILED.value
        job_data["completed_at"] = datetime.utcnow()
        if job_data["started_at"]:
            job_data["duration_seconds"] = int((job_data["completed_at"] - job_data["started_at"]).total_seconds())

async def execute_hyperparameter_tuning(tuning_job_id: str, tuning_request: HyperparameterTuningRequest):
    """Background task to execute hyperparameter tuning"""
    try:
        logger.info(f"Starting hyperparameter tuning {tuning_job_id}")
        
        base_job = training_jobs[tuning_request.base_job_id]
        base_config = base_job["config"]
        
        best_score = 0
        best_params = None
        
        for trial in range(tuning_request.max_trials):
            # Generate random hyperparameters from search space
            trial_params = {}
            for param, space in tuning_request.parameter_space.items():
                if isinstance(space, list):
                    trial_params[param] = np.random.choice(space)
                elif isinstance(space, dict):
                    if space.get("type") == "uniform":
                        trial_params[param] = np.random.uniform(space["low"], space["high"])
                    elif space.get("type") == "int":
                        trial_params[param] = np.random.randint(space["low"], space["high"])
            
            # Simulate training with these parameters
            await asyncio.sleep(2)  # Simulate training time
            
            # Generate mock score
            score = np.random.uniform(0.7, 0.95)
            
            if score > best_score:
                best_score = score
                best_params = trial_params.copy()
            
            logger.info(f"Hyperparameter tuning {tuning_job_id} - Trial {trial + 1}/{tuning_request.max_trials}, Score: {score:.4f}")
        
        logger.info(f"Hyperparameter tuning {tuning_job_id} completed. Best score: {best_score:.4f}")
        logger.info(f"Best parameters: {best_params}")
        
    except Exception as e:
        logger.error(f"Error in hyperparameter tuning {tuning_job_id}: {e}")

if __name__ == "__main__":
    uvicorn.run(
        "main:app",
        host="0.0.0.0",
        port=int(os.getenv("PORT", 8080)),
        reload=os.getenv("ENVIRONMENT") == "development"
    )
