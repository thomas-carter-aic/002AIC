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
	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/model"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// MetricDefinition represents a custom metric definition
type MetricDefinition struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"uniqueIndex;not null"`
	Type        string    `json:"type" gorm:"not null"` // counter, gauge, histogram, summary
	Description string    `json:"description"`
	Labels      string    `json:"labels" gorm:"type:jsonb"`
	Query       string    `json:"query"`
	Unit        string    `json:"unit"`
	Namespace   string    `json:"namespace" gorm:"default:'002aic'"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	CreatedBy   string    `json:"created_by"`
}

// Alert represents an alert rule
type Alert struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"uniqueIndex;not null"`
	MetricName  string    `json:"metric_name" gorm:"not null"`
	Condition   string    `json:"condition" gorm:"not null"` // >, <, >=, <=, ==, !=
	Threshold   float64   `json:"threshold" gorm:"not null"`
	Duration    string    `json:"duration" gorm:"default:'5m'"`
	Severity    string    `json:"severity" gorm:"default:'warning'"` // critical, warning, info
	Enabled     bool      `json:"enabled" gorm:"default:true"`
	Channels    string    `json:"channels" gorm:"type:jsonb"` // notification channels
	Labels      string    `json:"labels" gorm:"type:jsonb"`
	Annotations string    `json:"annotations" gorm:"type:jsonb"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	CreatedBy   string    `json:"created_by"`
}

// Dashboard represents a monitoring dashboard
type Dashboard struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"uniqueIndex;not null"`
	Description string    `json:"description"`
	Config      string    `json:"config" gorm:"type:jsonb"`
	Tags        string    `json:"tags" gorm:"type:jsonb"`
	Public      bool      `json:"public" gorm:"default:false"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	CreatedBy   string    `json:"created_by"`
}

// MonitoringService handles metrics collection and monitoring
type MonitoringService struct {
	db             *gorm.DB
	redis          *redis.Client
	prometheusAPI  v1.API
	logger         *zap.Logger
	customMetrics  map[string]prometheus.Collector
}

// Custom metrics
var (
	serviceHealth = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "service_health_status",
			Help: "Health status of services (1=healthy, 0=unhealthy)",
		},
		[]string{"service", "instance"},
	)
	
	apiRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "api_request_duration_seconds",
			Help: "Duration of API requests",
		},
		[]string{"service", "endpoint", "method", "status"},
	)
	
	systemResourceUsage = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_resource_usage_percent",
			Help: "System resource usage percentage",
		},
		[]string{"resource", "instance"},
	)
	
	alertsTriggered = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "alerts_triggered_total",
			Help: "Total number of alerts triggered",
		},
		[]string{"alert_name", "severity"},
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

	// Initialize Redis
	redisClient := initRedis()

	// Initialize Prometheus API client
	prometheusClient, err := api.NewClient(api.Config{
		Address: getEnv("PROMETHEUS_URL", "http://localhost:9090"),
	})
	if err != nil {
		logger.Fatal("Failed to create Prometheus client", zap.Error(err))
	}
	prometheusAPI := v1.NewAPI(prometheusClient)

	// Initialize service
	monitoringService := &MonitoringService{
		db:            db,
		redis:         redisClient,
		prometheusAPI: prometheusAPI,
		logger:        logger,
		customMetrics: make(map[string]prometheus.Collector),
	}

	// Start background routines
	go monitoringService.startMetricsCollection()
	go monitoringService.startAlertEvaluation()
	go monitoringService.startHealthChecks()

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
			"service":   "monitoring-metrics-service",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"version":   "1.0.0",
		})
	})

	// Metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Monitoring API routes
	v1 := router.Group("/v1/monitoring")
	{
		// Metrics endpoints
		v1.GET("/metrics", monitoringService.listMetrics)
		v1.POST("/metrics", monitoringService.createMetric)
		v1.GET("/metrics/:name", monitoringService.getMetric)
		v1.PUT("/metrics/:name", monitoringService.updateMetric)
		v1.DELETE("/metrics/:name", monitoringService.deleteMetric)
		v1.GET("/metrics/:name/values", monitoringService.getMetricValues)
		
		// Query endpoints
		v1.GET("/query", monitoringService.queryMetrics)
		v1.GET("/query_range", monitoringService.queryRangeMetrics)
		
		// Alerts endpoints
		v1.GET("/alerts", monitoringService.listAlerts)
		v1.POST("/alerts", monitoringService.createAlert)
		v1.GET("/alerts/:id", monitoringService.getAlert)
		v1.PUT("/alerts/:id", monitoringService.updateAlert)
		v1.DELETE("/alerts/:id", monitoringService.deleteAlert)
		v1.GET("/alerts/active", monitoringService.getActiveAlerts)
		
		// Dashboard endpoints
		v1.GET("/dashboards", monitoringService.listDashboards)
		v1.POST("/dashboards", monitoringService.createDashboard)
		v1.GET("/dashboards/:id", monitoringService.getDashboard)
		v1.PUT("/dashboards/:id", monitoringService.updateDashboard)
		v1.DELETE("/dashboards/:id", monitoringService.deleteDashboard)
		
		// Health check endpoints
		v1.GET("/health/services", monitoringService.getServicesHealth)
		v1.POST("/health/check", monitoringService.performHealthCheck)
		
		// System metrics
		v1.GET("/system/resources", monitoringService.getSystemResources)
		v1.GET("/system/performance", monitoringService.getSystemPerformance)
	}

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logger.Info("Starting Monitoring Metrics Service", zap.String("port", port))
	log.Fatal(http.ListenAndServe(":"+port, router))
}

