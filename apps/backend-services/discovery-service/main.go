package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// ServiceInstance represents a registered service instance
type ServiceInstance struct {
	ID          string            `json:"id" gorm:"primaryKey"`
	ServiceName string            `json:"service_name" gorm:"not null;index"`
	Version     string            `json:"version" gorm:"not null"`
	Host        string            `json:"host" gorm:"not null"`
	Port        int               `json:"port" gorm:"not null"`
	Protocol    string            `json:"protocol" gorm:"default:'http'"`
	HealthCheck string            `json:"health_check"`
	Status      string            `json:"status" gorm:"default:'healthy'"`
	Metadata    map[string]string `json:"metadata" gorm:"type:jsonb"`
	Tags        []string          `json:"tags" gorm:"type:jsonb"`
	Environment string            `json:"environment" gorm:"default:'production'"`
	Region      string            `json:"region" gorm:"default:'us-east-1'"`
	LastSeen    time.Time         `json:"last_seen"`
	RegisteredAt time.Time        `json:"registered_at"`
	TTL         int               `json:"ttl" gorm:"default:30"` // seconds
}

// ServiceHealth represents health check status
type ServiceHealth struct {
	ServiceID   string    `json:"service_id"`
	Status      string    `json:"status"`
	LastCheck   time.Time `json:"last_check"`
	ResponseTime int64    `json:"response_time_ms"`
	Error       string    `json:"error,omitempty"`
}

// DiscoveryService handles service registration and discovery
type DiscoveryService struct {
	db       *gorm.DB
	redis    *redis.Client
	logger   *zap.Logger
	services map[string]*ServiceInstance
	mutex    sync.RWMutex
}

// Metrics
var (
	registeredServices = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "discovery_registered_services_total",
			Help: "Total number of registered services",
		},
		[]string{"service_name", "environment"},
	)
	healthyServices = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "discovery_healthy_services_total",
			Help: "Total number of healthy services",
		},
		[]string{"service_name", "environment"},
	)
	serviceRegistrations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "discovery_service_registrations_total",
			Help: "Total number of service registrations",
		},
		[]string{"service_name", "status"},
	)
	serviceDiscoveries = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "discovery_service_discoveries_total",
			Help: "Total number of service discoveries",
		},
		[]string{"service_name", "status"},
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

	// Initialize service
	discoveryService := &DiscoveryService{
		db:       db,
		redis:    redisClient,
		logger:   logger,
		services: make(map[string]*ServiceInstance),
	}

	// Start health check routine
	go discoveryService.startHealthChecker()

	// Start cleanup routine
	go discoveryService.startCleanupRoutine()

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
			"service":   "discovery-service",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"version":   "1.0.0",
		})
	})

	// Metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Discovery API routes
	v1 := router.Group("/v1/discovery")
	{
		// Service registration
		v1.POST("/register", discoveryService.registerService)
		v1.PUT("/register/:id", discoveryService.updateService)
		v1.DELETE("/register/:id", discoveryService.deregisterService)
		v1.POST("/heartbeat/:id", discoveryService.heartbeat)
		
		// Service discovery
		v1.GET("/services", discoveryService.listServices)
		v1.GET("/services/:name", discoveryService.getService)
		v1.GET("/services/:name/instances", discoveryService.getServiceInstances)
		v1.GET("/services/:name/healthy", discoveryService.getHealthyInstances)
		
		// Health checks
		v1.GET("/health/:id", discoveryService.getServiceHealth)
		v1.GET("/health", discoveryService.getAllHealth)
		
		// Service mesh integration
		v1.GET("/endpoints", discoveryService.getEndpoints)
		v1.GET("/catalog", discoveryService.getServiceCatalog)
	}

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logger.Info("Starting Discovery Service", zap.String("port", port))
	log.Fatal(http.ListenAndServe(":"+port, router))
}

func initDatabase() (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_USER", "postgres"),
		getEnv("DB_PASSWORD", "password"),
		getEnv("DB_NAME", "discovery"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_SSLMODE", "disable"),
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Auto-migrate the schema
	err = db.AutoMigrate(&ServiceInstance{})
	if err != nil {
		return nil, err
	}

	return db, nil
}

func initRedis() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
		Password: getEnv("REDIS_PASSWORD", ""),
		DB:       1, // Use different DB than config service
	})
}

