"""
Recommendation Engine Service - Python
AI-powered recommendation system for the 002AIC platform
Handles collaborative filtering, content-based filtering, and hybrid recommendations
"""

import asyncio
import json
import logging
import os
import pickle
from datetime import datetime, timedelta
from typing import Dict, List, Optional, Any, Tuple
from dataclasses import dataclass
import numpy as np
import pandas as pd
from scipy.sparse import csr_matrix
from scipy.spatial.distance import cosine
from sklearn.feature_extraction.text import TfidfVectorizer
from sklearn.metrics.pairwise import cosine_similarity
from sklearn.decomposition import TruncatedSVD
from sklearn.cluster import KMeans
import implicit

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
        self.database_url = os.getenv("DATABASE_URL", "postgresql://postgres:password@localhost:5432/recommendations")
        self.redis_url = os.getenv("REDIS_URL", "redis://localhost:6379")
        self.environment = os.getenv("ENVIRONMENT", "development")
        self.model_update_interval = int(os.getenv("MODEL_UPDATE_INTERVAL", "3600"))  # 1 hour
        self.cache_ttl = int(os.getenv("CACHE_TTL", "1800"))  # 30 minutes
        self.min_interactions = int(os.getenv("MIN_INTERACTIONS", "5"))
        self.max_recommendations = int(os.getenv("MAX_RECOMMENDATIONS", "50"))

config = Config()

# Recommendation types
class RecommendationType(str):
    COLLABORATIVE = "collaborative"
    CONTENT_BASED = "content_based"
    HYBRID = "hybrid"
    TRENDING = "trending"
    SIMILAR_USERS = "similar_users"
    POPULAR = "popular"

# Interaction types
class InteractionType(str):
    VIEW = "view"
    LIKE = "like"
    SHARE = "share"
    PURCHASE = "purchase"
    DOWNLOAD = "download"
    RATING = "rating"
    BOOKMARK = "bookmark"
    COMMENT = "comment"

# Pydantic models
class UserInteraction(BaseModel):
    user_id: str
    item_id: str
    interaction_type: InteractionType
    value: float = Field(default=1.0, ge=0.0, le=5.0)
    timestamp: Optional[datetime] = None
    context: Dict[str, Any] = Field(default_factory=dict)

class ItemMetadata(BaseModel):
    item_id: str
    title: str
    description: Optional[str] = None
    category: str
    tags: List[str] = Field(default_factory=list)
    features: Dict[str, Any] = Field(default_factory=dict)
    created_at: Optional[datetime] = None

class RecommendationRequest(BaseModel):
    user_id: str
    recommendation_type: RecommendationType = RecommendationType.HYBRID
    num_recommendations: int = Field(default=10, ge=1, le=50)
    exclude_items: List[str] = Field(default_factory=list)
    include_categories: List[str] = Field(default_factory=list)
    exclude_categories: List[str] = Field(default_factory=list)
    context: Dict[str, Any] = Field(default_factory=dict)
    explain: bool = False

class RecommendationResponse(BaseModel):
    user_id: str
    recommendations: List[Dict[str, Any]]
    recommendation_type: RecommendationType
    generated_at: datetime
    model_version: str
    total_items: int
    explanation: Optional[Dict[str, Any]] = None

class SimilarItemsRequest(BaseModel):
    item_id: str
    num_similar: int = Field(default=10, ge=1, le=50)
    similarity_threshold: float = Field(default=0.1, ge=0.0, le=1.0)

class UserProfileResponse(BaseModel):
    user_id: str
    preferences: Dict[str, float]
    categories: Dict[str, float]
    tags: Dict[str, float]
    interaction_count: int
    last_activity: Optional[datetime]

# Prometheus metrics
recommendations_generated = Counter(
    'recommendations_generated_total',
    'Total recommendations generated',
    ['recommendation_type', 'user_type']
)

recommendation_latency = Histogram(
    'recommendation_latency_seconds',
    'Time taken to generate recommendations',
    ['recommendation_type']
)

