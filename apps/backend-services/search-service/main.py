"""
Search Service - Python
Advanced search and indexing for the 002AIC platform
Handles full-text search, semantic search, and content indexing
"""

import asyncio
import json
import logging
import os
import time
from datetime import datetime, timedelta
from typing import Dict, List, Optional, Any
from uuid import uuid4

import aioredis
import asyncpg
from elasticsearch import AsyncElasticsearch
from fastapi import FastAPI, HTTPException, Depends, Query, BackgroundTasks
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel, Field
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
        self.database_url = os.getenv("DATABASE_URL", "postgresql://postgres:password@localhost:5432/search")
        self.redis_url = os.getenv("REDIS_URL", "redis://localhost:6379")
        self.elasticsearch_url = os.getenv("ELASTICSEARCH_URL", "http://localhost:9200")
        self.environment = os.getenv("ENVIRONMENT", "development")
        self.max_search_results = int(os.getenv("MAX_SEARCH_RESULTS", "1000"))
        self.index_batch_size = int(os.getenv("INDEX_BATCH_SIZE", "100"))

config = Config()

# Pydantic models
class SearchQuery(BaseModel):
    query: str = Field(..., min_length=1, max_length=1000)
    filters: Optional[Dict[str, Any]] = None
    sort: Optional[List[Dict[str, str]]] = None
    limit: int = Field(default=20, ge=1, le=100)
    offset: int = Field(default=0, ge=0)
    search_type: str = Field(default="full_text", regex="^(full_text|semantic|fuzzy|exact)$")
    indices: Optional[List[str]] = None
    highlight: bool = True
    facets: Optional[List[str]] = None

class IndexDocument(BaseModel):
    id: str
    index_name: str
    document_type: str
    title: str
    content: str
    metadata: Optional[Dict[str, Any]] = None
    tags: Optional[List[str]] = None
    user_id: Optional[str] = None
    project_id: Optional[str] = None
    created_at: Optional[datetime] = None
    updated_at: Optional[datetime] = None

class SearchResult(BaseModel):
    id: str
    index_name: str
    document_type: str
    title: str
    content: str
    score: float
    highlights: Optional[Dict[str, List[str]]] = None
    metadata: Optional[Dict[str, Any]] = None
    tags: Optional[List[str]] = None

class SearchResponse(BaseModel):
    query: str
    total_hits: int
    max_score: float
    took_ms: int
    results: List[SearchResult]
    facets: Optional[Dict[str, Any]] = None
    suggestions: Optional[List[str]] = None

class IndexStats(BaseModel):
    index_name: str
    document_count: int
    size_bytes: int
    created_at: datetime
    last_updated: datetime

# Prometheus metrics
search_requests_total = Counter(
    'search_requests_total',
    'Total number of search requests',
    ['search_type', 'status']
)

search_duration = Histogram(
    'search_duration_seconds',
    'Time taken to process search requests',
    ['search_type']
)

index_operations_total = Counter(
    'index_operations_total',
    'Total number of indexing operations',
    ['operation', 'index_name', 'status']
)

active_indices = Gauge(
    'active_indices_total',
    'Number of active search indices'
)

documents_indexed = Gauge(
    'documents_indexed_total',
    'Total number of documents indexed',
    ['index_name']
)

# Global variables
app = FastAPI(title="Search Service", version="1.0.0")
db_pool = None
redis_client = None
es_client = None

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

# Elasticsearch connection
async def get_elasticsearch():
    return es_client

