agent-orchestration-service/
├── Dockerfile
├── pyproject.toml
├── poetry.lock
├── app/
│   ├── main.py
│   ├── api/
│   │   └── agents.py
│   ├── orchestrator/
│   │   └── workflow.py
│   ├── adapters/
│   │   └── kafka_adapter.py
│   └── models/
│       └── agent.py
└── tests/
    └── test_workflow.py
