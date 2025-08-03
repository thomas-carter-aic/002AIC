# Kong + Kuma API Gateway & Service Mesh Setup

This directory contains the configuration and deployment files for Kong API Gateway and Kuma Service Mesh, which together provide the API gateway and service mesh infrastructure for the 002AIC platform.

## Architecture Overview

- **Kong Gateway**: Handles external API traffic, authentication, rate limiting, and routing
- **Kuma Service Mesh**: Provides service-to-service communication, security (mTLS), observability, and traffic management
- **PostgreSQL**: Kong's datastore for configuration
- **Jaeger**: Distributed tracing
- **Prometheus + Grafana**: Metrics and monitoring

## Directory Structure

```
infra/
├── helm/kong/                    # Helm charts and values
│   ├── Chart.yaml
│   └── values.yaml
├── k8s/                         # Kubernetes manifests
│   ├── namespaces.yaml
│   ├── kong-plugins.yaml
│   ├── kong-ingress.yaml
│   └── kuma-mesh-policies.yaml
├── argocd/                      # ArgoCD applications
│   └── kong-kuma-app.yaml
├── docker-compose/              # Local development
│   ├── kong-kuma-local.yml
│   └── prometheus.yml
└── deployments/                 # Deployment scripts
    └── kong-kuma-deployment.sh
```

## Quick Start

### Local Development (Docker Compose)

1. **Start the stack:**
   ```bash
   cd infra/docker-compose
   docker-compose -f kong-kuma-local.yml up -d
   ```

2. **Access services:**
   - Kong Admin API: http://localhost:8001
   - Kong Manager (GUI): http://localhost:8002
   - Kong Proxy (API Gateway): http://localhost:8000
   - Kuma Control Plane GUI: http://localhost:5681/gui
   - Jaeger UI: http://localhost:16686
   - Prometheus: http://localhost:9090
   - Grafana: http://localhost:3000 (admin/admin)

### Kubernetes Deployment

1. **Deploy using the script:**
   ```bash
   ./infra/deployments/kong-kuma-deployment.sh
   ```

2. **Or deploy with ArgoCD:**
   ```bash
   kubectl apply -f infra/argocd/kong-kuma-app.yaml
   ```

## Configuration

### Kong Plugins

The following plugins are configured globally:

- **Rate Limiting**: 1000/minute, 10000/hour
- **CORS**: Configured for web applications
- **Prometheus**: Metrics collection
- **JWT Authentication**: For secured endpoints
- **Request Size Limiting**: 10MB max payload

### Kuma Mesh Policies

- **mTLS**: Enabled by default for all services
- **Traffic Permissions**: Configured for service-to-service communication
- **Circuit Breaker**: Configured for AI/ML services
- **Retry Policy**: Configured for critical services
- **Timeout Policy**: 30s request timeout, 60s idle timeout

### Service Routing

API routes are configured in `kong-ingress.yaml`:

- `/v1/auth` → authentication-authorization-service
- `/v1/ai` → agent-orchestration-service
- `/v1/models` → model-management-service
- `/v1/data` → data-management-service
- `/v1/pipelines` → pipeline-execution-service
- `/v1/workflows` → workflow-orchestration-service
- `/v1/analytics` → analytics-service
- `/v1/monitoring` → monitoring-metrics-service

## Microservice Integration

### Adding a New Service

1. **Deploy your service** with Kuma sidecar injection:
   ```yaml
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     name: your-service
     namespace: ai-services  # or appropriate namespace
   spec:
     template:
       metadata:
         annotations:
           kuma.io/sidecar-injection: enabled
   ```

2. **Add Kong route** in `kong-ingress.yaml`:
   ```yaml
   - path: /v1/your-service
     pathType: Prefix
     backend:
       service:
         name: your-service
         port:
           number: 8080
   ```

3. **Configure Kuma traffic permissions** in `kuma-mesh-policies.yaml`

### Authentication Integration

Services requiring authentication should:

1. Use the `jwt-auth` Kong plugin
2. Validate JWT tokens with the authentication-authorization-service
3. Include the `Authorization: Bearer <token>` header

Example Kong service configuration:
```yaml
apiVersion: configuration.konghq.com/v1
kind: KongService
metadata:
  name: protected-service
  annotations:
    konghq.com/plugins: jwt-auth
```

## Monitoring & Observability

### Metrics

- **Kong metrics**: Available at `http://kong:8001/metrics`
- **Kuma metrics**: Available at `http://kuma-control-plane:5681/metrics`
- **Service metrics**: Each service should expose `/metrics` endpoint

### Tracing

- **Jaeger**: Distributed tracing is automatically enabled for all mesh services
- **Access**: http://localhost:16686 (local) or through ingress

### Logging

- **Kong logs**: Structured JSON logs to stdout/stderr
- **Kuma logs**: Access logs configured for all services
- **Service logs**: Should follow structured logging format

## Security

### mTLS

- Automatically enabled between all mesh services
- Certificates rotated daily
- CA certificate valid for 10 years

### Network Policies

- Default deny-all traffic policy
- Explicit allow rules for service communication
- Kong proxy allowed to communicate with all services

### Authentication

- JWT-based authentication for external API access
- Service-to-service authentication via mTLS
- Admin interfaces protected by network policies

## Troubleshooting

### Common Issues

1. **Kong not starting**: Check PostgreSQL connection
   ```bash
   kubectl logs deployment/kong-kong -n kong-system
   ```

2. **Kuma sidecar not injecting**: Verify namespace labels
   ```bash
   kubectl get namespace ai-services -o yaml
   ```

3. **Service communication failing**: Check traffic permissions
   ```bash
   kubectl get trafficpermissions -n kuma-system
   ```

### Useful Commands

```bash
# Check Kong configuration
curl http://localhost:8001/services

# Check Kuma mesh status
curl http://localhost:5681/meshes

# View Kong plugins
kubectl get kongplugins -n kong-system

# Check service mesh policies
kubectl get mesh,trafficpermission,circuitbreaker -n kuma-system
```

## Next Steps

1. **Deploy authentication-authorization-service** (Go)
2. **Configure JWT issuer** and validation
3. **Set up monitoring dashboards** in Grafana
4. **Implement API versioning** strategy
5. **Add API documentation** with Kong Developer Portal
6. **Configure backup** and disaster recovery

## Resources

- [Kong Documentation](https://docs.konghq.com/)
- [Kuma Documentation](https://kuma.io/docs/)
- [Kong Kubernetes Ingress Controller](https://github.com/Kong/kubernetes-ingress-controller)
- [Kuma Service Mesh Policies](https://kuma.io/docs/latest/policies/)
