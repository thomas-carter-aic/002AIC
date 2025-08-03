"""
Feature Flag Service - Python
Dynamic feature management and A/B testing for the 002AIC platform
Handles feature toggles, gradual rollouts, and experimentation
"""

import asyncio
import json
import logging
import os
import hashlib
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

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Configuration
class Config:
    def __init__(self):
        self.port = int(os.getenv("PORT", "8080"))
        self.database_url = os.getenv("DATABASE_URL", "postgresql://postgres:password@localhost:5432/feature_flags")
        self.redis_url = os.getenv("REDIS_URL", "redis://localhost:6379")
        self.environment = os.getenv("ENVIRONMENT", "development")
        self.cache_ttl = int(os.getenv("CACHE_TTL", "300"))  # 5 minutes
        self.evaluation_timeout = int(os.getenv("EVALUATION_TIMEOUT", "100"))  # 100ms

config = Config()

# Enums
class FlagType(str, Enum):
    BOOLEAN = "boolean"
    STRING = "string"
    NUMBER = "number"
    JSON = "json"

class FlagStatus(str, Enum):
    ACTIVE = "active"
    INACTIVE = "inactive"
    ARCHIVED = "archived"

class RolloutStrategy(str, Enum):
    ALL_USERS = "all_users"
    PERCENTAGE = "percentage"
    USER_LIST = "user_list"
    USER_ATTRIBUTES = "user_attributes"
    GRADUAL = "gradual"

class ExperimentStatus(str, Enum):
    DRAFT = "draft"
    RUNNING = "running"
    COMPLETED = "completed"
    PAUSED = "paused"

# Pydantic models
class FeatureFlag(BaseModel):
    key: str = Field(..., min_length=1, max_length=255)
    name: str = Field(..., min_length=1, max_length=255)
    description: Optional[str] = None
    type: FlagType = FlagType.BOOLEAN
    default_value: Any = True
    status: FlagStatus = FlagStatus.ACTIVE
    tags: List[str] = Field(default_factory=list)
    created_by: str
    project_id: Optional[str] = None

class RolloutRule(BaseModel):
    id: Optional[str] = None
    strategy: RolloutStrategy
    percentage: Optional[float] = Field(None, ge=0, le=100)
    user_ids: Optional[List[str]] = None
    attributes: Optional[Dict[str, Any]] = None
    value: Any = True
    description: Optional[str] = None

class CreateFeatureFlagRequest(BaseModel):
    flag: FeatureFlag
    rollout_rules: List[RolloutRule] = Field(default_factory=list)

class UpdateFeatureFlagRequest(BaseModel):
    name: Optional[str] = None
    description: Optional[str] = None
    status: Optional[FlagStatus] = None
    default_value: Optional[Any] = None
    tags: Optional[List[str]] = None

class EvaluationContext(BaseModel):
    user_id: str
    attributes: Dict[str, Any] = Field(default_factory=dict)
    session_id: Optional[str] = None
    ip_address: Optional[str] = None
    user_agent: Optional[str] = None

class FlagEvaluationRequest(BaseModel):
    flag_key: str
    context: EvaluationContext
    default_value: Optional[Any] = None

class FlagEvaluationResponse(BaseModel):
    flag_key: str
    value: Any
    variation_id: Optional[str] = None
    reason: str
    timestamp: datetime

class Experiment(BaseModel):
    id: Optional[str] = None
    name: str = Field(..., min_length=1, max_length=255)
    description: Optional[str] = None
    flag_key: str
    status: ExperimentStatus = ExperimentStatus.DRAFT
    traffic_allocation: float = Field(100.0, ge=0, le=100)
    variations: List[Dict[str, Any]]
    metrics: List[str] = Field(default_factory=list)
    start_date: Optional[datetime] = None
    end_date: Optional[datetime] = None
    created_by: str

# Prometheus metrics
flag_evaluations_total = Counter(
    'feature_flag_evaluations_total',
    'Total feature flag evaluations',
    ['flag_key', 'variation', 'reason']
)

flag_evaluation_duration = Histogram(
    'feature_flag_evaluation_duration_seconds',
    'Time taken to evaluate feature flags',
    ['flag_key']
)

active_flags = Gauge(
    'feature_flags_active_total',
    'Total number of active feature flags'
)

active_experiments = Gauge(
    'experiments_active_total',
    'Total number of active experiments'
)

cache_hit_rate = Gauge(
    'feature_flag_cache_hit_rate',
    'Cache hit rate for feature flag evaluations'
)

