"""
Analytics Service - Python
Business intelligence and analytics service for the 002AIC platform
Provides insights, reports, and real-time analytics capabilities
"""

from fastapi import FastAPI, HTTPException, Depends, BackgroundTasks, Query
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel, Field, validator
from typing import List, Optional, Dict, Any, Union
import uvicorn
import os
import logging
from datetime import datetime, timedelta
import asyncio
import pandas as pd
import numpy as np
from sqlalchemy import create_engine, text
from sqlalchemy.orm import sessionmaker
import redis
import plotly.graph_objects as go
import plotly.express as px
from plotly.utils import PlotlyJSONEncoder
import json
from sklearn.preprocessing import StandardScaler
from sklearn.cluster import KMeans
from sklearn.metrics import silhouette_score
import uuid

# Import our auth middleware
import sys
sys.path.append('/home/oss/002AIC/libs/auth-middleware/python')
from auth_middleware import FastAPIAuthMiddleware, AuthConfig

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Initialize FastAPI app
app = FastAPI(
    title="002AIC Analytics Service",
    description="Business intelligence and analytics service",
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
    service_name="analytics-service"
)

auth = FastAPIAuthMiddleware(auth_config)

# Initialize external services
def init_postgres():
    """Initialize PostgreSQL connection for analytics data"""
    database_url = os.getenv(
        "DATABASE_URL",
        "postgresql://postgres:password@localhost:5432/analytics"
    )
    engine = create_engine(database_url)
    SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)
    return engine, SessionLocal

def init_redis():
    """Initialize Redis for caching analytics results"""
    return redis.Redis(
        host=os.getenv("REDIS_HOST", "localhost"),
        port=int(os.getenv("REDIS_PORT", "6379")),
        password=os.getenv("REDIS_PASSWORD", ""),
        db=3,  # Use different DB than other services
        decode_responses=True
    )

# Initialize services
postgres_engine, SessionLocal = init_postgres()
redis_client = init_redis()

# Pydantic models
class MetricDefinition(BaseModel):
    name: str = Field(..., description="Metric name")
    description: Optional[str] = Field(None, description="Metric description")
    query: str = Field(..., description="SQL query to calculate metric")
    aggregation: str = Field(..., description="Aggregation type (sum, avg, count, etc.)")
    dimensions: List[str] = Field(default_factory=list, description="Metric dimensions")
    filters: Dict[str, Any] = Field(default_factory=dict, description="Default filters")
    refresh_interval: int = Field(default=3600, description="Refresh interval in seconds")
    
    @validator('aggregation')
    def validate_aggregation(cls, v):
        allowed = ['sum', 'avg', 'count', 'min', 'max', 'distinct_count']
        if v.lower() not in allowed:
            raise ValueError(f'Aggregation must be one of {allowed}')
        return v.lower()

class DashboardDefinition(BaseModel):
    name: str = Field(..., description="Dashboard name")
    description: Optional[str] = Field(None, description="Dashboard description")
    widgets: List[Dict[str, Any]] = Field(..., description="Dashboard widgets")
    layout: Dict[str, Any] = Field(..., description="Dashboard layout")
    filters: Dict[str, Any] = Field(default_factory=dict, description="Dashboard filters")
    refresh_interval: int = Field(default=300, description="Auto-refresh interval in seconds")
    access_level: str = Field(default="private", description="Access level")

class ReportRequest(BaseModel):
    name: str = Field(..., description="Report name")
    type: str = Field(..., description="Report type")
    parameters: Dict[str, Any] = Field(..., description="Report parameters")
    format: str = Field(default="json", description="Output format")
    schedule: Optional[str] = Field(None, description="Report schedule (cron format)")
    recipients: List[str] = Field(default_factory=list, description="Report recipients")

class AnalyticsQuery(BaseModel):
    query: str = Field(..., description="Analytics query")
    dimensions: List[str] = Field(default_factory=list, description="Query dimensions")
    metrics: List[str] = Field(..., description="Metrics to calculate")
    filters: Dict[str, Any] = Field(default_factory=dict, description="Query filters")
    time_range: Optional[Dict[str, str]] = Field(None, description="Time range filter")
    limit: Optional[int] = Field(1000, description="Result limit")

class MetricResponse(BaseModel):
    name: str
    value: Union[float, int, str]
    change: Optional[float]
    change_percentage: Optional[float]
    trend: Optional[str]
    timestamp: datetime

class ChartResponse(BaseModel):
    chart_id: str
    title: str
    type: str
    data: Dict[str, Any]
    config: Dict[str, Any]
    generated_at: datetime

