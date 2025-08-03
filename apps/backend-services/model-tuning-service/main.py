"""
Model Tuning Service - Python
Advanced hyperparameter optimization and model tuning for the 002AIC platform
Supports Bayesian optimization, multi-objective optimization, and AutoML
"""

from fastapi import FastAPI, HTTPException, Depends, BackgroundTasks
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
from enum import Enum
import optuna
from optuna.samplers import TPESampler, CmaEsSampler
from optuna.pruners import MedianPruner
import mlflow
import mlflow.optuna
from sklearn.model_selection import cross_val_score
from sklearn.ensemble import RandomForestClassifier
from sklearn.svm import SVC
from sklearn.neural_network import MLPClassifier
import redis

# Import our auth middleware
import sys
sys.path.append('/home/oss/002AIC/libs/auth-middleware/python')
from auth_middleware import FastAPIAuthMiddleware, AuthConfig

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Initialize FastAPI app
app = FastAPI(
    title="002AIC Model Tuning Service",
    description="Advanced hyperparameter optimization and model tuning",
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
    service_name="model-tuning-service"
)

auth = FastAPIAuthMiddleware(auth_config)

# Initialize MLflow and Redis
mlflow.set_tracking_uri(os.getenv("MLFLOW_TRACKING_URI", "http://mlflow:5000"))
redis_client = redis.Redis(
    host=os.getenv("REDIS_HOST", "localhost"),
    port=int(os.getenv("REDIS_PORT", "6379")),
    password=os.getenv("REDIS_PASSWORD", ""),
    db=5,
    decode_responses=True
)

# Enums
class TuningStatus(str, Enum):
    QUEUED = "queued"
    RUNNING = "running"
    COMPLETED = "completed"
    FAILED = "failed"
    CANCELLED = "cancelled"

class OptimizationAlgorithm(str, Enum):
    TPE = "tpe"  # Tree-structured Parzen Estimator
    CMAES = "cmaes"  # Covariance Matrix Adaptation Evolution Strategy
    RANDOM = "random"
    GRID = "grid"
    BAYESIAN = "bayesian"

class ObjectiveDirection(str, Enum):
    MINIMIZE = "minimize"
    MAXIMIZE = "maximize"

# Pydantic models
class ParameterSpace(BaseModel):
    name: str = Field(..., description="Parameter name")
    type: str = Field(..., description="Parameter type (float, int, categorical)")
    low: Optional[float] = Field(None, description="Lower bound for numeric parameters")
    high: Optional[float] = Field(None, description="Upper bound for numeric parameters")
    choices: Optional[List[Any]] = Field(None, description="Choices for categorical parameters")
    log: bool = Field(default=False, description="Use log scale for numeric parameters")
    step: Optional[float] = Field(None, description="Step size for discrete parameters")

class TuningJobRequest(BaseModel):
    name: str = Field(..., description="Tuning job name")
    model_id: str = Field(..., description="Base model ID to tune")
    dataset_id: str = Field(..., description="Dataset ID for tuning")
    algorithm: OptimizationAlgorithm = Field(..., description="Optimization algorithm")
    parameter_space: List[ParameterSpace] = Field(..., description="Parameter search space")
    objective_metric: str = Field(..., description="Metric to optimize")
    objective_direction: ObjectiveDirection = Field(..., description="Optimization direction")
    n_trials: int = Field(default=100, description="Number of optimization trials")
    timeout_seconds: Optional[int] = Field(None, description="Timeout for optimization")
    n_jobs: int = Field(default=1, description="Number of parallel jobs")
    cv_folds: int = Field(default=5, description="Cross-validation folds")
    early_stopping: bool = Field(default=True, description="Enable early stopping")
    pruning: bool = Field(default=True, description="Enable trial pruning")
    multi_objective: bool = Field(default=False, description="Multi-objective optimization")
    secondary_metrics: List[str] = Field(default_factory=list, description="Additional metrics to track")

class AutoMLRequest(BaseModel):
    name: str = Field(..., description="AutoML job name")
    dataset_id: str = Field(..., description="Dataset ID")
    target_column: str = Field(..., description="Target column name")
    problem_type: str = Field(..., description="Problem type (classification, regression)")
    time_budget_minutes: int = Field(default=60, description="Time budget in minutes")
    algorithms: List[str] = Field(default_factory=list, description="Algorithms to try")
    feature_engineering: bool = Field(default=True, description="Enable feature engineering")
    ensemble: bool = Field(default=True, description="Enable ensemble methods")
    interpretability: str = Field(default="medium", description="Model interpretability level")

