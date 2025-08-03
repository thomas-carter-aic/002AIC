"""
Pipeline Execution Service - Python
Workflow execution engine for the 002AIC platform
Handles ML pipelines, data workflows, and orchestration
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
from enum import Enum
import networkx as nx
from celery import Celery
import redis
from sqlalchemy import create_engine, text
from sqlalchemy.orm import sessionmaker

# Import our auth middleware
import sys
sys.path.append('/home/oss/002AIC/libs/auth-middleware/python')
from auth_middleware import FastAPIAuthMiddleware, AuthConfig

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Initialize FastAPI app
app = FastAPI(
    title="002AIC Pipeline Execution Service",
    description="Workflow execution engine and pipeline orchestration",
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
    service_name="pipeline-execution-service"
)

auth = FastAPIAuthMiddleware(auth_config)

# Initialize Celery
celery_app = Celery(
    'pipeline_execution',
    broker=os.getenv('CELERY_BROKER_URL', 'redis://localhost:6379/0'),
    backend=os.getenv('CELERY_RESULT_BACKEND', 'redis://localhost:6379/0')
)

# Enums
class PipelineStatus(str, Enum):
    DRAFT = "draft"
    ACTIVE = "active"
    RUNNING = "running"
    PAUSED = "paused"
    COMPLETED = "completed"
    FAILED = "failed"
    CANCELLED = "cancelled"

class StepStatus(str, Enum):
    PENDING = "pending"
    RUNNING = "running"
    COMPLETED = "completed"
    FAILED = "failed"
    SKIPPED = "skipped"
    CANCELLED = "cancelled"

class StepType(str, Enum):
    DATA_INGESTION = "data_ingestion"
    DATA_TRANSFORMATION = "data_transformation"
    MODEL_TRAINING = "model_training"
    MODEL_EVALUATION = "model_evaluation"
    MODEL_DEPLOYMENT = "model_deployment"
    NOTIFICATION = "notification"
    CUSTOM = "custom"

class TriggerType(str, Enum):
    MANUAL = "manual"
    SCHEDULED = "scheduled"
    EVENT = "event"
    WEBHOOK = "webhook"

# Pydantic models
class PipelineStep(BaseModel):
    id: str = Field(..., description="Step ID")
    name: str = Field(..., description="Step name")
    type: StepType = Field(..., description="Step type")
    config: Dict[str, Any] = Field(..., description="Step configuration")
    dependencies: List[str] = Field(default_factory=list, description="Dependent step IDs")
    timeout_seconds: int = Field(default=3600, description="Step timeout")
    retry_count: int = Field(default=3, description="Retry attempts")
    retry_delay: int = Field(default=60, description="Retry delay in seconds")
    enabled: bool = Field(default=True, description="Whether step is enabled")
    conditions: Dict[str, Any] = Field(default_factory=dict, description="Execution conditions")

class PipelineDefinition(BaseModel):
    name: str = Field(..., description="Pipeline name")
    description: Optional[str] = Field(None, description="Pipeline description")
    steps: List[PipelineStep] = Field(..., description="Pipeline steps")
    triggers: List[Dict[str, Any]] = Field(default_factory=list, description="Pipeline triggers")
    schedule: Optional[str] = Field(None, description="Cron schedule")
    timeout_seconds: int = Field(default=7200, description="Pipeline timeout")
    max_concurrent_runs: int = Field(default=1, description="Max concurrent executions")
    failure_policy: str = Field(default="fail_fast", description="Failure handling policy")
    notification_config: Dict[str, Any] = Field(default_factory=dict, description="Notification settings")
    tags: List[str] = Field(default_factory=list, description="Pipeline tags")

class PipelineExecution(BaseModel):
    pipeline_id: str = Field(..., description="Pipeline ID")
    trigger_type: TriggerType = Field(..., description="Execution trigger")
    parameters: Dict[str, Any] = Field(default_factory=dict, description="Execution parameters")
    priority: int = Field(default=5, description="Execution priority (1-10)")

class PipelineResponse(BaseModel):
    id: str
    name: str
    description: Optional[str]
    status: PipelineStatus
    steps_count: int
    last_execution: Optional[datetime]
    next_execution: Optional[datetime]
    success_rate: float
    average_duration: Optional[int]
    created_at: datetime
    updated_at: datetime

class ExecutionResponse(BaseModel):
    id: str
    pipeline_id: str
    status: PipelineStatus
    trigger_type: TriggerType
    started_at: datetime
    completed_at: Optional[datetime]
    duration_seconds: Optional[int]
    steps_completed: int
    steps_total: int
    progress: float
    current_step: Optional[str]
    error_message: Optional[str]
    metrics: Dict[str, Any]

class StepExecutionResponse(BaseModel):
    id: str
    execution_id: str
    step_id: str
    step_name: str
    status: StepStatus
    started_at: Optional[datetime]
    completed_at: Optional[datetime]
    duration_seconds: Optional[int]
    retry_count: int
    error_message: Optional[str]
    output: Optional[Dict[str, Any]]
    logs: List[str]

# Storage
pipelines = {}
executions = {}
step_executions = {}

# Initialize external services
def init_redis():
    return redis.Redis(
        host=os.getenv("REDIS_HOST", "localhost"),
        port=int(os.getenv("REDIS_PORT", "6379")),
        password=os.getenv("REDIS_PASSWORD", ""),
        db=7,
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
        "service": "pipeline-execution-service",
        "timestamp": datetime.utcnow().isoformat(),
        "version": "1.0.0",
        "dependencies": {
            "redis": "healthy" if redis_healthy else "unhealthy",
            "celery": "healthy" if celery_healthy else "unhealthy"
        }
    }

# Pipeline management endpoints
@app.get("/v1/pipelines", response_model=List[PipelineResponse])
@auth.require_auth("pipeline", "read")
async def list_pipelines(
    status: Optional[PipelineStatus] = None,
    tag: Optional[str] = None,
    skip: int = 0,
    limit: int = 100
):
    """List pipelines"""
    try:
        pipeline_list = list(pipelines.values())
        
        if status:
            pipeline_list = [p for p in pipeline_list if p["status"] == status.value]
        if tag:
            pipeline_list = [p for p in pipeline_list if tag in p.get("tags", [])]
        
        pipeline_list = pipeline_list[skip:skip + limit]
        return [PipelineResponse(**pipeline) for pipeline in pipeline_list]
    except Exception as e:
        logger.error(f"Error listing pipelines: {e}")
        raise HTTPException(status_code=500, detail="Failed to list pipelines")

@app.post("/v1/pipelines", response_model=PipelineResponse)
@auth.require_auth("pipeline", "create")
async def create_pipeline(pipeline_def: PipelineDefinition):
    """Create a new pipeline"""
    try:
        pipeline_id = str(uuid.uuid4())
        
        # Validate pipeline definition
        validate_pipeline_definition(pipeline_def)
        
        pipeline_data = {
            "id": pipeline_id,
            "name": pipeline_def.name,
            "description": pipeline_def.description,
            "status": PipelineStatus.DRAFT.value,
            "steps_count": len(pipeline_def.steps),
            "last_execution": None,
            "next_execution": None,
            "success_rate": 0.0,
            "average_duration": None,
            "created_at": datetime.utcnow(),
            "updated_at": datetime.utcnow(),
            "definition": pipeline_def.dict(),
            "tags": pipeline_def.tags
        }
        
        pipelines[pipeline_id] = pipeline_data
        
        # Cache pipeline definition
        redis_client.setex(
            f"pipeline:{pipeline_id}",
            3600,
            json.dumps(pipeline_def.dict(), default=str)
        )
        
        return PipelineResponse(**pipeline_data)
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error creating pipeline: {e}")
        raise HTTPException(status_code=500, detail="Failed to create pipeline")

@app.get("/v1/pipelines/{pipeline_id}", response_model=PipelineResponse)
@auth.require_auth("pipeline", "read")
async def get_pipeline(pipeline_id: str):
    """Get pipeline details"""
    try:
        if pipeline_id not in pipelines:
            raise HTTPException(status_code=404, detail="Pipeline not found")
        
        pipeline_data = pipelines[pipeline_id]
        return PipelineResponse(**pipeline_data)
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error getting pipeline: {e}")
        raise HTTPException(status_code=500, detail="Failed to get pipeline")

@app.post("/v1/pipelines/{pipeline_id}/execute", response_model=ExecutionResponse)
@auth.require_auth("pipeline", "execute")
async def execute_pipeline(
    pipeline_id: str,
    execution_request: PipelineExecution,
    background_tasks: BackgroundTasks
):
    """Execute a pipeline"""
    try:
        if pipeline_id not in pipelines:
            raise HTTPException(status_code=404, detail="Pipeline not found")
        
        pipeline_data = pipelines[pipeline_id]
        
        # Check if pipeline can be executed
        if pipeline_data["status"] not in [PipelineStatus.DRAFT.value, PipelineStatus.ACTIVE.value]:
            raise HTTPException(status_code=400, detail="Pipeline cannot be executed in current status")
        
        execution_id = str(uuid.uuid4())
        
        # Create execution record
        execution_data = {
            "id": execution_id,
            "pipeline_id": pipeline_id,
            "status": PipelineStatus.RUNNING.value,
            "trigger_type": execution_request.trigger_type.value,
            "started_at": datetime.utcnow(),
            "completed_at": None,
            "duration_seconds": None,
            "steps_completed": 0,
            "steps_total": pipeline_data["steps_count"],
            "progress": 0.0,
            "current_step": None,
            "error_message": None,
            "metrics": {},
            "parameters": execution_request.parameters
        }
        
        executions[execution_id] = execution_data
        
        # Update pipeline last execution
        pipeline_data["last_execution"] = datetime.utcnow()
        
        # Queue pipeline execution
        background_tasks.add_task(execute_pipeline_background, pipeline_id, execution_id)
        
        return ExecutionResponse(**execution_data)
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error executing pipeline: {e}")
        raise HTTPException(status_code=500, detail="Failed to execute pipeline")

@app.get("/v1/pipelines/{pipeline_id}/executions", response_model=List[ExecutionResponse])
@auth.require_auth("pipeline", "read")
async def get_pipeline_executions(
    pipeline_id: str,
    status: Optional[PipelineStatus] = None,
    limit: int = 50
):
    """Get pipeline execution history"""
    try:
        if pipeline_id not in pipelines:
            raise HTTPException(status_code=404, detail="Pipeline not found")
        
        pipeline_executions = [
            exec_data for exec_data in executions.values()
            if exec_data["pipeline_id"] == pipeline_id
        ]
        
        if status:
            pipeline_executions = [e for e in pipeline_executions if e["status"] == status.value]
        
        # Sort by started_at descending
        pipeline_executions.sort(key=lambda x: x["started_at"], reverse=True)
        pipeline_executions = pipeline_executions[:limit]
        
        return [ExecutionResponse(**execution) for execution in pipeline_executions]
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error getting pipeline executions: {e}")
        raise HTTPException(status_code=500, detail="Failed to get pipeline executions")

@app.get("/v1/executions/{execution_id}", response_model=ExecutionResponse)
@auth.require_auth("pipeline", "read")
async def get_execution(execution_id: str):
    """Get execution details"""
    try:
        if execution_id not in executions:
            raise HTTPException(status_code=404, detail="Execution not found")
        
        execution_data = executions[execution_id]
        return ExecutionResponse(**execution_data)
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error getting execution: {e}")
        raise HTTPException(status_code=500, detail="Failed to get execution")

@app.get("/v1/executions/{execution_id}/steps", response_model=List[StepExecutionResponse])
@auth.require_auth("pipeline", "read")
async def get_execution_steps(execution_id: str):
    """Get execution step details"""
    try:
        if execution_id not in executions:
            raise HTTPException(status_code=404, detail="Execution not found")
        
        execution_steps = [
            step for step in step_executions.values()
            if step["execution_id"] == execution_id
        ]
        
        # Sort by started_at
        execution_steps.sort(key=lambda x: x.get("started_at") or datetime.min)
        
        return [StepExecutionResponse(**step) for step in execution_steps]
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error getting execution steps: {e}")
        raise HTTPException(status_code=500, detail="Failed to get execution steps")

@app.post("/v1/executions/{execution_id}/cancel")
@auth.require_auth("pipeline", "execute")
async def cancel_execution(execution_id: str):
    """Cancel a running execution"""
    try:
        if execution_id not in executions:
            raise HTTPException(status_code=404, detail="Execution not found")
        
        execution_data = executions[execution_id]
        
        if execution_data["status"] != PipelineStatus.RUNNING.value:
            raise HTTPException(status_code=400, detail="Execution is not running")
        
        # Update execution status
        execution_data["status"] = PipelineStatus.CANCELLED.value
        execution_data["completed_at"] = datetime.utcnow()
        execution_data["duration_seconds"] = int(
            (execution_data["completed_at"] - execution_data["started_at"]).total_seconds()
        )
        
        # Cancel running steps
        for step in step_executions.values():
            if step["execution_id"] == execution_id and step["status"] == StepStatus.RUNNING.value:
                step["status"] = StepStatus.CANCELLED.value
                step["completed_at"] = datetime.utcnow()
        
        return {"message": "Execution cancelled successfully"}
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error cancelling execution: {e}")
        raise HTTPException(status_code=500, detail="Failed to cancel execution")

# Pipeline validation and execution logic
def validate_pipeline_definition(pipeline_def: PipelineDefinition):
    """Validate pipeline definition"""
    step_ids = {step.id for step in pipeline_def.steps}
    
    # Check for duplicate step IDs
    if len(step_ids) != len(pipeline_def.steps):
        raise HTTPException(status_code=400, detail="Duplicate step IDs found")
    
    # Check dependencies
    for step in pipeline_def.steps:
        for dep_id in step.dependencies:
            if dep_id not in step_ids:
                raise HTTPException(
                    status_code=400,
                    detail=f"Step {step.id} depends on non-existent step {dep_id}"
                )
    
    # Check for circular dependencies
    graph = nx.DiGraph()
    for step in pipeline_def.steps:
        graph.add_node(step.id)
        for dep_id in step.dependencies:
            graph.add_edge(dep_id, step.id)
    
    if not nx.is_directed_acyclic_graph(graph):
        raise HTTPException(status_code=400, detail="Circular dependencies detected")

async def execute_pipeline_background(pipeline_id: str, execution_id: str):
    """Background task to execute pipeline"""
    try:
        logger.info(f"Starting pipeline execution {execution_id}")
        
        pipeline_data = pipelines[pipeline_id]
        execution_data = executions[execution_id]
        pipeline_def = PipelineDefinition(**pipeline_data["definition"])
        
        # Create dependency graph
        graph = nx.DiGraph()
        step_map = {step.id: step for step in pipeline_def.steps}
        
        for step in pipeline_def.steps:
            graph.add_node(step.id)
            for dep_id in step.dependencies:
                graph.add_edge(dep_id, step.id)
        
        # Execute steps in topological order
        completed_steps = set()
        failed_steps = set()
        
        for step_id in nx.topological_sort(graph):
            step = step_map[step_id]
            
            # Check if execution was cancelled
            if executions[execution_id]["status"] == PipelineStatus.CANCELLED.value:
                logger.info(f"Pipeline execution {execution_id} was cancelled")
                return
            
            # Check if dependencies are satisfied
            if not all(dep_id in completed_steps for dep_id in step.dependencies):
                logger.warning(f"Skipping step {step_id} due to failed dependencies")
                continue
            
            # Execute step
            step_success = await execute_step(execution_id, step)
            
            if step_success:
                completed_steps.add(step_id)
                execution_data["steps_completed"] += 1
            else:
                failed_steps.add(step_id)
                
                # Check failure policy
                if pipeline_def.failure_policy == "fail_fast":
                    logger.error(f"Step {step_id} failed, stopping pipeline execution")
                    break
            
            # Update progress
            execution_data["progress"] = (execution_data["steps_completed"] / execution_data["steps_total"]) * 100
            execution_data["current_step"] = step_id
        
        # Complete execution
        now = datetime.utcnow()
        execution_data["completed_at"] = now
        execution_data["duration_seconds"] = int((now - execution_data["started_at"]).total_seconds())
        
        if failed_steps and pipeline_def.failure_policy == "fail_fast":
            execution_data["status"] = PipelineStatus.FAILED.value
        else:
            execution_data["status"] = PipelineStatus.COMPLETED.value
        
        # Update pipeline statistics
        update_pipeline_statistics(pipeline_id)
        
        logger.info(f"Pipeline execution {execution_id} completed")
        
    except Exception as e:
        logger.error(f"Error in pipeline execution {execution_id}: {e}")
        
        # Update execution status
        execution_data = executions[execution_id]
        execution_data["status"] = PipelineStatus.FAILED.value
        execution_data["completed_at"] = datetime.utcnow()
        execution_data["error_message"] = str(e)

async def execute_step(execution_id: str, step: PipelineStep) -> bool:
    """Execute a single pipeline step"""
    try:
        step_execution_id = str(uuid.uuid4())
        
        # Create step execution record
        step_data = {
            "id": step_execution_id,
            "execution_id": execution_id,
            "step_id": step.id,
            "step_name": step.name,
            "status": StepStatus.RUNNING.value,
            "started_at": datetime.utcnow(),
            "completed_at": None,
            "duration_seconds": None,
            "retry_count": 0,
            "error_message": None,
            "output": None,
            "logs": []
        }
        
        step_executions[step_execution_id] = step_data
        
        logger.info(f"Executing step {step.id}: {step.name}")
        
        # Execute step based on type
        success = False
        for attempt in range(step.retry_count + 1):
            try:
                if step.type == StepType.DATA_INGESTION:
                    success = await execute_data_ingestion_step(step, step_data)
                elif step.type == StepType.DATA_TRANSFORMATION:
                    success = await execute_data_transformation_step(step, step_data)
                elif step.type == StepType.MODEL_TRAINING:
                    success = await execute_model_training_step(step, step_data)
                elif step.type == StepType.MODEL_EVALUATION:
                    success = await execute_model_evaluation_step(step, step_data)
                elif step.type == StepType.MODEL_DEPLOYMENT:
                    success = await execute_model_deployment_step(step, step_data)
                elif step.type == StepType.NOTIFICATION:
                    success = await execute_notification_step(step, step_data)
                else:
                    success = await execute_custom_step(step, step_data)
                
                if success:
                    break
                    
            except Exception as e:
                step_data["retry_count"] = attempt + 1
                step_data["error_message"] = str(e)
                step_data["logs"].append(f"Attempt {attempt + 1} failed: {str(e)}")
                
                if attempt < step.retry_count:
                    logger.warning(f"Step {step.id} attempt {attempt + 1} failed, retrying...")
                    await asyncio.sleep(step.retry_delay)
                else:
                    logger.error(f"Step {step.id} failed after {step.retry_count + 1} attempts")
        
        # Complete step execution
        now = datetime.utcnow()
        step_data["completed_at"] = now
        step_data["duration_seconds"] = int((now - step_data["started_at"]).total_seconds())
        step_data["status"] = StepStatus.COMPLETED.value if success else StepStatus.FAILED.value
        
        return success
        
    except Exception as e:
        logger.error(f"Error executing step {step.id}: {e}")
        return False

# Step execution implementations
async def execute_data_ingestion_step(step: PipelineStep, step_data: dict) -> bool:
    """Execute data ingestion step"""
    await asyncio.sleep(5)  # Simulate processing
    step_data["output"] = {"records_ingested": 10000, "source": "database"}
    step_data["logs"].append("Data ingestion completed successfully")
    return True

async def execute_data_transformation_step(step: PipelineStep, step_data: dict) -> bool:
    """Execute data transformation step"""
    await asyncio.sleep(3)  # Simulate processing
    step_data["output"] = {"records_transformed": 9500, "transformations_applied": 5}
    step_data["logs"].append("Data transformation completed successfully")
    return True

async def execute_model_training_step(step: PipelineStep, step_data: dict) -> bool:
    """Execute model training step"""
    await asyncio.sleep(10)  # Simulate training
    step_data["output"] = {"model_id": "model_123", "accuracy": 0.95, "epochs": 100}
    step_data["logs"].append("Model training completed successfully")
    return True

async def execute_model_evaluation_step(step: PipelineStep, step_data: dict) -> bool:
    """Execute model evaluation step"""
    await asyncio.sleep(2)  # Simulate evaluation
    step_data["output"] = {"accuracy": 0.95, "precision": 0.92, "recall": 0.94}
    step_data["logs"].append("Model evaluation completed successfully")
    return True

async def execute_model_deployment_step(step: PipelineStep, step_data: dict) -> bool:
    """Execute model deployment step"""
    await asyncio.sleep(5)  # Simulate deployment
    step_data["output"] = {"deployment_id": "deploy_456", "endpoint": "https://api.002aic.com/predict"}
    step_data["logs"].append("Model deployment completed successfully")
    return True

async def execute_notification_step(step: PipelineStep, step_data: dict) -> bool:
    """Execute notification step"""
    await asyncio.sleep(1)  # Simulate notification
    step_data["output"] = {"notification_sent": True, "recipients": ["admin@002aic.com"]}
    step_data["logs"].append("Notification sent successfully")
    return True

async def execute_custom_step(step: PipelineStep, step_data: dict) -> bool:
    """Execute custom step"""
    await asyncio.sleep(2)  # Simulate custom processing
    step_data["output"] = {"custom_result": "success"}
    step_data["logs"].append("Custom step completed successfully")
    return True

def update_pipeline_statistics(pipeline_id: str):
    """Update pipeline success rate and average duration"""
    pipeline_data = pipelines[pipeline_id]
    
    # Get all executions for this pipeline
    pipeline_executions = [
        exec_data for exec_data in executions.values()
        if exec_data["pipeline_id"] == pipeline_id and exec_data["status"] in [
            PipelineStatus.COMPLETED.value, PipelineStatus.FAILED.value
        ]
    ]
    
    if not pipeline_executions:
        return
    
    # Calculate success rate
    successful_executions = [
        exec_data for exec_data in pipeline_executions
        if exec_data["status"] == PipelineStatus.COMPLETED.value
    ]
    
    pipeline_data["success_rate"] = (len(successful_executions) / len(pipeline_executions)) * 100
    
    # Calculate average duration
    durations = [
        exec_data["duration_seconds"] for exec_data in pipeline_executions
        if exec_data["duration_seconds"] is not None
    ]
    
    if durations:
        pipeline_data["average_duration"] = int(sum(durations) / len(durations))

if __name__ == "__main__":
    uvicorn.run(
        "main:app",
        host="0.0.0.0",
        port=int(os.getenv("PORT", 8080)),
        reload=os.getenv("ENVIRONMENT") == "development"
    )