model_training_duration = Histogram(
    'model_training_duration_seconds',
    'Time taken to train recommendation models',
    ['model_type']
)

active_users = Gauge(
    'recommendation_active_users',
    'Number of users with recent interactions'
)

total_items = Gauge(
    'recommendation_total_items',
    'Total number of items in catalog'
)

cache_hit_rate = Gauge(
    'recommendation_cache_hit_rate',
    'Cache hit rate for recommendations'
)

# Global variables
app = FastAPI(title="Recommendation Engine Service", version="1.0.0")
db_pool = None
redis_client = None

# Recommendation models
collaborative_model = None
content_model = None
item_features_matrix = None
user_item_matrix = None
item_similarity_matrix = None

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
        "service": "recommendation-engine-service",
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
    
    # Check model status
    global collaborative_model, content_model
    status["models"] = {
        "collaborative": collaborative_model is not None,
        "content_based": content_model is not None
    }
    
    if status["status"] == "unhealthy":
        raise HTTPException(status_code=503, detail=status)
    
    return status

# Metrics endpoint
@app.get("/metrics")
async def metrics():
    return Response(generate_latest(), media_type="text/plain")

# Interaction tracking endpoints
@app.post("/v1/interactions")
async def track_interaction(
    interaction: UserInteraction,
    background_tasks: BackgroundTasks,
    db = Depends(get_db)
):
    """Track user interaction with an item"""
    try:
        # Store interaction in database
        async with db.acquire() as conn:
            await conn.execute("""
                INSERT INTO user_interactions (
                    user_id, item_id, interaction_type, value, timestamp, context
                ) VALUES ($1, $2, $3, $4, $5, $6)
                ON CONFLICT (user_id, item_id, interaction_type) 
                DO UPDATE SET value = $3, timestamp = $5, context = $6
            """, interaction.user_id, interaction.item_id, interaction.interaction_type.value,
                interaction.value, interaction.timestamp or datetime.utcnow(),
                json.dumps(interaction.context))
        
        # Update user profile in background
        background_tasks.add_task(update_user_profile, interaction.user_id)
        
        # Invalidate cached recommendations
        background_tasks.add_task(invalidate_user_cache, interaction.user_id)
        
        return {"status": "success", "message": "Interaction tracked"}
        
    except Exception as e:
        logger.error(f"Error tracking interaction: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to track interaction: {str(e)}")

@app.post("/v1/interactions/batch")
async def track_batch_interactions(
    interactions: List[UserInteraction],
    background_tasks: BackgroundTasks,
    db = Depends(get_db)
):
    """Track multiple user interactions"""
    try:
        # Store interactions in database
        async with db.acquire() as conn:
            for interaction in interactions:
                await conn.execute("""
                    INSERT INTO user_interactions (
                        user_id, item_id, interaction_type, value, timestamp, context
                    ) VALUES ($1, $2, $3, $4, $5, $6)
                    ON CONFLICT (user_id, item_id, interaction_type) 
                    DO UPDATE SET value = $3, timestamp = $5, context = $6
                """, interaction.user_id, interaction.item_id, interaction.interaction_type.value,
                    interaction.value, interaction.timestamp or datetime.utcnow(),
                    json.dumps(interaction.context))
        
        # Update user profiles in background
        user_ids = list(set(interaction.user_id for interaction in interactions))
        for user_id in user_ids:
            background_tasks.add_task(update_user_profile, user_id)
            background_tasks.add_task(invalidate_user_cache, user_id)
        
        return {
            "status": "success", 
            "message": f"Tracked {len(interactions)} interactions",
            "affected_users": len(user_ids)
        }
        
    except Exception as e:
        logger.error(f"Error tracking batch interactions: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to track interactions: {str(e)}")