class TuningJobResponse(BaseModel):
    id: str
    name: str
    status: TuningStatus
    algorithm: OptimizationAlgorithm
    progress: float
    trials_completed: int
    total_trials: int
    best_trial: Optional[Dict[str, Any]]
    best_score: Optional[float]
    best_parameters: Optional[Dict[str, Any]]
    study_url: Optional[str]
    created_at: datetime
    started_at: Optional[datetime]
    completed_at: Optional[datetime]
    duration_seconds: Optional[int]

class TrialResult(BaseModel):
    trial_id: int
    parameters: Dict[str, Any]
    score: float
    metrics: Dict[str, float]
    duration_seconds: float
    status: str
    datetime_start: datetime
    datetime_complete: Optional[datetime]

# Storage for tuning jobs
tuning_jobs = {}
optuna_studies = {}

# Health check endpoint
@app.get("/health")
async def health_check():
    """Health check endpoint"""
    mlflow_healthy = True
    redis_healthy = True
    
    try:
        mlflow.get_tracking_uri()
    except Exception:
        mlflow_healthy = False
    
    try:
        redis_client.ping()
    except Exception:
        redis_healthy = False
    
    return {
        "status": "healthy" if all([mlflow_healthy, redis_healthy]) else "degraded",
        "service": "model-tuning-service",
        "timestamp": datetime.utcnow().isoformat(),
        "version": "1.0.0",
        "dependencies": {
            "mlflow": "healthy" if mlflow_healthy else "unhealthy",
            "redis": "healthy" if redis_healthy else "unhealthy"
        }
    }

# Tuning job endpoints
@app.get("/v1/tuning/jobs", response_model=List[TuningJobResponse])
@auth.require_auth("model", "read")
async def list_tuning_jobs(
    status: Optional[TuningStatus] = None,
    skip: int = 0,
    limit: int = 100
):
    """List hyperparameter tuning jobs"""
    try:
        jobs = list(tuning_jobs.values())
        
        if status:
            jobs = [job for job in jobs if job["status"] == status.value]
        
        jobs = jobs[skip:skip + limit]
        return [TuningJobResponse(**job) for job in jobs]
    except Exception as e:
        logger.error(f"Error listing tuning jobs: {e}")
        raise HTTPException(status_code=500, detail="Failed to list tuning jobs")

@app.post("/v1/tuning/jobs", response_model=TuningJobResponse)
@auth.require_auth("model", "tune")
async def create_tuning_job(
    job_request: TuningJobRequest,
    background_tasks: BackgroundTasks
):
    """Create a new hyperparameter tuning job"""
    try:
        job_id = str(uuid.uuid4())
        
        # Create tuning job record
        job_data = {
            "id": job_id,
            "name": job_request.name,
            "status": TuningStatus.QUEUED.value,
            "algorithm": job_request.algorithm.value,
            "progress": 0.0,
            "trials_completed": 0,
            "total_trials": job_request.n_trials,
            "best_trial": None,
            "best_score": None,
            "best_parameters": None,
            "study_url": None,
            "created_at": datetime.utcnow(),
            "started_at": None,
            "completed_at": None,
            "duration_seconds": None,
            "config": job_request.dict()
        }
        
        tuning_jobs[job_id] = job_data
        
        # Queue tuning job
        background_tasks.add_task(execute_tuning_job, job_id, job_request)
        
        return TuningJobResponse(**job_data)
    except Exception as e:
        logger.error(f"Error creating tuning job: {e}")
        raise HTTPException(status_code=500, detail="Failed to create tuning job")

@app.get("/v1/tuning/jobs/{job_id}", response_model=TuningJobResponse)
@auth.require_auth("model", "read")
async def get_tuning_job(job_id: str):
    """Get tuning job details"""
    try:
        if job_id not in tuning_jobs:
            raise HTTPException(status_code=404, detail="Tuning job not found")
        
        job_data = tuning_jobs[job_id]
        return TuningJobResponse(**job_data)
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error getting tuning job {job_id}: {e}")
        raise HTTPException(status_code=500, detail="Failed to get tuning job")

@app.get("/v1/tuning/jobs/{job_id}/trials", response_model=List[TrialResult])
@auth.require_auth("model", "read")
async def get_tuning_trials(job_id: str):
    """Get trials for a tuning job"""
    try:
        if job_id not in tuning_jobs:
            raise HTTPException(status_code=404, detail="Tuning job not found")
        
        if job_id not in optuna_studies:
            return []
        
        study = optuna_studies[job_id]
        trials = []
        
        for trial in study.trials:
            trials.append(TrialResult(
                trial_id=trial.number,
                parameters=trial.params,
                score=trial.value if trial.value is not None else 0.0,
                metrics=trial.user_attrs,
                duration_seconds=(trial.datetime_complete - trial.datetime_start).total_seconds() if trial.datetime_complete else 0.0,
                status=trial.state.name,
                datetime_start=trial.datetime_start,
                datetime_complete=trial.datetime_complete
            ))
        
        return trials
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error getting tuning trials: {e}")
        raise HTTPException(status_code=500, detail="Failed to get tuning trials")

