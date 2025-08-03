package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
)

// CRD GroupVersionResource for Agent
var gvr = schema.GroupVersionResource{
	Group:    "ai.example.com",
	Version:  "v1",
	Resource: "agents",
}

// Controller struct for hexagonal architecture
type Controller struct {
	dynClient     dynamic.Interface
	kubeClient    kubernetes.Interface
	informer      cache.SharedIndexInformer
	queue         workqueue.RateLimitingInterface
	reconciler    Reconciler // Domain interface
	maxRetries    int
	resyncPeriod  time.Duration
}

// Reconciler interface (hexagonal domain port)
type Reconciler interface {
	Reconcile(agent *unstructured.Unstructured) error
}

// K8sAdapter implements Reconciler (adapter for K8s)
type K8sAdapter struct {
	kubeClient kubernetes.Interface
	namespace  string
}

func (a *K8sAdapter) Reconcile(agent *unstructured.Unstructured) error {
	agentName := agent.GetName()
	podName := agentName + "-pod"

	// Check if Pod exists (actual state)
	_, err := a.kubeClient.CoreV1().Pods(a.namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err == nil {
		klog.Infof("Pod %s already exists for agent %s", podName, agentName)
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

	// Create Pod (desired state - simulate agent deployment)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: a.namespace,
			Labels:    map[string]string{"app": "ai-agent"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "agent-container",
					Image: "busybox", // Simulate agent image
					Command: []string{"sleep", "3600"},
				},
			},
		},
	}
	_, err = a.kubeClient.CoreV1().Pods(a.namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	klog.Infof("Deployed Pod %s for agent %s", podName, agentName)
	return nil
}

func NewController(config *rest.Config, namespace string, resyncPeriod time.Duration) (*Controller, error) {
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynClient, resyncPeriod, namespace, nil)
	informer := factory.ForResource(gvr).Informer()

	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "agent-deployment")

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
		UpdateFunc: func(old, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				queue.Add(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
	})

	return &Controller{
		dynClient:    dynClient,
		kubeClient:   kubeClient,
		informer:     informer,
		queue:        queue,
		reconciler:   &K8sAdapter{kubeClient: kubeClient, namespace: namespace},
		maxRetries:   5,
		resyncPeriod: resyncPeriod,
	}, nil
}

func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Info("Starting informer")
	go c.informer.Run(stopCh)

	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		klog.Error("Failed to sync cache")
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}

	<-stopCh
	klog.Info("Stopping controller")
}

func (c *Controller) worker() {
	for c.processNextItem() {
	}
}

func (c *Controller) processNextItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.reconcile(key.(string))
	if err == nil {
		c.queue.Forget(key)
		return true
	}

	if c.queue.NumRequeues(key) < c.maxRetries {
		klog.Errorf("Error reconciling %v: %v", key, err)
		c.queue.AddRateLimited(key)
		return true
	}

	utilruntime.HandleError(err)
	c.queue.Forget(key)
	return true
}

func (c *Controller) reconcile(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	agent, err := c.dynClient.Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		klog.Infof("Agent %s deleted", key)
		return nil // Handle deletion if needed
	}
	if err != nil {
		return err
	}

	return c.reconciler.Reconcile(agent)
}

func main() {
	var kubeconfig *string
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %v", err)
	}

	controller, err := NewController(config, metav1.NamespaceDefault, 5*time.Minute)
	if err != nil {
		klog.Fatalf("Error creating controller: %v", err)
	}

	stopCh := make(chan struct{})
	defer close(stopCh)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		close(stopCh)
	}()

	controller.Run(3, stopCh) // 3 worker goroutines for concurrency
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // Windows
}

// Note: For testing, apply a sample Agent CRD and CR YAML (not included here; see Kubernetes docs for CRD setup).
