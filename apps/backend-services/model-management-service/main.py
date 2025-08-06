"""
Nexus Platform - Model Management Service
Handles ML model lifecycle, versioning, and metadata management
"""

import os
import asyncio
import logging
from datetime import datetime
from typing import List, Optional, Dict, Any
from contextlib import asynccontextmanager

import uvicorn
from fastapi import FastAPI, HTTPException, Depends, BackgroundTasks
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel, Field
import mlflow
import mlflow.sklearn
import mlflow.tensorflow
import mlflow.pytorch
from sqlalchemy import create_engine, Column, String, DateTime, Text, Integer, Float, Boolean
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.orm import sessionmaker, Session
from sqlalchemy.dialects.postgresql import UUID
import uuid
import redis
import json
import httpx

# Configuration
DATABASE_URL = os.getenv("DATABASE_URL", "postgresql://nexus:nexus-password@postgres:5432/nexus")
REDIS_URL = os.getenv("REDIS_URL", "redis://:nexus-password@redis:6379/0")
MLFLOW_TRACKING_URI = os.getenv("MLFLOW_TRACKING_URI", "http://mlflow-server:5000")
SERVICE_PORT = int(os.getenv("PORT", "8086"))

# Setup logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Database setup
engine = create_engine(DATABASE_URL)
SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)
Base = declarative_base()

# Redis setup
redis_client = redis.from_url(REDIS_URL, decode_responses=True)

# MLflow setup
mlflow.set_tracking_uri(MLFLOW_TRACKING_URI)

# Database Models
class ModelRecord(Base):
    __tablename__ = "models"
    
    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    name = Column(String(255), nullable=False, index=True)
    version = Column(String(50), nullable=False)
    framework = Column(String(50), nullable=False)  # sklearn, tensorflow, pytorch, etc.
    model_type = Column(String(100), nullable=False)  # classifier, regressor, etc.
    description = Column(Text)
    tags = Column(Text)  # JSON string
    metrics = Column(Text)  # JSON string
    parameters = Column(Text)  # JSON string
    artifact_uri = Column(String(500))
    mlflow_run_id = Column(String(255))
    status = Column(String(50), default="training")  # training, ready, deployed, archived
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)
    created_by = Column(String(255))

class ModelDeployment(Base):
    __tablename__ = "model_deployments"
    
    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    model_id = Column(UUID(as_uuid=True), nullable=False)
    deployment_name = Column(String(255), nullable=False)
    endpoint_url = Column(String(500))
    environment = Column(String(50), default="development")  # development, staging, production
    status = Column(String(50), default="deploying")  # deploying, active, inactive, failed
    replicas = Column(Integer, default=1)
    cpu_request = Column(String(20), default="100m")
    memory_request = Column(String(20), default="256Mi")
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)

# Create tables
Base.metadata.create_all(bind=engine)

# Pydantic Models
class ModelCreate(BaseModel):
    name: str = Field(..., description="Model name")
    framework: str = Field(..., description="ML framework (sklearn, tensorflow, pytorch)")
    model_type: str = Field(..., description="Model type (classifier, regressor, etc.)")
    description: Optional[str] = None
    tags: Optional[Dict[str, str]] = None
    mlflow_run_id: Optional[str] = None

class ModelUpdate(BaseModel):
    description: Optional[str] = None
    tags: Optional[Dict[str, str]] = None
    status: Optional[str] = None

class ModelResponse(BaseModel):
    id: str
    name: str
    version: str
    framework: str
    model_type: str
    description: Optional[str]
    tags: Optional[Dict[str, str]]
    metrics: Optional[Dict[str, float]]
    parameters: Optional[Dict[str, Any]]
    artifact_uri: Optional[str]
    status: str
    created_at: datetime
    updated_at: datetime
    created_by: Optional[str]

class DeploymentCreate(BaseModel):
    model_id: str
    deployment_name: str
    environment: str = "development"
    replicas: int = 1
    cpu_request: str = "100m"
    memory_request: str = "256Mi"

class DeploymentResponse(BaseModel):
    id: str
    model_id: str
    deployment_name: str
    endpoint_url: Optional[str]
    environment: str
    status: str
    replicas: int
    created_at: datetime
    updated_at: datetime

class PredictionRequest(BaseModel):
    model_id: str
    input_data: Dict[str, Any]
    deployment_name: Optional[str] = None

class PredictionResponse(BaseModel):
    prediction: Any
    model_id: str
    model_version: str
    prediction_id: str
    timestamp: datetime

# Dependency to get database session
def get_db():
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()