@app.post("/v1/tuning/jobs/{job_id}/cancel")
@auth.require_auth("model", "tune")
async def cancel_tuning_job(job_id: str):
    """Cancel a tuning job"""
    try:
        if job_id not in tuning_jobs:
            raise HTTPException(status_code=404, detail="Tuning job not found")
        
        job_data = tuning_jobs[job_id]
        
        if job_data["status"] not in [TuningStatus.QUEUED.value, TuningStatus.RUNNING.value]:
            raise HTTPException(status_code=400, detail="Cannot cancel job in current status")
        
        job_data["status"] = TuningStatus.CANCELLED.value
        job_data["completed_at"] = datetime.utcnow()
        
        return {"message": "Tuning job cancelled successfully"}
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error cancelling tuning job {job_id}: {e}")
        raise HTTPException(status_code=500, detail="Failed to cancel tuning job")

# AutoML endpoints
@app.post("/v1/automl/jobs")
@auth.require_auth("model", "create")
async def create_automl_job(
    automl_request: AutoMLRequest,
    background_tasks: BackgroundTasks
):
    """Create an AutoML job"""
    try:
        job_id = str(uuid.uuid4())
        
        # Create AutoML job record
        job_data = {
            "id": job_id,
            "name": automl_request.name,
            "type": "automl",
            "status": "running",
            "progress": 0.0,
            "models_tried": 0,
            "best_model": None,
            "best_score": None,
            "leaderboard": [],
            "created_at": datetime.utcnow(),
            "config": automl_request.dict()
        }
        
        # Queue AutoML job
        background_tasks.add_task(execute_automl_job, job_id, automl_request)
        
        return {
            "job_id": job_id,
            "message": "AutoML job started",
            "estimated_completion": (datetime.utcnow() + timedelta(minutes=automl_request.time_budget_minutes)).isoformat()
        }
    except Exception as e:
        logger.error(f"Error creating AutoML job: {e}")
        raise HTTPException(status_code=500, detail="Failed to create AutoML job")

# Background tasks
async def execute_tuning_job(job_id: str, job_request: TuningJobRequest):
    """Background task to execute hyperparameter tuning"""
    try:
        logger.info(f"Starting tuning job {job_id}")
        
        # Update job status
        job_data = tuning_jobs[job_id]
        job_data["status"] = TuningStatus.RUNNING.value
        job_data["started_at"] = datetime.utcnow()
        
        # Create Optuna study
        sampler = create_sampler(job_request.algorithm)
        pruner = MedianPruner() if job_request.pruning else optuna.pruners.NopPruner()
        
        direction = "maximize" if job_request.objective_direction == ObjectiveDirection.MAXIMIZE else "minimize"
        
        study = optuna.create_study(
            direction=direction,
            sampler=sampler,
            pruner=pruner,
            study_name=f"tuning_job_{job_id}"
        )
        
        optuna_studies[job_id] = study
        
        # Define objective function
        def objective(trial):
            # Check if job was cancelled
            if tuning_jobs[job_id]["status"] == TuningStatus.CANCELLED.value:
                raise optuna.TrialPruned()
            
            # Suggest parameters based on parameter space
            params = {}
            for param_def in job_request.parameter_space:
                if param_def.type == "float":
                    if param_def.log:
                        params[param_def.name] = trial.suggest_loguniform(
                            param_def.name, param_def.low, param_def.high
                        )
                    else:
                        params[param_def.name] = trial.suggest_uniform(
                            param_def.name, param_def.low, param_def.high
                        )
                elif param_def.type == "int":
                    params[param_def.name] = trial.suggest_int(
                        param_def.name, int(param_def.low), int(param_def.high)
                    )
                elif param_def.type == "categorical":
                    params[param_def.name] = trial.suggest_categorical(
                        param_def.name, param_def.choices
                    )
            
            # Simulate model training and evaluation
            score = simulate_model_training(params, job_request)
            
            # Update progress
            trials_completed = len(study.trials)
            progress = (trials_completed / job_request.n_trials) * 100
            job_data["progress"] = progress
            job_data["trials_completed"] = trials_completed
            
            # Update best trial if this is better
            if job_data["best_score"] is None or (
                (job_request.objective_direction == ObjectiveDirection.MAXIMIZE and score > job_data["best_score"]) or
                (job_request.objective_direction == ObjectiveDirection.MINIMIZE and score < job_data["best_score"])
            ):
                job_data["best_score"] = score
                job_data["best_parameters"] = params
                job_data["best_trial"] = {
                    "trial_id": trial.number,
                    "score": score,
                    "parameters": params
                }
            
            return score
        
        # Run optimization
        study.optimize(
            objective,
            n_trials=job_request.n_trials,
            timeout=job_request.timeout_seconds,
            n_jobs=job_request.n_jobs
        )
        
        # Complete the job
        job_data["status"] = TuningStatus.COMPLETED.value
        job_data["completed_at"] = datetime.utcnow()
        job_data["duration_seconds"] = int((job_data["completed_at"] - job_data["started_at"]).total_seconds())
        
        # Log to MLflow
        mlflow.optuna.log_study(study)
        
        logger.info(f"Tuning job {job_id} completed successfully")
        logger.info(f"Best score: {job_data['best_score']}")
        logger.info(f"Best parameters: {job_data['best_parameters']}")
        
    except Exception as e:
        logger.error(f"Error in tuning job {job_id}: {e}")
        
        # Update job status to failed
        job_data = tuning_jobs[job_id]
        job_data["status"] = TuningStatus.FAILED.value
        job_data["completed_at"] = datetime.utcnow()
        if job_data["started_at"]:
            job_data["duration_seconds"] = int((job_data["completed_at"] - job_data["started_at"]).total_seconds())

