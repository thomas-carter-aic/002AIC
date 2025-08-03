"""
Model Management Service - Python
Core MLOps service for managing AI/ML models in the 002AIC platform
Handles model lifecycle, versioning, deployment, and monitoring
"""

from fastapi import FastAPI, HTTPException, Depends, BackgroundTasks
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel, Field
from typing import List, Optional, Dict, Any
import uvicorn
import os
import logging
from datetime import datetime
import asyncio
import mlflow
import mlflow.tracking
from mlflow.tracking import MlflowClient

# Import our auth middleware
import sys
sys.path.append('/home/oss/002AIC/libs/auth-middleware/python')
from auth_middleware import FastAPIAuthMiddleware, AuthConfig

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Initialize FastAPI app
app = FastAPI(
    title="002AIC Model Management Service",
    description="MLOps service for AI/ML model lifecycle management",
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
    service_name="model-management-service"
)

auth = FastAPIAuthMiddleware(auth_config)

# Initialize MLflow
mlflow.set_tracking_uri(os.getenv("MLFLOW_TRACKING_URI", "http://mlflow:5000"))
mlflow_client = MlflowClient()

# Pydantic models
class ModelMetadata(BaseModel):
    name: str = Field(..., description="Model name")
    version: str = Field(..., description="Model version")
    description: Optional[str] = Field(None, description="Model description")
    framework: str = Field(..., description="ML framework (tensorflow, pytorch, sklearn, etc.)")
    model_type: str = Field(..., description="Model type (classification, regression, nlp, cv, etc.)")
    tags: Dict[str, str] = Field(default_factory=dict, description="Model tags")
    metrics: Dict[str, float] = Field(default_factory=dict, description="Model performance metrics")

class ModelDeploymentConfig(BaseModel):
    model_id: str = Field(..., description="Model ID to deploy")
    deployment_name: str = Field(..., description="Deployment name")
    environment: str = Field(..., description="Target environment (dev, staging, prod)")
    scaling_config: Dict[str, Any] = Field(default_factory=dict, description="Scaling configuration")
    resource_requirements: Dict[str, str] = Field(default_factory=dict, description="Resource requirements")

class ModelResponse(BaseModel):
    id: str
    name: str
    version: str
    status: str
    created_at: datetime
    updated_at: datetime
    metadata: Dict[str, Any]

class DeploymentResponse(BaseModel):
    id: str
    model_id: str
    deployment_name: str
    status: str
    endpoint_url: Optional[str]
    created_at: datetime

# Health check endpoint
@app.get("/health")
async def health_check():
    """Health check endpoint"""
    return {
        "status": "healthy",
        "service": "model-management-service",
        "timestamp": datetime.utcnow().isoformat(),
        "version": "1.0.0",
        "mlflow_status": "connected" if mlflow_client else "disconnected"
    }

# Model management endpoints
@app.get("/v1/models", response_model=List[ModelResponse])
@auth.require_auth("model", "read")
async def list_models(skip: int = 0, limit: int = 100):
    """List all registered models"""
    try:
        # Get models from MLflow
        registered_models = mlflow_client.search_registered_models()
        
        models = []
        for rm in registered_models[skip:skip+limit]:
            latest_version = mlflow_client.get_latest_versions(rm.name, stages=["None", "Staging", "Production"])
            if latest_version:
                version = latest_version[0]
                models.append(ModelResponse(
                    id=f"{rm.name}:{version.version}",
                    name=rm.name,
                    version=version.version,
                    status=version.current_stage,
                    created_at=datetime.fromtimestamp(version.creation_timestamp / 1000),
                    updated_at=datetime.fromtimestamp(version.last_updated_timestamp / 1000),
                    metadata={
                        "description": rm.description,
                        "tags": dict(rm.tags) if rm.tags else {},
                        "run_id": version.run_id
                    }
                ))
        
        return models
    except Exception as e:
        logger.error(f"Error listing models: {e}")
        raise HTTPException(status_code=500, detail="Failed to list models")

@app.post("/v1/models", response_model=ModelResponse)
@auth.require_auth("model", "create")
async def register_model(model_metadata: ModelMetadata):
    """Register a new model"""
    try:
        # Create registered model in MLflow
        registered_model = mlflow_client.create_registered_model(
            name=model_metadata.name,
            description=model_metadata.description,
            tags=model_metadata.tags
        )
        
        # Create initial version
        model_version = mlflow_client.create_model_version(
            name=model_metadata.name,
            source="placeholder",  # This would be the actual model artifact URI
            run_id=None,
            tags=model_metadata.tags,
            description=f"Version {model_metadata.version}"
        )
        
        return ModelResponse(
            id=f"{registered_model.name}:{model_version.version}",
            name=registered_model.name,
            version=model_version.version,
            status="None",
            created_at=datetime.fromtimestamp(model_version.creation_timestamp / 1000),
            updated_at=datetime.fromtimestamp(model_version.last_updated_timestamp / 1000),
            metadata={
                "description": registered_model.description,
                "tags": dict(registered_model.tags) if registered_model.tags else {},
                "framework": model_metadata.framework,
                "model_type": model_metadata.model_type
            }
        )
    except Exception as e:
        logger.error(f"Error registering model: {e}")
        raise HTTPException(status_code=500, detail="Failed to register model")