# Lifespan context manager
@asynccontextmanager
async def lifespan(app: FastAPI):
    # Startup
    logger.info("🚀 Model Management Service starting up...")
    
    # Test connections
    try:
        # Test database
        db = SessionLocal()
        db.execute("SELECT 1")
        db.close()
        logger.info("✅ Database connection successful")
        
        # Test Redis
        redis_client.ping()
        logger.info("✅ Redis connection successful")
        
        # Test MLflow
        mlflow.set_tracking_uri(MLFLOW_TRACKING_URI)
        logger.info("✅ MLflow connection configured")
        
    except Exception as e:
        logger.error(f"❌ Startup connection test failed: {e}")
    
    yield
    
    # Shutdown
    logger.info("🛑 Model Management Service shutting down...")

# Create FastAPI app
app = FastAPI(
    title="Nexus Model Management Service",
    description="AI/ML Model Lifecycle Management for Nexus Platform",
    version="1.0.0",
    lifespan=lifespan
)

# Add CORS middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Health check endpoint
@app.get("/health")
async def health_check():
    """Health check endpoint"""
    try:
        # Check database
        db = SessionLocal()
        db.execute("SELECT 1")
        db.close()
        
        # Check Redis
        redis_client.ping()
        
        return {
            "status": "healthy",
            "service": "model-management-service",
            "timestamp": datetime.utcnow().isoformat(),
            "version": "1.0.0"
        }
    except Exception as e:
        raise HTTPException(status_code=503, detail=f"Service unhealthy: {str(e)}")

# Model Management Endpoints
@app.post("/models", response_model=ModelResponse)
async def create_model(model: ModelCreate, db: Session = Depends(get_db)):
    """Create a new model record"""
    try:
        # Generate version
        existing_models = db.query(ModelRecord).filter(ModelRecord.name == model.name).count()
        version = f"v{existing_models + 1}"
        
        # Create model record
        db_model = ModelRecord(
            name=model.name,
            version=version,
            framework=model.framework,
            model_type=model.model_type,
            description=model.description,
            tags=json.dumps(model.tags) if model.tags else None,
            mlflow_run_id=model.mlflow_run_id,
            created_by="system"  # TODO: Get from auth context
        )
        
        db.add(db_model)
        db.commit()
        db.refresh(db_model)
        
        # Cache in Redis
        cache_key = f"model:{db_model.id}"
        model_data = {
            "id": str(db_model.id),
            "name": db_model.name,
            "version": db_model.version,
            "status": db_model.status
        }
        redis_client.setex(cache_key, 3600, json.dumps(model_data))
        
        logger.info(f"✅ Created model: {model.name} {version}")
        
        return ModelResponse(
            id=str(db_model.id),
            name=db_model.name,
            version=db_model.version,
            framework=db_model.framework,
            model_type=db_model.model_type,
            description=db_model.description,
            tags=json.loads(db_model.tags) if db_model.tags else None,
            metrics=json.loads(db_model.metrics) if db_model.metrics else None,
            parameters=json.loads(db_model.parameters) if db_model.parameters else None,
            artifact_uri=db_model.artifact_uri,
            status=db_model.status,
            created_at=db_model.created_at,
            updated_at=db_model.updated_at,
            created_by=db_model.created_by
        )
        
    except Exception as e:
        logger.error(f"❌ Error creating model: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/models", response_model=List[ModelResponse])
async def list_models(
    skip: int = 0, 
    limit: int = 100, 
    status: Optional[str] = None,
    framework: Optional[str] = None,
    db: Session = Depends(get_db)
):
    """List all models with optional filtering"""
    try:
        query = db.query(ModelRecord)
        
        if status:
            query = query.filter(ModelRecord.status == status)
        if framework:
            query = query.filter(ModelRecord.framework == framework)
        
        models = query.offset(skip).limit(limit).all()
        
        return [
            ModelResponse(
                id=str(model.id),
                name=model.name,
                version=model.version,
                framework=model.framework,
                model_type=model.model_type,
                description=model.description,
                tags=json.loads(model.tags) if model.tags else None,
                metrics=json.loads(model.metrics) if model.metrics else None,
                parameters=json.loads(model.parameters) if model.parameters else None,
                artifact_uri=model.artifact_uri,
                status=model.status,
                created_at=model.created_at,
                updated_at=model.updated_at,
                created_by=model.created_by
            )
            for model in models
        ]
        
    except Exception as e:
        logger.error(f"❌ Error listing models: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/models/{model_id}", response_model=ModelResponse)
