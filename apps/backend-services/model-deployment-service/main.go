package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// ModelDeployment represents a deployed model
type ModelDeployment struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	Name            string    `json:"name" gorm:"uniqueIndex;not null"`
	ModelID         string    `json:"model_id" gorm:"not null"`
	ModelVersion    string    `json:"model_version" gorm:"not null"`
	Framework       string    `json:"framework" gorm:"not null"`
	Status          string    `json:"status" gorm:"default:'pending'"`
	Environment     string    `json:"environment" gorm:"default:'production'"`
	Replicas        int       `json:"replicas" gorm:"default:1"`
	CPU             string    `json:"cpu" gorm:"default:'500m'"`
	Memory          string    `json:"memory" gorm:"default:'1Gi'"`
	GPU             int       `json:"gpu" gorm:"default:0"`
	EndpointURL     string    `json:"endpoint_url"`
	HealthCheckURL  string    `json:"health_check_url"`
	MetricsURL      string    `json:"metrics_url"`
	AutoScaling     bool      `json:"auto_scaling" gorm:"default:true"`
	MinReplicas     int       `json:"min_replicas" gorm:"default:1"`
	MaxReplicas     int       `json:"max_replicas" gorm:"default:10"`
	TargetCPU       int       `json:"target_cpu" gorm:"default:70"`
	TargetMemory    int       `json:"target_memory" gorm:"default:80"`
	Config          string    `json:"config" gorm:"type:jsonb"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	DeployedAt      *time.Time `json:"deployed_at"`
	CreatedBy       string    `json:"created_by"`
}

// DeploymentMetrics represents deployment performance metrics
type DeploymentMetrics struct {
	ID               uint      `json:"id" gorm:"primaryKey"`
	DeploymentID     uint      `json:"deployment_id" gorm:"not null"`
	Deployment       ModelDeployment `json:"deployment" gorm:"foreignKey:DeploymentID"`
	RequestCount     int64     `json:"request_count"`
	ErrorCount       int64     `json:"error_count"`
	AvgLatencyMs     float64   `json:"avg_latency_ms"`
	P95LatencyMs     float64   `json:"p95_latency_ms"`
	P99LatencyMs     float64   `json:"p99_latency_ms"`
	ThroughputRPS    float64   `json:"throughput_rps"`
	CPUUtilization   float64   `json:"cpu_utilization"`
	MemoryUtilization float64  `json:"memory_utilization"`
	GPUUtilization   float64   `json:"gpu_utilization"`
	Timestamp        time.Time `json:"timestamp"`
}

// ModelDeploymentService handles model deployment operations
type ModelDeploymentService struct {
	db        *gorm.DB
	k8sClient *kubernetes.Clientset
	logger    *zap.Logger
}

// Metrics
var (
	activeDeployments = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "model_deployments_active_total",
			Help: "Total number of active model deployments",
		},
		[]string{"framework", "environment"},
	)
	deploymentRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "model_deployment_requests_total",
			Help: "Total number of model deployment requests",
		},
		[]string{"framework", "status"},
	)
	deploymentLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "model_deployment_latency_seconds",
			Help: "Model deployment latency",
		},
		[]string{"framework", "environment"},
	)
	modelInferenceRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "model_inference_requests_total",
			Help: "Total number of model inference requests",
		},
		[]string{"deployment", "status"},
	)
)

func main() {
	// Initialize logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Initialize database
	db, err := initDatabase()
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}

	// Initialize Kubernetes client
	k8sClient, err := initKubernetesClient()
	if err != nil {
		logger.Fatal("Failed to initialize Kubernetes client", zap.Error(err))
	}

	// Initialize service
	deploymentService := &ModelDeploymentService{
		db:        db,
		k8sClient: k8sClient,
		logger:    logger,
	}

	// Start metrics collection routine
	go deploymentService.startMetricsCollection()

	// Initialize Gin router
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	// CORS middleware
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":    "healthy",
			"service":   "model-deployment-service",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"version":   "1.0.0",
		})
	})

	// Metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Model deployment API routes
	v1 := router.Group("/v1/deployments")
	{
		// Deployment management
		v1.GET("/", deploymentService.listDeployments)
		v1.POST("/", deploymentService.createDeployment)
		v1.GET("/:id", deploymentService.getDeployment)
		v1.PUT("/:id", deploymentService.updateDeployment)
		v1.DELETE("/:id", deploymentService.deleteDeployment)
		
		// Deployment operations
		v1.POST("/:id/scale", deploymentService.scaleDeployment)
		v1.POST("/:id/restart", deploymentService.restartDeployment)
		v1.POST("/:id/rollback", deploymentService.rollbackDeployment)
		v1.GET("/:id/status", deploymentService.getDeploymentStatus)
		v1.GET("/:id/logs", deploymentService.getDeploymentLogs)
		
		// Model serving
		v1.POST("/:id/predict", deploymentService.predict)
		v1.POST("/:id/batch-predict", deploymentService.batchPredict)
		
		// Metrics and monitoring
		v1.GET("/:id/metrics", deploymentService.getDeploymentMetrics)
		v1.GET("/:id/health", deploymentService.checkDeploymentHealth)
		
		// A/B testing
		v1.POST("/:id/ab-test", deploymentService.createABTest)
		v1.GET("/:id/ab-test", deploymentService.getABTestResults)
		
		// Canary deployments
		v1.POST("/:id/canary", deploymentService.createCanaryDeployment)
		v1.POST("/:id/canary/promote", deploymentService.promoteCanaryDeployment)
	}

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logger.Info("Starting Model Deployment Service", zap.String("port", port))
	log.Fatal(http.ListenAndServe(":"+port, router))
}

func initDatabase() (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_USER", "postgres"),
		getEnv("DB_PASSWORD", "password"),
		getEnv("DB_NAME", "model_deployment"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_SSLMODE", "disable"),
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Auto-migrate the schema
	err = db.AutoMigrate(&ModelDeployment{}, &DeploymentMetrics{})
	if err != nil {
		return nil, err
	}

	return db, nil
}

func initKubernetesClient() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get Kubernetes config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return clientset, nil
}

func (ds *ModelDeploymentService) listDeployments(c *gin.Context) {
	framework := c.Query("framework")
	environment := c.Query("environment")
	status := c.Query("status")
	
	var deployments []ModelDeployment
	query := ds.db
	
	if framework != "" {
		query = query.Where("framework = ?", framework)
	}
	if environment != "" {
		query = query.Where("environment = ?", environment)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	
	if err := query.Find(&deployments).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to fetch deployments"})
		return
	}
	
	c.JSON(200, gin.H{"deployments": deployments})
}

func (ds *ModelDeploymentService) createDeployment(c *gin.Context) {
	var deployment ModelDeployment
	if err := c.ShouldBindJSON(&deployment); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	
	start := time.Now()
	
	deployment.CreatedAt = time.Now()
	deployment.UpdatedAt = time.Now()
	deployment.Status = "deploying"
	
	// Save deployment to database
	if err := ds.db.Create(&deployment).Error; err != nil {
		deploymentRequests.WithLabelValues(deployment.Framework, "failed").Inc()
		c.JSON(500, gin.H{"error": "Failed to create deployment"})
		return
	}
	
	// Deploy to Kubernetes
	err := ds.deployModelToKubernetes(&deployment)
	if err != nil {
		deployment.Status = "failed"
		ds.db.Save(&deployment)
		deploymentRequests.WithLabelValues(deployment.Framework, "failed").Inc()
		deploymentLatency.WithLabelValues(deployment.Framework, deployment.Environment).Observe(time.Since(start).Seconds())
		c.JSON(500, gin.H{"error": "Failed to deploy model"})
		return
	}
	
	// Update deployment status
	now := time.Now()
	deployment.Status = "running"
	deployment.DeployedAt = &now
	deployment.EndpointURL = fmt.Sprintf("https://api.002aic.com/v1/models/%s/predict", deployment.Name)
	deployment.HealthCheckURL = fmt.Sprintf("https://api.002aic.com/v1/models/%s/health", deployment.Name)
	deployment.MetricsURL = fmt.Sprintf("https://api.002aic.com/v1/models/%s/metrics", deployment.Name)
	ds.db.Save(&deployment)
	
	// Update metrics
	activeDeployments.WithLabelValues(deployment.Framework, deployment.Environment).Inc()
	deploymentRequests.WithLabelValues(deployment.Framework, "success").Inc()
	deploymentLatency.WithLabelValues(deployment.Framework, deployment.Environment).Observe(time.Since(start).Seconds())
	
	ds.logger.Info("Model deployed", 
		zap.String("name", deployment.Name),
		zap.String("model_id", deployment.ModelID),
		zap.String("framework", deployment.Framework),
		zap.String("endpoint", deployment.EndpointURL))
	
	c.JSON(201, deployment)
}

func (ds *ModelDeploymentService) deployModelToKubernetes(deployment *ModelDeployment) error {
	namespace := "model-serving"
	
	// Parse configuration
	var config map[string]interface{}
	if deployment.Config != "" {
		json.Unmarshal([]byte(deployment.Config), &config)
	}
	
	// Determine container image based on framework
	var image string
	switch deployment.Framework {
	case "tensorflow":
		image = "tensorflow/serving:latest"
	case "pytorch":
		image = "pytorch/torchserve:latest"
	case "sklearn":
		image = "002aic/sklearn-serving:latest"
	case "onnx":
		image = "mcr.microsoft.com/onnxruntime/server:latest"
	default:
		image = "002aic/generic-serving:latest"
	}
	
	// Create environment variables
	envVars := []corev1.EnvVar{
		{Name: "MODEL_ID", Value: deployment.ModelID},
		{Name: "MODEL_VERSION", Value: deployment.ModelVersion},
		{Name: "FRAMEWORK", Value: deployment.Framework},
		{Name: "DEPLOYMENT_NAME", Value: deployment.Name},
	}
	
	// Add custom environment variables from config
	if envConfig, ok := config["environment"]; ok {
		if envMap, ok := envConfig.(map[string]interface{}); ok {
			for key, value := range envMap {
				envVars = append(envVars, corev1.EnvVar{
					Name:  key,
					Value: fmt.Sprintf("%v", value),
				})
			}
		}
	}
	
	// Create Deployment
	k8sDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deployment.Name,
			Namespace: namespace,
			Labels: map[string]string{
				"app":        deployment.Name,
				"model-id":   deployment.ModelID,
				"framework":  deployment.Framework,
				"managed-by": "002aic-platform",
				"component":  "model-serving",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(int32(deployment.Replicas)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": deployment.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":       deployment.Name,
						"model-id":  deployment.ModelID,
						"framework": deployment.Framework,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "model-server",
							Image: image,
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 8080,
								},
								{
									Name:          "grpc",
									ContainerPort: 8081,
								},
								{
									Name:          "metrics",
									ContainerPort: 8082,
								},
							},
							Env: envVars,
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    parseQuantity(deployment.CPU),
									corev1.ResourceMemory: parseQuantity(deployment.Memory),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    parseQuantity(deployment.CPU),
									corev1.ResourceMemory: parseQuantity(deployment.Memory),
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/health",
										Port: intstr.FromInt(8080),
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       10,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/ready",
										Port: intstr.FromInt(8080),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       5,
							},
						},
					},
				},
			},
		},
	}
	
	// Add GPU resources if specified
	if deployment.GPU > 0 {
		gpuResource := corev1.ResourceList{
			"nvidia.com/gpu": parseQuantity(strconv.Itoa(deployment.GPU)),
		}
		k8sDeployment.Spec.Template.Spec.Containers[0].Resources.Requests["nvidia.com/gpu"] = gpuResource["nvidia.com/gpu"]
		k8sDeployment.Spec.Template.Spec.Containers[0].Resources.Limits["nvidia.com/gpu"] = gpuResource["nvidia.com/gpu"]
	}
	
	_, err := ds.k8sClient.AppsV1().Deployments(namespace).Create(
		context.TODO(), k8sDeployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}
	
	// Create Service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deployment.Name,
			Namespace: namespace,
			Labels: map[string]string{
				"app":        deployment.Name,
				"model-id":   deployment.ModelID,
				"framework":  deployment.Framework,
				"managed-by": "002aic-platform",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": deployment.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromInt(8080),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       "grpc",
					Port:       8081,
					TargetPort: intstr.FromInt(8081),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       "metrics",
					Port:       8082,
					TargetPort: intstr.FromInt(8082),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
	
	_, err = ds.k8sClient.CoreV1().Services(namespace).Create(
		context.TODO(), service, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}
	
	// Create HorizontalPodAutoscaler if auto-scaling is enabled
	if deployment.AutoScaling {
		hpa := &autoscalingv2.HorizontalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deployment.Name,
				Namespace: namespace,
			},
			Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       deployment.Name,
				},
				MinReplicas: int32Ptr(int32(deployment.MinReplicas)),
				MaxReplicas: int32(deployment.MaxReplicas),
				Metrics: []autoscalingv2.MetricSpec{
					{
						Type: autoscalingv2.ResourceMetricSourceType,
						Resource: &autoscalingv2.ResourceMetricSource{
							Name: corev1.ResourceCPU,
							Target: autoscalingv2.MetricTarget{
								Type:               autoscalingv2.UtilizationMetricType,
								AverageUtilization: int32Ptr(int32(deployment.TargetCPU)),
							},
						},
					},
				},
			},
		}
		
		_, err = ds.k8sClient.AutoscalingV2().HorizontalPodAutoscalers(namespace).Create(
			context.TODO(), hpa, metav1.CreateOptions{})
		if err != nil {
			ds.logger.Warn("Failed to create HPA", zap.Error(err))
		}
	}
	
	return nil
}

func (ds *ModelDeploymentService) predict(c *gin.Context) {
	id := c.Param("id")
	
	var deployment ModelDeployment
	if err := ds.db.First(&deployment, id).Error; err != nil {
		c.JSON(404, gin.H{"error": "Deployment not found"})
		return
	}
	
	if deployment.Status != "running" {
		c.JSON(503, gin.H{"error": "Deployment not ready"})
		return
	}
	
	// Parse request body
	var requestData map[string]interface{}
	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request data"})
		return
	}
	
	start := time.Now()
	
	// Forward request to model serving endpoint
	// This is simplified - in production, use proper HTTP client with retries
	prediction := map[string]interface{}{
		"prediction": []float64{0.85, 0.15}, // Mock prediction
		"confidence": 0.85,
		"model_id":   deployment.ModelID,
		"version":    deployment.ModelVersion,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}
	
	// Record metrics
	latency := time.Since(start).Milliseconds()
	modelInferenceRequests.WithLabelValues(deployment.Name, "success").Inc()
	
	// Log prediction request
	ds.logger.Info("Model prediction", 
		zap.String("deployment", deployment.Name),
		zap.String("model_id", deployment.ModelID),
		zap.Int64("latency_ms", latency))
	
	c.JSON(200, gin.H{
		"result":     prediction,
		"latency_ms": latency,
		"deployment": deployment.Name,
	})
}

func (ds *ModelDeploymentService) scaleDeployment(c *gin.Context) {
	id := c.Param("id")
	
	var scaleRequest struct {
		Replicas int `json:"replicas" binding:"required,min=0,max=50"`
	}
	
	if err := c.ShouldBindJSON(&scaleRequest); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	
	var deployment ModelDeployment
	if err := ds.db.First(&deployment, id).Error; err != nil {
		c.JSON(404, gin.H{"error": "Deployment not found"})
		return
	}
	
	// Update database
	deployment.Replicas = scaleRequest.Replicas
	deployment.UpdatedAt = time.Now()
	ds.db.Save(&deployment)
	
	// Scale in Kubernetes
	namespace := "model-serving"
	k8sDeployment, err := ds.k8sClient.AppsV1().Deployments(namespace).Get(
		context.TODO(), deployment.Name, metav1.GetOptions{})
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to get deployment"})
		return
	}
	
	k8sDeployment.Spec.Replicas = int32Ptr(int32(scaleRequest.Replicas))
	
	_, err = ds.k8sClient.AppsV1().Deployments(namespace).Update(
		context.TODO(), k8sDeployment, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to scale deployment"})
		return
	}
	
	ds.logger.Info("Deployment scaled", 
		zap.String("name", deployment.Name),
		zap.Int("replicas", scaleRequest.Replicas))
	
	c.JSON(200, gin.H{
		"message":  "Deployment scaled successfully",
		"replicas": scaleRequest.Replicas,
	})
}

func (ds *ModelDeploymentService) startMetricsCollection() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		ds.collectDeploymentMetrics()
	}
}

func (ds *ModelDeploymentService) collectDeploymentMetrics() {
	var deployments []ModelDeployment
	if err := ds.db.Where("status = ?", "running").Find(&deployments).Error; err != nil {
		ds.logger.Error("Failed to fetch deployments for metrics", zap.Error(err))
		return
	}

	for _, deployment := range deployments {
		// Collect metrics from Kubernetes and model serving endpoints
		// This is simplified - in production, integrate with Prometheus/monitoring
		
		metrics := DeploymentMetrics{
			DeploymentID:      deployment.ID,
			RequestCount:      int64(1000 + (deployment.ID * 100)), // Mock data
			ErrorCount:        int64(5 + (deployment.ID % 10)),
			AvgLatencyMs:      float64(50 + (deployment.ID % 100)),
			P95LatencyMs:      float64(100 + (deployment.ID % 200)),
			P99LatencyMs:      float64(200 + (deployment.ID % 300)),
			ThroughputRPS:     float64(10 + (deployment.ID % 50)),
			CPUUtilization:    float64(30 + (deployment.ID % 40)),
			MemoryUtilization: float64(40 + (deployment.ID % 30)),
			GPUUtilization:    float64(20 + (deployment.ID % 60)),
			Timestamp:         time.Now(),
		}
		
		ds.db.Create(&metrics)
	}
}

// Helper functions
func int32Ptr(i int32) *int32 { return &i }

func parseQuantity(s string) resource.Quantity {
	return resource.MustParse(s)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
