"""
Workflow Service - Python
Business process automation and workflow orchestration for the 002AIC platform
Handles workflow definition, execution, scheduling, and monitoring
"""

import asyncio
import json
import logging
import os
import uuid
from datetime import datetime, timedelta
from enum import Enum
from typing import Dict, List, Optional, Any, Union
from dataclasses import dataclass, asdict

import aioredis
import asyncpg
from fastapi import FastAPI, HTTPException, Depends, BackgroundTasks, Query
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel, Field, validator
from prometheus_client import Counter, Histogram, Gauge, generate_latest
from starlette.responses import Response
import uvicorn
import yaml
from celery import Celery
from kombu import Queue

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Configuration
class Config:
    def __init__(self):
        self.port = int(os.getenv("PORT", "8080"))
        self.database_url = os.getenv("DATABASE_URL", "postgresql://postgres:password@localhost:5432/workflow")
        self.redis_url = os.getenv("REDIS_URL", "redis://localhost:6379")
        self.celery_broker = os.getenv("CELERY_BROKER", "redis://localhost:6379/0")
        self.celery_backend = os.getenv("CELERY_BACKEND", "redis://localhost:6379/0")
        self.environment = os.getenv("ENVIRONMENT", "development")
        self.max_workflow_steps = int(os.getenv("MAX_WORKFLOW_STEPS", "100"))
        self.default_timeout = int(os.getenv("DEFAULT_TIMEOUT", "3600"))

config = Config()

# Celery configuration
celery_app = Celery(
    'workflow-service',
    broker=config.celery_broker,
    backend=config.celery_backend,
    include=['workflow_tasks']
)

celery_app.conf.update(
    task_serializer='json',
    accept_content=['json'],
    result_serializer='json',
    timezone='UTC',
    enable_utc=True,
    task_routes={
        'workflow_tasks.*': {'queue': 'workflow'},
    },
    task_default_queue='workflow',
    task_queues=(
        Queue('workflow', routing_key='workflow'),
        Queue('high_priority', routing_key='high_priority'),
        Queue('low_priority', routing_key='low_priority'),
    ),
)

# Enums
class WorkflowStatus(str, Enum):
    DRAFT = "draft"
    ACTIVE = "active"
    PAUSED = "paused"
    COMPLETED = "completed"
    FAILED = "failed"
    CANCELLED = "cancelled"

class ExecutionStatus(str, Enum):
    PENDING = "pending"
    RUNNING = "running"
    COMPLETED = "completed"
    FAILED = "failed"
    CANCELLED = "cancelled"
    RETRYING = "retrying"

class StepType(str, Enum):
    HTTP_REQUEST = "http_request"
    DATABASE_QUERY = "database_query"
    EMAIL_NOTIFICATION = "email_notification"
    WEBHOOK = "webhook"
    SCRIPT_EXECUTION = "script_execution"
    CONDITION = "condition"
    LOOP = "loop"
    PARALLEL = "parallel"
    DELAY = "delay"
    APPROVAL = "approval"

class TriggerType(str, Enum):
    MANUAL = "manual"
    SCHEDULED = "scheduled"
    EVENT = "event"
    WEBHOOK = "webhook"
    API = "api"

# Pydantic models
class WorkflowStep(BaseModel):
    id: str = Field(default_factory=lambda: str(uuid.uuid4()))
    name: str
    type: StepType
    config: Dict[str, Any] = Field(default_factory=dict)
    inputs: Dict[str, Any] = Field(default_factory=dict)
    outputs: Dict[str, str] = Field(default_factory=dict)
    conditions: Optional[Dict[str, Any]] = None
    retry_config: Optional[Dict[str, Any]] = None
    timeout: Optional[int] = None
    depends_on: List[str] = Field(default_factory=list)
    position: Optional[Dict[str, float]] = None

class WorkflowTrigger(BaseModel):
    type: TriggerType
    config: Dict[str, Any] = Field(default_factory=dict)
    schedule: Optional[str] = None  # Cron expression
    event_filters: Optional[Dict[str, Any]] = None

class CreateWorkflowRequest(BaseModel):
    name: str = Field(..., min_length=1, max_length=255)
    description: Optional[str] = None
    steps: List[WorkflowStep]
    triggers: List[WorkflowTrigger] = Field(default_factory=list)
    variables: Dict[str, Any] = Field(default_factory=dict)
    tags: List[str] = Field(default_factory=list)
    timeout: Optional[int] = None
    retry_policy: Optional[Dict[str, Any]] = None
    
    @validator('steps')
    def validate_steps(cls, v):
        if len(v) > config.max_workflow_steps:
            raise ValueError(f'Too many steps. Maximum allowed: {config.max_workflow_steps}')
        
        step_ids = [step.id for step in v]
        if len(step_ids) != len(set(step_ids)):
            raise ValueError('Step IDs must be unique')
        
        # Validate dependencies
        for step in v:
            for dep in step.depends_on:
                if dep not in step_ids:
                    raise ValueError(f'Step {step.id} depends on non-existent step {dep}')
        
        return v