async def get_model(model_id: str, db: Session = Depends(get_db)):
    """Get a specific model by ID"""
    try:
        # Try cache first
        cache_key = f"model:{model_id}"
        cached_data = redis_client.get(cache_key)
        
        if cached_data:
            cached_model = json.loads(cached_data)
            logger.info(f"📦 Retrieved model from cache: {model_id}")
        
        # Get from database
        model = db.query(ModelRecord).filter(ModelRecord.id == model_id).first()
        if not model:
            raise HTTPException(status_code=404, detail="Model not found")
        
        return ModelResponse(
            id=str(model.id),
            name=model.name,
            version=model.version,
            framework=model.framework,
            model_type=model.model_type,
            description=model.description,
            tags=json.loads(model.tags) if model.tags else None,
            metrics=json.loads(model.metrics) if model.metrics else None,
            parameters=json.loads(model.parameters) if model.parameters else None,
            artifact_uri=model.artifact_uri,
            status=model.status,
            created_at=model.created_at,
            updated_at=model.updated_at,
            created_by=model.created_by
        )
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"❌ Error getting model: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.put("/models/{model_id}", response_model=ModelResponse)
async def update_model(model_id: str, model_update: ModelUpdate, db: Session = Depends(get_db)):
    """Update a model record"""
    try:
        model = db.query(ModelRecord).filter(ModelRecord.id == model_id).first()
        if not model:
            raise HTTPException(status_code=404, detail="Model not found")
        
        # Update fields
        if model_update.description is not None:
            model.description = model_update.description
        if model_update.tags is not None:
            model.tags = json.dumps(model_update.tags)
        if model_update.status is not None:
            model.status = model_update.status
        
        model.updated_at = datetime.utcnow()
        
        db.commit()
        db.refresh(model)
        
        # Update cache
        cache_key = f"model:{model_id}"
        redis_client.delete(cache_key)
        
        logger.info(f"✅ Updated model: {model_id}")
        
        return ModelResponse(
            id=str(model.id),
            name=model.name,
            version=model.version,
            framework=model.framework,
            model_type=model.model_type,
            description=model.description,
            tags=json.loads(model.tags) if model.tags else None,
            metrics=json.loads(model.metrics) if model.metrics else None,
            parameters=json.loads(model.parameters) if model.parameters else None,
            artifact_uri=model.artifact_uri,
            status=model.status,
            created_at=model.created_at,
            updated_at=model.updated_at,
            created_by=model.created_by
        )
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"❌ Error updating model: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.delete("/models/{model_id}")
async def delete_model(model_id: str, db: Session = Depends(get_db)):
    """Delete a model record"""
    try:
        model = db.query(ModelRecord).filter(ModelRecord.id == model_id).first()
        if not model:
            raise HTTPException(status_code=404, detail="Model not found")
        
        # Check if model has active deployments
        active_deployments = db.query(ModelDeployment).filter(
            ModelDeployment.model_id == model_id,
            ModelDeployment.status == "active"
        ).count()
        
        if active_deployments > 0:
            raise HTTPException(
                status_code=400, 
                detail="Cannot delete model with active deployments"
            )
        
        db.delete(model)
        db.commit()
        
        # Remove from cache
        cache_key = f"model:{model_id}"
        redis_client.delete(cache_key)
        
        logger.info(f"✅ Deleted model: {model_id}")
        
        return {"message": "Model deleted successfully"}
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"❌ Error deleting model: {e}")
        raise HTTPException(status_code=500, detail=str(e))

# Model Deployment Endpoints
@app.post("/deployments", response_model=DeploymentResponse)
async def create_deployment(deployment: DeploymentCreate, background_tasks: BackgroundTasks, db: Session = Depends(get_db)):
    """Create a new model deployment"""
    try:
        # Verify model exists
        model = db.query(ModelRecord).filter(ModelRecord.id == deployment.model_id).first()
        if not model:
            raise HTTPException(status_code=404, detail="Model not found")
        
        if model.status != "ready":
            raise HTTPException(status_code=400, detail="Model is not ready for deployment")
        
        # Create deployment record
        db_deployment = ModelDeployment(
            model_id=deployment.model_id,
            deployment_name=deployment.deployment_name,
            environment=deployment.environment,
            replicas=deployment.replicas,
            cpu_request=deployment.cpu_request,
            memory_request=deployment.memory_request
        )
        
        db.add(db_deployment)
        db.commit()
        db.refresh(db_deployment)
        
        # Start deployment process in background
        background_tasks.add_task(deploy_model_async, str(db_deployment.id), model.artifact_uri)
        
        logger.info(f"✅ Created deployment: {deployment.deployment_name}")
        
        return DeploymentResponse(
            id=str(db_deployment.id),
            model_id=str(db_deployment.model_id),
            deployment_name=db_deployment.deployment_name,
            endpoint_url=db_deployment.endpoint_url,
            environment=db_deployment.environment,
            status=db_deployment.status,
            replicas=db_deployment.replicas,
            created_at=db_deployment.created_at,
            updated_at=db_deployment.updated_at
        )
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"❌ Error creating deployment: {e}")
        raise HTTPException(status_code=500, detail=str(e))

