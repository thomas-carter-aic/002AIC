"""
Model Registry Service - Python
AI model management and versioning for the 002AIC platform
Handles model registration, versioning, metadata, lineage tracking, and lifecycle management
"""

import asyncio
import json
import logging
import os
import hashlib
import shutil
from datetime import datetime, timedelta
from typing import Dict, List, Optional, Any, Union
from pathlib import Path
from dataclasses import dataclass, asdict
import pickle
import joblib

import aioredis
import asyncpg
from fastapi import FastAPI, HTTPException, Depends, UploadFile, File, Form, BackgroundTasks
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel, Field, validator
from prometheus_client import Counter, Histogram, Gauge, generate_latest
from starlette.responses import Response, FileResponse
import uvicorn
import mlflow
import mlflow.tracking
from mlflow.tracking import MlflowClient
import boto3
from minio import Minio
import numpy as np
import pandas as pd
from sklearn.base import BaseEstimator
import tensorflow as tf
import torch

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Configuration
class Config:
    def __init__(self):
        self.port = int(os.getenv("PORT", "8080"))
        self.database_url = os.getenv("DATABASE_URL", "postgresql://postgres:password@localhost:5432/model_registry")
        self.redis_url = os.getenv("REDIS_URL", "redis://localhost:6379")
        self.mlflow_tracking_uri = os.getenv("MLFLOW_TRACKING_URI", "http://localhost:5000")
        self.model_storage_path = os.getenv("MODEL_STORAGE_PATH", "/tmp/models")
        self.minio_endpoint = os.getenv("MINIO_ENDPOINT", "localhost:9000")
        self.minio_access_key = os.getenv("MINIO_ACCESS_KEY", "minioadmin")
        self.minio_secret_key = os.getenv("MINIO_SECRET_KEY", "minioadmin")
        self.minio_bucket = os.getenv("MINIO_BUCKET", "model-registry")
        self.environment = os.getenv("ENVIRONMENT", "development")
        self.max_model_size = int(os.getenv("MAX_MODEL_SIZE", "1073741824"))  # 1GB

config = Config()

# Enums
class ModelStatus(str):
    DRAFT = "draft"
    REGISTERED = "registered"
    STAGING = "staging"
    PRODUCTION = "production"
    ARCHIVED = "archived"
    DEPRECATED = "deprecated"

class ModelType(str):
    SKLEARN = "sklearn"
    TENSORFLOW = "tensorflow"
    PYTORCH = "pytorch"
    XGBOOST = "xgboost"
    LIGHTGBM = "lightgbm"
    ONNX = "onnx"
    CUSTOM = "custom"

class ModelFramework(str):
    SCIKIT_LEARN = "scikit-learn"
    TENSORFLOW = "tensorflow"
    PYTORCH = "pytorch"
    KERAS = "keras"
    XGBOOST = "xgboost"
    LIGHTGBM = "lightgbm"
    CATBOOST = "catboost"
    HUGGINGFACE = "huggingface"
    CUSTOM = "custom"

# Pydantic models
class ModelMetadata(BaseModel):
    name: str = Field(..., min_length=1, max_length=255)
    version: str = Field(..., min_length=1, max_length=50)
    description: Optional[str] = None
    framework: ModelFramework
    model_type: ModelType
    task_type: str = Field(..., description="e.g., classification, regression, nlp, cv")
    tags: List[str] = Field(default_factory=list)
    parameters: Dict[str, Any] = Field(default_factory=dict)
    metrics: Dict[str, float] = Field(default_factory=dict)
    dataset_info: Dict[str, Any] = Field(default_factory=dict)
    training_info: Dict[str, Any] = Field(default_factory=dict)
    requirements: List[str] = Field(default_factory=list)
    input_schema: Optional[Dict[str, Any]] = None
    output_schema: Optional[Dict[str, Any]] = None
    model_size_bytes: Optional[int] = None
    created_by: str
    project_id: Optional[str] = None

class RegisterModelRequest(BaseModel):
    metadata: ModelMetadata
    model_uri: Optional[str] = None  # If model is already stored elsewhere
    experiment_id: Optional[str] = None
    run_id: Optional[str] = None
    stage: ModelStatus = ModelStatus.DRAFT

