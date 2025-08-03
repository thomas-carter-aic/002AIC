Using Kong + Kuma as an off-the-shelf solution instead of building a custom Node.js 
API gateway has several compelling advantages for your use case:

## Why Kong + Kuma Makes Sense

Kong Benefits:
• **Battle-tested** at enterprise scale with proven reliability
• **Rich plugin ecosystem** for authentication, rate limiting, logging, and monitoring
• **Multi-protocol support** (HTTP/HTTPS, gRPC, WebSocket, TCP/TLS)
• **Declarative configuration** that fits well with your GitOps approach (ArgoCD)
• **Built-in observability** with metrics, tracing, and logging
• **Developer portal** capabilities for API documentation

Kuma Service Mesh Benefits:
• **Zero-trust security** between your 36 microservices
• **Traffic management** and load balancing
• **Observability** with distributed tracing across services
• **Policy enforcement** at the mesh level
• **mTLS** automatic encryption between services

## Strategic Advantages for Your Architecture

1. Faster time-to-market: Focus development effort on your core AI/ML services instead of gateway infrastructure
2. Reduced maintenance burden: Let Kong handle gateway concerns while you focus on business logic
3. Better integration: Kong integrates well with your planned auth service and monitoring stack
4. Scalability: Kong can handle the traffic patterns of 36 microservices better than a custom solution
5. Security: Mature security features vs. building your own

## Implementation Approach

I'd recommend:
1. Deploy Kong as your API gateway with Kubernetes ingress
2. Implement Kuma service mesh for inter-service communication
3. Keep the api-gateway-service slot in your architecture but treat it as Kong configuration/management
4. Build a thin configuration service in Node.js to manage Kong declaratively if needed

This approach lets you move faster to your authentication-authorization-service (Go) and start building the AI/ML services 
that differentiate your platform.