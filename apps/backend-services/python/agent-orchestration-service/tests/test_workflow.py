import pytest
from app.models.agent import AgentTaskRequest
from app.orchestrator.workflow import execute_workflow

@pytest.mark.asyncio
async def test_execute_workflow():
    request = AgentTaskRequest(workflow_id="test123", parameters={"topic": "AI"})
    result = await execute_workflow(request)
    assert "summary" in result