class UpdateModelRequest(BaseModel):
    description: Optional[str] = None
    tags: Optional[List[str]] = None
    stage: Optional[ModelStatus] = None
    metrics: Optional[Dict[str, float]] = None
    parameters: Optional[Dict[str, Any]] = None

class ModelSearchRequest(BaseModel):
    name: Optional[str] = None
    framework: Optional[ModelFramework] = None
    model_type: Optional[ModelType] = None
    task_type: Optional[str] = None
    stage: Optional[ModelStatus] = None
    tags: Optional[List[str]] = None
    created_by: Optional[str] = None
    project_id: Optional[str] = None
    min_accuracy: Optional[float] = None
    limit: int = Field(default=50, ge=1, le=100)
    offset: int = Field(default=0, ge=0)

class ModelResponse(BaseModel):
    id: str
    name: str
    version: str
    description: Optional[str]
    framework: ModelFramework
    model_type: ModelType
    task_type: str
    stage: ModelStatus
    tags: List[str]
    parameters: Dict[str, Any]
    metrics: Dict[str, float]
    model_uri: str
    model_size_bytes: int
    checksum: str
    created_by: str
    project_id: Optional[str]
    created_at: datetime
    updated_at: datetime
    download_count: int
    lineage: Optional[Dict[str, Any]] = None

class ModelLineage(BaseModel):
    parent_models: List[str] = Field(default_factory=list)
    child_models: List[str] = Field(default_factory=list)
    datasets: List[str] = Field(default_factory=list)
    experiments: List[str] = Field(default_factory=list)
    pipelines: List[str] = Field(default_factory=list)

# Prometheus metrics
models_total = Gauge('model_registry_models_total', 'Total number of registered models', ['framework', 'stage'])
model_downloads = Counter('model_registry_downloads_total', 'Total model downloads', ['model_id', 'version'])
model_uploads = Counter('model_registry_uploads_total', 'Total model uploads', ['framework', 'model_type'])
model_size_bytes = Histogram('model_registry_size_bytes', 'Model size distribution', ['framework'])
registry_operations = Counter('model_registry_operations_total', 'Registry operations', ['operation', 'status'])

# Global variables
app = FastAPI(title="Model Registry Service", version="1.0.0")
db_pool = None
redis_client = None
minio_client = None
mlflow_client = None

# CORS middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Database connection
async def get_db():
    return db_pool

# Redis connection
async def get_redis():
    return redis_client

# Health check endpoint
@app.get("/health")
async def health_check():
    status = {
        "status": "healthy",
        "service": "model-registry-service",
        "timestamp": datetime.utcnow().isoformat(),
        "version": "1.0.0"
    }
    
    try:
        # Check database
        async with db_pool.acquire() as conn:
            await conn.fetchval("SELECT 1")
        status["database"] = "connected"
    except Exception as e:
        status["status"] = "unhealthy"
        status["database"] = f"disconnected: {str(e)}"
    
    try:
        # Check Redis
        await redis_client.ping()
        status["redis"] = "connected"
    except Exception as e:
        status["status"] = "unhealthy"
        status["redis"] = f"disconnected: {str(e)}"
    
    try:
        # Check MLflow
        mlflow_client.list_experiments(max_results=1)
        status["mlflow"] = "connected"
    except Exception as e:
        status["status"] = "unhealthy"
        status["mlflow"] = f"disconnected: {str(e)}"
    
    try:
        # Check MinIO
        minio_client.bucket_exists(config.minio_bucket)
        status["minio"] = "connected"
    except Exception as e:
        status["status"] = "unhealthy"
        status["minio"] = f"disconnected: {str(e)}"
    
    if status["status"] == "unhealthy":
        raise HTTPException(status_code=503, detail=status)
    
    return status

# Metrics endpoint
@app.get("/metrics")
async def metrics():
    return Response(generate_latest(), media_type="text/plain")