# Item metadata endpoints
@app.post("/v1/items")
async def add_item_metadata(
    item: ItemMetadata,
    background_tasks: BackgroundTasks,
    db = Depends(get_db)
):
    """Add or update item metadata"""
    try:
        async with db.acquire() as conn:
            await conn.execute("""
                INSERT INTO item_metadata (
                    item_id, title, description, category, tags, features, created_at
                ) VALUES ($1, $2, $3, $4, $5, $6, $7)
                ON CONFLICT (item_id) 
                DO UPDATE SET title = $2, description = $3, category = $4, 
                             tags = $5, features = $6, updated_at = NOW()
            """, item.item_id, item.title, item.description, item.category,
                item.tags, json.dumps(item.features), 
                item.created_at or datetime.utcnow())
        
        # Update content model in background
        background_tasks.add_task(update_content_model)
        
        return {"status": "success", "message": "Item metadata added"}
        
    except Exception as e:
        logger.error(f"Error adding item metadata: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to add item: {str(e)}")

# Recommendation endpoints
@app.post("/v1/recommendations", response_model=RecommendationResponse)
async def get_recommendations(
    request: RecommendationRequest,
    db = Depends(get_db)
):
    """Generate recommendations for a user"""
    start_time = datetime.utcnow()
    
    try:
        # Check cache first
        cache_key = f"recommendations:{request.user_id}:{request.recommendation_type.value}:{request.num_recommendations}"
        cached_result = await get_cached_recommendations(cache_key)
        
        if cached_result:
            cache_hit_rate.inc()
            return cached_result
        
        # Generate recommendations based on type
        if request.recommendation_type == RecommendationType.COLLABORATIVE:
            recommendations = await generate_collaborative_recommendations(request)
        elif request.recommendation_type == RecommendationType.CONTENT_BASED:
            recommendations = await generate_content_based_recommendations(request)
        elif request.recommendation_type == RecommendationType.HYBRID:
            recommendations = await generate_hybrid_recommendations(request)
        elif request.recommendation_type == RecommendationType.TRENDING:
            recommendations = await generate_trending_recommendations(request)
        elif request.recommendation_type == RecommendationType.POPULAR:
            recommendations = await generate_popular_recommendations(request)
        else:
            raise HTTPException(status_code=400, detail="Unsupported recommendation type")
        
        # Create response
        response = RecommendationResponse(
            user_id=request.user_id,
            recommendations=recommendations,
            recommendation_type=request.recommendation_type,
            generated_at=datetime.utcnow(),
            model_version="1.0.0",
            total_items=len(recommendations)
        )
        
        # Add explanation if requested
        if request.explain:
            response.explanation = await generate_explanation(request, recommendations)
        
        # Cache the result
        await cache_recommendations(cache_key, response)
        
        # Update metrics
        duration = (datetime.utcnow() - start_time).total_seconds()
        recommendation_latency.labels(
            recommendation_type=request.recommendation_type.value
        ).observe(duration)
        
        recommendations_generated.labels(
            recommendation_type=request.recommendation_type.value,
            user_type="registered"
        ).inc()
        
        return response
        
    except Exception as e:
        logger.error(f"Error generating recommendations: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to generate recommendations: {str(e)}")

@app.get("/v1/items/{item_id}/similar")
async def get_similar_items(
    item_id: str,
    num_similar: int = 10,
    similarity_threshold: float = 0.1,
    db = Depends(get_db)
):
    """Get items similar to a given item"""
    try:
        # Check if item exists
        async with db.acquire() as conn:
            item_exists = await conn.fetchval(
                "SELECT EXISTS(SELECT 1 FROM item_metadata WHERE item_id = $1)",
                item_id
            )
            
            if not item_exists:
                raise HTTPException(status_code=404, detail="Item not found")
        
        # Get similar items using content-based similarity
        similar_items = await find_similar_items(item_id, num_similar, similarity_threshold)
        
        return {
            "item_id": item_id,
            "similar_items": similar_items,
            "count": len(similar_items),
            "similarity_threshold": similarity_threshold
        }
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error finding similar items: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to find similar items: {str(e)}")

