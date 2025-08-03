| API Layer (gin/gRPC) |
           |
| Deployment Controller |
           |
| Deployment Orchestrator |-----> [Worker Pool (Concurrency)]
           |
| Container Management Adapter |----> Kubernetes API / Docker API
           |
| Messaging Adapter (Kafka) |
           |
| Observability Layer (OpenTelemetry) |