class ExecuteWorkflowRequest(BaseModel):
    workflow_id: str
    inputs: Dict[str, Any] = Field(default_factory=dict)
    priority: str = Field(default="normal", regex="^(low|normal|high)$")
    scheduled_at: Optional[datetime] = None
    context: Dict[str, Any] = Field(default_factory=dict)

class WorkflowResponse(BaseModel):
    id: str
    name: str
    description: Optional[str]
    status: WorkflowStatus
    version: int
    steps_count: int
    executions_count: int
    success_rate: float
    avg_duration_seconds: Optional[float]
    created_by: str
    created_at: datetime
    updated_at: datetime
    tags: List[str]

class ExecutionResponse(BaseModel):
    id: str
    workflow_id: str
    workflow_name: str
    status: ExecutionStatus
    inputs: Dict[str, Any]
    outputs: Dict[str, Any]
    context: Dict[str, Any]
    started_at: datetime
    completed_at: Optional[datetime]
    duration_seconds: Optional[float]
    error_message: Optional[str]
    step_results: List[Dict[str, Any]]

# Prometheus metrics
workflows_total = Gauge('workflows_total', 'Total number of workflows', ['status'])
executions_total = Counter('workflow_executions_total', 'Total workflow executions', ['workflow_id', 'status'])
execution_duration = Histogram('workflow_execution_duration_seconds', 'Workflow execution duration', ['workflow_id'])
active_executions = Gauge('workflow_active_executions', 'Currently active workflow executions')
step_executions_total = Counter('workflow_step_executions_total', 'Total step executions', ['step_type', 'status'])

# Global variables
app = FastAPI(title="Workflow Service", version="1.0.0")
db_pool = None
redis_client = None

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
        "service": "workflow-service",
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
        # Check Celery
        celery_status = celery_app.control.inspect().stats()
        if celery_status:
            status["celery"] = "connected"
        else:
            status["status"] = "unhealthy"
            status["celery"] = "no workers available"
    except Exception as e:
        status["status"] = "unhealthy"
        status["celery"] = f"disconnected: {str(e)}"
    
    if status["status"] == "unhealthy":
        raise HTTPException(status_code=503, detail=status)
    
    return status

# Metrics endpoint
@app.get("/metrics")
async def metrics():
    return Response(generate_latest(), media_type="text/plain")

# Workflow management endpoints
@app.post("/v1/workflows", response_model=WorkflowResponse)
async def create_workflow(
    request: CreateWorkflowRequest,
    created_by: str = Query(..., description="User ID creating the workflow"),
    db = Depends(get_db)
):
    """Create a new workflow definition"""
    try:
        workflow_id = str(uuid.uuid4())
        
        # Validate workflow definition
        await validate_workflow_definition(request)
        
        # Store workflow in database
        async with db.acquire() as conn:
            await conn.execute("""
                INSERT INTO workflows (
                    id, name, description, definition, status, version, 
                    created_by, created_at, updated_at, tags
                ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
            """, workflow_id, request.name, request.description, 
                json.dumps(request.dict()), WorkflowStatus.DRAFT.value, 1,
                created_by, datetime.utcnow(), datetime.utcnow(), request.tags)
        
        # Update metrics
        workflows_total.labels(status=WorkflowStatus.DRAFT.value).inc()
        
        # Cache workflow definition
        await cache_workflow_definition(workflow_id, request)
        
        logger.info(f"Created workflow: {workflow_id}")
        
        return WorkflowResponse(
            id=workflow_id,
            name=request.name,
            description=request.description,
            status=WorkflowStatus.DRAFT,
            version=1,
            steps_count=len(request.steps),
            executions_count=0,
            success_rate=0.0,
            avg_duration_seconds=None,
            created_by=created_by,
            created_at=datetime.utcnow(),
            updated_at=datetime.utcnow(),
            tags=request.tags
        )
        
    except Exception as e:
        logger.error(f"Error creating workflow: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to create workflow: {str(e)}")