@app.get("/v1/users/{user_id}/profile", response_model=UserProfileResponse)
async def get_user_profile(user_id: str, db = Depends(get_db)):
    """Get user preference profile"""
    try:
        async with db.acquire() as conn:
            # Get user interactions
            interactions = await conn.fetch("""
                SELECT item_id, interaction_type, value, timestamp
                FROM user_interactions 
                WHERE user_id = $1
                ORDER BY timestamp DESC
            """, user_id)
            
            if not interactions:
                raise HTTPException(status_code=404, detail="User profile not found")
            
            # Get item metadata for user's interactions
            item_ids = [interaction['item_id'] for interaction in interactions]
            items = await conn.fetch("""
                SELECT item_id, category, tags, features
                FROM item_metadata 
                WHERE item_id = ANY($1)
            """, item_ids)
        
        # Build user profile
        profile = await build_user_profile(user_id, interactions, items)
        
        return profile
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error getting user profile: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to get user profile: {str(e)}")

# Model training endpoints
@app.post("/v1/models/train")
async def train_models(
    background_tasks: BackgroundTasks,
    model_types: List[str] = None
):
    """Trigger model training"""
    try:
        if not model_types:
            model_types = ["collaborative", "content_based"]
        
        for model_type in model_types:
            if model_type == "collaborative":
                background_tasks.add_task(train_collaborative_model)
            elif model_type == "content_based":
                background_tasks.add_task(train_content_model)
        
        return {
            "status": "success",
            "message": f"Training started for models: {model_types}"
        }
        
    except Exception as e:
        logger.error(f"Error starting model training: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to start training: {str(e)}")

@app.get("/v1/models/status")
async def get_model_status():
    """Get status of recommendation models"""
    global collaborative_model, content_model
    
    return {
        "models": {
            "collaborative": {
                "trained": collaborative_model is not None,
                "last_updated": "2024-01-01T00:00:00Z"  # Placeholder
            },
            "content_based": {
                "trained": content_model is not None,
                "last_updated": "2024-01-01T00:00:00Z"  # Placeholder
            }
        },
        "status": "healthy" if collaborative_model and content_model else "training_required"
    }

# Analytics endpoints
@app.get("/v1/analytics/recommendations")
async def get_recommendation_analytics(
    start_date: Optional[str] = None,
    end_date: Optional[str] = None,
    db = Depends(get_db)
):
    """Get recommendation analytics"""
    try:
        # Default to last 7 days if no dates provided
        if not start_date:
            start_date = (datetime.utcnow() - timedelta(days=7)).isoformat()
        if not end_date:
            end_date = datetime.utcnow().isoformat()
        
        async with db.acquire() as conn:
            # Get recommendation stats
            stats = await conn.fetchrow("""
                SELECT 
                    COUNT(*) as total_interactions,
                    COUNT(DISTINCT user_id) as unique_users,
                    COUNT(DISTINCT item_id) as unique_items,
                    AVG(value) as avg_rating
                FROM user_interactions 
                WHERE timestamp BETWEEN $1 AND $2
            """, start_date, end_date)
            
            # Get top categories
            top_categories = await conn.fetch("""
                SELECT im.category, COUNT(*) as interaction_count
                FROM user_interactions ui
                JOIN item_metadata im ON ui.item_id = im.item_id
                WHERE ui.timestamp BETWEEN $1 AND $2
                GROUP BY im.category
                ORDER BY interaction_count DESC
                LIMIT 10
            """, start_date, end_date)
            
            # Get interaction types distribution
            interaction_types = await conn.fetch("""
                SELECT interaction_type, COUNT(*) as count
                FROM user_interactions 
                WHERE timestamp BETWEEN $1 AND $2
                GROUP BY interaction_type
                ORDER BY count DESC
            """, start_date, end_date)
        
        return {
            "period": {"start": start_date, "end": end_date},
            "overview": dict(stats),
            "top_categories": [dict(row) for row in top_categories],
            "interaction_types": [dict(row) for row in interaction_types]
        }
        
    except Exception as e:
        logger.error(f"Error getting analytics: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to get analytics: {str(e)}")

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
        
        # Load existing models
        await load_models()
        
        # Start background tasks
        asyncio.create_task(periodic_model_training())
        asyncio.create_task(update_metrics_periodically())
        
        logger.info("Recommendation Engine service started successfully")
        
    except Exception as e:
        logger.error(f"Failed to start recommendation service: {str(e)}")
        raise