# Model registration endpoints
@app.post("/v1/models/register", response_model=ModelResponse)
async def register_model(
    request: RegisterModelRequest,
    model_file: Optional[UploadFile] = File(None),
    background_tasks: BackgroundTasks,
    db = Depends(get_db)
):
    """Register a new model in the registry"""
    try:
        model_id = f"{request.metadata.name}:{request.metadata.version}"
        
        # Check if model already exists
        existing_model = await get_model_by_name_version(
            request.metadata.name, 
            request.metadata.version
        )
        if existing_model:
            raise HTTPException(
                status_code=409, 
                detail=f"Model {model_id} already exists"
            )
        
        # Handle model file upload
        model_uri = request.model_uri
        model_size = 0
        checksum = ""
        
        if model_file:
            # Validate file size
            if model_file.size > config.max_model_size:
                raise HTTPException(
                    status_code=413,
                    detail=f"Model file too large. Max size: {config.max_model_size} bytes"
                )
            
            # Store model file
            model_uri, model_size, checksum = await store_model_file(
                model_file, 
                request.metadata.name, 
                request.metadata.version
            )
        
        # Create model record
        model_data = {
            "id": model_id,
            "name": request.metadata.name,
            "version": request.metadata.version,
            "description": request.metadata.description,
            "framework": request.metadata.framework.value,
            "model_type": request.metadata.model_type.value,
            "task_type": request.metadata.task_type,
            "stage": request.stage.value,
            "tags": request.metadata.tags,
            "parameters": request.metadata.parameters,
            "metrics": request.metadata.metrics,
            "dataset_info": request.metadata.dataset_info,
            "training_info": request.metadata.training_info,
            "requirements": request.metadata.requirements,
            "input_schema": request.metadata.input_schema,
            "output_schema": request.metadata.output_schema,
            "model_uri": model_uri,
            "model_size_bytes": model_size,
            "checksum": checksum,
            "created_by": request.metadata.created_by,
            "project_id": request.metadata.project_id,
            "experiment_id": request.experiment_id,
            "run_id": request.run_id,
            "download_count": 0,
            "created_at": datetime.utcnow(),
            "updated_at": datetime.utcnow()
        }
        
        # Store in database
        await store_model(model_data)
        
        # Register with MLflow if experiment/run provided
        if request.experiment_id and request.run_id:
            background_tasks.add_task(
                register_with_mlflow, 
                model_data, 
                request.experiment_id, 
                request.run_id
            )
        
        # Update metrics
        models_total.labels(
            framework=request.metadata.framework.value,
            stage=request.stage.value
        ).inc()
        
        model_uploads.labels(
            framework=request.metadata.framework.value,
            model_type=request.metadata.model_type.value
        ).inc()
        
        if model_size > 0:
            model_size_bytes.labels(
                framework=request.metadata.framework.value
            ).observe(model_size)
        
        registry_operations.labels(operation="register", status="success").inc()
        
        # Create response
        response = ModelResponse(
            id=model_id,
            name=request.metadata.name,
            version=request.metadata.version,
            description=request.metadata.description,
            framework=request.metadata.framework,
            model_type=request.metadata.model_type,
            task_type=request.metadata.task_type,
            stage=request.stage,
            tags=request.metadata.tags,
            parameters=request.metadata.parameters,
            metrics=request.metadata.metrics,
            model_uri=model_uri or "",
            model_size_bytes=model_size,
            checksum=checksum,
            created_by=request.metadata.created_by,
            project_id=request.metadata.project_id,
            created_at=model_data["created_at"],
            updated_at=model_data["updated_at"],
            download_count=0
        )
        
        logger.info(f"Registered model: {model_id}")
        return response
        
    except HTTPException:
        registry_operations.labels(operation="register", status="error").inc()
        raise
    except Exception as e:
        registry_operations.labels(operation="register", status="error").inc()
        logger.error(f"Error registering model: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to register model: {str(e)}")

@app.get("/v1/models/{model_id}", response_model=ModelResponse)
async def get_model(model_id: str, db = Depends(get_db)):
    """Get model by ID"""
    try:
        model = await get_model_by_id(model_id)
        if not model:
            raise HTTPException(status_code=404, detail="Model not found")
        
        # Get lineage information
        lineage = await get_model_lineage(model_id)
        
        response = ModelResponse(**model)
        response.lineage = lineage
        
        return response
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error getting model {model_id}: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to get model: {str(e)}")