func (ds *DiscoveryService) registerService(c *gin.Context) {
	var service ServiceInstance
	if err := c.ShouldBindJSON(&service); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Generate ID if not provided
	if service.ID == "" {
		service.ID = fmt.Sprintf("%s-%s-%d", service.ServiceName, service.Host, service.Port)
	}

	service.RegisteredAt = time.Now()
	service.LastSeen = time.Now()
	service.Status = "healthy"

	// Save to database
	if err := ds.db.Create(&service).Error; err != nil {
		serviceRegistrations.WithLabelValues(service.ServiceName, "error").Inc()
		c.JSON(500, gin.H{"error": "Failed to register service"})
		return
	}

	// Cache in memory
	ds.mutex.Lock()
	ds.services[service.ID] = &service
	ds.mutex.Unlock()

	// Cache in Redis
	serviceData, _ := json.Marshal(service)
	cacheKey := fmt.Sprintf("service:%s", service.ID)
	ds.redis.Set(context.Background(), cacheKey, serviceData, time.Duration(service.TTL*2)*time.Second)

	// Update metrics
	registeredServices.WithLabelValues(service.ServiceName, service.Environment).Inc()
	healthyServices.WithLabelValues(service.ServiceName, service.Environment).Inc()
	serviceRegistrations.WithLabelValues(service.ServiceName, "success").Inc()

	ds.logger.Info("Service registered", 
		zap.String("service_id", service.ID),
		zap.String("service_name", service.ServiceName),
		zap.String("host", service.Host),
		zap.Int("port", service.Port))

	c.JSON(201, service)
}

func (ds *DiscoveryService) updateService(c *gin.Context) {
	id := c.Param("id")
	
	var updateData ServiceInstance
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var service ServiceInstance
	if err := ds.db.Where("id = ?", id).First(&service).Error; err != nil {
		c.JSON(404, gin.H{"error": "Service not found"})
		return
	}

	// Update fields
	service.Host = updateData.Host
	service.Port = updateData.Port
	service.Version = updateData.Version
	service.HealthCheck = updateData.HealthCheck
	service.Metadata = updateData.Metadata
	service.Tags = updateData.Tags
	service.LastSeen = time.Now()

	if err := ds.db.Save(&service).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to update service"})
		return
	}

	// Update cache
	ds.mutex.Lock()
	ds.services[service.ID] = &service
	ds.mutex.Unlock()

	serviceData, _ := json.Marshal(service)
	cacheKey := fmt.Sprintf("service:%s", service.ID)
	ds.redis.Set(context.Background(), cacheKey, serviceData, time.Duration(service.TTL*2)*time.Second)

	c.JSON(200, service)
}

func (ds *DiscoveryService) deregisterService(c *gin.Context) {
	id := c.Param("id")

	var service ServiceInstance
	if err := ds.db.Where("id = ?", id).First(&service).Error; err != nil {
		c.JSON(404, gin.H{"error": "Service not found"})
		return
	}

	// Remove from database
	if err := ds.db.Delete(&service).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to deregister service"})
		return
	}

	// Remove from cache
	ds.mutex.Lock()
	delete(ds.services, id)
	ds.mutex.Unlock()

	cacheKey := fmt.Sprintf("service:%s", id)
	ds.redis.Del(context.Background(), cacheKey)

	// Update metrics
	registeredServices.WithLabelValues(service.ServiceName, service.Environment).Dec()
	if service.Status == "healthy" {
		healthyServices.WithLabelValues(service.ServiceName, service.Environment).Dec()
	}

	ds.logger.Info("Service deregistered", zap.String("service_id", id))
	c.JSON(200, gin.H{"message": "Service deregistered successfully"})
}

