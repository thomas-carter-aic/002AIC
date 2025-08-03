package container

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/thomas-caarter-aic/agent-deployment-service/internal/model"
)

// Adapter defines Kubernetes interactions
type Adapter interface {
	DeployAgent(ctx context.Context, task model.DeploymentTask) error
	UpdateAgent(ctx context.Context, task model.DeploymentTask) error
	DeleteAgent(ctx context.Context, task model.DeploymentTask) error
}

// KubernetesAdapter implementation
type KubernetesAdapter struct {
	clientset *kubernetes.Clientset
	namespace string
}

// NewKubernetesAdapter initializes Kubernetes client
func NewKubernetesAdapter(namespace string) (*KubernetesAdapter, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &KubernetesAdapter{
		clientset: clientset,
		namespace: namespace,
	}, nil
}

func (k *KubernetesAdapter) DeployAgent(ctx context.Context, task model.DeploymentTask) error {
	deployments := k.clientset.AppsV1().Deployments(k.namespace)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: task.AgentID,
			Labels: map[string]string{
				"tenant": task.TenantID,
				"agent":  task.AgentID,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"agent": task.AgentID,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"agent": task.AgentID,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  task.AgentID,
							Image: task.ContainerImg,
						},
					},
				},
			},
		},
	}

	_, err := deployments.Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("deploy error: %w", err)
	}
	return nil
}

func (k *KubernetesAdapter) UpdateAgent(ctx context.Context, task model.DeploymentTask) error {
	deployments := k.clientset.AppsV1().Deployments(k.namespace)

	deployment, err := deployments.Get(ctx, task.AgentID, metav1.GetOptions{})
	if err != nil {
		return err
	}

	deployment.Spec.Template.Spec.Containers[0].Image = task.ContainerImg
	_, err = deployments.Update(ctx, deployment, metav1.UpdateOptions{})
	return err
}

func (k *KubernetesAdapter) DeleteAgent(ctx context.Context, task model.DeploymentTask) error {
	deployments := k.clientset.AppsV1().Deployments(k.namespace)

	return deployments.Delete(ctx, task.AgentID, metav1.DeleteOptions{})
}

func int32Ptr(i int32) *int32 { return &i }