func initDatabase() (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_USER", "postgres"),
		getEnv("DB_PASSWORD", "password"),
		getEnv("DB_NAME", "monitoring"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_SSLMODE", "disable"),
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Auto-migrate the schema
	err = db.AutoMigrate(&MetricDefinition{}, &Alert{}, &Dashboard{})
	if err != nil {
		return nil, err
	}

	return db, nil
}

func initRedis() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
		Password: getEnv("REDIS_PASSWORD", ""),
		DB:       8, // Use different DB than other services
	})
}

func (ms *MonitoringService) listMetrics(c *gin.Context) {
	var metrics []MetricDefinition
	if err := ms.db.Find(&metrics).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to fetch metrics"})
		return
	}
	
	c.JSON(200, gin.H{"metrics": metrics})
}

func (ms *MonitoringService) createMetric(c *gin.Context) {
	var metric MetricDefinition
	if err := c.ShouldBindJSON(&metric); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	
	metric.CreatedAt = time.Now()
	metric.UpdatedAt = time.Now()
	
	if err := ms.db.Create(&metric).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to create metric"})
		return
	}
	
	// Register custom metric with Prometheus
	ms.registerCustomMetric(&metric)
	
	ms.logger.Info("Metric created", 
		zap.String("name", metric.Name),
		zap.String("type", metric.Type))
	
	c.JSON(201, metric)
}

func (ms *MonitoringService) queryMetrics(c *gin.Context) {
	query := c.Query("query")
	if query == "" {
		c.JSON(400, gin.H{"error": "Query parameter is required"})
		return
	}
	
	timeParam := c.DefaultQuery("time", "")
	var queryTime time.Time
	if timeParam != "" {
		var err error
		queryTime, err = time.Parse(time.RFC3339, timeParam)
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid time format"})
			return
		}
	} else {
		queryTime = time.Now()
	}
	
	// Query Prometheus
	result, warnings, err := ms.prometheusAPI.Query(context.Background(), query, queryTime)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to query metrics"})
		return
	}
	
	if len(warnings) > 0 {
		ms.logger.Warn("Query warnings", zap.Strings("warnings", warnings))
	}
	
	c.JSON(200, gin.H{
		"query":  query,
		"result": result,
		"time":   queryTime,
	})
}

func (ms *MonitoringService) queryRangeMetrics(c *gin.Context) {
	query := c.Query("query")
	start := c.Query("start")
	end := c.Query("end")
	step := c.DefaultQuery("step", "15s")
	
	if query == "" || start == "" || end == "" {
		c.JSON(400, gin.H{"error": "Query, start, and end parameters are required"})
		return
	}
	
	startTime, err := time.Parse(time.RFC3339, start)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid start time format"})
		return
	}
	
	endTime, err := time.Parse(time.RFC3339, end)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid end time format"})
		return
	}
	
	stepDuration, err := time.ParseDuration(step)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid step format"})
		return
	}
	
	// Query Prometheus range
	r := v1.Range{
		Start: startTime,
		End:   endTime,
		Step:  stepDuration,
	}
	
	result, warnings, err := ms.prometheusAPI.QueryRange(context.Background(), query, r)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to query range metrics"})
		return
	}
	
	if len(warnings) > 0 {
		ms.logger.Warn("Query range warnings", zap.Strings("warnings", warnings))
	}
	
	c.JSON(200, gin.H{
		"query":  query,
		"result": result,
		"range":  r,
	})
}

func (ms *MonitoringService) getServicesHealth(c *gin.Context) {
	// Get service health from Redis cache
	services := []string{
		"api-gateway-service",
		"user-management-service",
		"model-management-service",
		"data-management-service",
		"analytics-service",
		"authorization-service",
	}
	
	healthStatus := make(map[string]interface{})
	
	for _, service := range services {
		key := fmt.Sprintf("health:%s", service)
		status, err := ms.redis.Get(context.Background(), key).Result()
		if err != nil {
			healthStatus[service] = gin.H{
				"status":      "unknown",
				"last_check":  nil,
				"error":       "No health data available",
			}
		} else {
			var health map[string]interface{}
			json.Unmarshal([]byte(status), &health)
			healthStatus[service] = health
		}
	}
	
	c.JSON(200, gin.H{"services": healthStatus})
}

