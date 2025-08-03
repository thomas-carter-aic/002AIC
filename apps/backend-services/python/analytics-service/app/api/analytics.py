from fastapi import APIRouter, HTTPException
from app.models.analytics import AnalysisRequest, AnalysisResponse
from app.analytics_engine.processor import perform_analysis

router = APIRouter()

@router.post("/analyze", response_model=AnalysisResponse)
async def analyze_data(request: AnalysisRequest):
    try:
        results = perform_analysis(request.data)
        return AnalysisResponse(status="success", results=results)
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))
