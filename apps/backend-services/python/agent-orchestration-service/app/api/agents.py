from fastapi import APIRouter, HTTPException
from app.models.agent import AgentTaskRequest, AgentTaskResponse
from app.orchestrator.workflow import execute_workflow

router = APIRouter()

@router.post("/execute", response_model=AgentTaskResponse)
async def execute_agent_task(request: AgentTaskRequest):
    try:
        result = await execute_workflow(request)
        return AgentTaskResponse(status="success", result=result)
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))
