"""
Queue Service - Python
Message queue management and task processing for the 002AIC platform
Handles job queues, task scheduling, and distributed processing
"""

import asyncio
import json
import logging
import os
import uuid
from datetime import datetime, timedelta
from typing import Dict, List, Optional, Any, Union
from enum import Enum

import aioredis
import asyncpg
from fastapi import FastAPI, HTTPException, Depends, BackgroundTasks
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel, Field, validator
from prometheus_client import Counter, Histogram, Gauge, generate_latest
from starlette.responses import Response
import uvicorn
from celery import Celery
from kombu import Queue as KombuQueue
import pika

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Configuration
class Config:
    def __init__(self):
        self.port = int(os.getenv("PORT", "8080"))
        self.database_url = os.getenv("DATABASE_URL", "postgresql://postgres:password@localhost:5432/queue")
        self.redis_url = os.getenv("REDIS_URL", "redis://localhost:6379")
        self.rabbitmq_url = os.getenv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")
        self.celery_broker = os.getenv("CELERY_BROKER", "redis://localhost:6379/0")
        self.environment = os.getenv("ENVIRONMENT", "development")
        self.max_retries = int(os.getenv("MAX_RETRIES", "3"))
        self.default_timeout = int(os.getenv("DEFAULT_TIMEOUT", "300"))

config = Config()

# Enums
class QueueType(str, Enum):
    FIFO = "fifo"
    PRIORITY = "priority"
    DELAY = "delay"
    DEAD_LETTER = "dead_letter"

class JobStatus(str, Enum):
    PENDING = "pending"
    RUNNING = "running"
    COMPLETED = "completed"
    FAILED = "failed"
    CANCELLED = "cancelled"
    RETRYING = "retrying"

class JobPriority(str, Enum):
    LOW = "low"
    NORMAL = "normal"
    HIGH = "high"
    CRITICAL = "critical"

# Pydantic models
class QueueConfig(BaseModel):
    name: str = Field(..., min_length=1, max_length=255)
    type: QueueType = QueueType.FIFO
    max_size: Optional[int] = None
    ttl: Optional[int] = None  # seconds
    dead_letter_queue: Optional[str] = None
    retry_policy: Dict[str, Any] = Field(default_factory=dict)
    routing_key: Optional[str] = None
    durable: bool = True
    auto_delete: bool = False

class JobPayload(BaseModel):
    task_name: str = Field(..., min_length=1)
    args: List[Any] = Field(default_factory=list)
    kwargs: Dict[str, Any] = Field(default_factory=dict)
    priority: JobPriority = JobPriority.NORMAL
    delay: Optional[int] = None  # seconds
    timeout: Optional[int] = None
    max_retries: Optional[int] = None
    metadata: Dict[str, Any] = Field(default_factory=dict)

class CreateJobRequest(BaseModel):
    queue_name: str
    payload: JobPayload
    scheduled_at: Optional[datetime] = None

class JobResponse(BaseModel):
    id: str
    queue_name: str
    task_name: str
    status: JobStatus
    priority: JobPriority
    created_at: datetime
    started_at: Optional[datetime] = None
    completed_at: Optional[datetime] = None
    result: Optional[Any] = None
    error: Optional[str] = None
    retry_count: int
    metadata: Dict[str, Any]

class QueueStats(BaseModel):
    name: str
    type: QueueType
    size: int
    pending_jobs: int
    running_jobs: int
    completed_jobs: int
    failed_jobs: int
    throughput: float  # jobs per second
    avg_processing_time: float  # seconds

# Celery configuration
celery_app = Celery(
    'queue-service',
    broker=config.celery_broker,
    backend=config.redis_url,
    include=['tasks']
)

celery_app.conf.update(
    task_serializer='json',
    accept_content=['json'],
    result_serializer='json',
    timezone='UTC',
    enable_utc=True,
    task_routes={
        'tasks.*': {'queue': 'default'},
    },
    task_default_queue='default',
    task_queues=(
        KombuQueue('default', routing_key='default'),
        KombuQueue('high_priority', routing_key='high_priority'),
        KombuQueue('low_priority', routing_key='low_priority'),
        KombuQueue('critical', routing_key='critical'),
    ),
)

# Prometheus metrics
jobs_created = Counter(
    'queue_jobs_created_total',
    'Total jobs created',
    ['queue_name', 'task_name', 'priority']
)

jobs_processed = Counter(
    'queue_jobs_processed_total',
    'Total jobs processed',
    ['queue_name', 'status']
)