@app.get("/v1/workflows/{workflow_id}", response_model=WorkflowResponse)
async def get_workflow(workflow_id: str, db = Depends(get_db)):
    """Get workflow by ID"""
    try:
        async with db.acquire() as conn:
            row = await conn.fetchrow("""
                SELECT w.*, 
                       COUNT(e.id) as executions_count,
                       AVG(CASE WHEN e.status = 'completed' THEN 1.0 ELSE 0.0 END) as success_rate,
                       AVG(EXTRACT(EPOCH FROM (e.completed_at - e.started_at))) as avg_duration
                FROM workflows w
                LEFT JOIN workflow_executions e ON w.id = e.workflow_id
                WHERE w.id = $1
                GROUP BY w.id
            """, workflow_id)
            
            if not row:
                raise HTTPException(status_code=404, detail="Workflow not found")
            
            definition = json.loads(row['definition'])
            
            return WorkflowResponse(
                id=row['id'],
                name=row['name'],
                description=row['description'],
                status=WorkflowStatus(row['status']),
                version=row['version'],
                steps_count=len(definition.get('steps', [])),
                executions_count=row['executions_count'] or 0,
                success_rate=float(row['success_rate'] or 0.0),
                avg_duration_seconds=float(row['avg_duration']) if row['avg_duration'] else None,
                created_by=row['created_by'],
                created_at=row['created_at'],
                updated_at=row['updated_at'],
                tags=row['tags'] or []
            )
            
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error getting workflow {workflow_id}: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to get workflow: {str(e)}")

@app.get("/v1/workflows")
async def list_workflows(
    status: Optional[WorkflowStatus] = None,
    created_by: Optional[str] = None,
    tag: Optional[str] = None,
    limit: int = Query(default=50, ge=1, le=100),
    offset: int = Query(default=0, ge=0),
    db = Depends(get_db)
):
    """List workflows with filtering and pagination"""
    try:
        conditions = []
        params = []
        param_count = 0
        
        if status:
            param_count += 1
            conditions.append(f"status = ${param_count}")
            params.append(status.value)
        
        if created_by:
            param_count += 1
            conditions.append(f"created_by = ${param_count}")
            params.append(created_by)
        
        if tag:
            param_count += 1
            conditions.append(f"${param_count} = ANY(tags)")
            params.append(tag)
        
        where_clause = "WHERE " + " AND ".join(conditions) if conditions else ""
        
        async with db.acquire() as conn:
            # Get total count
            count_query = f"SELECT COUNT(*) FROM workflows {where_clause}"
            total = await conn.fetchval(count_query, *params)
            
            # Get workflows
            param_count += 1
            limit_param = param_count
            param_count += 1
            offset_param = param_count
            
            query = f"""
                SELECT w.*, 
                       COUNT(e.id) as executions_count,
                       AVG(CASE WHEN e.status = 'completed' THEN 1.0 ELSE 0.0 END) as success_rate
                FROM workflows w
                LEFT JOIN workflow_executions e ON w.id = e.workflow_id
                {where_clause}
                GROUP BY w.id
                ORDER BY w.created_at DESC
                LIMIT ${limit_param} OFFSET ${offset_param}
            """
            
            params.extend([limit, offset])
            rows = await conn.fetch(query, *params)
            
            workflows = []
            for row in rows:
                definition = json.loads(row['definition'])
                workflows.append(WorkflowResponse(
                    id=row['id'],
                    name=row['name'],
                    description=row['description'],
                    status=WorkflowStatus(row['status']),
                    version=row['version'],
                    steps_count=len(definition.get('steps', [])),
                    executions_count=row['executions_count'] or 0,
                    success_rate=float(row['success_rate'] or 0.0),
                    avg_duration_seconds=None,
                    created_by=row['created_by'],
                    created_at=row['created_at'],
                    updated_at=row['updated_at'],
                    tags=row['tags'] or []
                ))
            
            return {
                "workflows": workflows,
                "total": total,
                "limit": limit,
                "offset": offset
            }
            
    except Exception as e:
        logger.error(f"Error listing workflows: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to list workflows: {str(e)}")

