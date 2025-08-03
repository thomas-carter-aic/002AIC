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

// Runtime represents a PaaS runtime environment
type Runtime struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"uniqueIndex;not null"`
	Language    string    `json:"language" gorm:"not null"`
	Version     string    `json:"version" gorm:"not null"`
	Image       string    `json:"image" gorm:"not null"`
	Status      string    `json:"status" gorm:"default:'active'"`
	CPU         string    `json:"cpu" gorm:"default:'100m'"`
	Memory      string    `json:"memory" gorm:"default:'128Mi'"`
	Replicas    int       `json:"replicas" gorm:"default:1"`
	Environment string    `json:"environment" gorm:"default:'production'"`
	Config      string    `json:"config" gorm:"type:jsonb"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	CreatedBy   string    `json:"created_by"`
}

// Application represents a deployed application
type Application struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"uniqueIndex;not null"`
	RuntimeID   uint      `json:"runtime_id" gorm:"not null"`
	Runtime     Runtime   `json:"runtime" gorm:"foreignKey:RuntimeID"`
	Status      string    `json:"status" gorm:"default:'pending'"`
	URL         string    `json:"url"`
	SourceURL   string    `json:"source_url"`
	BuildStatus string    `json:"build_status" gorm:"default:'pending'"`
	Replicas    int       `json:"replicas" gorm:"default:1"`
	CPU         string    `json:"cpu" gorm:"default:'100m'"`
	Memory      string    `json:"memory" gorm:"default:'128Mi'"`
	EnvVars     string    `json:"env_vars" gorm:"type:jsonb"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	DeployedAt  *time.Time `json:"deployed_at"`
	CreatedBy   string    `json:"created_by"`
}

// RuntimeService handles PaaS runtime management
type RuntimeService struct {
	db        *gorm.DB
	k8sClient *kubernetes.Clientset
	logger    *zap.Logger
}

// Metrics
var (
	activeRuntimes = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "runtime_active_total",
			Help: "Total number of active runtimes",
		},
		[]string{"language", "environment"},
	)
	deployedApps = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "runtime_deployed_apps_total",
			Help: "Total number of deployed applications",
		},
		[]string{"runtime", "status"},
	)
	deploymentDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "runtime_deployment_duration_seconds",
			Help: "Duration of application deployments",
		},
		[]string{"runtime", "status"},
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
	runtimeService := &RuntimeService{
		db:        db,
		k8sClient: k8sClient,
		logger:    logger,
	}

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
			"service":   "runtime-management-service",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"version":   "1.0.0",
		})
	})

	// Metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Runtime management API routes
	v1 := router.Group("/v1/runtime")
	{
		// Runtime management
		v1.GET("/runtimes", runtimeService.listRuntimes)
		v1.POST("/runtimes", runtimeService.createRuntime)
		v1.GET("/runtimes/:id", runtimeService.getRuntime)
		v1.PUT("/runtimes/:id", runtimeService.updateRuntime)
		v1.DELETE("/runtimes/:id", runtimeService.deleteRuntime)
		
		// Application management
		v1.GET("/applications", runtimeService.listApplications)
		v1.POST("/applications", runtimeService.deployApplication)
		v1.GET("/applications/:id", runtimeService.getApplication)
		v1.PUT("/applications/:id", runtimeService.updateApplication)
		v1.DELETE("/applications/:id", runtimeService.deleteApplication)
		v1.POST("/applications/:id/scale", runtimeService.scaleApplication)
		v1.POST("/applications/:id/restart", runtimeService.restartApplication)
		v1.GET("/applications/:id/logs", runtimeService.getApplicationLogs)
		v1.GET("/applications/:id/metrics", runtimeService.getApplicationMetrics)
		
		// Build management
		v1.POST("/applications/:id/build", runtimeService.buildApplication)
		v1.GET("/applications/:id/builds", runtimeService.getBuildHistory)
		
		// Environment management
		v1.GET("/environments", runtimeService.listEnvironments)
		v1.POST("/environments", runtimeService.createEnvironment)
	}

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logger.Info("Starting Runtime Management Service", zap.String("port", port))
	log.Fatal(http.ListenAndServe(":"+port, router))
}