@app.on_event("shutdown")
async def shutdown_event():
    global db_pool, redis_client
    
    try:
        if db_pool:
            await db_pool.close()
        if redis_client:
            await redis_client.close()
        
        logger.info("Recommendation Engine service shut down successfully")
        
    except Exception as e:
        logger.error(f"Error during shutdown: {str(e)}")

# Helper functions (implementations would be in separate files due to length)
async def generate_collaborative_recommendations(request: RecommendationRequest) -> List[Dict[str, Any]]:
    """Generate collaborative filtering recommendations"""
    # Implementation would use matrix factorization or similar techniques
    return []

async def generate_content_based_recommendations(request: RecommendationRequest) -> List[Dict[str, Any]]:
    """Generate content-based recommendations"""
    # Implementation would use item features and user preferences
    return []

async def generate_hybrid_recommendations(request: RecommendationRequest) -> List[Dict[str, Any]]:
    """Generate hybrid recommendations combining multiple approaches"""
    # Implementation would combine collaborative and content-based recommendations
    return []

async def generate_trending_recommendations(request: RecommendationRequest) -> List[Dict[str, Any]]:
    """Generate trending item recommendations"""
    # Implementation would find items with recent high interaction rates
    return []

async def generate_popular_recommendations(request: RecommendationRequest) -> List[Dict[str, Any]]:
    """Generate popular item recommendations"""
    # Implementation would find most popular items overall
    return []

async def create_database_tables():
    """Create database tables for recommendations"""
    create_tables_sql = """
    CREATE TABLE IF NOT EXISTS user_interactions (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        user_id VARCHAR(255) NOT NULL,
        item_id VARCHAR(255) NOT NULL,
        interaction_type VARCHAR(50) NOT NULL,
        value FLOAT NOT NULL DEFAULT 1.0,
        timestamp TIMESTAMP DEFAULT NOW(),
        context JSONB DEFAULT '{}',
        created_at TIMESTAMP DEFAULT NOW(),
        UNIQUE(user_id, item_id, interaction_type)
    );
    
    CREATE TABLE IF NOT EXISTS item_metadata (
        item_id VARCHAR(255) PRIMARY KEY,
        title VARCHAR(500) NOT NULL,
        description TEXT,
        category VARCHAR(100) NOT NULL,
        tags TEXT[] DEFAULT '{}',
        features JSONB DEFAULT '{}',
        created_at TIMESTAMP DEFAULT NOW(),
        updated_at TIMESTAMP DEFAULT NOW()
    );
    
    CREATE TABLE IF NOT EXISTS user_profiles (
        user_id VARCHAR(255) PRIMARY KEY,
        preferences JSONB DEFAULT '{}',
        categories JSONB DEFAULT '{}',
        tags JSONB DEFAULT '{}',
        interaction_count INTEGER DEFAULT 0,
        last_activity TIMESTAMP,
        created_at TIMESTAMP DEFAULT NOW(),
        updated_at TIMESTAMP DEFAULT NOW()
    );
    
    CREATE INDEX IF NOT EXISTS idx_user_interactions_user_id ON user_interactions(user_id);
    CREATE INDEX IF NOT EXISTS idx_user_interactions_item_id ON user_interactions(item_id);
    CREATE INDEX IF NOT EXISTS idx_user_interactions_timestamp ON user_interactions(timestamp);
    CREATE INDEX IF NOT EXISTS idx_item_metadata_category ON item_metadata(category);
    CREATE INDEX IF NOT EXISTS idx_item_metadata_tags ON item_metadata USING GIN(tags);
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