job_processing_duration = Histogram(
    'queue_job_processing_duration_seconds',
    'Job processing duration',
    ['queue_name', 'task_name']
)

queue_size = Gauge(
    'queue_size_total',
    'Current queue size',
    ['queue_name', 'status']
)

active_workers = Gauge(
    'queue_active_workers',
    'Number of active workers',
    ['queue_name']
)

# Global variables
app = FastAPI(title="Queue Service", version="1.0.0")
db_pool = None
redis_client = None
rabbitmq_connection = None

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
        "service": "queue-service",
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
        # Check RabbitMQ
        if rabbitmq_connection and not rabbitmq_connection.is_closed:
            status["rabbitmq"] = "connected"
        else:
            status["rabbitmq"] = "disconnected"
    except Exception as e:
        status["rabbitmq"] = f"error: {str(e)}"
    
    try:
        # Check Celery
        celery_status = celery_app.control.inspect().stats()
        if celery_status:
            status["celery"] = "connected"
            status["celery_workers"] = len(celery_status)
        else:
            status["celery"] = "no_workers"
    except Exception as e:
        status["celery"] = f"error: {str(e)}"
    
    if status["status"] == "unhealthy":
        raise HTTPException(status_code=503, detail=status)
    
    return status

# Metrics endpoint
@app.get("/metrics")
async def metrics():
    return Response(generate_latest(), media_type="text/plain")

# Queue management endpoints
@app.post("/v1/queues")
async def create_queue(
    config_data: QueueConfig,
    db = Depends(get_db)
):
    """Create a new queue"""
    try:
        # Check if queue already exists
        async with db.acquire() as conn:
            existing = await conn.fetchval(
                "SELECT EXISTS(SELECT 1 FROM queues WHERE name = $1)",
                config_data.name
            )
            
            if existing:
                raise HTTPException(
                    status_code=409,
                    detail=f"Queue '{config_data.name}' already exists"
                )
        
        # Create queue in database
        queue_id = await create_queue_in_db(config_data)
        
        # Create queue in RabbitMQ
        await create_rabbitmq_queue(config_data)
        
        return {
            "queue_id": queue_id,
            "queue_name": config_data.name,
            "message": "Queue created successfully"
        }
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error creating queue: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to create queue: {str(e)}")

@app.get("/v1/queues")
async def list_queues(
    type_filter: Optional[QueueType] = None,
    limit: int = 50,
    offset: int = 0,
    db = Depends(get_db)
):
    """List all queues"""
    try:
        queues = await list_queues_from_db(type_filter, limit, offset)
        
        # Get queue statistics
        for queue in queues:
            stats = await get_queue_statistics(queue['name'])
            queue.update(stats)
        
        return {
            "queues": queues,
            "total": len(queues),
            "limit": limit,
            "offset": offset
        }
        
    except Exception as e:
        logger.error(f"Error listing queues: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to list queues: {str(e)}")

@app.get("/v1/queues/{queue_name}")
async def get_queue(queue_name: str, db = Depends(get_db)):
    """Get queue details"""
    try:
        queue = await get_queue_from_db(queue_name)
        if not queue:
            raise HTTPException(status_code=404, detail="Queue not found")
        
        # Get queue statistics
        stats = await get_queue_statistics(queue_name)
        queue.update(stats)
        
        return queue
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error getting queue: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to get queue: {str(e)}")

@app.delete("/v1/queues/{queue_name}")
async def delete_queue(
    queue_name: str,
    force: bool = False,
    db = Depends(get_db)
):
    """Delete a queue"""
    try:
        # Check if queue exists
        queue = await get_queue_from_db(queue_name)
        if not queue:
            raise HTTPException(status_code=404, detail="Queue not found")
        
        # Check if queue has pending jobs
        if not force:
            pending_count = await count_pending_jobs(queue_name)
            if pending_count > 0:
                raise HTTPException(
                    status_code=400,
                    detail=f"Queue has {pending_count} pending jobs. Use force=true to delete anyway."
                )
        
        # Delete queue from database
        await delete_queue_from_db(queue_name)
        
        # Delete queue from RabbitMQ
        await delete_rabbitmq_queue(queue_name)
        
        return {"message": "Queue deleted successfully"}
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error deleting queue: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to delete queue: {str(e)}")

