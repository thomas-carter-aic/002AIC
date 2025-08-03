FROM golang:1.22-alpine AS build
WORKDIR /app
COPY . .
RUN go mod download && go build -o agent-deployment-service ./cmd/agent-deployment-service

FROM alpine:latest
COPY --from=build /app/agent-deployment-service /agent-deployment-service
ENTRYPOINT [\"/agent-deployment-service\"]