@app.get("/v1/models")
async def search_models(
    name: Optional[str] = None,
    framework: Optional[ModelFramework] = None,
    model_type: Optional[ModelType] = None,
    task_type: Optional[str] = None,
    stage: Optional[ModelStatus] = None,
    tags: Optional[str] = None,  # Comma-separated
    created_by: Optional[str] = None,
    project_id: Optional[str] = None,
    min_accuracy: Optional[float] = None,
    limit: int = 50,
    offset: int = 0,
    db = Depends(get_db)
):
    """Search models with filters"""
    try:
        # Parse tags
        tag_list = tags.split(",") if tags else None
        
        search_request = ModelSearchRequest(
            name=name,
            framework=framework,
            model_type=model_type,
            task_type=task_type,
            stage=stage,
            tags=tag_list,
            created_by=created_by,
            project_id=project_id,
            min_accuracy=min_accuracy,
            limit=limit,
            offset=offset
        )
        
        models = await search_models_in_db(search_request)
        
        return {
            "models": [ModelResponse(**model) for model in models],
            "total": len(models),
            "limit": limit,
            "offset": offset
        }
        
    except Exception as e:
        logger.error(f"Error searching models: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to search models: {str(e)}")

@app.put("/v1/models/{model_id}", response_model=ModelResponse)
async def update_model(
    model_id: str,
    request: UpdateModelRequest,
    db = Depends(get_db)
):
    """Update model metadata"""
    try:
        model = await get_model_by_id(model_id)
        if not model:
            raise HTTPException(status_code=404, detail="Model not found")
        
        # Update fields
        updates = {}
        if request.description is not None:
            updates["description"] = request.description
        if request.tags is not None:
            updates["tags"] = request.tags
        if request.stage is not None:
            updates["stage"] = request.stage.value
        if request.metrics is not None:
            updates["metrics"] = request.metrics
        if request.parameters is not None:
            updates["parameters"] = request.parameters
        
        updates["updated_at"] = datetime.utcnow()
        
        # Update in database
        await update_model_in_db(model_id, updates)
        
        # Get updated model
        updated_model = await get_model_by_id(model_id)
        
        registry_operations.labels(operation="update", status="success").inc()
        
        logger.info(f"Updated model: {model_id}")
        return ModelResponse(**updated_model)
        
    except HTTPException:
        registry_operations.labels(operation="update", status="error").inc()
        raise
    except Exception as e:
        registry_operations.labels(operation="update", status="error").inc()
        logger.error(f"Error updating model {model_id}: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to update model: {str(e)}")

@app.delete("/v1/models/{model_id}")
async def delete_model(model_id: str, db = Depends(get_db)):
    """Delete model from registry"""
    try:
        model = await get_model_by_id(model_id)
        if not model:
            raise HTTPException(status_code=404, detail="Model not found")
        
        # Delete model file from storage
        if model.get("model_uri"):
            await delete_model_file(model["model_uri"])
        
        # Delete from database
        await delete_model_from_db(model_id)
        
        registry_operations.labels(operation="delete", status="success").inc()
        
        logger.info(f"Deleted model: {model_id}")
        return {"message": "Model deleted successfully"}
        
    except HTTPException:
        registry_operations.labels(operation="delete", status="error").inc()
        raise
    except Exception as e:
        registry_operations.labels(operation="delete", status="error").inc()
        logger.error(f"Error deleting model {model_id}: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to delete model: {str(e)}")

@app.get("/v1/models/{model_id}/download")
async def download_model(model_id: str, db = Depends(get_db)):
    """Download model file"""
    try:
        model = await get_model_by_id(model_id)
        if not model:
            raise HTTPException(status_code=404, detail="Model not found")
        
        if not model.get("model_uri"):
            raise HTTPException(status_code=404, detail="Model file not found")
        
        # Get model file
        file_path = await get_model_file(model["model_uri"])
        
        # Update download count
        await increment_download_count(model_id)
        
        # Update metrics
        model_downloads.labels(
            model_id=model_id,
            version=model["version"]
        ).inc()
        
        return FileResponse(
            file_path,
            filename=f"{model['name']}-{model['version']}.pkl",
            media_type="application/octet-stream"
        )
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error downloading model {model_id}: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to download model: {str(e)}")