# Global variables
app = FastAPI(title="Feature Flag Service", version="1.0.0")
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
        "service": "feature-flag-service",
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
    
    if status["status"] == "unhealthy":
        raise HTTPException(status_code=503, detail=status)
    
    return status

# Metrics endpoint
@app.get("/metrics")
async def metrics():
    return Response(generate_latest(), media_type="text/plain")

# Feature flag management endpoints
@app.post("/v1/flags")
async def create_feature_flag(
    request: CreateFeatureFlagRequest,
    background_tasks: BackgroundTasks,
    db = Depends(get_db)
):
    """Create a new feature flag"""
    try:
        # Check if flag already exists
        async with db.acquire() as conn:
            existing = await conn.fetchval(
                "SELECT EXISTS(SELECT 1 FROM feature_flags WHERE key = $1)",
                request.flag.key
            )
            
            if existing:
                raise HTTPException(
                    status_code=409,
                    detail=f"Feature flag with key '{request.flag.key}' already exists"
                )
        
        # Create flag
        flag_id = await create_flag_in_db(request.flag, request.rollout_rules)
        
        # Invalidate cache
        background_tasks.add_task(invalidate_flag_cache, request.flag.key)
        
        # Update metrics
        active_flags.inc()
        
        return {
            "flag_id": flag_id,
            "flag_key": request.flag.key,
            "message": "Feature flag created successfully"
        }
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error creating feature flag: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to create feature flag: {str(e)}")

@app.get("/v1/flags/{flag_key}")
async def get_feature_flag(flag_key: str, db = Depends(get_db)):
    """Get feature flag by key"""
    try:
        flag = await get_flag_from_db(flag_key)
        if not flag:
            raise HTTPException(status_code=404, detail="Feature flag not found")
        
        return flag
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error getting feature flag: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to get feature flag: {str(e)}")

@app.get("/v1/flags")
async def list_feature_flags(
    status: Optional[FlagStatus] = None,
    project_id: Optional[str] = None,
    tag: Optional[str] = None,
    limit: int = 50,
    offset: int = 0,
    db = Depends(get_db)
):
    """List feature flags with filtering"""
    try:
        flags = await list_flags_from_db(status, project_id, tag, limit, offset)
        
        return {
            "flags": flags,
            "total": len(flags),
            "limit": limit,
            "offset": offset
        }
        
    except Exception as e:
        logger.error(f"Error listing feature flags: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to list feature flags: {str(e)}")

@app.put("/v1/flags/{flag_key}")
async def update_feature_flag(
    flag_key: str,
    request: UpdateFeatureFlagRequest,
    background_tasks: BackgroundTasks,
    db = Depends(get_db)
):
    """Update feature flag"""
    try:
        # Check if flag exists
        flag = await get_flag_from_db(flag_key)
        if not flag:
            raise HTTPException(status_code=404, detail="Feature flag not found")
        
        # Update flag
        await update_flag_in_db(flag_key, request)
        
        # Invalidate cache
        background_tasks.add_task(invalidate_flag_cache, flag_key)
        
        return {"message": "Feature flag updated successfully"}
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error updating feature flag: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to update feature flag: {str(e)}")

@app.delete("/v1/flags/{flag_key}")
async def delete_feature_flag(
    flag_key: str,
    background_tasks: BackgroundTasks,
    db = Depends(get_db)
):
    """Delete feature flag"""
    try:
        # Check if flag exists
        flag = await get_flag_from_db(flag_key)
        if not flag:
            raise HTTPException(status_code=404, detail="Feature flag not found")
        
        # Archive flag instead of deleting
        await update_flag_in_db(flag_key, UpdateFeatureFlagRequest(status=FlagStatus.ARCHIVED))
        
        # Invalidate cache
        background_tasks.add_task(invalidate_flag_cache, flag_key)
        
        # Update metrics
        active_flags.dec()
        
        return {"message": "Feature flag archived successfully"}
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error deleting feature flag: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to delete feature flag: {str(e)}")