func (ds *DiscoveryService) heartbeat(c *gin.Context) {
	id := c.Param("id")

	var service ServiceInstance
	if err := ds.db.Where("id = ?", id).First(&service).Error; err != nil {
		c.JSON(404, gin.H{"error": "Service not found"})
		return
	}

	// Update last seen
	service.LastSeen = time.Now()
	service.Status = "healthy"

	if err := ds.db.Save(&service).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to update heartbeat"})
		return
	}

	// Update cache
	ds.mutex.Lock()
	ds.services[service.ID] = &service
	ds.mutex.Unlock()

	serviceData, _ := json.Marshal(service)
	cacheKey := fmt.Sprintf("service:%s", service.ID)
	ds.redis.Set(context.Background(), cacheKey, serviceData, time.Duration(service.TTL*2)*time.Second)

	c.JSON(200, gin.H{"message": "Heartbeat received", "last_seen": service.LastSeen})
}

func (ds *DiscoveryService) listServices(c *gin.Context) {
	environment := c.DefaultQuery("environment", "")
	region := c.DefaultQuery("region", "")
	
	var services []ServiceInstance
	query := ds.db
	
	if environment != "" {
		query = query.Where("environment = ?", environment)
	}
	if region != "" {
		query = query.Where("region = ?", region)
	}
	
	if err := query.Find(&services).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to fetch services"})
		return
	}

	serviceDiscoveries.WithLabelValues("all", "success").Inc()
	c.JSON(200, gin.H{"services": services})
}

func (ds *DiscoveryService) getService(c *gin.Context) {
	serviceName := c.Param("name")
	environment := c.DefaultQuery("environment", "")
	
	var services []ServiceInstance
	query := ds.db.Where("service_name = ?", serviceName)
	
	if environment != "" {
		query = query.Where("environment = ?", environment)
	}
	
	if err := query.Find(&services).Error; err != nil {
		serviceDiscoveries.WithLabelValues(serviceName, "error").Inc()
		c.JSON(500, gin.H{"error": "Failed to fetch service"})
		return
	}

	if len(services) == 0 {
		serviceDiscoveries.WithLabelValues(serviceName, "not_found").Inc()
		c.JSON(404, gin.H{"error": "Service not found"})
		return
	}

	serviceDiscoveries.WithLabelValues(serviceName, "success").Inc()
	c.JSON(200, gin.H{"service": serviceName, "instances": services})
}

func (ds *DiscoveryService) getServiceInstances(c *gin.Context) {
	serviceName := c.Param("name")
	
	var services []ServiceInstance
	if err := ds.db.Where("service_name = ?", serviceName).Find(&services).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to fetch service instances"})
		return
	}

	c.JSON(200, gin.H{"instances": services})
}

func (ds *DiscoveryService) getHealthyInstances(c *gin.Context) {
	serviceName := c.Param("name")
	
	var services []ServiceInstance
	if err := ds.db.Where("service_name = ? AND status = ?", serviceName, "healthy").Find(&services).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to fetch healthy instances"})
		return
	}

	c.JSON(200, gin.H{"healthy_instances": services})
}

func (ds *DiscoveryService) getServiceHealth(c *gin.Context) {
	id := c.Param("id")

	var service ServiceInstance
	if err := ds.db.Where("id = ?", id).First(&service).Error; err != nil {
		c.JSON(404, gin.H{"error": "Service not found"})
		return
	}

	health := ServiceHealth{
		ServiceID: service.ID,
		Status:    service.Status,
		LastCheck: service.LastSeen,
	}

	c.JSON(200, health)
}

func (ds *DiscoveryService) getAllHealth(c *gin.Context) {
	var services []ServiceInstance
	if err := ds.db.Find(&services).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to fetch services"})
		return
	}

	var healthChecks []ServiceHealth
	for _, service := range services {
		healthChecks = append(healthChecks, ServiceHealth{
			ServiceID: service.ID,
			Status:    service.Status,
			LastCheck: service.LastSeen,
		})
	}

	c.JSON(200, gin.H{"health_checks": healthChecks})
}

func (ds *DiscoveryService) getEndpoints(c *gin.Context) {
	var services []ServiceInstance
	if err := ds.db.Where("status = ?", "healthy").Find(&services).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to fetch endpoints"})
		return
	}

	endpoints := make(map[string][]string)
	for _, service := range services {
		endpoint := fmt.Sprintf("%s://%s:%d", service.Protocol, service.Host, service.Port)
		endpoints[service.ServiceName] = append(endpoints[service.ServiceName], endpoint)
	}

	c.JSON(200, gin.H{"endpoints": endpoints})
}

