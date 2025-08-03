"""
Data Integration Service - Python
ETL pipelines and data connectors for the 002AIC platform
Handles data ingestion from various sources and transformation workflows
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
import pandas as pd
import numpy as np
from enum import Enum
import sqlalchemy
from sqlalchemy import create_engine
import redis
import boto3
from minio import Minio
import requests
from celery import Celery
import apache_beam as beam
from apache_beam.options.pipeline_options import PipelineOptions

# Import our auth middleware
import sys
sys.path.append('/home/oss/002AIC/libs/auth-middleware/python')
from auth_middleware import FastAPIAuthMiddleware, AuthConfig

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Initialize FastAPI app
app = FastAPI(
    title="002AIC Data Integration Service",
    description="ETL pipelines and data connectors",
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
    service_name="data-integration-service"
)

auth = FastAPIAuthMiddleware(auth_config)

# Initialize Celery for background tasks
celery_app = Celery(
    'data_integration',
    broker=os.getenv('CELERY_BROKER_URL', 'redis://localhost:6379/0'),
    backend=os.getenv('CELERY_RESULT_BACKEND', 'redis://localhost:6379/0')
)

# Enums
class ConnectorType(str, Enum):
    DATABASE = "database"
    API = "api"
    FILE = "file"
    STREAM = "stream"
    CLOUD_STORAGE = "cloud_storage"
    MESSAGE_QUEUE = "message_queue"

class PipelineStatus(str, Enum):
    DRAFT = "draft"
    ACTIVE = "active"
    RUNNING = "running"
    PAUSED = "paused"
    COMPLETED = "completed"
    FAILED = "failed"

class TransformationType(str, Enum):
    FILTER = "filter"
    MAP = "map"
    AGGREGATE = "aggregate"
    JOIN = "join"
    PIVOT = "pivot"
    NORMALIZE = "normalize"
    VALIDATE = "validate"
    ENRICH = "enrich"

# Pydantic models
class DataSource(BaseModel):
    name: str = Field(..., description="Data source name")
    type: ConnectorType = Field(..., description="Connector type")
    connection_config: Dict[str, Any] = Field(..., description="Connection configuration")
    schema_config: Optional[Dict[str, Any]] = Field(None, description="Schema configuration")
    credentials: Dict[str, str] = Field(..., description="Authentication credentials")
    refresh_interval: Optional[int] = Field(None, description="Refresh interval in seconds")
    batch_size: int = Field(default=1000, description="Batch size for data processing")
    tags: List[str] = Field(default_factory=list, description="Source tags")

class TransformationStep(BaseModel):
    name: str = Field(..., description="Transformation step name")
    type: TransformationType = Field(..., description="Transformation type")
    config: Dict[str, Any] = Field(..., description="Transformation configuration")
    order: int = Field(..., description="Execution order")
    enabled: bool = Field(default=True, description="Whether step is enabled")

class ETLPipeline(BaseModel):
    name: str = Field(..., description="Pipeline name")
    description: Optional[str] = Field(None, description="Pipeline description")
    source_id: str = Field(..., description="Data source ID")
    destination_id: str = Field(..., description="Destination ID")
    transformations: List[TransformationStep] = Field(..., description="Transformation steps")
    schedule: Optional[str] = Field(None, description="Cron schedule")
    parallel_processing: bool = Field(default=False, description="Enable parallel processing")
    error_handling: Dict[str, Any] = Field(default_factory=dict, description="Error handling config")
    monitoring: Dict[str, Any] = Field(default_factory=dict, description="Monitoring config")

class DataSourceResponse(BaseModel):
    id: str
    name: str
    type: ConnectorType
    status: str
    last_sync: Optional[datetime]
    records_count: int
    error_count: int
    created_at: datetime
    updated_at: datetime

class PipelineResponse(BaseModel):
    id: str
    name: str
    status: PipelineStatus
    source_name: str
    destination_name: str
    last_run: Optional[datetime]
    next_run: Optional[datetime]
    success_rate: float
    records_processed: int
    created_at: datetime

class PipelineRun(BaseModel):
    id: str
    pipeline_id: str
    status: str
    started_at: datetime
    completed_at: Optional[datetime]
    records_processed: int
    records_failed: int
    duration_seconds: Optional[int]
    error_message: Optional[str]
    metrics: Dict[str, Any]

# Storage for data sources and pipelines
data_sources = {}
pipelines = {}
pipeline_runs = {}

# Initialize external services
def init_redis():
    return redis.Redis(
        host=os.getenv("REDIS_HOST", "localhost"),
        port=int(os.getenv("REDIS_PORT", "6379")),
        password=os.getenv("REDIS_PASSWORD", ""),
        db=6,
        decode_responses=True
    )

redis_client = init_redis()

# Health check endpoint
@app.get("/health")
async def health_check():
    """Health check endpoint"""
    redis_healthy = True
    celery_healthy = True
    
    try:
        redis_client.ping()
    except Exception:
        redis_healthy = False
    
    try:
        celery_app.control.inspect().active()
    except Exception:
        celery_healthy = False
    
    return {
        "status": "healthy" if all([redis_healthy, celery_healthy]) else "degraded",
        "service": "data-integration-service",
        "timestamp": datetime.utcnow().isoformat(),
        "version": "1.0.0",
        "dependencies": {
            "redis": "healthy" if redis_healthy else "unhealthy",
            "celery": "healthy" if celery_healthy else "unhealthy"
        }
    }

# Data source endpoints
@app.get("/v1/data-sources", response_model=List[DataSourceResponse])
@auth.require_auth("data", "read")
async def list_data_sources(
    type: Optional[ConnectorType] = None,
    status: Optional[str] = None,
    skip: int = 0,
    limit: int = 100
):
    """List data sources"""
    try:
        sources = list(data_sources.values())
        
        if type:
            sources = [s for s in sources if s["type"] == type.value]
        if status:
            sources = [s for s in sources if s["status"] == status]
        
        sources = sources[skip:skip + limit]
        return [DataSourceResponse(**source) for source in sources]
    except Exception as e:
        logger.error(f"Error listing data sources: {e}")
        raise HTTPException(status_code=500, detail="Failed to list data sources")

@app.post("/v1/data-sources", response_model=DataSourceResponse)
@auth.require_auth("data", "create")
async def create_data_source(data_source: DataSource):
    """Create a new data source"""
    try:
        source_id = str(uuid.uuid4())
        
        # Test connection
        connection_status = await test_data_source_connection(data_source)
        
        source_data = {
            "id": source_id,
            "name": data_source.name,
            "type": data_source.type.value,
            "status": "active" if connection_status else "error",
            "last_sync": None,
            "records_count": 0,
            "error_count": 0,
            "created_at": datetime.utcnow(),
            "updated_at": datetime.utcnow(),
            "config": data_source.dict()
        }
        
        data_sources[source_id] = source_data
        
        # Cache connection config in Redis
        redis_client.setex(
            f"data_source:{source_id}",
            3600,
            json.dumps(data_source.dict())
        )
        
        return DataSourceResponse(**source_data)
    except Exception as e:
        logger.error(f"Error creating data source: {e}")
        raise HTTPException(status_code=500, detail="Failed to create data source")

@app.post("/v1/data-sources/{source_id}/test")
@auth.require_auth("data", "read")
async def test_data_source(source_id: str):
    """Test data source connection"""
    try:
        if source_id not in data_sources:
            raise HTTPException(status_code=404, detail="Data source not found")
        
        source_config = data_sources[source_id]["config"]
        data_source = DataSource(**source_config)
        
        connection_status = await test_data_source_connection(data_source)
        
        return {
            "source_id": source_id,
            "connection_status": "success" if connection_status else "failed",
            "tested_at": datetime.utcnow().isoformat()
        }
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error testing data source: {e}")
        raise HTTPException(status_code=500, detail="Failed to test data source")

# Pipeline endpoints
@app.get("/v1/pipelines", response_model=List[PipelineResponse])
@auth.require_auth("pipeline", "read")
async def list_pipelines(
    status: Optional[PipelineStatus] = None,
    skip: int = 0,
    limit: int = 100
):
    """List ETL pipelines"""
    try:
        pipeline_list = list(pipelines.values())
        
        if status:
            pipeline_list = [p for p in pipeline_list if p["status"] == status.value]
        
        pipeline_list = pipeline_list[skip:skip + limit]
        return [PipelineResponse(**pipeline) for pipeline in pipeline_list]
    except Exception as e:
        logger.error(f"Error listing pipelines: {e}")
        raise HTTPException(status_code=500, detail="Failed to list pipelines")

@app.post("/v1/pipelines", response_model=PipelineResponse)
@auth.require_auth("pipeline", "create")
async def create_pipeline(pipeline: ETLPipeline):
    """Create a new ETL pipeline"""
    try:
        pipeline_id = str(uuid.uuid4())
        
        # Validate source and destination exist
        if pipeline.source_id not in data_sources:
            raise HTTPException(status_code=400, detail="Source not found")
        if pipeline.destination_id not in data_sources:
            raise HTTPException(status_code=400, detail="Destination not found")
        
        source_name = data_sources[pipeline.source_id]["name"]
        destination_name = data_sources[pipeline.destination_id]["name"]
        
        pipeline_data = {
            "id": pipeline_id,
            "name": pipeline.name,
            "status": PipelineStatus.DRAFT.value,
            "source_name": source_name,
            "destination_name": destination_name,
            "last_run": None,
            "next_run": None,
            "success_rate": 0.0,
            "records_processed": 0,
            "created_at": datetime.utcnow(),
            "config": pipeline.dict()
        }
        
        pipelines[pipeline_id] = pipeline_data
        
        return PipelineResponse(**pipeline_data)
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error creating pipeline: {e}")
        raise HTTPException(status_code=500, detail="Failed to create pipeline")

@app.post("/v1/pipelines/{pipeline_id}/run")
@auth.require_auth("pipeline", "execute")
async def run_pipeline(pipeline_id: str, background_tasks: BackgroundTasks):
    """Run an ETL pipeline"""
    try:
        if pipeline_id not in pipelines:
            raise HTTPException(status_code=404, detail="Pipeline not found")
        
        pipeline_data = pipelines[pipeline_id]
        
        if pipeline_data["status"] == PipelineStatus.RUNNING.value:
            raise HTTPException(status_code=400, detail="Pipeline is already running")
        
        run_id = str(uuid.uuid4())
        
        # Create pipeline run record
        run_data = {
            "id": run_id,
            "pipeline_id": pipeline_id,
            "status": "running",
            "started_at": datetime.utcnow(),
            "completed_at": None,
            "records_processed": 0,
            "records_failed": 0,
            "duration_seconds": None,
            "error_message": None,
            "metrics": {}
        }
        
        pipeline_runs[run_id] = run_data
        
        # Update pipeline status
        pipeline_data["status"] = PipelineStatus.RUNNING.value
        pipeline_data["last_run"] = datetime.utcnow()
        
        # Queue pipeline execution
        background_tasks.add_task(execute_pipeline, pipeline_id, run_id)
        
        return {
            "run_id": run_id,
            "pipeline_id": pipeline_id,
            "status": "running",
            "message": "Pipeline execution started"
        }
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error running pipeline: {e}")
        raise HTTPException(status_code=500, detail="Failed to run pipeline")

@app.get("/v1/pipelines/{pipeline_id}/runs", response_model=List[PipelineRun])
@auth.require_auth("pipeline", "read")
async def get_pipeline_runs(pipeline_id: str, limit: int = 50):
    """Get pipeline run history"""
    try:
        if pipeline_id not in pipelines:
            raise HTTPException(status_code=404, detail="Pipeline not found")
        
        runs = [
            run for run in pipeline_runs.values() 
            if run["pipeline_id"] == pipeline_id
        ]
        
        # Sort by started_at descending
        runs.sort(key=lambda x: x["started_at"], reverse=True)
        runs = runs[:limit]
        
        return [PipelineRun(**run) for run in runs]
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error getting pipeline runs: {e}")
        raise HTTPException(status_code=500, detail="Failed to get pipeline runs")

# Data preview and schema inference
@app.post("/v1/data-sources/{source_id}/preview")
@auth.require_auth("data", "read")
async def preview_data_source(source_id: str, limit: int = 100):
    """Preview data from a source"""
    try:
        if source_id not in data_sources:
            raise HTTPException(status_code=404, detail="Data source not found")
        
        source_config = data_sources[source_id]["config"]
        data_source = DataSource(**source_config)
        
        # Mock data preview
        preview_data = {
            "source_id": source_id,
            "sample_data": [
                {"id": 1, "name": "Sample 1", "value": 100.5, "timestamp": "2024-01-01T00:00:00Z"},
                {"id": 2, "name": "Sample 2", "value": 200.3, "timestamp": "2024-01-01T01:00:00Z"},
                {"id": 3, "name": "Sample 3", "value": 150.7, "timestamp": "2024-01-01T02:00:00Z"}
            ],
            "schema": {
                "id": {"type": "integer", "nullable": False},
                "name": {"type": "string", "nullable": False},
                "value": {"type": "float", "nullable": True},
                "timestamp": {"type": "datetime", "nullable": False}
            },
            "total_records": 1000000,
            "preview_limit": limit
        }
        
        return preview_data
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error previewing data source: {e}")
        raise HTTPException(status_code=500, detail="Failed to preview data source")

@app.post("/v1/data-sources/{source_id}/infer-schema")
@auth.require_auth("data", "read")
async def infer_schema(source_id: str):
    """Infer schema from data source"""
    try:
        if source_id not in data_sources:
            raise HTTPException(status_code=404, detail="Data source not found")
        
        # Mock schema inference
        inferred_schema = {
            "source_id": source_id,
            "schema": {
                "columns": [
                    {"name": "id", "type": "integer", "nullable": False, "primary_key": True},
                    {"name": "name", "type": "string", "nullable": False, "max_length": 255},
                    {"name": "email", "type": "string", "nullable": True, "format": "email"},
                    {"name": "age", "type": "integer", "nullable": True, "min": 0, "max": 150},
                    {"name": "created_at", "type": "datetime", "nullable": False},
                    {"name": "score", "type": "float", "nullable": True, "min": 0.0, "max": 100.0}
                ],
                "indexes": [
                    {"name": "idx_email", "columns": ["email"], "unique": True},
                    {"name": "idx_created_at", "columns": ["created_at"]}
                ]
            },
            "confidence": 0.95,
            "sample_size": 10000,
            "inferred_at": datetime.utcnow().isoformat()
        }
        
        return inferred_schema
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error inferring schema: {e}")
        raise HTTPException(status_code=500, detail="Failed to infer schema")

# Background tasks
async def test_data_source_connection(data_source: DataSource) -> bool:
    """Test connection to a data source"""
    try:
        if data_source.type == ConnectorType.DATABASE:
            # Test database connection
            connection_string = data_source.connection_config.get("connection_string")
            if connection_string:
                engine = create_engine(connection_string)
                with engine.connect() as conn:
                    conn.execute(sqlalchemy.text("SELECT 1"))
                return True
        
        elif data_source.type == ConnectorType.API:
            # Test API connection
            url = data_source.connection_config.get("base_url")
            if url:
                response = requests.get(f"{url}/health", timeout=10)
                return response.status_code == 200
        
        elif data_source.type == ConnectorType.FILE:
            # Test file access
            file_path = data_source.connection_config.get("file_path")
            if file_path:
                return os.path.exists(file_path)
        
        # Default to success for other types
        return True
        
    except Exception as e:
        logger.error(f"Connection test failed: {e}")
        return False

async def execute_pipeline(pipeline_id: str, run_id: str):
    """Background task to execute ETL pipeline"""
    try:
        logger.info(f"Executing pipeline {pipeline_id}, run {run_id}")
        
        pipeline_data = pipelines[pipeline_id]
        run_data = pipeline_runs[run_id]
        pipeline_config = ETLPipeline(**pipeline_data["config"])
        
        # Simulate pipeline execution
        total_records = 10000
        processed_records = 0
        failed_records = 0
        
        # Execute transformation steps
        for step in pipeline_config.transformations:
            if not step.enabled:
                continue
                
            logger.info(f"Executing transformation step: {step.name}")
            
            # Simulate step execution
            await asyncio.sleep(2)
            
            # Simulate processing records
            step_processed = int(total_records * 0.8)  # 80% success rate
            step_failed = total_records - step_processed
            
            processed_records += step_processed
            failed_records += step_failed
            
            # Update run metrics
            run_data["records_processed"] = processed_records
            run_data["records_failed"] = failed_records
            run_data["metrics"][step.name] = {
                "processed": step_processed,
                "failed": step_failed,
                "duration_seconds": 2
            }
        
        # Complete the run
        now = datetime.utcnow()
        run_data["status"] = "completed"
        run_data["completed_at"] = now
        run_data["duration_seconds"] = int((now - run_data["started_at"]).total_seconds())
        
        # Update pipeline status
        pipeline_data["status"] = PipelineStatus.COMPLETED.value
        pipeline_data["records_processed"] += processed_records
        
        # Calculate success rate
        total_runs = len([r for r in pipeline_runs.values() if r["pipeline_id"] == pipeline_id])
        successful_runs = len([r for r in pipeline_runs.values() 
                             if r["pipeline_id"] == pipeline_id and r["status"] == "completed"])
        pipeline_data["success_rate"] = (successful_runs / total_runs) * 100 if total_runs > 0 else 0
        
        logger.info(f"Pipeline {pipeline_id} completed successfully")
        logger.info(f"Processed: {processed_records}, Failed: {failed_records}")
        
    except Exception as e:
        logger.error(f"Error executing pipeline {pipeline_id}: {e}")
        
        # Update run status to failed
        run_data = pipeline_runs[run_id]
        run_data["status"] = "failed"
        run_data["completed_at"] = datetime.utcnow()
        run_data["error_message"] = str(e)
        
        # Update pipeline status
        pipeline_data = pipelines[pipeline_id]
        pipeline_data["status"] = PipelineStatus.FAILED.value

if __name__ == "__main__":
    uvicorn.run(
        "main:app",
        host="0.0.0.0",
        port=int(os.getenv("PORT", 8080)),
        reload=os.getenv("ENVIRONMENT") == "development"
    )
