"""
Search utilities and helper functions
"""

import json
import logging
from datetime import datetime, timedelta
from typing import Dict, List, Optional, Any

import aioredis
from elasticsearch import AsyncElasticsearch

logger = logging.getLogger(__name__)

async def build_elasticsearch_query(query) -> Dict[str, Any]:
    """Build Elasticsearch query based on search parameters"""
    
    es_query = {
        "query": {},
        "highlight": {},
        "sort": [],
        "aggs": {}
    }
    
    # Build main query based on search type
    if query.search_type == "full_text":
        es_query["query"] = {
            "multi_match": {
                "query": query.query,
                "fields": ["title^2", "content", "tags"],
                "type": "best_fields",
                "fuzziness": "AUTO"
            }
        }
    elif query.search_type == "semantic":
        # For semantic search, you'd typically use vector similarity
        # This is a simplified version using more_like_this
        es_query["query"] = {
            "more_like_this": {
                "fields": ["title", "content"],
                "like": query.query,
                "min_term_freq": 1,
                "max_query_terms": 12
            }
        }
    elif query.search_type == "fuzzy":
        es_query["query"] = {
            "multi_match": {
                "query": query.query,
                "fields": ["title", "content"],
                "fuzziness": 2,
                "prefix_length": 1
            }
        }
    elif query.search_type == "exact":
        es_query["query"] = {
            "multi_match": {
                "query": query.query,
                "fields": ["title.keyword", "content"],
                "type": "phrase"
            }
        }
    
    # Apply filters
    if query.filters:
        filters = []
        
        for field, value in query.filters.items():
            if isinstance(value, list):
                filters.append({"terms": {field: value}})
            elif isinstance(value, dict):
                if "range" in value:
                    filters.append({"range": {field: value["range"]}})
                elif "exists" in value:
                    if value["exists"]:
                        filters.append({"exists": {"field": field}})
                    else:
                        filters.append({"bool": {"must_not": {"exists": {"field": field}}}})
            else:
                filters.append({"term": {field: value}})
        
        if filters:
            # Wrap existing query in bool query with filters
            original_query = es_query["query"]
            es_query["query"] = {
                "bool": {
                    "must": [original_query],
                    "filter": filters
                }
            }
    
    # Add sorting
    if query.sort:
        for sort_item in query.sort:
            es_query["sort"].append(sort_item)
    else:
        # Default sort by relevance score
        es_query["sort"] = ["_score"]
    
    # Add highlighting
    if query.highlight:
        es_query["highlight"] = {
            "fields": {
                "title": {
                    "pre_tags": ["<mark>"],
                    "post_tags": ["</mark>"],
                    "number_of_fragments": 1
                },
                "content": {
                    "pre_tags": ["<mark>"],
                    "post_tags": ["</mark>"],
                    "fragment_size": 150,
                    "number_of_fragments": 3
                }
            }
        }
    
    # Add facets/aggregations
    if query.facets:
        for facet in query.facets:
            if facet == "document_type":
                es_query["aggs"]["document_types"] = {
                    "terms": {"field": "document_type", "size": 10}
                }
            elif facet == "tags":
                es_query["aggs"]["tags"] = {
                    "terms": {"field": "tags", "size": 20}
                }
            elif facet == "user_id":
                es_query["aggs"]["users"] = {
                    "terms": {"field": "user_id", "size": 10}
                }
            elif facet == "created_date":
                es_query["aggs"]["created_date"] = {
                    "date_histogram": {
                        "field": "created_at",
                        "calendar_interval": "day"
                    }
                }
    
    return es_query