# Flag evaluation endpoints
@app.post("/v1/evaluate", response_model=FlagEvaluationResponse)
async def evaluate_feature_flag(request: FlagEvaluationRequest):
    """Evaluate a feature flag for a user"""
    start_time = datetime.utcnow()
    
    try:
        # Try cache first
        cached_result = await get_cached_evaluation(request.flag_key, request.context.user_id)
        if cached_result:
            cache_hit_rate.inc()
            return cached_result
        
        # Get flag configuration
        flag = await get_flag_from_cache_or_db(request.flag_key)
        if not flag or flag.get('status') != FlagStatus.ACTIVE.value:
            # Return default value
            result = FlagEvaluationResponse(
                flag_key=request.flag_key,
                value=request.default_value if request.default_value is not None else False,
                reason="flag_not_found_or_inactive",
                timestamp=datetime.utcnow()
            )
        else:
            # Evaluate flag
            result = await evaluate_flag(flag, request.context)
        
        # Cache result
        await cache_evaluation_result(request.flag_key, request.context.user_id, result)
        
        # Update metrics
        duration = (datetime.utcnow() - start_time).total_seconds()
        flag_evaluation_duration.labels(flag_key=request.flag_key).observe(duration)
        flag_evaluations_total.labels(
            flag_key=request.flag_key,
            variation=str(result.variation_id or "default"),
            reason=result.reason
        ).inc()
        
        return result
        
    except Exception as e:
        logger.error(f"Error evaluating feature flag: {str(e)}")
        # Return default value on error
        return FlagEvaluationResponse(
            flag_key=request.flag_key,
            value=request.default_value if request.default_value is not None else False,
            reason="evaluation_error",
            timestamp=datetime.utcnow()
        )

@app.post("/v1/evaluate/batch")
async def evaluate_multiple_flags(
    flag_keys: List[str],
    context: EvaluationContext,
    default_values: Optional[Dict[str, Any]] = None
):
    """Evaluate multiple feature flags for a user"""
    try:
        results = {}
        
        for flag_key in flag_keys:
            default_value = default_values.get(flag_key) if default_values else None
            request = FlagEvaluationRequest(
                flag_key=flag_key,
                context=context,
                default_value=default_value
            )
            
            result = await evaluate_feature_flag(request)
            results[flag_key] = result
        
        return {
            "evaluations": results,
            "context": context,
            "timestamp": datetime.utcnow()
        }
        
    except Exception as e:
        logger.error(f"Error evaluating multiple flags: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to evaluate flags: {str(e)}")

# Experiment management endpoints
@app.post("/v1/experiments")
async def create_experiment(experiment: Experiment, db = Depends(get_db)):
    """Create a new A/B test experiment"""
    try:
        experiment_id = await create_experiment_in_db(experiment)
        
        # Update metrics
        active_experiments.inc()
        
        return {
            "experiment_id": experiment_id,
            "message": "Experiment created successfully"
        }
        
    except Exception as e:
        logger.error(f"Error creating experiment: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to create experiment: {str(e)}")

@app.get("/v1/experiments")
async def list_experiments(
    status: Optional[ExperimentStatus] = None,
    flag_key: Optional[str] = None,
    limit: int = 50,
    offset: int = 0,
    db = Depends(get_db)
):
    """List experiments"""
    try:
        experiments = await list_experiments_from_db(status, flag_key, limit, offset)
        
        return {
            "experiments": experiments,
            "total": len(experiments),
            "limit": limit,
            "offset": offset
        }
        
    except Exception as e:
        logger.error(f"Error listing experiments: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to list experiments: {str(e)}")

# Analytics endpoints
@app.get("/v1/analytics/flags/{flag_key}")
async def get_flag_analytics(
    flag_key: str,
    start_date: Optional[str] = None,
    end_date: Optional[str] = None,
    db = Depends(get_db)
):
    """Get analytics for a specific feature flag"""
    try:
        analytics = await get_flag_analytics_from_db(flag_key, start_date, end_date)
        return analytics
        
    except Exception as e:
        logger.error(f"Error getting flag analytics: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to get analytics: {str(e)}")

# Helper functions
async def evaluate_flag(flag: Dict[str, Any], context: EvaluationContext) -> FlagEvaluationResponse:
    """Evaluate a feature flag based on rollout rules"""
    
    # Get rollout rules
    rules = flag.get('rollout_rules', [])
    
    for rule in rules:
        if await matches_rule(rule, context):
            return FlagEvaluationResponse(
                flag_key=flag['key'],
                value=rule['value'],
                variation_id=rule.get('id'),
                reason=f"matched_rule_{rule['strategy']}",
                timestamp=datetime.utcnow()
            )
    
    # No rules matched, return default value
    return FlagEvaluationResponse(
        flag_key=flag['key'],
        value=flag['default_value'],
        reason="default_value",
        timestamp=datetime.utcnow()
    )