async def deploy_model_async(deployment_id: str, artifact_uri: str):
    """Async function to deploy model"""
    try:
        db = SessionLocal()
        deployment = db.query(ModelDeployment).filter(ModelDeployment.id == deployment_id).first()
        
        if deployment:
            # Simulate deployment process
            await asyncio.sleep(5)  # Simulate deployment time
            
            # Update deployment status
            deployment.status = "active"
            deployment.endpoint_url = f"http://model-serving:8501/v1/models/{deployment.deployment_name}:predict"
            deployment.updated_at = datetime.utcnow()
            
            db.commit()
            
            logger.info(f"✅ Deployment completed: {deployment_id}")
        
        db.close()
        
    except Exception as e:
        logger.error(f"❌ Deployment failed: {e}")
        # Update deployment status to failed
        db = SessionLocal()
        deployment = db.query(ModelDeployment).filter(ModelDeployment.id == deployment_id).first()
        if deployment:
            deployment.status = "failed"
            deployment.updated_at = datetime.utcnow()
            db.commit()
        db.close()

@app.get("/deployments", response_model=List[DeploymentResponse])
async def list_deployments(
    skip: int = 0, 
    limit: int = 100, 
    environment: Optional[str] = None,
    status: Optional[str] = None,
    db: Session = Depends(get_db)
):
    """List all deployments"""
    try:
        query = db.query(ModelDeployment)
        
        if environment:
            query = query.filter(ModelDeployment.environment == environment)
        if status:
            query = query.filter(ModelDeployment.status == status)
        
        deployments = query.offset(skip).limit(limit).all()
        
        return [
            DeploymentResponse(
                id=str(deployment.id),
                model_id=str(deployment.model_id),
                deployment_name=deployment.deployment_name,
                endpoint_url=deployment.endpoint_url,
                environment=deployment.environment,
                status=deployment.status,
                replicas=deployment.replicas,
                created_at=deployment.created_at,
                updated_at=deployment.updated_at
            )
            for deployment in deployments
        ]
        
    except Exception as e:
        logger.error(f"❌ Error listing deployments: {e}")
        raise HTTPException(status_code=500, detail=str(e))

# Prediction endpoint
@app.post("/predict", response_model=PredictionResponse)
async def make_prediction(request: PredictionRequest, db: Session = Depends(get_db)):
    """Make a prediction using a deployed model"""
    try:
        # Get model info
        model = db.query(ModelRecord).filter(ModelRecord.id == request.model_id).first()
        if not model:
            raise HTTPException(status_code=404, detail="Model not found")
        
        # Find active deployment
        deployment = db.query(ModelDeployment).filter(
            ModelDeployment.model_id == request.model_id,
            ModelDeployment.status == "active"
        ).first()
        
        if not deployment:
            raise HTTPException(status_code=404, detail="No active deployment found for model")
        
        # Make prediction (simulate for now)
        prediction_id = str(uuid.uuid4())
        
        # TODO: Implement actual prediction logic based on model framework
        # For now, return a mock prediction
        mock_prediction = {
            "class": "positive",
            "probability": 0.85,
            "confidence": "high"
        }
        
        # Log prediction for monitoring
        prediction_log = {
            "prediction_id": prediction_id,
            "model_id": request.model_id,
            "model_version": model.version,
            "input_data": request.input_data,
            "prediction": mock_prediction,
            "timestamp": datetime.utcnow().isoformat()
        }
        
        # Store in Redis for monitoring
        redis_client.setex(f"prediction:{prediction_id}", 86400, json.dumps(prediction_log))
        
        logger.info(f"✅ Prediction made: {prediction_id}")
        
        return PredictionResponse(
            prediction=mock_prediction,
            model_id=request.model_id,
            model_version=model.version,
            prediction_id=prediction_id,
            timestamp=datetime.utcnow()
        )
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"❌ Error making prediction: {e}")
        raise HTTPException(status_code=500, detail=str(e))

# Metrics endpoint
@app.get("/metrics")
async def get_metrics():
    """Get service metrics"""
    try:
        db = SessionLocal()
        
        # Count models by status
        model_counts = {}
        for status in ["training", "ready", "deployed", "archived"]:
            count = db.query(ModelRecord).filter(ModelRecord.status == status).count()
            model_counts[f"models_{status}"] = count
        
        # Count deployments by status
        deployment_counts = {}
        for status in ["deploying", "active", "inactive", "failed"]:
            count = db.query(ModelDeployment).filter(ModelDeployment.status == status).count()
            deployment_counts[f"deployments_{status}"] = count
        
        db.close()
        
        return {
            "service": "model-management-service",
            "timestamp": datetime.utcnow().isoformat(),
            **model_counts,
            **deployment_counts
        }
        
    except Exception as e:
        logger.error(f"❌ Error getting metrics: {e}")
        raise HTTPException(status_code=500, detail=str(e))

if __name__ == "__main__":
    uvicorn.run(
        "main:app",
        host="0.0.0.0",
        port=SERVICE_PORT,
        reload=True,
        log_level="info"
    )
