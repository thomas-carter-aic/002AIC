from pydantic import BaseModel
from typing import List, Dict, Any

class AnalysisRequest(BaseModel):
    data: List[Dict[str, Any]]

class AnalysisResponse(BaseModel):
    status: str
    results: Dict[str, Any]