async def matches_rule(rule: Dict[str, Any], context: EvaluationContext) -> bool:
    """Check if a rollout rule matches the evaluation context"""
    
    strategy = rule['strategy']
    
    if strategy == RolloutStrategy.ALL_USERS.value:
        return True
    
    elif strategy == RolloutStrategy.PERCENTAGE.value:
        # Use consistent hashing for percentage rollout
        hash_input = f"{context.user_id}:{rule.get('id', '')}"
        hash_value = int(hashlib.md5(hash_input.encode()).hexdigest(), 16)
        percentage = (hash_value % 100) + 1
        return percentage <= rule.get('percentage', 0)
    
    elif strategy == RolloutStrategy.USER_LIST.value:
        user_ids = rule.get('user_ids', [])
        return context.user_id in user_ids
    
    elif strategy == RolloutStrategy.USER_ATTRIBUTES.value:
        attributes = rule.get('attributes', {})
        for key, expected_value in attributes.items():
            if context.attributes.get(key) != expected_value:
                return False
        return True
    
    return False

# Database functions (simplified implementations)
async def create_flag_in_db(flag: FeatureFlag, rollout_rules: List[RolloutRule]) -> str:
    """Create feature flag in database"""
    # Implementation would create flag and rules in database
    return "flag_id_placeholder"

async def get_flag_from_db(flag_key: str) -> Optional[Dict[str, Any]]:
    """Get feature flag from database"""
    # Implementation would fetch flag from database
    return None

async def create_database_tables():
    """Create database tables for feature flags"""
    create_tables_sql = """
    CREATE TABLE IF NOT EXISTS feature_flags (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        key VARCHAR(255) UNIQUE NOT NULL,
        name VARCHAR(255) NOT NULL,
        description TEXT,
        type VARCHAR(50) NOT NULL,
        default_value JSONB NOT NULL,
        status VARCHAR(50) NOT NULL DEFAULT 'active',
        tags TEXT[] DEFAULT '{}',
        created_by VARCHAR(255) NOT NULL,
        project_id VARCHAR(255),
        created_at TIMESTAMP DEFAULT NOW(),
        updated_at TIMESTAMP DEFAULT NOW()
    );
    
    CREATE TABLE IF NOT EXISTS rollout_rules (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        flag_id UUID NOT NULL REFERENCES feature_flags(id),
        strategy VARCHAR(50) NOT NULL,
        percentage FLOAT,
        user_ids TEXT[],
        attributes JSONB,
        value JSONB NOT NULL,
        description TEXT,
        created_at TIMESTAMP DEFAULT NOW()
    );
    
    CREATE TABLE IF NOT EXISTS experiments (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        name VARCHAR(255) NOT NULL,
        description TEXT,
        flag_key VARCHAR(255) NOT NULL,
        status VARCHAR(50) NOT NULL DEFAULT 'draft',
        traffic_allocation FLOAT NOT NULL DEFAULT 100.0,
        variations JSONB NOT NULL,
        metrics TEXT[] DEFAULT '{}',
        start_date TIMESTAMP,
        end_date TIMESTAMP,
        created_by VARCHAR(255) NOT NULL,
        created_at TIMESTAMP DEFAULT NOW(),
        updated_at TIMESTAMP DEFAULT NOW()
    );
    
    CREATE INDEX IF NOT EXISTS idx_feature_flags_key ON feature_flags(key);
    CREATE INDEX IF NOT EXISTS idx_feature_flags_status ON feature_flags(status);
    CREATE INDEX IF NOT EXISTS idx_rollout_rules_flag_id ON rollout_rules(flag_id);
    CREATE INDEX IF NOT EXISTS idx_experiments_flag_key ON experiments(flag_key);
    """
    
    try:
        async with db_pool.acquire() as conn:
            await conn.execute(create_tables_sql)
        logger.info("Database tables created successfully")
    except Exception as e:
        logger.error(f"Error creating database tables: {str(e)}")

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
        
        logger.info("Feature Flag service started successfully")
        
    except Exception as e:
        logger.error(f"Failed to start feature flag service: {str(e)}")
        raise

@app.on_event("shutdown")
async def shutdown_event():
    global db_pool, redis_client
    
    try:
        if db_pool:
            await db_pool.close()
        if redis_client:
            await redis_client.close()
        
        logger.info("Feature Flag service shut down successfully")
        
    except Exception as e:
        logger.error(f"Error during shutdown: {str(e)}")

# Additional helper functions would be implemented here...

if __name__ == "__main__":
    uvicorn.run(
        "main:app",
        host="0.0.0.0",
        port=config.port,
        log_level="info",
        reload=config.environment == "development"
    )
