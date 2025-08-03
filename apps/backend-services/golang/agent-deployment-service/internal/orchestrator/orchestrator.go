package orchestrator

import (
	"context"
	"log"
	"sync"

	"github.com/thomas-caarter-aic/agent-deployment-service/internal/adapters/container"
	"github.com/thomas-caarter-aic/agent-deployment-service/internal/model"
)

// Orchestrator manages concurrent deployment operations
type Orchestrator struct {
	taskQueue chan model.DeploymentTask
	wg        sync.WaitGroup
	k8s       container.Adapter
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewOrchestrator initializes orchestrator with Kubernetes adapter
func NewOrchestrator(k8sAdapter container.Adapter, workers int) *Orchestrator {
	ctx, cancel := context.WithCancel(context.Background())
	o := &Orchestrator{
		taskQueue: make(chan model.DeploymentTask, 100),
		k8s:       k8sAdapter,
		ctx:       ctx,
		cancel:    cancel,
	}
	o.startWorkers(workers)
	return o
}

func (o *Orchestrator) startWorkers(count int) {
	for i := 0; i < count; i++ {
		o.wg.Add(1)
		go func(workerID int) {
			defer o.wg.Done()
			log.Printf("Worker %d started", workerID)
			for {
				select {
				case <-o.ctx.Done():
					log.Printf("Worker %d stopping", workerID)
					return
				case task := <-o.taskQueue:
					log.Printf("Worker %d processing task: %+v", workerID, task)
					if err := o.handleTask(task); err != nil {
						log.Printf("Worker %d error: %v", workerID, err)
					}
				}
			}
		}(i)
	}
}

func (o *Orchestrator) handleTask(task model.DeploymentTask) error {
	switch task.Action {
	case "CREATE":
		return o.k8s.DeployAgent(o.ctx, task)
	case "UPDATE":
		return o.k8s.UpdateAgent(o.ctx, task)
	case "DELETE":
		return o.k8s.DeleteAgent(o.ctx, task)
	default:
		return nil
	}
}

// SubmitTask enqueues a deployment task
func (o *Orchestrator) SubmitTask(task model.DeploymentTask) {
	o.taskQueue <- task
}

// Shutdown gracefully stops orchestrator
func (o *Orchestrator) Shutdown() {
	o.cancel()
	o.wg.Wait()
	close(o.taskQueue)
}