async def get_search_suggestions(query: str, es: AsyncElasticsearch, limit: int = 10) -> List[str]:
    """Get search suggestions based on query"""
    try:
        # Use completion suggester if available
        suggest_query = {
            "suggest": {
                "title_suggest": {
                    "prefix": query,
                    "completion": {
                        "field": "title.suggest",
                        "size": limit
                    }
                }
            }
        }
        
        response = await es.search(body=suggest_query)
        
        suggestions = []
        if "suggest" in response and "title_suggest" in response["suggest"]:
            for suggestion in response["suggest"]["title_suggest"]:
                for option in suggestion["options"]:
                    suggestions.append(option["text"])
        
        # If no completion suggestions, fall back to term suggestions
        if not suggestions:
            suggest_query = {
                "suggest": {
                    "text": query,
                    "term_suggest": {
                        "term": {
                            "field": "title",
                            "size": limit
                        }
                    }
                }
            }
            
            response = await es.search(body=suggest_query)
            
            if "suggest" in response and "term_suggest" in response["suggest"]:
                for suggestion in response["suggest"]["term_suggest"]:
                    for option in suggestion["options"]:
                        suggestions.append(option["text"])
        
        return list(set(suggestions))[:limit]  # Remove duplicates and limit
        
    except Exception as e:
        logger.error(f"Error getting suggestions: {str(e)}")
        return []

def process_facets(aggregations: Dict[str, Any]) -> Dict[str, Any]:
    """Process Elasticsearch aggregations into facets"""
    facets = {}
    
    for agg_name, agg_data in aggregations.items():
        if "buckets" in agg_data:
            facets[agg_name] = []
            for bucket in agg_data["buckets"]:
                facets[agg_name].append({
                    "key": bucket["key"],
                    "count": bucket["doc_count"]
                })
    
    return facets

async def cache_search_result(query, result, redis_client: aioredis.Redis, ttl: int = 300):
    """Cache search results for popular queries"""
    try:
        # Only cache if query is not too specific and has results
        if len(query.query) >= 3 and result.total_hits > 0:
            cache_key = f"search_cache:{hash(query.query + str(query.filters))}"
            
            # Serialize result (simplified)
            cached_data = {
                "query": result.query,
                "total_hits": result.total_hits,
                "results": [r.dict() for r in result.results[:10]],  # Cache only top 10
                "cached_at": datetime.utcnow().isoformat()
            }
            
            await redis_client.setex(
                cache_key,
                ttl,
                json.dumps(cached_data, default=str)
            )
            
    except Exception as e:
        logger.error(f"Error caching search result: {str(e)}")

async def get_cached_search_result(query, redis_client: aioredis.Redis):
    """Get cached search result if available"""
    try:
        cache_key = f"search_cache:{hash(query.query + str(query.filters))}"
        cached_data = await redis_client.get(cache_key)
        
        if cached_data:
            return json.loads(cached_data)
        
        return None
        
    except Exception as e:
        logger.error(f"Error getting cached search result: {str(e)}")
        return None

async def update_index_stats(index_name: str):
    """Update index statistics in background"""
    try:
        # This would typically update database with index stats
        # For now, just update Prometheus metrics
        from main import documents_indexed, es_client
        
        if es_client:
            stats = await es_client.indices.stats(index=index_name)
            if index_name in stats['indices']:
                doc_count = stats['indices'][index_name]['total']['docs']['count']
                documents_indexed.labels(index_name=index_name).set(doc_count)
                
    except Exception as e:
        logger.error(f"Error updating index stats for {index_name}: {str(e)}")

async def create_database_tables():
    """Create database tables for search metadata"""
    from main import db_pool
    
    create_tables_sql = """
    CREATE TABLE IF NOT EXISTS search_queries (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        query_text TEXT NOT NULL,
        search_type VARCHAR(50) NOT NULL,
        filters JSONB,
        user_id VARCHAR(255),
        results_count INTEGER,
        took_ms INTEGER,
        created_at TIMESTAMP DEFAULT NOW()
    );
    
    CREATE TABLE IF NOT EXISTS search_analytics (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        date DATE NOT NULL,
        total_searches INTEGER DEFAULT 0,
        unique_queries INTEGER DEFAULT 0,
        avg_response_time_ms FLOAT DEFAULT 0,
        top_queries JSONB,
        created_at TIMESTAMP DEFAULT NOW(),
        UNIQUE(date)
    );
    
    CREATE TABLE IF NOT EXISTS index_metadata (
        index_name VARCHAR(255) PRIMARY KEY,
        document_count INTEGER DEFAULT 0,
        size_bytes BIGINT DEFAULT 0,
        created_at TIMESTAMP DEFAULT NOW(),
        updated_at TIMESTAMP DEFAULT NOW()
    );
    
    CREATE INDEX IF NOT EXISTS idx_search_queries_created_at ON search_queries(created_at);
    CREATE INDEX IF NOT EXISTS idx_search_queries_user_id ON search_queries(user_id);
    CREATE INDEX IF NOT EXISTS idx_search_analytics_date ON search_analytics(date);
    """
    
    try:
        async with db_pool.acquire() as conn:
            await conn.execute(create_tables_sql)
        logger.info("Database tables created successfully")
    except Exception as e:
        logger.error(f"Error creating database tables: {str(e)}")

