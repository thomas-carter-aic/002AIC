package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.etcd.io/etcd/clientv3"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Configuration represents a configuration entry
type Configuration struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Key         string    `json:"key" gorm:"uniqueIndex;not null"`
	Value       string    `json:"value" gorm:"not null"`
	Environment string    `json:"environment" gorm:"not null;default:'default'"`
	Service     string    `json:"service" gorm:"not null;default:'global'"`
	Version     int       `json:"version" gorm:"not null;default:1"`
	Encrypted   bool      `json:"encrypted" gorm:"default:false"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	CreatedBy   string    `json:"created_by"`
	UpdatedBy   string    `json:"updated_by"`
}

// ConfigurationService handles distributed configuration management
type ConfigurationService struct {
	db     *gorm.DB
	redis  *redis.Client
	etcd   *clientv3.Client
	logger *zap.Logger
}

// Metrics
var (
	configReads = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "config_reads_total",
			Help: "Total number of configuration reads",
		},
		[]string{"service", "environment", "status"},
	)
	configWrites = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "config_writes_total",
			Help: "Total number of configuration writes",
		},
		[]string{"service", "environment", "status"},
	)
	configCacheHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "config_cache_hits_total",
			Help: "Total number of configuration cache hits",
		},
		[]string{"service", "environment"},
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

	// Initialize etcd
	etcdClient, err := initEtcd()
	if err != nil {
		logger.Warn("Failed to connect to etcd, continuing without it", zap.Error(err))
	}

	// Initialize service
	configService := &ConfigurationService{
		db:     db,
		redis:  redisClient,
		etcd:   etcdClient,
		logger: logger,
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
			"service":   "configuration-service",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"version":   "1.0.0",
		})
	})

	// Metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Configuration API routes
	v1 := router.Group("/v1/config")
	{
		v1.GET("/", configService.listConfigurations)
		v1.GET("/:key", configService.getConfiguration)
		v1.POST("/", configService.createConfiguration)
		v1.PUT("/:key", configService.updateConfiguration)
		v1.DELETE("/:key", configService.deleteConfiguration)
		v1.GET("/service/:service", configService.getServiceConfigurations)
		v1.GET("/environment/:environment", configService.getEnvironmentConfigurations)
		v1.POST("/bulk", configService.bulkUpdateConfigurations)
		v1.GET("/watch/:key", configService.watchConfiguration)
	}

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logger.Info("Starting Configuration Service", zap.String("port", port))
	log.Fatal(http.ListenAndServe(":"+port, router))
}

func initDatabase() (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_USER", "postgres"),
		getEnv("DB_PASSWORD", "password"),
		getEnv("DB_NAME", "configuration"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_SSLMODE", "disable"),
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Auto-migrate the schema
	err = db.AutoMigrate(&Configuration{})
	if err != nil {
		return nil, err
	}

	return db, nil
}

func initRedis() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
		Password: getEnv("REDIS_PASSWORD", ""),
		DB:       0,
	})
}

func initEtcd() (*clientv3.Client, error) {
	endpoints := strings.Split(getEnv("ETCD_ENDPOINTS", "localhost:2379"), ",")
	return clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
}

func (cs *ConfigurationService) listConfigurations(c *gin.Context) {
	service := c.Query("service")
	environment := c.Query("environment")
	
	var configs []Configuration
	query := cs.db
	
	if service != "" {
		query = query.Where("service = ?", service)
	}
	if environment != "" {
		query = query.Where("environment = ?", environment)
	}
	
	if err := query.Find(&configs).Error; err != nil {
		configReads.WithLabelValues(service, environment, "error").Inc()
		c.JSON(500, gin.H{"error": "Failed to fetch configurations"})
		return
	}
	
	configReads.WithLabelValues(service, environment, "success").Inc()
	c.JSON(200, gin.H{"configurations": configs})
}

func (cs *ConfigurationService) getConfiguration(c *gin.Context) {
	key := c.Param("key")
	service := c.DefaultQuery("service", "global")
	environment := c.DefaultQuery("environment", "default")
	
	// Try cache first
	cacheKey := fmt.Sprintf("config:%s:%s:%s", service, environment, key)
	cached, err := cs.redis.Get(context.Background(), cacheKey).Result()
	if err == nil {
		configCacheHits.WithLabelValues(service, environment).Inc()
		var config Configuration
		json.Unmarshal([]byte(cached), &config)
		c.JSON(200, config)
		return
	}
	
	// Fetch from database
	var config Configuration
	if err := cs.db.Where("key = ? AND service = ? AND environment = ?", key, service, environment).First(&config).Error; err != nil {
		configReads.WithLabelValues(service, environment, "error").Inc()
		c.JSON(404, gin.H{"error": "Configuration not found"})
		return
	}
	
	// Cache the result
	configData, _ := json.Marshal(config)
	cs.redis.Set(context.Background(), cacheKey, configData, 5*time.Minute)
	
	configReads.WithLabelValues(service, environment, "success").Inc()
	c.JSON(200, config)
}

func (cs *ConfigurationService) createConfiguration(c *gin.Context) {
	var config Configuration
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	
	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()
	config.Version = 1
	
	if err := cs.db.Create(&config).Error; err != nil {
		configWrites.WithLabelValues(config.Service, config.Environment, "error").Inc()
		c.JSON(500, gin.H{"error": "Failed to create configuration"})
		return
	}
	
	// Invalidate cache
	cacheKey := fmt.Sprintf("config:%s:%s:%s", config.Service, config.Environment, config.Key)
	cs.redis.Del(context.Background(), cacheKey)
	
	// Notify etcd watchers
	if cs.etcd != nil {
		etcdKey := fmt.Sprintf("/config/%s/%s/%s", config.Service, config.Environment, config.Key)
		cs.etcd.Put(context.Background(), etcdKey, config.Value)
	}
	
	configWrites.WithLabelValues(config.Service, config.Environment, "success").Inc()
	c.JSON(201, config)
}

func (cs *ConfigurationService) updateConfiguration(c *gin.Context) {
	key := c.Param("key")
	service := c.DefaultQuery("service", "global")
	environment := c.DefaultQuery("environment", "default")
	
	var updateData Configuration
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	
	var config Configuration
	if err := cs.db.Where("key = ? AND service = ? AND environment = ?", key, service, environment).First(&config).Error; err != nil {
		c.JSON(404, gin.H{"error": "Configuration not found"})
		return
	}
	
	// Update fields
	config.Value = updateData.Value
	config.UpdatedAt = time.Now()
	config.Version++
	config.UpdatedBy = updateData.UpdatedBy
	
	if err := cs.db.Save(&config).Error; err != nil {
		configWrites.WithLabelValues(service, environment, "error").Inc()
		c.JSON(500, gin.H{"error": "Failed to update configuration"})
		return
	}
	
	// Invalidate cache
	cacheKey := fmt.Sprintf("config:%s:%s:%s", service, environment, key)
	cs.redis.Del(context.Background(), cacheKey)
	
	// Notify etcd watchers
	if cs.etcd != nil {
		etcdKey := fmt.Sprintf("/config/%s/%s/%s", service, environment, key)
		cs.etcd.Put(context.Background(), etcdKey, config.Value)
	}
	
	configWrites.WithLabelValues(service, environment, "success").Inc()
	c.JSON(200, config)
}

func (cs *ConfigurationService) deleteConfiguration(c *gin.Context) {
	key := c.Param("key")
	service := c.DefaultQuery("service", "global")
	environment := c.DefaultQuery("environment", "default")
	
	if err := cs.db.Where("key = ? AND service = ? AND environment = ?", key, service, environment).Delete(&Configuration{}).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to delete configuration"})
		return
	}
	
	// Invalidate cache
	cacheKey := fmt.Sprintf("config:%s:%s:%s", service, environment, key)
	cs.redis.Del(context.Background(), cacheKey)
	
	// Remove from etcd
	if cs.etcd != nil {
		etcdKey := fmt.Sprintf("/config/%s/%s/%s", service, environment, key)
		cs.etcd.Delete(context.Background(), etcdKey)
	}
	
	c.JSON(200, gin.H{"message": "Configuration deleted successfully"})
}

func (cs *ConfigurationService) getServiceConfigurations(c *gin.Context) {
	service := c.Param("service")
	environment := c.DefaultQuery("environment", "default")
	
	var configs []Configuration
	if err := cs.db.Where("service = ? AND environment = ?", service, environment).Find(&configs).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to fetch service configurations"})
		return
	}
	
	c.JSON(200, gin.H{"configurations": configs})
}

func (cs *ConfigurationService) getEnvironmentConfigurations(c *gin.Context) {
	environment := c.Param("environment")
	
	var configs []Configuration
	if err := cs.db.Where("environment = ?", environment).Find(&configs).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to fetch environment configurations"})
		return
	}
	
	c.JSON(200, gin.H{"configurations": configs})
}

func (cs *ConfigurationService) bulkUpdateConfigurations(c *gin.Context) {
	var configs []Configuration
	if err := c.ShouldBindJSON(&configs); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	
	tx := cs.db.Begin()
	for _, config := range configs {
		config.UpdatedAt = time.Now()
		if err := tx.Save(&config).Error; err != nil {
			tx.Rollback()
			c.JSON(500, gin.H{"error": "Failed to bulk update configurations"})
			return
		}
		
		// Invalidate cache
		cacheKey := fmt.Sprintf("config:%s:%s:%s", config.Service, config.Environment, config.Key)
		cs.redis.Del(context.Background(), cacheKey)
	}
	tx.Commit()
	
	c.JSON(200, gin.H{"message": "Configurations updated successfully"})
}

func (cs *ConfigurationService) watchConfiguration(c *gin.Context) {
	key := c.Param("key")
	service := c.DefaultQuery("service", "global")
	environment := c.DefaultQuery("environment", "default")
	
	if cs.etcd == nil {
		c.JSON(503, gin.H{"error": "Watch functionality not available"})
		return
	}
	
	etcdKey := fmt.Sprintf("/config/%s/%s/%s", service, environment, key)
	watchChan := cs.etcd.Watch(context.Background(), etcdKey)
	
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	
	for watchResp := range watchChan {
		for _, event := range watchResp.Events {
			c.SSEvent("config-change", string(event.Kv.Value))
			c.Writer.Flush()
		}
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