# Job management endpoints
@app.post("/v1/jobs", response_model=JobResponse)
async def create_job(
    request: CreateJobRequest,
    background_tasks: BackgroundTasks,
    db = Depends(get_db)
):
    """Create a new job"""
    try:
        # Validate queue exists
        queue = await get_queue_from_db(request.queue_name)
        if not queue:
            raise HTTPException(status_code=404, detail="Queue not found")
        
        # Create job
        job_id = str(uuid.uuid4())
        job_data = {
            "id": job_id,
            "queue_name": request.queue_name,
            "task_name": request.payload.task_name,
            "args": request.payload.args,
            "kwargs": request.payload.kwargs,
            "priority": request.payload.priority.value,
            "status": JobStatus.PENDING.value,
            "delay": request.payload.delay,
            "timeout": request.payload.timeout or config.default_timeout,
            "max_retries": request.payload.max_retries or config.max_retries,
            "retry_count": 0,
            "metadata": request.payload.metadata,
            "scheduled_at": request.scheduled_at,
            "created_at": datetime.utcnow(),
        }
        
        # Store job in database
        await store_job_in_db(job_data)
        
        # Queue job for processing
        if request.scheduled_at and request.scheduled_at > datetime.utcnow():
            # Schedule for later
            background_tasks.add_task(schedule_job, job_id, request.scheduled_at)
        else:
            # Queue immediately
            await queue_job_for_processing(job_data)
        
        # Update metrics
        jobs_created.labels(
            queue_name=request.queue_name,
            task_name=request.payload.task_name,
            priority=request.payload.priority.value
        ).inc()
        
        return JobResponse(
            id=job_id,
            queue_name=request.queue_name,
            task_name=request.payload.task_name,
            status=JobStatus.PENDING,
            priority=request.payload.priority,
            created_at=job_data["created_at"],
            retry_count=0,
            metadata=request.payload.metadata
        )
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error creating job: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to create job: {str(e)}")

@app.get("/v1/jobs/{job_id}", response_model=JobResponse)
async def get_job(job_id: str, db = Depends(get_db)):
    """Get job details"""
    try:
        job = await get_job_from_db(job_id)
        if not job:
            raise HTTPException(status_code=404, detail="Job not found")
        
        return JobResponse(**job)
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error getting job: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to get job: {str(e)}")

@app.get("/v1/jobs")
async def list_jobs(
    queue_name: Optional[str] = None,
    status: Optional[JobStatus] = None,
    task_name: Optional[str] = None,
    limit: int = 50,
    offset: int = 0,
    db = Depends(get_db)
):
    """List jobs with filtering"""
    try:
        jobs = await list_jobs_from_db(queue_name, status, task_name, limit, offset)
        
        return {
            "jobs": [JobResponse(**job) for job in jobs],
            "total": len(jobs),
            "limit": limit,
            "offset": offset
        }
        
    except Exception as e:
        logger.error(f"Error listing jobs: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to list jobs: {str(e)}")

@app.post("/v1/jobs/{job_id}/cancel")
async def cancel_job(job_id: str, db = Depends(get_db)):
    """Cancel a job"""
    try:
        job = await get_job_from_db(job_id)
        if not job:
            raise HTTPException(status_code=404, detail="Job not found")
        
        if job["status"] in [JobStatus.COMPLETED.value, JobStatus.FAILED.value, JobStatus.CANCELLED.value]:
            raise HTTPException(status_code=400, detail="Job cannot be cancelled")
        
        # Update job status
        await update_job_status(job_id, JobStatus.CANCELLED.value)
        
        # Cancel in Celery if running
        if job["status"] == JobStatus.RUNNING.value:
            celery_app.control.revoke(job_id, terminate=True)
        
        return {"message": "Job cancelled successfully"}
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error cancelling job: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to cancel job: {str(e)}")

@app.post("/v1/jobs/{job_id}/retry")
async def retry_job(job_id: str, db = Depends(get_db)):
    """Retry a failed job"""
    try:
        job = await get_job_from_db(job_id)
        if not job:
            raise HTTPException(status_code=404, detail="Job not found")
        
        if job["status"] != JobStatus.FAILED.value:
            raise HTTPException(status_code=400, detail="Only failed jobs can be retried")
        
        if job["retry_count"] >= job["max_retries"]:
            raise HTTPException(status_code=400, detail="Maximum retries exceeded")
        
        # Reset job for retry
        await update_job_for_retry(job_id)
        
        # Queue job for processing
        await queue_job_for_processing(job)
        
        return {"message": "Job queued for retry"}
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error retrying job: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to retry job: {str(e)}")