@app.get("/v1/models/{model_name}", response_model=ModelResponse)
@auth.require_auth("model", "read")
async def get_model(model_name: str, version: Optional[str] = None):
    """Get model details"""
    try:
        if version:
            model_version = mlflow_client.get_model_version(model_name, version)
        else:
            latest_versions = mlflow_client.get_latest_versions(model_name, stages=["Production", "Staging", "None"])
            if not latest_versions:
                raise HTTPException(status_code=404, detail="Model not found")
            model_version = latest_versions[0]
        
        registered_model = mlflow_client.get_registered_model(model_name)
        
        return ModelResponse(
            id=f"{model_name}:{model_version.version}",
            name=model_name,
            version=model_version.version,
            status=model_version.current_stage,
            created_at=datetime.fromtimestamp(model_version.creation_timestamp / 1000),
            updated_at=datetime.fromtimestamp(model_version.last_updated_timestamp / 1000),
            metadata={
                "description": registered_model.description,
                "tags": dict(registered_model.tags) if registered_model.tags else {},
                "run_id": model_version.run_id,
                "source": model_version.source
            }
        )
    except Exception as e:
        logger.error(f"Error getting model {model_name}: {e}")
        raise HTTPException(status_code=404, detail="Model not found")

@app.post("/v1/models/{model_name}/deploy", response_model=DeploymentResponse)
@auth.require_auth("model", "deploy")
async def deploy_model(model_name: str, deployment_config: ModelDeploymentConfig, background_tasks: BackgroundTasks):
    """Deploy a model to a target environment"""
    try:
        # Get the model version
        model_version = mlflow_client.get_model_version(model_name, deployment_config.model_id.split(':')[-1])
        
        # Create deployment record (in a real implementation, this would trigger actual deployment)
        deployment_id = f"deploy_{model_name}_{deployment_config.deployment_name}_{int(datetime.utcnow().timestamp())}"
        
        # Add background task for actual deployment
        background_tasks.add_task(perform_model_deployment, deployment_id, model_version, deployment_config)
        
        return DeploymentResponse(
            id=deployment_id,
            model_id=deployment_config.model_id,
            deployment_name=deployment_config.deployment_name,
            status="deploying",
            endpoint_url=f"https://api.002aic.com/v1/models/{model_name}/predict",
            created_at=datetime.utcnow()
        )
    except Exception as e:
        logger.error(f"Error deploying model {model_name}: {e}")
        raise HTTPException(status_code=500, detail="Failed to deploy model")

@app.post("/v1/models/{model_name}/promote")
@auth.require_auth("model", "update")
async def promote_model(model_name: str, version: str, stage: str):
    """Promote model to a different stage (Staging, Production)"""
    try:
        mlflow_client.transition_model_version_stage(
            name=model_name,
            version=version,
            stage=stage,
            archive_existing_versions=True
        )
        
        return {"message": f"Model {model_name} version {version} promoted to {stage}"}
    except Exception as e:
        logger.error(f"Error promoting model {model_name}: {e}")
        raise HTTPException(status_code=500, detail="Failed to promote model")

@app.delete("/v1/models/{model_name}")
@auth.require_auth("model", "delete")
async def delete_model(model_name: str):
    """Delete a registered model"""
    try:
        mlflow_client.delete_registered_model(model_name)
        return {"message": f"Model {model_name} deleted successfully"}
    except Exception as e:
        logger.error(f"Error deleting model {model_name}: {e}")
        raise HTTPException(status_code=500, detail="Failed to delete model")

# Background task for model deployment
async def perform_model_deployment(deployment_id: str, model_version, deployment_config: ModelDeploymentConfig):
    """Background task to perform actual model deployment"""
    try:
        logger.info(f"Starting deployment {deployment_id}")
        
        # Simulate deployment process
        await asyncio.sleep(5)  # Simulate deployment time
        
        # In a real implementation, this would:
        # 1. Pull model artifacts from MLflow
        # 2. Create Kubernetes deployment
        # 3. Set up service and ingress
        # 4. Configure monitoring and logging
        # 5. Run health checks
        
        logger.info(f"Deployment {deployment_id} completed successfully")
    except Exception as e:
        logger.error(f"Deployment {deployment_id} failed: {e}")

if __name__ == "__main__":
    uvicorn.run(
        "main:app",
        host="0.0.0.0",
        port=int(os.getenv("PORT", 8080)),
        reload=os.getenv("ENVIRONMENT") == "development"
    )