# Health check endpoint
@app.get("/health")
async def health_check():
    status = {
        "status": "healthy",
        "service": "search-service",
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
        # Check Elasticsearch
        await es_client.ping()
        status["elasticsearch"] = "connected"
    except Exception as e:
        status["status"] = "unhealthy"
        status["elasticsearch"] = f"disconnected: {str(e)}"
    
    if status["status"] == "unhealthy":
        raise HTTPException(status_code=503, detail=status)
    
    return status

# Metrics endpoint
@app.get("/metrics")
async def metrics():
    return Response(generate_latest(), media_type="text/plain")

# Search endpoints
@app.post("/v1/search", response_model=SearchResponse)
async def search_documents(
    query: SearchQuery,
    es: AsyncElasticsearch = Depends(get_elasticsearch)
):
    """Perform advanced search across indexed documents"""
    start_time = time.time()
    
    try:
        # Build Elasticsearch query
        es_query = await build_elasticsearch_query(query)
        
        # Execute search
        response = await es.search(
            index=query.indices or "_all",
            body=es_query,
            size=query.limit,
            from_=query.offset
        )
        
        # Process results
        results = []
        for hit in response['hits']['hits']:
            result = SearchResult(
                id=hit['_id'],
                index_name=hit['_index'],
                document_type=hit['_source'].get('document_type', 'unknown'),
                title=hit['_source'].get('title', ''),
                content=hit['_source'].get('content', ''),
                score=hit['_score'],
                highlights=hit.get('highlight'),
                metadata=hit['_source'].get('metadata'),
                tags=hit['_source'].get('tags')
            )
            results.append(result)
        
        # Get suggestions if no results
        suggestions = []
        if response['hits']['total']['value'] == 0:
            suggestions = await get_search_suggestions(query.query, es)
        
        # Process facets
        facets = None
        if query.facets and 'aggregations' in response:
            facets = process_facets(response['aggregations'])
        
        search_response = SearchResponse(
            query=query.query,
            total_hits=response['hits']['total']['value'],
            max_score=response['hits']['max_score'] or 0.0,
            took_ms=response['took'],
            results=results,
            facets=facets,
            suggestions=suggestions
        )
        
        # Update metrics
        duration = time.time() - start_time
        search_requests_total.labels(search_type=query.search_type, status='success').inc()
        search_duration.labels(search_type=query.search_type).observe(duration)
        
        # Cache popular searches
        await cache_search_result(query, search_response)
        
        return search_response
        
    except Exception as e:
        search_requests_total.labels(search_type=query.search_type, status='error').inc()
        logger.error(f"Search error: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Search failed: {str(e)}")

@app.get("/v1/search/suggestions")
async def get_suggestions(
    q: str = Query(..., min_length=1),
    limit: int = Query(default=10, ge=1, le=50),
    es: AsyncElasticsearch = Depends(get_elasticsearch)
):
    """Get search suggestions based on query"""
    try:
        suggestions = await get_search_suggestions(q, es, limit)
        return {"query": q, "suggestions": suggestions}
    except Exception as e:
        logger.error(f"Suggestions error: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to get suggestions: {str(e)}")

# Indexing endpoints
@app.post("/v1/index/documents")
async def index_document(
    document: IndexDocument,
    background_tasks: BackgroundTasks,
    es: AsyncElasticsearch = Depends(get_elasticsearch)
):
    """Index a single document"""
    try:
        # Prepare document for indexing
        doc_body = {
            "document_type": document.document_type,
            "title": document.title,
            "content": document.content,
            "metadata": document.metadata or {},
            "tags": document.tags or [],
            "user_id": document.user_id,
            "project_id": document.project_id,
            "created_at": document.created_at or datetime.utcnow(),
            "updated_at": document.updated_at or datetime.utcnow(),
            "indexed_at": datetime.utcnow()
        }
        
        # Index document
        response = await es.index(
            index=document.index_name,
            id=document.id,
            body=doc_body
        )
        
        # Update metrics
        index_operations_total.labels(
            operation='index',
            index_name=document.index_name,
            status='success'
        ).inc()
        
        # Update document count in background
        background_tasks.add_task(update_index_stats, document.index_name)
        
        return {
            "document_id": document.id,
            "index_name": document.index_name,
            "result": response['result'],
            "message": "Document indexed successfully"
        }
        
    except Exception as e:
        index_operations_total.labels(
            operation='index',
            index_name=document.index_name,
            status='error'
        ).inc()
        logger.error(f"Indexing error: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to index document: {str(e)}")

@app.post("/v1/index/documents/batch")
async def batch_index_documents(
    documents: List[IndexDocument],
    background_tasks: BackgroundTasks,
    es: AsyncElasticsearch = Depends(get_elasticsearch)
):
    """Index multiple documents in batch"""
    try:
        # Prepare bulk operations
        bulk_body = []
        for doc in documents:
            # Index operation
            bulk_body.append({
                "index": {
                    "_index": doc.index_name,
                    "_id": doc.id
                }
            })
            
            # Document body
            doc_body = {
                "document_type": doc.document_type,
                "title": doc.title,
                "content": doc.content,
                "metadata": doc.metadata or {},
                "tags": doc.tags or [],
                "user_id": doc.user_id,
                "project_id": doc.project_id,
                "created_at": doc.created_at or datetime.utcnow(),
                "updated_at": doc.updated_at or datetime.utcnow(),
                "indexed_at": datetime.utcnow()
            }
            bulk_body.append(doc_body)
        
        # Execute bulk operation
        response = await es.bulk(body=bulk_body)
        
        # Process results
        success_count = 0
        error_count = 0
        errors = []
        
        for item in response['items']:
            if 'index' in item:
                if item['index'].get('status') in [200, 201]:
                    success_count += 1
                    index_operations_total.labels(
                        operation='bulk_index',
                        index_name=item['index']['_index'],
                        status='success'
                    ).inc()
                else:
                    error_count += 1
                    errors.append(item['index'].get('error', 'Unknown error'))
                    index_operations_total.labels(
                        operation='bulk_index',
                        index_name=item['index']['_index'],
                        status='error'
                    ).inc()
        
        # Update index stats in background
        indices = list(set(doc.index_name for doc in documents))
        for index_name in indices:
            background_tasks.add_task(update_index_stats, index_name)
        
        result = {
            "total_documents": len(documents),
            "success_count": success_count,
            "error_count": error_count,
            "took_ms": response['took'],
            "message": f"Batch indexing completed: {success_count} success, {error_count} errors"
        }
        
        if errors:
            result["errors"] = errors[:10]  # Limit error details
        
        return result
        
    except Exception as e:
        logger.error(f"Batch indexing error: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to batch index documents: {str(e)}")

@app.delete("/v1/index/documents/{document_id}")
async def delete_document(
    document_id: str,
    index_name: str = Query(...),
    background_tasks: BackgroundTasks,
    es: AsyncElasticsearch = Depends(get_elasticsearch)
):
    """Delete a document from the index"""
    try:
        response = await es.delete(
            index=index_name,
            id=document_id
        )
        
        # Update metrics
        index_operations_total.labels(
            operation='delete',
            index_name=index_name,
            status='success'
        ).inc()
        
        # Update index stats in background
        background_tasks.add_task(update_index_stats, index_name)
        
        return {
            "document_id": document_id,
            "index_name": index_name,
            "result": response['result'],
            "message": "Document deleted successfully"
        }
        
    except Exception as e:
        index_operations_total.labels(
            operation='delete',
            index_name=index_name,
            status='error'
        ).inc()
        logger.error(f"Delete error: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to delete document: {str(e)}")

# Index management endpoints
@app.post("/v1/indices/{index_name}")
async def create_index(
    index_name: str,
    mapping: Optional[Dict[str, Any]] = None,
    settings: Optional[Dict[str, Any]] = None,
    es: AsyncElasticsearch = Depends(get_elasticsearch)
):
    """Create a new search index"""
    try:
        # Default mapping for documents
        default_mapping = {
            "properties": {
                "document_type": {"type": "keyword"},
                "title": {
                    "type": "text",
                    "analyzer": "standard",
                    "fields": {
                        "keyword": {"type": "keyword"},
                        "suggest": {"type": "completion"}
                    }
                },
                "content": {
                    "type": "text",
                    "analyzer": "standard"
                },
                "metadata": {"type": "object"},
                "tags": {"type": "keyword"},
                "user_id": {"type": "keyword"},
                "project_id": {"type": "keyword"},
                "created_at": {"type": "date"},
                "updated_at": {"type": "date"},
                "indexed_at": {"type": "date"}
            }
        }
        
        # Default settings
        default_settings = {
            "number_of_shards": 1,
            "number_of_replicas": 0,
            "analysis": {
                "analyzer": {
                    "custom_analyzer": {
                        "type": "custom",
                        "tokenizer": "standard",
                        "filter": ["lowercase", "stop", "snowball"]
                    }
                }
            }
        }
        
        index_body = {
            "mappings": mapping or default_mapping,
            "settings": settings or default_settings
        }
        
        response = await es.indices.create(
            index=index_name,
            body=index_body
        )
        
        # Update metrics
        active_indices.inc()
        
        return {
            "index_name": index_name,
            "acknowledged": response['acknowledged'],
            "message": "Index created successfully"
        }
        
    except Exception as e:
        logger.error(f"Create index error: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to create index: {str(e)}")

@app.get("/v1/indices", response_model=List[IndexStats])
async def list_indices(
    es: AsyncElasticsearch = Depends(get_elasticsearch)
):
    """List all search indices with statistics"""
    try:
        # Get index stats
        stats_response = await es.indices.stats()
        
        indices = []
        for index_name, stats in stats_response['indices'].items():
            if not index_name.startswith('.'):  # Skip system indices
                index_stats = IndexStats(
                    index_name=index_name,
                    document_count=stats['total']['docs']['count'],
                    size_bytes=stats['total']['store']['size_in_bytes'],
                    created_at=datetime.utcnow(),  # Placeholder
                    last_updated=datetime.utcnow()  # Placeholder
                )
                indices.append(index_stats)
        
        return indices
        
    except Exception as e:
        logger.error(f"List indices error: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to list indices: {str(e)}")

@app.delete("/v1/indices/{index_name}")
async def delete_index(
    index_name: str,
    es: AsyncElasticsearch = Depends(get_elasticsearch)
):
    """Delete a search index"""
    try:
        response = await es.indices.delete(index=index_name)
        
        # Update metrics
        active_indices.dec()
        
        return {
            "index_name": index_name,
            "acknowledged": response['acknowledged'],
            "message": "Index deleted successfully"
        }
        
    except Exception as e:
        logger.error(f"Delete index error: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to delete index: {str(e)}")

# Startup and shutdown events
@app.on_event("startup")
async def startup_event():
    global db_pool, redis_client, es_client
    
    try:
        # Initialize database connection pool
        db_pool = await asyncpg.create_pool(config.database_url)
        logger.info("Database connection pool created")
        
        # Initialize Redis client
        redis_client = aioredis.from_url(config.redis_url)
        logger.info("Redis client initialized")
        
        # Initialize Elasticsearch client
        es_client = AsyncElasticsearch([config.elasticsearch_url])
        logger.info("Elasticsearch client initialized")
        
        # Create database tables
        await create_database_tables()
        
        # Start background tasks
        asyncio.create_task(update_metrics_periodically())
        
        logger.info("Search service started successfully")
        
    except Exception as e:
        logger.error(f"Failed to start search service: {str(e)}")
        raise

@app.on_event("shutdown")
async def shutdown_event():
    global db_pool, redis_client, es_client
    
    try:
        if db_pool:
            await db_pool.close()
        if redis_client:
            await redis_client.close()
        if es_client:
            await es_client.close()
        
        logger.info("Search service shut down successfully")
        
    except Exception as e:
        logger.error(f"Error during shutdown: {str(e)}")

if __name__ == "__main__":
    uvicorn.run(
        "main:app",
        host="0.0.0.0",
        port=config.port,
        log_level="info",
        reload=config.environment == "development"
    )