async def execute_automl_job(job_id: str, automl_request: AutoMLRequest):
    """Background task to execute AutoML job"""
    try:
        logger.info(f"Starting AutoML job {job_id}")
        
        # Define algorithms to try
        algorithms = automl_request.algorithms or [
            "random_forest", "svm", "neural_network", "gradient_boosting"
        ]
        
        leaderboard = []
        
        for i, algorithm in enumerate(algorithms):
            # Simulate model training
            await asyncio.sleep(5)  # Simulate training time
            
            # Generate mock performance metrics
            if automl_request.problem_type == "classification":
                score = np.random.uniform(0.75, 0.95)
                metrics = {
                    "accuracy": score,
                    "precision": np.random.uniform(0.7, 0.9),
                    "recall": np.random.uniform(0.7, 0.9),
                    "f1_score": np.random.uniform(0.7, 0.9)
                }
            else:  # regression
                score = np.random.uniform(0.1, 0.5)  # Lower is better for RMSE
                metrics = {
                    "rmse": score,
                    "mae": np.random.uniform(0.05, 0.3),
                    "r2_score": np.random.uniform(0.7, 0.95)
                }
            
            model_result = {
                "algorithm": algorithm,
                "score": score,
                "metrics": metrics,
                "training_time": np.random.uniform(10, 300),
                "model_size_mb": np.random.uniform(1, 50)
            }
            
            leaderboard.append(model_result)
            
            # Update progress
            progress = ((i + 1) / len(algorithms)) * 100
            
            logger.info(f"AutoML job {job_id} - Completed {algorithm}, Score: {score:.4f}")
        
        # Sort leaderboard by score
        if automl_request.problem_type == "classification":
            leaderboard.sort(key=lambda x: x["score"], reverse=True)
        else:
            leaderboard.sort(key=lambda x: x["score"])  # Lower is better for regression
        
        logger.info(f"AutoML job {job_id} completed successfully")
        logger.info(f"Best model: {leaderboard[0]['algorithm']} with score: {leaderboard[0]['score']:.4f}")
        
    except Exception as e:
        logger.error(f"Error in AutoML job {job_id}: {e}")

def create_sampler(algorithm: OptimizationAlgorithm):
    """Create Optuna sampler based on algorithm"""
    if algorithm == OptimizationAlgorithm.TPE:
        return TPESampler()
    elif algorithm == OptimizationAlgorithm.CMAES:
        return CmaEsSampler()
    elif algorithm == OptimizationAlgorithm.RANDOM:
        return optuna.samplers.RandomSampler()
    else:
        return TPESampler()  # Default

def simulate_model_training(params: Dict[str, Any], job_request: TuningJobRequest) -> float:
    """Simulate model training and return performance score"""
    # This is a simplified simulation
    # In production, this would train actual models
    
    # Generate a score based on parameters (mock optimization landscape)
    base_score = 0.8
    
    # Add some parameter-dependent variation
    for param_name, param_value in params.items():
        if isinstance(param_value, (int, float)):
            # Simulate parameter impact
            normalized_value = (param_value - 0.5) ** 2  # Quadratic relationship
            base_score += np.random.normal(0, 0.05) - normalized_value * 0.1
    
    # Add noise
    score = base_score + np.random.normal(0, 0.02)
    
    # Ensure score is in valid range
    if job_request.objective_metric in ["accuracy", "f1_score", "precision", "recall"]:
        score = np.clip(score, 0, 1)
    else:
        score = max(0, score)
    
    return score

if __name__ == "__main__":
    uvicorn.run(
        "main:app",
        host="0.0.0.0",
        port=int(os.getenv("PORT", 8080)),
        reload=os.getenv("ENVIRONMENT") == "development"
    )
