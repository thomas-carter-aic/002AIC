import pytest
from fastapi.testclient import TestClient
from app.main import app

client = TestClient(app)

def test_execute_agent_task():
    response = client.post("/agents/execute", json={
        "workflow_id": "test123",
        "parameters": {"topic": "AI"}
    })
    assert response.status_code == 200
    assert response.json()["status"] == "success"
