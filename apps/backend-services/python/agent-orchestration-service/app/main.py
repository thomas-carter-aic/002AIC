from fastapi import FastAPI
from app.api.agents import router as agent_router

app = FastAPI(title="Agent Orchestration Service")

app.include_router(agent_router, prefix="/agents")