# Queue statistics endpoints
@app.get("/v1/queues/{queue_name}/stats", response_model=QueueStats)
async def get_queue_stats(queue_name: str, db = Depends(get_db)):
    """Get detailed queue statistics"""
    try:
        queue = await get_queue_from_db(queue_name)
        if not queue:
            raise HTTPException(status_code=404, detail="Queue not found")
        
        stats = await get_detailed_queue_statistics(queue_name)
        
        return QueueStats(
            name=queue_name,
            type=QueueType(queue["type"]),
            **stats
        )
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error getting queue stats: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to get queue stats: {str(e)}")

# Startup and shutdown events
@app.on_event("startup")
async def startup_event():
    global db_pool, redis_client, rabbitmq_connection
    
    try:
        # Initialize database connection pool
        db_pool = await asyncpg.create_pool(config.database_url)
        logger.info("Database connection pool created")
        
        # Initialize Redis client
        redis_client = aioredis.from_url(config.redis_url)
        logger.info("Redis client initialized")
        
        # Initialize RabbitMQ connection
        try:
            connection_params = pika.URLParameters(config.rabbitmq_url)
            rabbitmq_connection = pika.BlockingConnection(connection_params)
            logger.info("RabbitMQ connection established")
        except Exception as e:
            logger.warning(f"RabbitMQ connection failed: {str(e)}")
        
        # Create database tables
        await create_database_tables()
        
        # Start background tasks
        asyncio.create_task(update_metrics_periodically())
        asyncio.create_task(process_scheduled_jobs())
        
        logger.info("Queue service started successfully")
        
    except Exception as e:
        logger.error(f"Failed to start queue service: {str(e)}")
        raise

@app.on_event("shutdown")
async def shutdown_event():
    global db_pool, redis_client, rabbitmq_connection
    
    try:
        if db_pool:
            await db_pool.close()
        if redis_client:
            await redis_client.close()
        if rabbitmq_connection and not rabbitmq_connection.is_closed:
            rabbitmq_connection.close()
        
        logger.info("Queue service shut down successfully")
        
    except Exception as e:
        logger.error(f"Error during shutdown: {str(e)}")

# Helper functions (implementations would be in separate files due to length)
async def create_database_tables():
    """Create database tables for queues and jobs"""
    create_tables_sql = """
    CREATE TABLE IF NOT EXISTS queues (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        name VARCHAR(255) UNIQUE NOT NULL,
        type VARCHAR(50) NOT NULL,
        config JSONB NOT NULL DEFAULT '{}',
        created_at TIMESTAMP DEFAULT NOW(),
        updated_at TIMESTAMP DEFAULT NOW()
    );
    
    CREATE TABLE IF NOT EXISTS jobs (
        id UUID PRIMARY KEY,
        queue_name VARCHAR(255) NOT NULL,
        task_name VARCHAR(255) NOT NULL,
        args JSONB DEFAULT '[]',
        kwargs JSONB DEFAULT '{}',
        priority VARCHAR(20) NOT NULL DEFAULT 'normal',
        status VARCHAR(20) NOT NULL DEFAULT 'pending',
        result JSONB,
        error TEXT,
        timeout INTEGER,
        max_retries INTEGER DEFAULT 3,
        retry_count INTEGER DEFAULT 0,
        metadata JSONB DEFAULT '{}',
        scheduled_at TIMESTAMP,
        started_at TIMESTAMP,
        completed_at TIMESTAMP,
        created_at TIMESTAMP DEFAULT NOW(),
        updated_at TIMESTAMP DEFAULT NOW()
    );
    
    CREATE INDEX IF NOT EXISTS idx_queues_name ON queues(name);
    CREATE INDEX IF NOT EXISTS idx_jobs_queue_name ON jobs(queue_name);
    CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
    CREATE INDEX IF NOT EXISTS idx_jobs_priority ON jobs(priority);
    CREATE INDEX IF NOT EXISTS idx_jobs_scheduled_at ON jobs(scheduled_at);
    """
    
    try:
        async with db_pool.acquire() as conn:
            await conn.execute(create_tables_sql)
        logger.info("Database tables created successfully")
    except Exception as e:
        logger.error(f"Error creating database tables: {str(e)}")

# Additional helper functions would be implemented here...

if __name__ == "__main__":
    uvicorn.run(
        "main:app",
        host="0.0.0.0",
        port=config.port,
        log_level="info",
        reload=config.environment == "development"
    )
