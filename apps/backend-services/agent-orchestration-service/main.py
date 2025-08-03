"""
Agent Orchestration Service - Python
AI agent workflow orchestration for the 002AIC platform
Manages autonomous AI agents, workflows, and multi-agent coordination
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
from sqlalchemy import create_engine, text
from sqlalchemy.orm import sessionmaker
import redis
import aiohttp
from celery import Celery

# Import our auth middleware
import sys
sys.path.append('/home/oss/002AIC/libs/auth-middleware/python')
from auth_middleware import FastAPIAuthMiddleware, AuthConfig

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Initialize FastAPI app
app = FastAPI(
    title="002AIC Agent Orchestration Service",
    description="AI agent workflow orchestration and management",
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
    service_name="agent-orchestration-service"
)

auth = FastAPIAuthMiddleware(auth_config)

# Initialize Celery for background tasks
celery_app = Celery(
    'agent_orchestration',
    broker=os.getenv('CELERY_BROKER_URL', 'redis://localhost:6379/0'),
    backend=os.getenv('CELERY_RESULT_BACKEND', 'redis://localhost:6379/0')
)

# Enums
class AgentStatus(str, Enum):
    IDLE = "idle"
    RUNNING = "running"
    PAUSED = "paused"
    FAILED = "failed"
    COMPLETED = "completed"

class WorkflowStatus(str, Enum):
    DRAFT = "draft"
    ACTIVE = "active"
    PAUSED = "paused"
    COMPLETED = "completed"
    FAILED = "failed"

class AgentType(str, Enum):
    CONVERSATIONAL = "conversational"
    ANALYTICAL = "analytical"
    CREATIVE = "creative"
    AUTOMATION = "automation"
    MONITORING = "monitoring"

# Pydantic models
class AgentDefinition(BaseModel):
    name: str = Field(..., description="Agent name")
    type: AgentType = Field(..., description="Agent type")
    description: Optional[str] = Field(None, description="Agent description")
    capabilities: List[str] = Field(..., description="Agent capabilities")
    model_config: Dict[str, Any] = Field(..., description="AI model configuration")
    tools: List[Dict[str, Any]] = Field(default_factory=list, description="Available tools")
    memory_config: Dict[str, Any] = Field(default_factory=dict, description="Memory configuration")
    constraints: Dict[str, Any] = Field(default_factory=dict, description="Agent constraints")
    max_iterations: int = Field(default=10, description="Maximum iterations per task")
    timeout_seconds: int = Field(default=300, description="Task timeout in seconds")

class WorkflowDefinition(BaseModel):
    name: str = Field(..., description="Workflow name")
    description: Optional[str] = Field(None, description="Workflow description")
    agents: List[str] = Field(..., description="Agent IDs in workflow")
    steps: List[Dict[str, Any]] = Field(..., description="Workflow steps")
    triggers: List[Dict[str, Any]] = Field(default_factory=list, description="Workflow triggers")
    schedule: Optional[str] = Field(None, description="Cron schedule")
    parallel_execution: bool = Field(default=False, description="Allow parallel execution")
    retry_config: Dict[str, Any] = Field(default_factory=dict, description="Retry configuration")

class TaskRequest(BaseModel):
    agent_id: str = Field(..., description="Target agent ID")
    task_type: str = Field(..., description="Task type")
    input_data: Dict[str, Any] = Field(..., description="Task input data")
    context: Dict[str, Any] = Field(default_factory=dict, description="Task context")
    priority: int = Field(default=5, description="Task priority (1-10)")
    timeout_seconds: Optional[int] = Field(None, description="Task timeout override")

class AgentResponse(BaseModel):
    id: str
    name: str
    type: AgentType
    status: AgentStatus
    capabilities: List[str]
    current_task: Optional[str]
    tasks_completed: int
    success_rate: float
    created_at: datetime
    last_active: Optional[datetime]

class WorkflowResponse(BaseModel):
    id: str
    name: str
    status: WorkflowStatus
    agents: List[str]
    current_step: int
    total_steps: int
    progress: float
    started_at: Optional[datetime]
    completed_at: Optional[datetime]
    created_at: datetime

class TaskResponse(BaseModel):
    id: str
    agent_id: str
    task_type: str
    status: str
    progress: float
    result: Optional[Dict[str, Any]]
    error: Optional[str]
    started_at: datetime
    completed_at: Optional[datetime]
    execution_time_ms: Optional[int]

# Initialize external services
def init_postgres():
    database_url = os.getenv(
        "DATABASE_URL",
        "postgresql://postgres:password@localhost:5432/agent_orchestration"
    )
    engine = create_engine(database_url)
    SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)
    return engine, SessionLocal

def init_redis():
    return redis.Redis(
        host=os.getenv("REDIS_HOST", "localhost"),
        port=int(os.getenv("REDIS_PORT", "6379")),
        password=os.getenv("REDIS_PASSWORD", ""),
        db=4,
        decode_responses=True
    )

postgres_engine, SessionLocal = init_postgres()
redis_client = init_redis()

# Health check endpoint
@app.get("/health")
async def health_check():
    postgres_healthy = True
    redis_healthy = True
    celery_healthy = True
    
    try:
        with postgres_engine.connect() as conn:
            conn.execute(text("SELECT 1"))
    except Exception:
        postgres_healthy = False
    
    try:
        redis_client.ping()
    except Exception:
        redis_healthy = False
    
    try:
        celery_app.control.inspect().active()
    except Exception:
        celery_healthy = False
    
    return {
        "status": "healthy" if all([postgres_healthy, redis_healthy, celery_healthy]) else "degraded",
        "service": "agent-orchestration-service",
        "timestamp": datetime.utcnow().isoformat(),
        "version": "1.0.0",
        "dependencies": {
            "postgres": "healthy" if postgres_healthy else "unhealthy",
            "redis": "healthy" if redis_healthy else "unhealthy",
            "celery": "healthy" if celery_healthy else "unhealthy"
        }
    }

# Agent management endpoints
@app.get("/v1/agents", response_model=List[AgentResponse])
@auth.require_auth("agent", "read")
async def list_agents(
    type: Optional[AgentType] = None,
    status: Optional[AgentStatus] = None,
    skip: int = 0,
    limit: int = 100
):
    """List all agents"""
    try:
        query = "SELECT * FROM agents WHERE 1=1"
        params = {}
        
        if type:
            query += " AND type = :type"
            params["type"] = type.value
        
        if status:
            query += " AND status = :status"
            params["status"] = status.value
        
        query += " ORDER BY created_at DESC LIMIT :limit OFFSET :skip"
        params["limit"] = limit
        params["skip"] = skip
        
        with postgres_engine.connect() as conn:
            result = conn.execute(text(query), params)
            agents = []
            
            for row in result:
                agents.append(AgentResponse(
                    id=row.id,
                    name=row.name,
                    type=AgentType(row.type),
                    status=AgentStatus(row.status),
                    capabilities=json.loads(row.capabilities),
                    current_task=row.current_task,
                    tasks_completed=row.tasks_completed,
                    success_rate=row.success_rate,
                    created_at=row.created_at,
                    last_active=row.last_active
                ))
        
        return agents
    except Exception as e:
        logger.error(f"Error listing agents: {e}")
        raise HTTPException(status_code=500, detail="Failed to list agents")

@app.post("/v1/agents", response_model=AgentResponse)
@auth.require_auth("agent", "create")
async def create_agent(agent_definition: AgentDefinition):
    """Create a new AI agent"""
    try:
        agent_id = str(uuid.uuid4())
        
        query = """
        INSERT INTO agents (id, name, type, description, capabilities, model_config,
                          tools, memory_config, constraints, max_iterations, timeout_seconds,
                          status, tasks_completed, success_rate, created_at)
        VALUES (:id, :name, :type, :description, :capabilities, :model_config,
                :tools, :memory_config, :constraints, :max_iterations, :timeout_seconds,
                :status, :tasks_completed, :success_rate, :created_at)
        """
        
        now = datetime.utcnow()
        
        with postgres_engine.connect() as conn:
            conn.execute(text(query), {
                "id": agent_id,
                "name": agent_definition.name,
                "type": agent_definition.type.value,
                "description": agent_definition.description,
                "capabilities": json.dumps(agent_definition.capabilities),
                "model_config": json.dumps(agent_definition.model_config),
                "tools": json.dumps(agent_definition.tools),
                "memory_config": json.dumps(agent_definition.memory_config),
                "constraints": json.dumps(agent_definition.constraints),
                "max_iterations": agent_definition.max_iterations,
                "timeout_seconds": agent_definition.timeout_seconds,
                "status": AgentStatus.IDLE.value,
                "tasks_completed": 0,
                "success_rate": 0.0,
                "created_at": now
            })
            conn.commit()
        
        # Initialize agent in Redis
        agent_key = f"agent:{agent_id}"
        redis_client.hset(agent_key, mapping={
            "status": AgentStatus.IDLE.value,
            "created_at": now.isoformat(),
            "last_heartbeat": now.isoformat()
        })
        
        return AgentResponse(
            id=agent_id,
            name=agent_definition.name,
            type=agent_definition.type,
            status=AgentStatus.IDLE,
            capabilities=agent_definition.capabilities,
            current_task=None,
            tasks_completed=0,
            success_rate=0.0,
            created_at=now,
            last_active=None
        )
    except Exception as e:
        logger.error(f"Error creating agent: {e}")
        raise HTTPException(status_code=500, detail="Failed to create agent")

@app.post("/v1/agents/{agent_id}/tasks", response_model=TaskResponse)
@auth.require_auth("agent", "execute")
async def assign_task(agent_id: str, task_request: TaskRequest, background_tasks: BackgroundTasks):
    """Assign a task to an agent"""
    try:
        # Verify agent exists
        query = "SELECT * FROM agents WHERE id = :agent_id"
        
        with postgres_engine.connect() as conn:
            result = conn.execute(text(query), {"agent_id": agent_id})
            agent = result.fetchone()
            
            if not agent:
                raise HTTPException(status_code=404, detail="Agent not found")
        
        task_id = str(uuid.uuid4())
        
        # Create task record
        task_query = """
        INSERT INTO tasks (id, agent_id, task_type, input_data, context, priority,
                          timeout_seconds, status, progress, started_at)
        VALUES (:id, :agent_id, :task_type, :input_data, :context, :priority,
                :timeout_seconds, :status, :progress, :started_at)
        """
        
        now = datetime.utcnow()
        
        with postgres_engine.connect() as conn:
            conn.execute(text(task_query), {
                "id": task_id,
                "agent_id": agent_id,
                "task_type": task_request.task_type,
                "input_data": json.dumps(task_request.input_data),
                "context": json.dumps(task_request.context),
                "priority": task_request.priority,
                "timeout_seconds": task_request.timeout_seconds or agent.timeout_seconds,
                "status": "queued",
                "progress": 0.0,
                "started_at": now
            })
            conn.commit()
        
        # Queue task for execution
        background_tasks.add_task(execute_agent_task, task_id, agent_id, task_request)
        
        return TaskResponse(
            id=task_id,
            agent_id=agent_id,
            task_type=task_request.task_type,
            status="queued",
            progress=0.0,
            result=None,
            error=None,
            started_at=now,
            completed_at=None,
            execution_time_ms=None
        )
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error assigning task: {e}")
        raise HTTPException(status_code=500, detail="Failed to assign task")

# Workflow management endpoints
@app.get("/v1/workflows", response_model=List[WorkflowResponse])
@auth.require_auth("workflow", "read")
async def list_workflows(
    status: Optional[WorkflowStatus] = None,
    skip: int = 0,
    limit: int = 100
):
    """List all workflows"""
    try:
        query = "SELECT * FROM workflows WHERE 1=1"
        params = {}
        
        if status:
            query += " AND status = :status"
            params["status"] = status.value
        
        query += " ORDER BY created_at DESC LIMIT :limit OFFSET :skip"
        params["limit"] = limit
        params["skip"] = skip
        
        with postgres_engine.connect() as conn:
            result = conn.execute(text(query), params)
            workflows = []
            
            for row in result:
                workflows.append(WorkflowResponse(
                    id=row.id,
                    name=row.name,
                    status=WorkflowStatus(row.status),
                    agents=json.loads(row.agents),
                    current_step=row.current_step,
                    total_steps=row.total_steps,
                    progress=row.progress,
                    started_at=row.started_at,
                    completed_at=row.completed_at,
                    created_at=row.created_at
                ))
        
        return workflows
    except Exception as e:
        logger.error(f"Error listing workflows: {e}")
        raise HTTPException(status_code=500, detail="Failed to list workflows")

@app.post("/v1/workflows", response_model=WorkflowResponse)
@auth.require_auth("workflow", "create")
async def create_workflow(workflow_definition: WorkflowDefinition):
    """Create a new workflow"""
    try:
        workflow_id = str(uuid.uuid4())
        
        query = """
        INSERT INTO workflows (id, name, description, agents, steps, triggers,
                             schedule, parallel_execution, retry_config, status,
                             current_step, total_steps, progress, created_at)
        VALUES (:id, :name, :description, :agents, :steps, :triggers,
                :schedule, :parallel_execution, :retry_config, :status,
                :current_step, :total_steps, :progress, :created_at)
        """
        
        now = datetime.utcnow()
        
        with postgres_engine.connect() as conn:
            conn.execute(text(query), {
                "id": workflow_id,
                "name": workflow_definition.name,
                "description": workflow_definition.description,
                "agents": json.dumps(workflow_definition.agents),
                "steps": json.dumps(workflow_definition.steps),
                "triggers": json.dumps(workflow_definition.triggers),
                "schedule": workflow_definition.schedule,
                "parallel_execution": workflow_definition.parallel_execution,
                "retry_config": json.dumps(workflow_definition.retry_config),
                "status": WorkflowStatus.DRAFT.value,
                "current_step": 0,
                "total_steps": len(workflow_definition.steps),
                "progress": 0.0,
                "created_at": now
            })
            conn.commit()
        
        return WorkflowResponse(
            id=workflow_id,
            name=workflow_definition.name,
            status=WorkflowStatus.DRAFT,
            agents=workflow_definition.agents,
            current_step=0,
            total_steps=len(workflow_definition.steps),
            progress=0.0,
            started_at=None,
            completed_at=None,
            created_at=now
        )
    except Exception as e:
        logger.error(f"Error creating workflow: {e}")
        raise HTTPException(status_code=500, detail="Failed to create workflow")

@app.post("/v1/workflows/{workflow_id}/execute")
@auth.require_auth("workflow", "execute")
async def execute_workflow(workflow_id: str, background_tasks: BackgroundTasks):
    """Execute a workflow"""
    try:
        # Get workflow details
        query = "SELECT * FROM workflows WHERE id = :workflow_id"
        
        with postgres_engine.connect() as conn:
            result = conn.execute(text(query), {"workflow_id": workflow_id})
            workflow = result.fetchone()
            
            if not workflow:
                raise HTTPException(status_code=404, detail="Workflow not found")
            
            if workflow.status != WorkflowStatus.DRAFT.value:
                raise HTTPException(status_code=400, detail="Workflow is not in draft status")
        
        # Update workflow status
        update_query = """
        UPDATE workflows 
        SET status = :status, started_at = :started_at, updated_at = :updated_at
        WHERE id = :workflow_id
        """
        
        now = datetime.utcnow()
        
        with postgres_engine.connect() as conn:
            conn.execute(text(update_query), {
                "status": WorkflowStatus.ACTIVE.value,
                "started_at": now,
                "updated_at": now,
                "workflow_id": workflow_id
            })
            conn.commit()
        
        # Queue workflow execution
        background_tasks.add_task(execute_workflow_background, workflow_id)
        
        return {
            "message": "Workflow execution started",
            "workflow_id": workflow_id,
            "status": "active"
        }
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error executing workflow: {e}")
        raise HTTPException(status_code=500, detail="Failed to execute workflow")

# Background tasks
async def execute_agent_task(task_id: str, agent_id: str, task_request: TaskRequest):
    """Background task to execute agent task"""
    try:
        logger.info(f"Executing task {task_id} for agent {agent_id}")
        
        # Update task status
        with postgres_engine.connect() as conn:
            conn.execute(text("""
                UPDATE tasks 
                SET status = 'running', updated_at = :updated_at
                WHERE id = :task_id
            """), {"updated_at": datetime.utcnow(), "task_id": task_id})
            conn.commit()
        
        # Simulate task execution
        await asyncio.sleep(5)
        
        # Mock result
        result = {
            "output": f"Task {task_request.task_type} completed successfully",
            "data": {"processed_items": 42, "confidence": 0.95},
            "metadata": {"model_version": "1.0", "execution_node": "worker-1"}
        }
        
        # Update task completion
        now = datetime.utcnow()
        with postgres_engine.connect() as conn:
            conn.execute(text("""
                UPDATE tasks 
                SET status = 'completed', progress = 100.0, result = :result,
                    completed_at = :completed_at, updated_at = :updated_at
                WHERE id = :task_id
            """), {
                "result": json.dumps(result),
                "completed_at": now,
                "updated_at": now,
                "task_id": task_id
            })
            conn.commit()
        
        logger.info(f"Task {task_id} completed successfully")
    except Exception as e:
        logger.error(f"Error executing task {task_id}: {e}")
        
        # Update task failure
        with postgres_engine.connect() as conn:
            conn.execute(text("""
                UPDATE tasks 
                SET status = 'failed', error = :error, updated_at = :updated_at
                WHERE id = :task_id
            """), {
                "error": str(e),
                "updated_at": datetime.utcnow(),
                "task_id": task_id
            })
            conn.commit()

async def execute_workflow_background(workflow_id: str):
    """Background task to execute workflow"""
    try:
        logger.info(f"Executing workflow {workflow_id}")
        
        # Get workflow details
        with postgres_engine.connect() as conn:
            result = conn.execute(text("SELECT * FROM workflows WHERE id = :workflow_id"), 
                                {"workflow_id": workflow_id})
            workflow = result.fetchone()
        
        steps = json.loads(workflow.steps)
        
        for i, step in enumerate(steps):
            # Update current step
            with postgres_engine.connect() as conn:
                conn.execute(text("""
                    UPDATE workflows 
                    SET current_step = :current_step, progress = :progress, updated_at = :updated_at
                    WHERE id = :workflow_id
                """), {
                    "current_step": i + 1,
                    "progress": ((i + 1) / len(steps)) * 100,
                    "updated_at": datetime.utcnow(),
                    "workflow_id": workflow_id
                })
                conn.commit()
            
            # Execute step (simplified)
            await asyncio.sleep(10)
            logger.info(f"Completed step {i + 1} of workflow {workflow_id}")
        
        # Mark workflow as completed
        now = datetime.utcnow()
        with postgres_engine.connect() as conn:
            conn.execute(text("""
                UPDATE workflows 
                SET status = 'completed', progress = 100.0, completed_at = :completed_at, updated_at = :updated_at
                WHERE id = :workflow_id
            """), {
                "completed_at": now,
                "updated_at": now,
                "workflow_id": workflow_id
            })
            conn.commit()
        
        logger.info(f"Workflow {workflow_id} completed successfully")
    except Exception as e:
        logger.error(f"Error executing workflow {workflow_id}: {e}")
        
        # Mark workflow as failed
        with postgres_engine.connect() as conn:
            conn.execute(text("""
                UPDATE workflows 
                SET status = 'failed', updated_at = :updated_at
                WHERE id = :workflow_id
            """), {
                "updated_at": datetime.utcnow(),
                "workflow_id": workflow_id
            })
            conn.commit()

if __name__ == "__main__":
    uvicorn.run(
        "main:app",
        host="0.0.0.0",
        port=int(os.getenv("PORT", 8080)),
        reload=os.getenv("ENVIRONMENT") == "development"
    )