class DashboardResponse(BaseModel):
    id: str
    name: str
    description: Optional[str]
    widgets: List[Dict[str, Any]]
    layout: Dict[str, Any]
    last_updated: datetime
    auto_refresh: bool

# Health check endpoint
@app.get("/health")
async def health_check():
    """Health check endpoint"""
    # Test connections
    postgres_healthy = True
    redis_healthy = True
    
    try:
        with postgres_engine.connect() as conn:
            conn.execute(text("SELECT 1"))
    except Exception:
        postgres_healthy = False
    
    try:
        redis_client.ping()
    except Exception:
        redis_healthy = False
    
    return {
        "status": "healthy" if all([postgres_healthy, redis_healthy]) else "degraded",
        "service": "analytics-service",
        "timestamp": datetime.utcnow().isoformat(),
        "version": "1.0.0",
        "dependencies": {
            "postgres": "healthy" if postgres_healthy else "unhealthy",
            "redis": "healthy" if redis_healthy else "unhealthy"
        }
    }

# Metrics endpoints
@app.get("/v1/metrics", response_model=List[MetricResponse])
@auth.require_auth("analytics", "read")
async def get_metrics(
    names: Optional[List[str]] = Query(None),
    time_range: Optional[str] = Query("24h"),
    include_trend: bool = Query(True)
):
    """Get platform metrics"""
    try:
        # Cache key for metrics
        cache_key = f"metrics:{':'.join(names or ['all'])}:{time_range}:{include_trend}"
        cached_result = redis_client.get(cache_key)
        
        if cached_result:
            return json.loads(cached_result)
        
        # Calculate time range
        end_time = datetime.utcnow()
        if time_range == "1h":
            start_time = end_time - timedelta(hours=1)
        elif time_range == "24h":
            start_time = end_time - timedelta(days=1)
        elif time_range == "7d":
            start_time = end_time - timedelta(days=7)
        elif time_range == "30d":
            start_time = end_time - timedelta(days=30)
        else:
            start_time = end_time - timedelta(days=1)
        
        # Sample metrics (in production, these would come from actual data)
        metrics = [
            MetricResponse(
                name="active_users",
                value=1250,
                change=45,
                change_percentage=3.7,
                trend="up",
                timestamp=end_time
            ),
            MetricResponse(
                name="api_requests",
                value=125000,
                change=-2500,
                change_percentage=-2.0,
                trend="down",
                timestamp=end_time
            ),
            MetricResponse(
                name="model_predictions",
                value=45000,
                change=5000,
                change_percentage=12.5,
                trend="up",
                timestamp=end_time
            ),
            MetricResponse(
                name="data_processed_gb",
                value=2.5,
                change=0.3,
                change_percentage=13.6,
                trend="up",
                timestamp=end_time
            ),
            MetricResponse(
                name="error_rate",
                value=0.02,
                change=-0.005,
                change_percentage=-20.0,
                trend="down",
                timestamp=end_time
            )
        ]
        
        # Filter by names if provided
        if names:
            metrics = [m for m in metrics if m.name in names]
        
        # Cache result
        redis_client.setex(cache_key, 300, json.dumps([m.dict() for m in metrics], default=str))
        
        return metrics
    except Exception as e:
        logger.error(f"Error getting metrics: {e}")
        raise HTTPException(status_code=500, detail="Failed to get metrics")

@app.post("/v1/metrics", response_model=MetricResponse)
@auth.require_auth("analytics", "create")
async def create_metric(metric_definition: MetricDefinition):
    """Create a custom metric"""
    try:
        metric_id = str(uuid.uuid4())
        
        # Store metric definition
        query = """
        INSERT INTO metric_definitions (id, name, description, query, aggregation, 
                                      dimensions, filters, refresh_interval, created_at)
        VALUES (:id, :name, :description, :query, :aggregation, :dimensions, 
                :filters, :refresh_interval, :created_at)
        """
        
        with postgres_engine.connect() as conn:
            conn.execute(text(query), {
                "id": metric_id,
                "name": metric_definition.name,
                "description": metric_definition.description,
                "query": metric_definition.query,
                "aggregation": metric_definition.aggregation,
                "dimensions": json.dumps(metric_definition.dimensions),
                "filters": json.dumps(metric_definition.filters),
                "refresh_interval": metric_definition.refresh_interval,
                "created_at": datetime.utcnow()
            })
            conn.commit()
        
        # Calculate initial value
        # This is simplified - in production, execute the actual query
        initial_value = 0
        
        return MetricResponse(
            name=metric_definition.name,
            value=initial_value,
            change=None,
            change_percentage=None,
            trend=None,
            timestamp=datetime.utcnow()
        )
    except Exception as e:
        logger.error(f"Error creating metric: {e}")
        raise HTTPException(status_code=500, detail="Failed to create metric")