func (ds *DiscoveryService) getServiceCatalog(c *gin.Context) {
	var services []ServiceInstance
	if err := ds.db.Find(&services).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to fetch service catalog"})
		return
	}

	catalog := make(map[string]interface{})
	serviceMap := make(map[string][]ServiceInstance)
	
	for _, service := range services {
		serviceMap[service.ServiceName] = append(serviceMap[service.ServiceName], service)
	}

	for serviceName, instances := range serviceMap {
		healthyCount := 0
		for _, instance := range instances {
			if instance.Status == "healthy" {
				healthyCount++
			}
		}
		
		catalog[serviceName] = gin.H{
			"total_instances":   len(instances),
			"healthy_instances": healthyCount,
			"instances":         instances,
		}
	}

	c.JSON(200, gin.H{"catalog": catalog})
}

func (ds *DiscoveryService) startHealthChecker() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		ds.performHealthChecks()
	}
}

func (ds *DiscoveryService) performHealthChecks() {
	var services []ServiceInstance
	if err := ds.db.Find(&services).Error; err != nil {
		ds.logger.Error("Failed to fetch services for health check", zap.Error(err))
		return
	}

	for _, service := range services {
		go ds.checkServiceHealth(&service)
	}
}

func (ds *DiscoveryService) checkServiceHealth(service *ServiceInstance) {
	if service.HealthCheck == "" {
		return
	}

	client := &http.Client{Timeout: 5 * time.Second}
	start := time.Now()
	
	resp, err := client.Get(service.HealthCheck)
	responseTime := time.Since(start).Milliseconds()
	
	var status string
	var errorMsg string
	
	if err != nil {
		status = "unhealthy"
		errorMsg = err.Error()
	} else {
		defer resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			status = "healthy"
		} else {
			status = "unhealthy"
			errorMsg = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
	}

	// Update service status
	service.Status = status
	service.LastSeen = time.Now()
	
	if err := ds.db.Save(service).Error; err != nil {
		ds.logger.Error("Failed to update service health", zap.Error(err))
		return
	}

	// Update cache
	ds.mutex.Lock()
	ds.services[service.ID] = service
	ds.mutex.Unlock()

	// Update metrics
	if status == "healthy" {
		healthyServices.WithLabelValues(service.ServiceName, service.Environment).Set(1)
	} else {
		healthyServices.WithLabelValues(service.ServiceName, service.Environment).Set(0)
	}

	ds.logger.Debug("Health check completed",
		zap.String("service_id", service.ID),
		zap.String("status", status),
		zap.Int64("response_time_ms", responseTime),
		zap.String("error", errorMsg))
}

func (ds *DiscoveryService) startCleanupRoutine() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		ds.cleanupStaleServices()
	}
}

func (ds *DiscoveryService) cleanupStaleServices() {
	cutoff := time.Now().Add(-5 * time.Minute) // 5 minutes without heartbeat
	
	var staleServices []ServiceInstance
	if err := ds.db.Where("last_seen < ?", cutoff).Find(&staleServices).Error; err != nil {
		ds.logger.Error("Failed to find stale services", zap.Error(err))
		return
	}

	for _, service := range staleServices {
		// Mark as unhealthy first
		service.Status = "unhealthy"
		ds.db.Save(&service)
		
		// Remove after 10 minutes
		if service.LastSeen.Before(time.Now().Add(-10 * time.Minute)) {
			ds.db.Delete(&service)
			
			// Remove from cache
			ds.mutex.Lock()
			delete(ds.services, service.ID)
			ds.mutex.Unlock()
			
			cacheKey := fmt.Sprintf("service:%s", service.ID)
			ds.redis.Del(context.Background(), cacheKey)
			
			// Update metrics
			registeredServices.WithLabelValues(service.ServiceName, service.Environment).Dec()
			healthyServices.WithLabelValues(service.ServiceName, service.Environment).Dec()
			
			ds.logger.Info("Removed stale service", zap.String("service_id", service.ID))
		}
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
