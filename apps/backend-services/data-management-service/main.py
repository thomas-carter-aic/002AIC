"""
Data Management Service - Python
Core data lakehouse service for the 002AIC platform
Handles data ingestion, storage, cataloging, and governance
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
import pandas as pd
import pyarrow as pa
import pyarrow.parquet as pq
from deltalake import DeltaTable, write_deltalake
import boto3
from sqlalchemy import create_engine, text
from sqlalchemy.orm import sessionmaker
import redis
from minio import Minio
from minio.error import S3Error
import uuid
import json
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
    title="002AIC Data Management Service",
    description="Data lakehouse service for unified data storage and management",
    version="1.0.0",
    docs_url="/docs",
    redoc_url="/redoc"
)

# CORS middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],  # Configure appropriately for production
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
    service_name="data-management-service"
)

auth = FastAPIAuthMiddleware(auth_config)

# Initialize external services
def init_minio():
    """Initialize MinIO client for object storage"""
    return Minio(
        os.getenv("MINIO_ENDPOINT", "localhost:9000"),
        access_key=os.getenv("MINIO_ACCESS_KEY", "minioadmin"),
        secret_key=os.getenv("MINIO_SECRET_KEY", "minioadmin"),
        secure=os.getenv("MINIO_SECURE", "false").lower() == "true"
    )

def init_postgres():
    """Initialize PostgreSQL connection for metadata"""
    database_url = os.getenv(
        "DATABASE_URL",
        "postgresql://postgres:password@localhost:5432/data_management"
    )
    engine = create_engine(database_url)
    SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)
    return engine, SessionLocal

def init_redis():
    """Initialize Redis for caching"""
    return redis.Redis(
        host=os.getenv("REDIS_HOST", "localhost"),
        port=int(os.getenv("REDIS_PORT", "6379")),
        password=os.getenv("REDIS_PASSWORD", ""),
        db=2,  # Use different DB than other services
        decode_responses=True
    )

# Initialize services
minio_client = init_minio()
postgres_engine, SessionLocal = init_postgres()
redis_client = init_redis()

# Pydantic models
class DatasetMetadata(BaseModel):
    name: str = Field(..., description="Dataset name")
    description: Optional[str] = Field(None, description="Dataset description")
    schema_: Optional[Dict[str, str]] = Field(None, alias="schema", description="Dataset schema")
    tags: List[str] = Field(default_factory=list, description="Dataset tags")
    owner: str = Field(..., description="Dataset owner")
    data_source: str = Field(..., description="Data source type")
    format: str = Field(..., description="Data format (parquet, csv, json, etc.)")
    partition_columns: List[str] = Field(default_factory=list, description="Partition columns")
    quality_rules: Dict[str, Any] = Field(default_factory=dict, description="Data quality rules")
    retention_policy: Optional[Dict[str, Any]] = Field(None, description="Data retention policy")
    access_level: str = Field(default="private", description="Access level (public, private, restricted)")
    
    @validator('format')
    def validate_format(cls, v):
        allowed_formats = ['parquet', 'csv', 'json', 'avro', 'orc', 'delta']
        if v.lower() not in allowed_formats:
            raise ValueError(f'Format must be one of {allowed_formats}')
        return v.lower()

class DataIngestionRequest(BaseModel):
    dataset_name: str = Field(..., description="Target dataset name")
    source_path: Optional[str] = Field(None, description="Source data path")
    source_type: str = Field(..., description="Source type (file, database, api, stream)")
    source_config: Dict[str, Any] = Field(..., description="Source configuration")
    transformation_config: Optional[Dict[str, Any]] = Field(None, description="Data transformation configuration")
    schedule: Optional[str] = Field(None, description="Ingestion schedule (cron format)")
    incremental: bool = Field(default=False, description="Incremental ingestion")

class DataQueryRequest(BaseModel):
    query: str = Field(..., description="SQL query")
    dataset: Optional[str] = Field(None, description="Target dataset")
    format: str = Field(default="json", description="Output format")
    limit: Optional[int] = Field(None, description="Result limit")
    cache_ttl: Optional[int] = Field(300, description="Cache TTL in seconds")

class DatasetResponse(BaseModel):
    id: str
    name: str
    description: Optional[str]
    schema_: Optional[Dict[str, str]] = Field(alias="schema")
    size_bytes: int
    row_count: int
    file_count: int
    created_at: datetime
    updated_at: datetime
    last_accessed: Optional[datetime]
    owner: str
    tags: List[str]
    format: str
    location: str
    status: str

class DataQualityReport(BaseModel):
    dataset_id: str
    total_rows: int
    null_counts: Dict[str, int]
    duplicate_rows: int
    quality_score: float
    issues: List[Dict[str, Any]]
    recommendations: List[str]
    generated_at: datetime

# Health check endpoint
@app.get("/health")
async def health_check():
    """Health check endpoint"""
    # Test connections
    minio_healthy = True
    postgres_healthy = True
    redis_healthy = True
    
    try:
        minio_client.list_buckets()
    except Exception:
        minio_healthy = False
    
    try:
        with postgres_engine.connect() as conn:
            conn.execute(text("SELECT 1"))
    except Exception:
        postgres_healthy = False
    
    try:
        redis_client.ping()
    except Exception:
        redis_healthy = False
    
    return {
        "status": "healthy" if all([minio_healthy, postgres_healthy, redis_healthy]) else "degraded",
        "service": "data-management-service",
        "timestamp": datetime.utcnow().isoformat(),
        "version": "1.0.0",
        "dependencies": {
            "minio": "healthy" if minio_healthy else "unhealthy",
            "postgres": "healthy" if postgres_healthy else "unhealthy",
            "redis": "healthy" if redis_healthy else "unhealthy"
        }
    }

# Dataset management endpoints
@app.get("/v1/datasets", response_model=List[DatasetResponse])
@auth.require_auth("dataset", "read")
async def list_datasets(
    owner: Optional[str] = None,
    tag: Optional[str] = None,
    format: Optional[str] = None,
    skip: int = 0,
    limit: int = 100
):
    """List all datasets with optional filtering"""
    try:
        # Build query
        query = "SELECT * FROM datasets WHERE 1=1"
        params = {}
        
        if owner:
            query += " AND owner = :owner"
            params["owner"] = owner
        
        if tag:
            query += " AND :tag = ANY(tags)"
            params["tag"] = tag
        
        if format:
            query += " AND format = :format"
            params["format"] = format
        
        query += " ORDER BY created_at DESC LIMIT :limit OFFSET :skip"
        params["limit"] = limit
        params["skip"] = skip
        
        with postgres_engine.connect() as conn:
            result = conn.execute(text(query), params)
            datasets = []
            
            for row in result:
                datasets.append(DatasetResponse(
                    id=row.id,
                    name=row.name,
                    description=row.description,
                    schema_=json.loads(row.schema_) if row.schema_ else None,
                    size_bytes=row.size_bytes,
                    row_count=row.row_count,
                    file_count=row.file_count,
                    created_at=row.created_at,
                    updated_at=row.updated_at,
                    last_accessed=row.last_accessed,
                    owner=row.owner,
                    tags=row.tags,
                    format=row.format,
                    location=row.location,
                    status=row.status
                ))
        
        return datasets
    except Exception as e:
        logger.error(f"Error listing datasets: {e}")
        raise HTTPException(status_code=500, detail="Failed to list datasets")

@app.post("/v1/datasets", response_model=DatasetResponse)
@auth.require_auth("dataset", "create")
async def create_dataset(dataset_metadata: DatasetMetadata):
    """Create a new dataset"""
    try:
        dataset_id = str(uuid.uuid4())
        bucket_name = f"dataset-{dataset_id}"
        
        # Create MinIO bucket
        if not minio_client.bucket_exists(bucket_name):
            minio_client.make_bucket(bucket_name)
        
        # Store metadata in PostgreSQL
        query = """
        INSERT INTO datasets (id, name, description, schema_, owner, data_source, format, 
                            partition_columns, quality_rules, retention_policy, access_level,
                            tags, location, status, created_at, updated_at)
        VALUES (:id, :name, :description, :schema_, :owner, :data_source, :format,
                :partition_columns, :quality_rules, :retention_policy, :access_level,
                :tags, :location, :status, :created_at, :updated_at)
        """
        
        now = datetime.utcnow()
        location = f"s3://{bucket_name}/"
        
        with postgres_engine.connect() as conn:
            conn.execute(text(query), {
                "id": dataset_id,
                "name": dataset_metadata.name,
                "description": dataset_metadata.description,
                "schema_": json.dumps(dataset_metadata.schema_) if dataset_metadata.schema_ else None,
                "owner": dataset_metadata.owner,
                "data_source": dataset_metadata.data_source,
                "format": dataset_metadata.format,
                "partition_columns": dataset_metadata.partition_columns,
                "quality_rules": json.dumps(dataset_metadata.quality_rules),
                "retention_policy": json.dumps(dataset_metadata.retention_policy) if dataset_metadata.retention_policy else None,
                "access_level": dataset_metadata.access_level,
                "tags": dataset_metadata.tags,
                "location": location,
                "status": "created",
                "created_at": now,
                "updated_at": now
            })
            conn.commit()
        
        return DatasetResponse(
            id=dataset_id,
            name=dataset_metadata.name,
            description=dataset_metadata.description,
            schema_=dataset_metadata.schema_,
            size_bytes=0,
            row_count=0,
            file_count=0,
            created_at=now,
            updated_at=now,
            last_accessed=None,
            owner=dataset_metadata.owner,
            tags=dataset_metadata.tags,
            format=dataset_metadata.format,
            location=location,
            status="created"
        )
    except Exception as e:
        logger.error(f"Error creating dataset: {e}")
        raise HTTPException(status_code=500, detail="Failed to create dataset")

@app.get("/v1/datasets/{dataset_id}", response_model=DatasetResponse)
@auth.require_auth("dataset", "read")
async def get_dataset(dataset_id: str):
    """Get dataset details"""
    try:
        query = "SELECT * FROM datasets WHERE id = :dataset_id"
        
        with postgres_engine.connect() as conn:
            result = conn.execute(text(query), {"dataset_id": dataset_id})
            row = result.fetchone()
            
            if not row:
                raise HTTPException(status_code=404, detail="Dataset not found")
            
            # Update last accessed
            update_query = "UPDATE datasets SET last_accessed = :now WHERE id = :dataset_id"
            conn.execute(text(update_query), {"now": datetime.utcnow(), "dataset_id": dataset_id})
            conn.commit()
            
            return DatasetResponse(
                id=row.id,
                name=row.name,
                description=row.description,
                schema_=json.loads(row.schema_) if row.schema_ else None,
                size_bytes=row.size_bytes,
                row_count=row.row_count,
                file_count=row.file_count,
                created_at=row.created_at,
                updated_at=row.updated_at,
                last_accessed=row.last_accessed,
                owner=row.owner,
                tags=row.tags,
                format=row.format,
                location=row.location,
                status=row.status
            )
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error getting dataset {dataset_id}: {e}")
        raise HTTPException(status_code=500, detail="Failed to get dataset")

@app.post("/v1/datasets/{dataset_id}/upload")
@auth.require_auth("dataset", "write")
async def upload_data(
    dataset_id: str,
    file: UploadFile = File(...),
    background_tasks: BackgroundTasks = None
):
    """Upload data to a dataset"""
    try:
        # Get dataset info
        query = "SELECT * FROM datasets WHERE id = :dataset_id"
        
        with postgres_engine.connect() as conn:
            result = conn.execute(text(query), {"dataset_id": dataset_id})
            dataset = result.fetchone()
            
            if not dataset:
                raise HTTPException(status_code=404, detail="Dataset not found")
        
        # Generate unique filename
        file_id = str(uuid.uuid4())
        file_extension = Path(file.filename).suffix
        object_name = f"data/{file_id}{file_extension}"
        bucket_name = f"dataset-{dataset_id}"
        
        # Upload to MinIO
        file_content = await file.read()
        minio_client.put_object(
            bucket_name,
            object_name,
            data=file_content,
            length=len(file_content),
            content_type=file.content_type
        )
        
        # Add background task for data processing
        if background_tasks:
            background_tasks.add_task(
                process_uploaded_data,
                dataset_id,
                bucket_name,
                object_name,
                file.filename
            )
        
        return {
            "message": "File uploaded successfully",
            "dataset_id": dataset_id,
            "file_id": file_id,
            "object_name": object_name,
            "size_bytes": len(file_content)
        }
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error uploading data to dataset {dataset_id}: {e}")
        raise HTTPException(status_code=500, detail="Failed to upload data")

@app.post("/v1/datasets/{dataset_id}/ingest")
@auth.require_auth("dataset", "write")
async def ingest_data(
    dataset_id: str,
    ingestion_request: DataIngestionRequest,
    background_tasks: BackgroundTasks
):
    """Ingest data from external sources"""
    try:
        # Validate dataset exists
        query = "SELECT * FROM datasets WHERE id = :dataset_id"
        
        with postgres_engine.connect() as conn:
            result = conn.execute(text(query), {"dataset_id": dataset_id})
            dataset = result.fetchone()
            
            if not dataset:
                raise HTTPException(status_code=404, detail="Dataset not found")
        
        # Create ingestion job
        job_id = str(uuid.uuid4())
        
        # Add background task for data ingestion
        background_tasks.add_task(
            perform_data_ingestion,
            job_id,
            dataset_id,
            ingestion_request
        )
        
        return {
            "message": "Data ingestion started",
            "job_id": job_id,
            "dataset_id": dataset_id,
            "status": "running"
        }
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error starting data ingestion for dataset {dataset_id}: {e}")
        raise HTTPException(status_code=500, detail="Failed to start data ingestion")

@app.post("/v1/query")
@auth.require_auth("dataset", "read")
async def query_data(query_request: DataQueryRequest):
    """Query data using SQL"""
    try:
        # Check cache first
        cache_key = f"query:{hash(query_request.query)}"
        cached_result = redis_client.get(cache_key)
        
        if cached_result:
            return json.loads(cached_result)
        
        # Execute query (simplified - in production, use a proper query engine like Trino/Presto)
        # This is a placeholder implementation
        result = {
            "query": query_request.query,
            "columns": ["col1", "col2", "col3"],
            "data": [
                ["value1", "value2", "value3"],
                ["value4", "value5", "value6"]
            ],
            "row_count": 2,
            "execution_time_ms": 150,
            "cached": False
        }
        
        # Cache result
        if query_request.cache_ttl and query_request.cache_ttl > 0:
            redis_client.setex(
                cache_key,
                query_request.cache_ttl,
                json.dumps(result)
            )
        
        return result
    except Exception as e:
        logger.error(f"Error executing query: {e}")
        raise HTTPException(status_code=500, detail="Failed to execute query")

@app.get("/v1/datasets/{dataset_id}/quality", response_model=DataQualityReport)
@auth.require_auth("dataset", "read")
async def get_data_quality_report(dataset_id: str):
    """Get data quality report for a dataset"""
    try:
        # This is a simplified implementation
        # In production, this would analyze the actual data
        report = DataQualityReport(
            dataset_id=dataset_id,
            total_rows=10000,
            null_counts={"column1": 5, "column2": 0, "column3": 12},
            duplicate_rows=3,
            quality_score=0.95,
            issues=[
                {"type": "null_values", "column": "column1", "count": 5},
                {"type": "duplicates", "count": 3}
            ],
            recommendations=[
                "Consider adding validation for column1 to reduce null values",
                "Implement deduplication process"
            ],
            generated_at=datetime.utcnow()
        )
        
        return report
    except Exception as e:
        logger.error(f"Error generating quality report for dataset {dataset_id}: {e}")
        raise HTTPException(status_code=500, detail="Failed to generate quality report")

@app.delete("/v1/datasets/{dataset_id}")
@auth.require_auth("dataset", "delete")
async def delete_dataset(dataset_id: str):
    """Delete a dataset"""
    try:
        # Get dataset info
        query = "SELECT * FROM datasets WHERE id = :dataset_id"
        
        with postgres_engine.connect() as conn:
            result = conn.execute(text(query), {"dataset_id": dataset_id})
            dataset = result.fetchone()
            
            if not dataset:
                raise HTTPException(status_code=404, detail="Dataset not found")
            
            # Delete from MinIO
            bucket_name = f"dataset-{dataset_id}"
            try:
                # List and delete all objects in bucket
                objects = minio_client.list_objects(bucket_name, recursive=True)
                for obj in objects:
                    minio_client.remove_object(bucket_name, obj.object_name)
                
                # Remove bucket
                minio_client.remove_bucket(bucket_name)
            except S3Error as e:
                logger.warning(f"Error deleting MinIO bucket {bucket_name}: {e}")
            
            # Delete from database
            delete_query = "DELETE FROM datasets WHERE id = :dataset_id"
            conn.execute(text(delete_query), {"dataset_id": dataset_id})
            conn.commit()
        
        return {"message": "Dataset deleted successfully"}
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error deleting dataset {dataset_id}: {e}")
        raise HTTPException(status_code=500, detail="Failed to delete dataset")

# Background tasks
async def process_uploaded_data(dataset_id: str, bucket_name: str, object_name: str, filename: str):
    """Background task to process uploaded data"""
    try:
        logger.info(f"Processing uploaded data for dataset {dataset_id}: {filename}")
        
        # Download file from MinIO
        response = minio_client.get_object(bucket_name, object_name)
        data = response.read()
        
        # Process based on file type
        if filename.endswith('.csv'):
            df = pd.read_csv(data)
        elif filename.endswith('.json'):
            df = pd.read_json(data)
        elif filename.endswith('.parquet'):
            df = pd.read_parquet(data)
        else:
            logger.warning(f"Unsupported file format: {filename}")
            return
        
        # Update dataset statistics
        row_count = len(df)
        size_bytes = len(data)
        
        query = """
        UPDATE datasets 
        SET row_count = row_count + :row_count,
            size_bytes = size_bytes + :size_bytes,
            file_count = file_count + 1,
            updated_at = :updated_at,
            status = 'active'
        WHERE id = :dataset_id
        """
        
        with postgres_engine.connect() as conn:
            conn.execute(text(query), {
                "row_count": row_count,
                "size_bytes": size_bytes,
                "updated_at": datetime.utcnow(),
                "dataset_id": dataset_id
            })
            conn.commit()
        
        logger.info(f"Processed {row_count} rows for dataset {dataset_id}")
    except Exception as e:
        logger.error(f"Error processing uploaded data: {e}")

async def perform_data_ingestion(job_id: str, dataset_id: str, ingestion_request: DataIngestionRequest):
    """Background task to perform data ingestion"""
    try:
        logger.info(f"Starting data ingestion job {job_id} for dataset {dataset_id}")
        
        # This is a simplified implementation
        # In production, this would handle various data sources
        
        if ingestion_request.source_type == "database":
            # Handle database ingestion
            pass
        elif ingestion_request.source_type == "api":
            # Handle API ingestion
            pass
        elif ingestion_request.source_type == "file":
            # Handle file ingestion
            pass
        
        # Simulate processing time
        await asyncio.sleep(10)
        
        logger.info(f"Completed data ingestion job {job_id}")
    except Exception as e:
        logger.error(f"Error in data ingestion job {job_id}: {e}")

if __name__ == "__main__":
    uvicorn.run(
        "main:app",
        host="0.0.0.0",
        port=int(os.getenv("PORT", 8080)),
        reload=os.getenv("ENVIRONMENT") == "development"
    )
