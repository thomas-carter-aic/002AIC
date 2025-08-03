package main

import (
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"

    "github.com/gin-gonic/gin"
    "github.com/yourorg/agent-deployment-service/internal/api"
    "github.com/yourorg/agent-deployment-service/internal/orchestrator"
)

func main() {
    router := gin.Default()

    orch := orchestrator.NewDeploymentOrchestrator()
    api.RegisterRoutes(router, orch)

    srv := &http.Server{
        Addr:    ":8080",
        Handler: router,
    }

    go func() {
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("listen: %s\n", err)
        }
    }()
    
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    log.Println("Shutdown Server ...")
}