# Model lineage endpoints
@app.post("/v1/models/{model_id}/lineage")
async def update_model_lineage(
    model_id: str,
    lineage: ModelLineage,
    db = Depends(get_db)
):
    """Update model lineage information"""
    try:
        model = await get_model_by_id(model_id)
        if not model:
            raise HTTPException(status_code=404, detail="Model not found")
        
        await store_model_lineage(model_id, lineage.dict())
        
        return {"message": "Model lineage updated successfully"}
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error updating lineage for model {model_id}: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to update lineage: {str(e)}")

@app.get("/v1/models/{model_id}/lineage")
async def get_model_lineage_endpoint(model_id: str, db = Depends(get_db)):
    """Get model lineage information"""
    try:
        lineage = await get_model_lineage(model_id)
        return lineage or {}
        
    except Exception as e:
        logger.error(f"Error getting lineage for model {model_id}: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to get lineage: {str(e)}")

# Model comparison endpoints
@app.post("/v1/models/compare")
async def compare_models(
    model_ids: List[str],
    metrics: Optional[List[str]] = None,
    db = Depends(get_db)
):
    """Compare multiple models"""
    try:
        if len(model_ids) < 2:
            raise HTTPException(status_code=400, detail="At least 2 models required for comparison")
        
        models = []
        for model_id in model_ids:
            model = await get_model_by_id(model_id)
            if not model:
                raise HTTPException(status_code=404, detail=f"Model {model_id} not found")
            models.append(model)
        
        # Compare models
        comparison = await compare_model_metrics(models, metrics)
        
        return {
            "models": models,
            "comparison": comparison,
            "timestamp": datetime.utcnow().isoformat()
        }
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error comparing models: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to compare models: {str(e)}")

# Startup and shutdown events
@app.on_event("startup")
async def startup_event():
    global db_pool, redis_client, minio_client, mlflow_client
    
    try:
        # Initialize database connection pool
        db_pool = await asyncpg.create_pool(config.database_url)
        logger.info("Database connection pool created")
        
        # Initialize Redis client
        redis_client = aioredis.from_url(config.redis_url)
        logger.info("Redis client initialized")
        
        # Initialize MinIO client
        minio_client = Minio(
            config.minio_endpoint,
            access_key=config.minio_access_key,
            secret_key=config.minio_secret_key,
            secure=False
        )
        
        # Create bucket if it doesn't exist
        if not minio_client.bucket_exists(config.minio_bucket):
            minio_client.make_bucket(config.minio_bucket)
        logger.info("MinIO client initialized")
        
        # Initialize MLflow client
        mlflow.set_tracking_uri(config.mlflow_tracking_uri)
        mlflow_client = MlflowClient(config.mlflow_tracking_uri)
        logger.info("MLflow client initialized")
        
        # Create database tables
        await create_database_tables()
        
        # Create storage directories
        os.makedirs(config.model_storage_path, exist_ok=True)
        
        # Start background tasks
        asyncio.create_task(update_metrics_periodically())
        
        logger.info("Model Registry service started successfully")
        
    except Exception as e:
        logger.error(f"Failed to start model registry service: {str(e)}")
        raise

@app.on_event("shutdown")
async def shutdown_event():
    global db_pool, redis_client
    
    try:
        if db_pool:
            await db_pool.close()
        if redis_client:
            await redis_client.close()
        
        logger.info("Model Registry service shut down successfully")
        
    except Exception as e:
        logger.error(f"Error during shutdown: {str(e)}")

# Helper functions will be implemented in separate files due to length
# This includes database operations, file storage, MLflow integration, etc.

if __name__ == "__main__":
    uvicorn.run(
        "main:app",
        host="0.0.0.0",
        port=config.port,
        log_level="info",
        reload=config.environment == "development"
    )