# Charts and visualizations
@app.get("/v1/charts/{chart_type}", response_model=ChartResponse)
@auth.require_auth("analytics", "read")
async def generate_chart(
    chart_type: str,
    metric: str = Query(...),
    time_range: str = Query("24h"),
    dimensions: Optional[List[str]] = Query(None)
):
    """Generate charts and visualizations"""
    try:
        chart_id = str(uuid.uuid4())
        
        # Generate sample data based on chart type and metric
        if chart_type == "line":
            # Generate time series data
            dates = pd.date_range(
                start=datetime.utcnow() - timedelta(days=7),
                end=datetime.utcnow(),
                freq='H'
            )
            values = np.random.normal(100, 15, len(dates)) + np.linspace(0, 20, len(dates))
            
            fig = go.Figure()
            fig.add_trace(go.Scatter(
                x=dates,
                y=values,
                mode='lines+markers',
                name=metric,
                line=dict(color='#1f77b4', width=2)
            ))
            
            fig.update_layout(
                title=f"{metric.replace('_', ' ').title()} Over Time",
                xaxis_title="Time",
                yaxis_title="Value",
                hovermode='x unified'
            )
            
        elif chart_type == "bar":
            # Generate categorical data
            categories = ['Category A', 'Category B', 'Category C', 'Category D', 'Category E']
            values = np.random.randint(10, 100, len(categories))
            
            fig = go.Figure()
            fig.add_trace(go.Bar(
                x=categories,
                y=values,
                name=metric,
                marker_color='#ff7f0e'
            ))
            
            fig.update_layout(
                title=f"{metric.replace('_', ' ').title()} by Category",
                xaxis_title="Category",
                yaxis_title="Value"
            )
            
        elif chart_type == "pie":
            # Generate pie chart data
            labels = ['Segment 1', 'Segment 2', 'Segment 3', 'Segment 4']
            values = np.random.randint(10, 50, len(labels))
            
            fig = go.Figure()
            fig.add_trace(go.Pie(
                labels=labels,
                values=values,
                name=metric
            ))
            
            fig.update_layout(
                title=f"{metric.replace('_', ' ').title()} Distribution"
            )
            
        elif chart_type == "heatmap":
            # Generate heatmap data
            x_labels = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']
            y_labels = ['00:00', '06:00', '12:00', '18:00']
            z_values = np.random.randint(0, 100, (len(y_labels), len(x_labels)))
            
            fig = go.Figure()
            fig.add_trace(go.Heatmap(
                x=x_labels,
                y=y_labels,
                z=z_values,
                colorscale='Viridis'
            ))
            
            fig.update_layout(
                title=f"{metric.replace('_', ' ').title()} Heatmap",
                xaxis_title="Day of Week",
                yaxis_title="Time of Day"
            )
            
        else:
            raise HTTPException(status_code=400, detail=f"Unsupported chart type: {chart_type}")
        
        # Convert to JSON
        chart_json = json.loads(fig.to_json())
        
        response = ChartResponse(
            chart_id=chart_id,
            title=fig.layout.title.text,
            type=chart_type,
            data=chart_json,
            config={
                "displayModeBar": True,
                "responsive": True,
                "toImageButtonOptions": {
                    "format": "png",
                    "filename": f"{metric}_{chart_type}",
                    "height": 500,
                    "width": 700,
                    "scale": 1
                }
            },
            generated_at=datetime.utcnow()
        )
        
        # Cache the chart
        cache_key = f"chart:{chart_type}:{metric}:{time_range}"
        redis_client.setex(cache_key, 1800, json.dumps(response.dict(), default=str))
        
        return response
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error generating chart: {e}")
        raise HTTPException(status_code=500, detail="Failed to generate chart")

