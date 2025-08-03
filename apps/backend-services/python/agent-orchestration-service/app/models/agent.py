from pydantic import BaseModel

class AgentTaskRequest(BaseModel):
    workflow_id: str
    parameters: dict

class AgentTaskResponse(BaseModel):
    status: str
    result: dict