async def log_search_query(query, result, user_id: Optional[str] = None):
    """Log search query for analytics"""
    from main import db_pool
    
    try:
        async with db_pool.acquire() as conn:
            await conn.execute("""
                INSERT INTO search_queries (query_text, search_type, filters, user_id, results_count, took_ms)
                VALUES ($1, $2, $3, $4, $5, $6)
            """, query.query, query.search_type, json.dumps(query.filters), 
                user_id, result.total_hits, result.took_ms)
    except Exception as e:
        logger.error(f"Error logging search query: {str(e)}")

async def update_metrics_periodically():
    """Update metrics periodically in background"""
    from main import active_indices, es_client
    
    while True:
        try:
            await asyncio.sleep(300)  # Update every 5 minutes
            
            if es_client:
                # Update active indices count
                indices = await es_client.indices.get_alias()
                active_count = len([name for name in indices.keys() if not name.startswith('.')])
                active_indices.set(active_count)
                
        except Exception as e:
            logger.error(f"Error updating metrics: {str(e)}")

async def reindex_documents(source_index: str, dest_index: str, es: AsyncElasticsearch):
    """Reindex documents from one index to another"""
    try:
        reindex_body = {
            "source": {"index": source_index},
            "dest": {"index": dest_index}
        }
        
        response = await es.reindex(body=reindex_body, wait_for_completion=True)
        
        return {
            "total": response.get("total", 0),
            "created": response.get("created", 0),
            "updated": response.get("updated", 0),
            "took": response.get("took", 0)
        }
        
    except Exception as e:
        logger.error(f"Error reindexing from {source_index} to {dest_index}: {str(e)}")
        raise

async def optimize_index(index_name: str, es: AsyncElasticsearch):
    """Optimize index for better search performance"""
    try:
        # Force merge to optimize segments
        response = await es.indices.forcemerge(
            index=index_name,
            max_num_segments=1,
            wait_for_completion=True
        )
        
        return response
        
    except Exception as e:
        logger.error(f"Error optimizing index {index_name}: {str(e)}")
        raise

async def analyze_search_patterns():
    """Analyze search patterns for insights"""
    from main import db_pool
    
    try:
        async with db_pool.acquire() as conn:
            # Get top queries from last 7 days
            top_queries = await conn.fetch("""
                SELECT query_text, COUNT(*) as frequency, AVG(took_ms) as avg_time
                FROM search_queries 
                WHERE created_at >= NOW() - INTERVAL '7 days'
                GROUP BY query_text
                ORDER BY frequency DESC
                LIMIT 20
            """)
            
            # Get search trends by day
            daily_trends = await conn.fetch("""
                SELECT DATE(created_at) as date, COUNT(*) as searches
                FROM search_queries
                WHERE created_at >= NOW() - INTERVAL '30 days'
                GROUP BY DATE(created_at)
                ORDER BY date DESC
            """)
            
            return {
                "top_queries": [dict(row) for row in top_queries],
                "daily_trends": [dict(row) for row in daily_trends]
            }
            
    except Exception as e:
        logger.error(f"Error analyzing search patterns: {str(e)}")
        return {"top_queries": [], "daily_trends": []}

# Import asyncio at the end to avoid circular imports
import asyncio