# Dashboard endpoints
@app.get("/v1/dashboards", response_model=List[DashboardResponse])
@auth.require_auth("analytics", "read")
async def list_dashboards():
    """List available dashboards"""
    try:
        # Sample dashboards
        dashboards = [
            DashboardResponse(
                id="platform-overview",
                name="Platform Overview",
                description="High-level platform metrics and KPIs",
                widgets=[
                    {
                        "id": "active-users",
                        "type": "metric",
                        "title": "Active Users",
                        "metric": "active_users",
                        "size": {"width": 3, "height": 2}
                    },
                    {
                        "id": "api-requests",
                        "type": "chart",
                        "title": "API Requests",
                        "chart_type": "line",
                        "metric": "api_requests",
                        "size": {"width": 6, "height": 4}
                    }
                ],
                layout={"columns": 12, "rows": 8},
                last_updated=datetime.utcnow(),
                auto_refresh=True
            ),
            DashboardResponse(
                id="ai-ml-metrics",
                name="AI/ML Metrics",
                description="Machine learning model performance and usage",
                widgets=[
                    {
                        "id": "model-predictions",
                        "type": "metric",
                        "title": "Model Predictions",
                        "metric": "model_predictions",
                        "size": {"width": 3, "height": 2}
                    },
                    {
                        "id": "model-accuracy",
                        "type": "chart",
                        "title": "Model Accuracy Trends",
                        "chart_type": "line",
                        "metric": "model_accuracy",
                        "size": {"width": 6, "height": 4}
                    }
                ],
                layout={"columns": 12, "rows": 8},
                last_updated=datetime.utcnow(),
                auto_refresh=True
            )
        ]
        
        return dashboards
    except Exception as e:
        logger.error(f"Error listing dashboards: {e}")
        raise HTTPException(status_code=500, detail="Failed to list dashboards")

@app.get("/v1/dashboards/{dashboard_id}", response_model=DashboardResponse)
@auth.require_auth("analytics", "read")
async def get_dashboard(dashboard_id: str):
    """Get dashboard details"""
    try:
        # Check cache first
        cache_key = f"dashboard:{dashboard_id}"
        cached_result = redis_client.get(cache_key)
        
        if cached_result:
            return DashboardResponse(**json.loads(cached_result))
        
        # Sample dashboard data (in production, fetch from database)
        if dashboard_id == "platform-overview":
            dashboard = DashboardResponse(
                id=dashboard_id,
                name="Platform Overview",
                description="High-level platform metrics and KPIs",
                widgets=[
                    {
                        "id": "active-users",
                        "type": "metric",
                        "title": "Active Users",
                        "metric": "active_users",
                        "value": 1250,
                        "change": 3.7,
                        "size": {"width": 3, "height": 2}
                    },
                    {
                        "id": "api-requests",
                        "type": "chart",
                        "title": "API Requests Over Time",
                        "chart_type": "line",
                        "metric": "api_requests",
                        "size": {"width": 9, "height": 4}
                    },
                    {
                        "id": "error-rate",
                        "type": "metric",
                        "title": "Error Rate",
                        "metric": "error_rate",
                        "value": 0.02,
                        "change": -20.0,
                        "size": {"width": 3, "height": 2}
                    }
                ],
                layout={"columns": 12, "rows": 8},
                last_updated=datetime.utcnow(),
                auto_refresh=True
            )
        else:
            raise HTTPException(status_code=404, detail="Dashboard not found")
        
        # Cache the dashboard
        redis_client.setex(cache_key, 600, json.dumps(dashboard.dict(), default=str))
        
        return dashboard
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error getting dashboard {dashboard_id}: {e}")
        raise HTTPException(status_code=500, detail="Failed to get dashboard")

# Advanced analytics
@app.post("/v1/analytics/query")
@auth.require_auth("analytics", "read")
async def execute_analytics_query(query_request: AnalyticsQuery):
    """Execute advanced analytics query"""
    try:
        # This is a simplified implementation
        # In production, this would use a proper analytics engine
        
        # Generate sample results based on query
        results = {
            "query": query_request.query,
            "dimensions": query_request.dimensions,
            "metrics": query_request.metrics,
            "data": [
                {
                    "dimension_1": "Value A",
                    "dimension_2": "Category 1",
                    "metric_1": 125.5,
                    "metric_2": 89.2
                },
                {
                    "dimension_1": "Value B",
                    "dimension_2": "Category 2",
                    "metric_1": 234.7,
                    "metric_2": 156.8
                }
            ],
            "total_rows": 2,
            "execution_time_ms": 245,
            "generated_at": datetime.utcnow().isoformat()
        }
        
        return results
    except Exception as e:
        logger.error(f"Error executing analytics query: {e}")
        raise HTTPException(status_code=500, detail="Failed to execute analytics query")