func (ms *MonitoringService) getSystemResources(c *gin.Context) {
	// Mock system resource data
	resources := gin.H{
		"cpu": gin.H{
			"usage_percent": 45.2,
			"cores":         8,
			"load_average":  []float64{1.2, 1.5, 1.8},
		},
		"memory": gin.H{
			"usage_percent": 67.8,
			"total_gb":      32.0,
			"used_gb":       21.7,
			"available_gb":  10.3,
		},
		"disk": gin.H{
			"usage_percent": 34.5,
			"total_gb":      500.0,
			"used_gb":       172.5,
			"available_gb":  327.5,
		},
		"network": gin.H{
			"rx_bytes_per_sec": 1024000,
			"tx_bytes_per_sec": 512000,
			"connections":      150,
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	
	c.JSON(200, resources)
}

func (ms *MonitoringService) startMetricsCollection() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		ms.collectSystemMetrics()
		ms.collectServiceMetrics()
	}
}

func (ms *MonitoringService) collectSystemMetrics() {
	// Collect and update system metrics
	// This is simplified - in production, use actual system monitoring
	
	// CPU usage
	systemResourceUsage.WithLabelValues("cpu", "localhost").Set(45.2)
	
	// Memory usage
	systemResourceUsage.WithLabelValues("memory", "localhost").Set(67.8)
	
	// Disk usage
	systemResourceUsage.WithLabelValues("disk", "localhost").Set(34.5)
	
	ms.logger.Debug("System metrics collected")
}

func (ms *MonitoringService) collectServiceMetrics() {
	// Collect service health metrics
	services := []string{
		"api-gateway-service",
		"user-management-service", 
		"model-management-service",
		"data-management-service",
		"analytics-service",
		"authorization-service",
	}
	
	for _, service := range services {
		// Mock health check - in production, make actual HTTP calls
		healthy := true // Assume healthy for demo
		
		if healthy {
			serviceHealth.WithLabelValues(service, "localhost").Set(1)
		} else {
			serviceHealth.WithLabelValues(service, "localhost").Set(0)
		}
		
		// Cache health status in Redis
		healthData := gin.H{
			"status":     "healthy",
			"last_check": time.Now().UTC().Format(time.RFC3339),
			"response_time_ms": 50,
		}
		
		healthJSON, _ := json.Marshal(healthData)
		ms.redis.Set(context.Background(), fmt.Sprintf("health:%s", service), healthJSON, 5*time.Minute)
	}
}

func (ms *MonitoringService) startAlertEvaluation() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		ms.evaluateAlerts()
	}
}

func (ms *MonitoringService) evaluateAlerts() {
	var alerts []Alert
	if err := ms.db.Where("enabled = ?", true).Find(&alerts).Error; err != nil {
		ms.logger.Error("Failed to fetch alerts", zap.Error(err))
		return
	}

	for _, alert := range alerts {
		// Evaluate alert condition
		// This is simplified - in production, use proper alert evaluation
		
		// Mock alert triggering
		if alert.Name == "high_cpu_usage" {
			// Simulate high CPU alert
			alertsTriggered.WithLabelValues(alert.Name, alert.Severity).Inc()
			ms.logger.Warn("Alert triggered", 
				zap.String("alert", alert.Name),
				zap.String("severity", alert.Severity))
		}
	}
}

func (ms *MonitoringService) startHealthChecks() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		ms.performHealthChecks()
	}
}

func (ms *MonitoringService) performHealthChecks() {
	// Perform health checks on critical services
	services := map[string]string{
		"prometheus": getEnv("PROMETHEUS_URL", "http://localhost:9090"),
		"grafana":    getEnv("GRAFANA_URL", "http://localhost:3000"),
		"jaeger":     getEnv("JAEGER_URL", "http://localhost:16686"),
	}
	
	for service, url := range services {
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(url + "/api/health")
		
		if err != nil || resp.StatusCode != 200 {
			serviceHealth.WithLabelValues(service, "localhost").Set(0)
			ms.logger.Warn("Service health check failed", 
				zap.String("service", service),
				zap.String("url", url))
		} else {
			serviceHealth.WithLabelValues(service, "localhost").Set(1)
		}
		
		if resp != nil {
			resp.Body.Close()
		}
	}
}

func (ms *MonitoringService) registerCustomMetric(metric *MetricDefinition) {
	// Register custom metric with Prometheus
	// This is simplified - in production, implement proper metric registration
	
	switch metric.Type {
	case "counter":
		counter := prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: metric.Name,
				Help: metric.Description,
			},
			[]string{}, // Parse labels from metric.Labels
		)
		prometheus.MustRegister(counter)
		ms.customMetrics[metric.Name] = counter
		
	case "gauge":
		gauge := prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: metric.Name,
				Help: metric.Description,
			},
			[]string{}, // Parse labels from metric.Labels
		)
		prometheus.MustRegister(gauge)
		ms.customMetrics[metric.Name] = gauge
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