@app.post("/v1/workflows/{workflow_id}/execute", response_model=ExecutionResponse)
async def execute_workflow(
    workflow_id: str,
    request: ExecuteWorkflowRequest,
    background_tasks: BackgroundTasks,
    db = Depends(get_db)
):
    """Execute a workflow"""
    try:
        # Get workflow definition
        async with db.acquire() as conn:
            row = await conn.fetchrow("""
                SELECT * FROM workflows WHERE id = $1 AND status = $2
            """, workflow_id, WorkflowStatus.ACTIVE.value)
            
            if not row:
                raise HTTPException(status_code=404, detail="Active workflow not found")
        
        execution_id = str(uuid.uuid4())
        
        # Create execution record
        async with db.acquire() as conn:
            await conn.execute("""
                INSERT INTO workflow_executions (
                    id, workflow_id, status, inputs, context, started_at, created_at
                ) VALUES ($1, $2, $3, $4, $5, $6, $7)
            """, execution_id, workflow_id, ExecutionStatus.PENDING.value,
                json.dumps(request.inputs), json.dumps(request.context),
                datetime.utcnow(), datetime.utcnow())
        
        # Schedule execution
        if request.scheduled_at and request.scheduled_at > datetime.utcnow():
            # Schedule for later
            background_tasks.add_task(
                schedule_workflow_execution, 
                execution_id, 
                request.scheduled_at
            )
        else:
            # Execute immediately
            background_tasks.add_task(start_workflow_execution, execution_id)
        
        # Update metrics
        executions_total.labels(workflow_id=workflow_id, status='started').inc()
        active_executions.inc()
        
        logger.info(f"Started workflow execution: {execution_id}")
        
        return ExecutionResponse(
            id=execution_id,
            workflow_id=workflow_id,
            workflow_name=row['name'],
            status=ExecutionStatus.PENDING,
            inputs=request.inputs,
            outputs={},
            context=request.context,
            started_at=datetime.utcnow(),
            completed_at=None,
            duration_seconds=None,
            error_message=None,
            step_results=[]
        )
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error executing workflow {workflow_id}: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to execute workflow: {str(e)}")

# Startup and shutdown events
@app.on_event("startup")
async def startup_event():
    global db_pool, redis_client
    
    try:
        # Initialize database connection pool
        db_pool = await asyncpg.create_pool(config.database_url)
        logger.info("Database connection pool created")
        
        # Initialize Redis client
        redis_client = aioredis.from_url(config.redis_url)
        logger.info("Redis client initialized")
        
        # Create database tables
        await create_database_tables()
        
        # Start background tasks
        asyncio.create_task(update_metrics_periodically())
        asyncio.create_task(cleanup_completed_executions())
        
        logger.info("Workflow service started successfully")
        
    except Exception as e:
        logger.error(f"Failed to start workflow service: {str(e)}")
        raise

@app.on_event("shutdown")
async def shutdown_event():
    global db_pool, redis_client
    
    try:
        if db_pool:
            await db_pool.close()
        if redis_client:
            await redis_client.close()
        
        logger.info("Workflow service shut down successfully")
        
    except Exception as e:
        logger.error(f"Error during shutdown: {str(e)}")

# Helper functions
async def validate_workflow_definition(workflow: CreateWorkflowRequest):
    """Validate workflow definition"""
    # Check for circular dependencies
    def has_circular_dependency(steps):
        visited = set()
        rec_stack = set()
        
        def dfs(step_id):
            if step_id in rec_stack:
                return True
            if step_id in visited:
                return False
            
            visited.add(step_id)
            rec_stack.add(step_id)
            
            step = next((s for s in steps if s.id == step_id), None)
            if step:
                for dep in step.depends_on:
                    if dfs(dep):
                        return True
            
            rec_stack.remove(step_id)
            return False
        
        for step in steps:
            if step.id not in visited:
                if dfs(step.id):
                    return True
        return False
    
    if has_circular_dependency(workflow.steps):
        raise ValueError("Circular dependency detected in workflow steps")

async def cache_workflow_definition(workflow_id: str, definition: CreateWorkflowRequest):
    """Cache workflow definition in Redis"""
    try:
        key = f"workflow_def:{workflow_id}"
        await redis_client.setex(
            key, 
            3600,  # 1 hour TTL
            json.dumps(definition.dict(), default=str)
        )
    except Exception as e:
        logger.error(f"Error caching workflow definition: {str(e)}")