@app.post("/v1/analytics/clustering")
@auth.require_auth("analytics", "read")
async def perform_clustering_analysis(
    dataset: str,
    features: List[str],
    n_clusters: Optional[int] = None
):
    """Perform clustering analysis on data"""
    try:
        # Generate sample data for clustering
        np.random.seed(42)
        n_samples = 1000
        data = np.random.randn(n_samples, len(features))
        
        # Add some structure to the data
        for i in range(0, n_samples, 100):
            data[i:i+100] += np.random.randn(len(features)) * 2
        
        # Standardize the data
        scaler = StandardScaler()
        data_scaled = scaler.fit_transform(data)
        
        # Determine optimal number of clusters if not provided
        if n_clusters is None:
            silhouette_scores = []
            K_range = range(2, 11)
            
            for k in K_range:
                kmeans = KMeans(n_clusters=k, random_state=42, n_init=10)
                cluster_labels = kmeans.fit_predict(data_scaled)
                silhouette_avg = silhouette_score(data_scaled, cluster_labels)
                silhouette_scores.append(silhouette_avg)
            
            n_clusters = K_range[np.argmax(silhouette_scores)]
        
        # Perform clustering
        kmeans = KMeans(n_clusters=n_clusters, random_state=42, n_init=10)
        cluster_labels = kmeans.fit_predict(data_scaled)
        
        # Calculate cluster statistics
        cluster_stats = []
        for i in range(n_clusters):
            cluster_data = data[cluster_labels == i]
            stats = {
                "cluster_id": i,
                "size": len(cluster_data),
                "centroid": kmeans.cluster_centers_[i].tolist(),
                "mean_values": np.mean(cluster_data, axis=0).tolist(),
                "std_values": np.std(cluster_data, axis=0).tolist()
            }
            cluster_stats.append(stats)
        
        results = {
            "dataset": dataset,
            "features": features,
            "n_clusters": n_clusters,
            "total_samples": n_samples,
            "silhouette_score": silhouette_score(data_scaled, cluster_labels),
            "cluster_stats": cluster_stats,
            "generated_at": datetime.utcnow().isoformat()
        }
        
        return results
    except Exception as e:
        logger.error(f"Error performing clustering analysis: {e}")
        raise HTTPException(status_code=500, detail="Failed to perform clustering analysis")

# Reports
@app.post("/v1/reports")
@auth.require_auth("analytics", "create")
async def generate_report(
    report_request: ReportRequest,
    background_tasks: BackgroundTasks
):
    """Generate analytics report"""
    try:
        report_id = str(uuid.uuid4())
        
        # Add background task for report generation
        background_tasks.add_task(
            generate_report_background,
            report_id,
            report_request
        )
        
        return {
            "report_id": report_id,
            "name": report_request.name,
            "status": "generating",
            "estimated_completion": (datetime.utcnow() + timedelta(minutes=5)).isoformat()
        }
    except Exception as e:
        logger.error(f"Error generating report: {e}")
        raise HTTPException(status_code=500, detail="Failed to generate report")

@app.get("/v1/reports/{report_id}")
@auth.require_auth("analytics", "read")
async def get_report_status(report_id: str):
    """Get report generation status"""
    try:
        # Check cache for report status
        cache_key = f"report:{report_id}"
        cached_status = redis_client.get(cache_key)
        
        if cached_status:
            return json.loads(cached_status)
        else:
            raise HTTPException(status_code=404, detail="Report not found")
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error getting report status: {e}")
        raise HTTPException(status_code=500, detail="Failed to get report status")

# Background tasks
async def generate_report_background(report_id: str, report_request: ReportRequest):
    """Background task to generate report"""
    try:
        logger.info(f"Generating report {report_id}: {report_request.name}")
        
        # Update status to processing
        status = {
            "report_id": report_id,
            "name": report_request.name,
            "status": "processing",
            "progress": 0,
            "started_at": datetime.utcnow().isoformat()
        }
        redis_client.setex(f"report:{report_id}", 3600, json.dumps(status))
        
        # Simulate report generation
        for progress in [25, 50, 75, 100]:
            await asyncio.sleep(30)  # Simulate processing time
            status["progress"] = progress
            redis_client.setex(f"report:{report_id}", 3600, json.dumps(status))
        
        # Complete the report
        status.update({
            "status": "completed",
            "completed_at": datetime.utcnow().isoformat(),
            "download_url": f"/v1/reports/{report_id}/download",
            "file_size": "2.5MB"
        })
        redis_client.setex(f"report:{report_id}", 3600, json.dumps(status))
        
        logger.info(f"Report {report_id} generated successfully")
    except Exception as e:
        logger.error(f"Error generating report {report_id}: {e}")
        
        # Update status to failed
        status = {
            "report_id": report_id,
            "name": report_request.name,
            "status": "failed",
            "error": str(e),
            "failed_at": datetime.utcnow().isoformat()
        }
        redis_client.setex(f"report:{report_id}", 3600, json.dumps(status))

if __name__ == "__main__":
    uvicorn.run(
        "main:app",
        host="0.0.0.0",
        port=int(os.getenv("PORT", 8080)),
        reload=os.getenv("ENVIRONMENT") == "development"
    )
