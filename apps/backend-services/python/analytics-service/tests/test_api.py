from fastapi.testclient import TestClient
from app.main import app

client = TestClient(app)

def test_analyze_data():
    response = client.post("/analytics/analyze", json={
        "data": [{"val": 1}, {"val": 2}, {"val": 3}]
    })
    assert response.status_code == 200
    assert response.json()["status"] == "success"