func initDatabase() (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_USER", "postgres"),
		getEnv("DB_PASSWORD", "password"),
		getEnv("DB_NAME", "runtime_management"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_SSLMODE", "disable"),
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Auto-migrate the schema
	err = db.AutoMigrate(&Runtime{}, &Application{})
	if err != nil {
		return nil, err
	}

	return db, nil
}

func initKubernetesClient() (*kubernetes.Clientset, error) {
	// Try in-cluster config first
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fallback to local kubeconfig for development
		return nil, fmt.Errorf("failed to get Kubernetes config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return clientset, nil
}

func (rs *RuntimeService) listRuntimes(c *gin.Context) {
	language := c.Query("language")
	environment := c.Query("environment")
	
	var runtimes []Runtime
	query := rs.db
	
	if language != "" {
		query = query.Where("language = ?", language)
	}
	if environment != "" {
		query = query.Where("environment = ?", environment)
	}
	
	if err := query.Find(&runtimes).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to fetch runtimes"})
		return
	}
	
	c.JSON(200, gin.H{"runtimes": runtimes})
}

func (rs *RuntimeService) createRuntime(c *gin.Context) {
	var runtime Runtime
	if err := c.ShouldBindJSON(&runtime); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	
	runtime.CreatedAt = time.Now()
	runtime.UpdatedAt = time.Now()
	
	if err := rs.db.Create(&runtime).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to create runtime"})
		return
	}
	
	// Update metrics
	activeRuntimes.WithLabelValues(runtime.Language, runtime.Environment).Inc()
	
	rs.logger.Info("Runtime created", 
		zap.String("name", runtime.Name),
		zap.String("language", runtime.Language),
		zap.String("version", runtime.Version))
	
	c.JSON(201, runtime)
}

func (rs *RuntimeService) deployApplication(c *gin.Context) {
	var app Application
	if err := c.ShouldBindJSON(&app); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	
	start := time.Now()
	
	// Get runtime info
	var runtime Runtime
	if err := rs.db.First(&runtime, app.RuntimeID).Error; err != nil {
		c.JSON(404, gin.H{"error": "Runtime not found"})
		return
	}
	
	app.CreatedAt = time.Now()
	app.UpdatedAt = time.Now()
	app.Status = "deploying"
	
	// Save application to database
	if err := rs.db.Create(&app).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to create application"})
		return
	}
	
	// Deploy to Kubernetes
	err := rs.deployToKubernetes(&app, &runtime)
	if err != nil {
		app.Status = "failed"
		rs.db.Save(&app)
		deploymentDuration.WithLabelValues(runtime.Name, "failed").Observe(time.Since(start).Seconds())
		c.JSON(500, gin.H{"error": "Failed to deploy application"})
		return
	}
	
	// Update application status
	now := time.Now()
	app.Status = "running"
	app.DeployedAt = &now
	app.URL = fmt.Sprintf("https://%s.002aic.com", app.Name)
	rs.db.Save(&app)
	
	// Update metrics
	deployedApps.WithLabelValues(runtime.Name, "running").Inc()
	deploymentDuration.WithLabelValues(runtime.Name, "success").Observe(time.Since(start).Seconds())
	
	rs.logger.Info("Application deployed", 
		zap.String("name", app.Name),
		zap.String("runtime", runtime.Name),
		zap.String("url", app.URL))
	
	c.JSON(201, app)
}