async def create_database_tables():
    """Create database tables"""
    create_tables_sql = """
    CREATE TABLE IF NOT EXISTS workflows (
        id UUID PRIMARY KEY,
        name VARCHAR(255) NOT NULL,
        description TEXT,
        definition JSONB NOT NULL,
        status VARCHAR(50) NOT NULL,
        version INTEGER NOT NULL DEFAULT 1,
        created_by VARCHAR(255) NOT NULL,
        created_at TIMESTAMP NOT NULL DEFAULT NOW(),
        updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
        tags TEXT[] DEFAULT '{}'
    );
    
    CREATE TABLE IF NOT EXISTS workflow_executions (
        id UUID PRIMARY KEY,
        workflow_id UUID NOT NULL REFERENCES workflows(id),
        status VARCHAR(50) NOT NULL,
        inputs JSONB DEFAULT '{}',
        outputs JSONB DEFAULT '{}',
        context JSONB DEFAULT '{}',
        started_at TIMESTAMP NOT NULL,
        completed_at TIMESTAMP,
        error_message TEXT,
        created_at TIMESTAMP NOT NULL DEFAULT NOW()
    );
    
    CREATE TABLE IF NOT EXISTS workflow_step_executions (
        id UUID PRIMARY KEY,
        execution_id UUID NOT NULL REFERENCES workflow_executions(id),
        step_id VARCHAR(255) NOT NULL,
        step_name VARCHAR(255) NOT NULL,
        step_type VARCHAR(50) NOT NULL,
        status VARCHAR(50) NOT NULL,
        inputs JSONB DEFAULT '{}',
        outputs JSONB DEFAULT '{}',
        started_at TIMESTAMP NOT NULL,
        completed_at TIMESTAMP,
        error_message TEXT,
        retry_count INTEGER DEFAULT 0,
        created_at TIMESTAMP NOT NULL DEFAULT NOW()
    );
    
    CREATE INDEX IF NOT EXISTS idx_workflows_status ON workflows(status);
    CREATE INDEX IF NOT EXISTS idx_workflows_created_by ON workflows(created_by);
    CREATE INDEX IF NOT EXISTS idx_workflow_executions_workflow_id ON workflow_executions(workflow_id);
    CREATE INDEX IF NOT EXISTS idx_workflow_executions_status ON workflow_executions(status);
    CREATE INDEX IF NOT EXISTS idx_step_executions_execution_id ON workflow_step_executions(execution_id);
    """
    
    try:
        async with db_pool.acquire() as conn:
            await conn.execute(create_tables_sql)
        logger.info("Database tables created successfully")
    except Exception as e:
        logger.error(f"Error creating database tables: {str(e)}")

async def start_workflow_execution(execution_id: str):
    """Start workflow execution"""
    try:
        # This would typically trigger a Celery task
        from workflow_tasks import execute_workflow_task
        execute_workflow_task.delay(execution_id)
        
    except Exception as e:
        logger.error(f"Error starting workflow execution {execution_id}: {str(e)}")

async def schedule_workflow_execution(execution_id: str, scheduled_at: datetime):
    """Schedule workflow execution"""
    try:
        from workflow_tasks import execute_workflow_task
        execute_workflow_task.apply_async(
            args=[execution_id],
            eta=scheduled_at
        )
        
    except Exception as e:
        logger.error(f"Error scheduling workflow execution {execution_id}: {str(e)}")

async def update_metrics_periodically():
    """Update metrics periodically"""
    while True:
        try:
            await asyncio.sleep(60)  # Update every minute
            
            async with db_pool.acquire() as conn:
                # Update workflow counts by status
                status_counts = await conn.fetch("""
                    SELECT status, COUNT(*) as count
                    FROM workflows
                    GROUP BY status
                """)
                
                # Reset gauges
                for status in WorkflowStatus:
                    workflows_total.labels(status=status.value).set(0)
                
                # Update with actual counts
                for row in status_counts:
                    workflows_total.labels(status=row['status']).set(row['count'])
                
                # Update active executions
                active_count = await conn.fetchval("""
                    SELECT COUNT(*) FROM workflow_executions 
                    WHERE status IN ('pending', 'running')
                """)
                active_executions.set(active_count or 0)
                
        except Exception as e:
            logger.error(f"Error updating metrics: {str(e)}")

async def cleanup_completed_executions():
    """Clean up old completed executions"""
    while True:
        try:
            await asyncio.sleep(3600)  # Run every hour
            
            # Delete executions older than 30 days
            cutoff_date = datetime.utcnow() - timedelta(days=30)
            
            async with db_pool.acquire() as conn:
                deleted_count = await conn.fetchval("""
                    DELETE FROM workflow_executions 
                    WHERE completed_at < $1 AND status IN ('completed', 'failed', 'cancelled')
                    RETURNING COUNT(*)
                """, cutoff_date)
                
                if deleted_count:
                    logger.info(f"Cleaned up {deleted_count} old workflow executions")
                    
        except Exception as e:
            logger.error(f"Error cleaning up executions: {str(e)}")

if __name__ == "__main__":
    uvicorn.run(
        "main:app",
        host="0.0.0.0",
        port=config.port,
        log_level="info",
        reload=config.environment == "development"
    )