func (rs *RuntimeService) deployToKubernetes(app *Application, runtime *Runtime) error {
	namespace := "default" // In production, use proper namespace management
	
	// Parse environment variables
	var envVars []corev1.EnvVar
	if app.EnvVars != "" {
		var envMap map[string]string
		if err := json.Unmarshal([]byte(app.EnvVars), &envMap); err == nil {
			for key, value := range envMap {
				envVars = append(envVars, corev1.EnvVar{
					Name:  key,
					Value: value,
				})
			}
		}
	}
	
	// Create Deployment
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: namespace,
			Labels: map[string]string{
				"app":     app.Name,
				"runtime": runtime.Name,
				"managed": "002aic-platform",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(int32(app.Replicas)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": app.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":     app.Name,
						"runtime": runtime.Name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  app.Name,
							Image: runtime.Image,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 8080,
								},
							},
							Env: envVars,
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    parseQuantity(app.CPU),
									corev1.ResourceMemory: parseQuantity(app.Memory),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    parseQuantity(app.CPU),
									corev1.ResourceMemory: parseQuantity(app.Memory),
								},
							},
						},
					},
				},
			},
		},
	}
	
	_, err := rs.k8sClient.AppsV1().Deployments(namespace).Create(
		context.TODO(), deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}
	
	// Create Service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: namespace,
			Labels: map[string]string{
				"app":     app.Name,
				"runtime": runtime.Name,
				"managed": "002aic-platform",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": app.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Port:       80,
					TargetPort: intstr.FromInt(8080),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
	
	_, err = rs.k8sClient.CoreV1().Services(namespace).Create(
		context.TODO(), service, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}
	
	return nil
}

func (rs *RuntimeService) scaleApplication(c *gin.Context) {
	id := c.Param("id")
	
	var scaleRequest struct {
		Replicas int `json:"replicas" binding:"required,min=0,max=10"`
	}
	
	if err := c.ShouldBindJSON(&scaleRequest); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	
	var app Application
	if err := rs.db.First(&app, id).Error; err != nil {
		c.JSON(404, gin.H{"error": "Application not found"})
		return
	}
	
	// Update database
	app.Replicas = scaleRequest.Replicas
	app.UpdatedAt = time.Now()
	rs.db.Save(&app)
	
	// Scale in Kubernetes
	namespace := "default"
	deployment, err := rs.k8sClient.AppsV1().Deployments(namespace).Get(
		context.TODO(), app.Name, metav1.GetOptions{})
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to get deployment"})
		return
	}
	
	deployment.Spec.Replicas = int32Ptr(int32(scaleRequest.Replicas))
	
	_, err = rs.k8sClient.AppsV1().Deployments(namespace).Update(
		context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to scale application"})
		return
	}
	
	rs.logger.Info("Application scaled", 
		zap.String("name", app.Name),
		zap.Int("replicas", scaleRequest.Replicas))
	
	c.JSON(200, gin.H{
		"message":  "Application scaled successfully",
		"replicas": scaleRequest.Replicas,
	})
}

func (rs *RuntimeService) getApplicationLogs(c *gin.Context) {
	id := c.Param("id")
	lines := c.DefaultQuery("lines", "100")
	
	var app Application
	if err := rs.db.First(&app, id).Error; err != nil {
		c.JSON(404, gin.H{"error": "Application not found"})
		return
	}
	
	// Get logs from Kubernetes
	namespace := "default"
	tailLines, _ := strconv.ParseInt(lines, 10, 64)
	
	pods, err := rs.k8sClient.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", app.Name),
	})
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to get pods"})
		return
	}
	
	var allLogs []string
	for _, pod := range pods.Items {
		req := rs.k8sClient.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
			TailLines: &tailLines,
		})
		
		logs, err := req.Stream(context.TODO())
		if err != nil {
			continue
		}
		defer logs.Close()
		
		// Read logs (simplified - in production, stream properly)
		allLogs = append(allLogs, fmt.Sprintf("=== Pod: %s ===", pod.Name))
	}
	
	c.JSON(200, gin.H{
		"application": app.Name,
		"logs":        allLogs,
		"lines":       len(allLogs),
	})
}

// Helper functions
func int32Ptr(i int32) *int32 { return &i }

func parseQuantity(s string) resource.Quantity {
	// Simplified quantity parsing
	// In production, use resource.ParseQuantity
	return resource.MustParse(s)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
