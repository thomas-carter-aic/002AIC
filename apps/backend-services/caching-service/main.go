/**
 * Caching Service - Go
 * Distributed cache management for the 002AIC platform
 * Handles multi-tier caching, cache invalidation, and performance optimization
 */

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/go-redis/redis/v8"
	"github.com/bradfitz/gomemcache/memcache"
)

// Configuration
type Config struct {
	Port           string
	RedisURL       string
	MemcachedURL   string
	Environment    string
	DefaultTTL     time.Duration
	MaxKeySize     int
	MaxValueSize   int64
	ClusterMode    bool
}

// Cache tiers
const (
	TierL1 = "l1" // In-memory
	TierL2 = "l2" // Redis
	TierL3 = "l3" // Memcached
)

// Cache operations
const (
	OpGet    = "get"
	OpSet    = "set"
	OpDelete = "delete"
	OpFlush  = "flush"
)

// Models
type CacheEntry struct {
	Key       string      `json:"key"`
	Value     interface{} `json:"value"`
	TTL       int64       `json:"ttl"`
	Tier      string      `json:"tier"`
	CreatedAt time.Time   `json:"created_at"`
	ExpiresAt time.Time   `json:"expires_at"`
}

type CacheStats struct {
	Tier      string  `json:"tier"`
	Hits      int64   `json:"hits"`
	Misses    int64   `json:"misses"`
	HitRate   float64 `json:"hit_rate"`
	Keys      int64   `json:"keys"`
	Memory    int64   `json:"memory_bytes"`
	Evictions int64   `json:"evictions"`
}

// Service struct
type CachingService struct {
	config       *Config
	router       *gin.Engine
	httpServer   *http.Server
	redisClient  *redis.Client
	memcacheClient *memcache.Client
	l1Cache      map[string]*CacheEntry
}

// Prometheus metrics
var (
	cacheOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_operations_total",
			Help: "Total cache operations",
		},
		[]string{"operation", "tier", "status"},
	)

	cacheHits = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Total cache hits",
		},
		[]string{"tier"},
	)

	cacheMisses = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_misses_total",
			Help: "Total cache misses",
		},
		[]string{"tier"},
	)

	cacheLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "cache_operation_duration_seconds",
			Help: "Cache operation duration",
		},
		[]string{"operation", "tier"},
	)

	cacheSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cache_size_bytes",
			Help: "Cache size in bytes",
		},
		[]string{"tier"},
	)

	cacheKeys = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cache_keys_total",
			Help: "Total number of cache keys",
		},
		[]string{"tier"},
	)
)

func init() {
	prometheus.MustRegister(cacheOperations)
	prometheus.MustRegister(cacheHits)
	prometheus.MustRegister(cacheMisses)
	prometheus.MustRegister(cacheLatency)
	prometheus.MustRegister(cacheSize)
	prometheus.MustRegister(cacheKeys)
}

func main() {
	config := &Config{
		Port:         getEnv("PORT", "8080"),
		RedisURL:     getEnv("REDIS_URL", "redis://localhost:6379"),
		MemcachedURL: getEnv("MEMCACHED_URL", "localhost:11211"),
		Environment:  getEnv("ENVIRONMENT", "development"),
		DefaultTTL:   time.Duration(parseInt(getEnv("DEFAULT_TTL", "3600"))) * time.Second,
		MaxKeySize:   parseInt(getEnv("MAX_KEY_SIZE", "250")),
		MaxValueSize: parseInt64(getEnv("MAX_VALUE_SIZE", "1048576")), // 1MB
		ClusterMode:  getBool(getEnv("CLUSTER_MODE", "false")),
	}

	service, err := NewCachingService(config)
	if err != nil {
		log.Fatal("Failed to create caching service:", err)
	}

	if err := service.Start(); err != nil {
		log.Fatal("Failed to start caching service:", err)
	}
}

func NewCachingService(config *Config) (*CachingService, error) {
	// Initialize Redis client
	opt, err := redis.ParseURL(config.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}
	redisClient := redis.NewClient(opt)

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Initialize Memcached client
	memcacheClient := memcache.New(config.MemcachedURL)
	memcacheClient.Timeout = 100 * time.Millisecond

	service := &CachingService{
		config:         config,
		redisClient:    redisClient,
		memcacheClient: memcacheClient,
		l1Cache:        make(map[string]*CacheEntry),
	}

	service.setupRoutes()
	return service, nil
}

func (s *CachingService) setupRoutes() {
	if s.config.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	s.router = gin.Default()

	// Middleware
	s.router.Use(gin.Recovery())
	s.router.Use(corsMiddleware())
	s.router.Use(loggingMiddleware())

	// Health check
	s.router.GET("/health", s.healthCheck)
	s.router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API routes
	v1 := s.router.Group("/v1")
	{
		// Cache operations
		v1.GET("/cache/:key", s.getCache)
		v1.POST("/cache/:key", s.setCache)
		v1.PUT("/cache/:key", s.setCache)
		v1.DELETE("/cache/:key", s.deleteCache)
		
		// Batch operations
		v1.POST("/cache/batch/get", s.batchGet)
		v1.POST("/cache/batch/set", s.batchSet)
		v1.POST("/cache/batch/delete", s.batchDelete)

		// Cache management
		v1.POST("/cache/flush", s.flushCache)
		v1.POST("/cache/invalidate", s.invalidatePattern)
		v1.GET("/cache/stats", s.getCacheStats)
		v1.GET("/cache/keys", s.listKeys)

		// Multi-tier operations
		v1.GET("/cache/multi/:key", s.getMultiTier)
		v1.POST("/cache/multi/:key", s.setMultiTier)
		v1.DELETE("/cache/multi/:key", s.deleteMultiTier)

		// Cache warming
		v1.POST("/cache/warm", s.warmCache)
		v1.GET("/cache/health/:tier", s.getTierHealth)
	}
}

func (s *CachingService) Start() error {
	// Start background workers
	go s.startL1CacheEviction()
	go s.startMetricsUpdater()
	go s.startHealthChecker()

	// Start HTTP server
	s.httpServer = &http.Server{
		Addr:    ":" + s.config.Port,
		Handler: s.router,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down caching service...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := s.httpServer.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}

		s.cleanup()
	}()

	log.Printf("🚀 Caching Service starting on port %s", s.config.Port)
	log.Printf("📊 Health check: http://localhost:%s/health", s.config.Port)
	log.Printf("📈 Metrics: http://localhost:%s/metrics", s.config.Port)
	log.Printf("🔄 Redis: %s", s.config.RedisURL)
	log.Printf("💾 Memcached: %s", s.config.MemcachedURL)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	return nil
}

func (s *CachingService) cleanup() {
	if s.redisClient != nil {
		s.redisClient.Close()
	}
}

// Health check endpoint
func (s *CachingService) healthCheck(c *gin.Context) {
	status := gin.H{
		"status":    "healthy",
		"service":   "caching-service",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   "1.0.0",
	}

	// Check Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.redisClient.Ping(ctx).Err(); err != nil {
		status["status"] = "unhealthy"
		status["redis"] = "disconnected"
	} else {
		status["redis"] = "connected"
	}

	// Check Memcached connection
	if err := s.memcacheClient.Ping(); err != nil {
		status["memcached"] = "disconnected"
	} else {
		status["memcached"] = "connected"
	}

	// Add cache stats
	status["l1_cache_keys"] = len(s.l1Cache)

	if status["status"] == "unhealthy" {
		c.JSON(http.StatusServiceUnavailable, status)
		return
	}

	c.JSON(http.StatusOK, status)
}

// Cache operations
func (s *CachingService) getCache(c *gin.Context) {
	key := c.Param("key")
	tier := c.DefaultQuery("tier", TierL2)
	
	start := time.Now()
	
	value, found, err := s.getCacheValue(key, tier)
	
	// Update metrics
	duration := time.Since(start).Seconds()
	cacheLatency.WithLabelValues(OpGet, tier).Observe(duration)
	
	if err != nil {
		cacheOperations.WithLabelValues(OpGet, tier, "error").Inc()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	if found {
		cacheHits.WithLabelValues(tier).Inc()
		cacheOperations.WithLabelValues(OpGet, tier, "hit").Inc()
		c.JSON(http.StatusOK, gin.H{
			"key":   key,
			"value": value,
			"tier":  tier,
			"found": true,
		})
	} else {
		cacheMisses.WithLabelValues(tier).Inc()
		cacheOperations.WithLabelValues(OpGet, tier, "miss").Inc()
		c.JSON(http.StatusNotFound, gin.H{
			"key":   key,
			"found": false,
		})
	}
}

func (s *CachingService) setCache(c *gin.Context) {
	key := c.Param("key")
	tier := c.DefaultQuery("tier", TierL2)
	ttlStr := c.DefaultQuery("ttl", "3600")
	
	var requestBody struct {
		Value interface{} `json:"value"`
		TTL   *int64      `json:"ttl,omitempty"`
	}
	
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	
	// Parse TTL
	ttl, _ := strconv.ParseInt(ttlStr, 10, 64)
	if requestBody.TTL != nil {
		ttl = *requestBody.TTL
	}
	
	start := time.Now()
	
	err := s.setCacheValue(key, requestBody.Value, time.Duration(ttl)*time.Second, tier)
	
	// Update metrics
	duration := time.Since(start).Seconds()
	cacheLatency.WithLabelValues(OpSet, tier).Observe(duration)
	
	if err != nil {
		cacheOperations.WithLabelValues(OpSet, tier, "error").Inc()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	cacheOperations.WithLabelValues(OpSet, tier, "success").Inc()
	c.JSON(http.StatusOK, gin.H{
		"key":     key,
		"tier":    tier,
		"ttl":     ttl,
		"message": "Cache entry set successfully",
	})
}

func (s *CachingService) deleteCache(c *gin.Context) {
	key := c.Param("key")
	tier := c.DefaultQuery("tier", TierL2)
	
	start := time.Now()
	
	err := s.deleteCacheValue(key, tier)
	
	// Update metrics
	duration := time.Since(start).Seconds()
	cacheLatency.WithLabelValues(OpDelete, tier).Observe(duration)
	
	if err != nil {
		cacheOperations.WithLabelValues(OpDelete, tier, "error").Inc()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	cacheOperations.WithLabelValues(OpDelete, tier, "success").Inc()
	c.JSON(http.StatusOK, gin.H{
		"key":     key,
		"tier":    tier,
		"message": "Cache entry deleted successfully",
	})
}

// Multi-tier cache operations
func (s *CachingService) getMultiTier(c *gin.Context) {
	key := c.Param("key")
	
	start := time.Now()
	
	// Try L1 cache first
	if value, found := s.getL1Cache(key); found {
		cacheHits.WithLabelValues(TierL1).Inc()
		c.JSON(http.StatusOK, gin.H{
			"key":   key,
			"value": value.Value,
			"tier":  TierL1,
			"found": true,
		})
		return
	}
	
	// Try L2 cache (Redis)
	if value, found, err := s.getCacheValue(key, TierL2); err == nil && found {
		cacheHits.WithLabelValues(TierL2).Inc()
		// Promote to L1
		s.setL1Cache(key, value, s.config.DefaultTTL)
		c.JSON(http.StatusOK, gin.H{
			"key":   key,
			"value": value,
			"tier":  TierL2,
			"found": true,
		})
		return
	}
	
	// Try L3 cache (Memcached)
	if value, found, err := s.getCacheValue(key, TierL3); err == nil && found {
		cacheHits.WithLabelValues(TierL3).Inc()
		// Promote to L1 and L2
		s.setL1Cache(key, value, s.config.DefaultTTL)
		s.setCacheValue(key, value, s.config.DefaultTTL, TierL2)
		c.JSON(http.StatusOK, gin.H{
			"key":   key,
			"value": value,
			"tier":  TierL3,
			"found": true,
		})
		return
	}
	
	// Cache miss on all tiers
	cacheMisses.WithLabelValues("multi").Inc()
	duration := time.Since(start).Seconds()
	cacheLatency.WithLabelValues(OpGet, "multi").Observe(duration)
	
	c.JSON(http.StatusNotFound, gin.H{
		"key":   key,
		"found": false,
	})
}

// Cache tier implementations
func (s *CachingService) getCacheValue(key, tier string) (interface{}, bool, error) {
	switch tier {
	case TierL1:
		if entry, found := s.getL1Cache(key); found {
			return entry.Value, true, nil
		}
		return nil, false, nil
		
	case TierL2:
		ctx := context.Background()
		val, err := s.redisClient.Get(ctx, key).Result()
		if err == redis.Nil {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, err
		}
		
		var value interface{}
		if err := json.Unmarshal([]byte(val), &value); err != nil {
			return nil, false, err
		}
		return value, true, nil
		
	case TierL3:
		item, err := s.memcacheClient.Get(key)
		if err == memcache.ErrCacheMiss {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, err
		}
		
		var value interface{}
		if err := json.Unmarshal(item.Value, &value); err != nil {
			return nil, false, err
		}
		return value, true, nil
		
	default:
		return nil, false, fmt.Errorf("unsupported cache tier: %s", tier)
	}
}

func (s *CachingService) setCacheValue(key string, value interface{}, ttl time.Duration, tier string) error {
	switch tier {
	case TierL1:
		s.setL1Cache(key, value, ttl)
		return nil
		
	case TierL2:
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		
		ctx := context.Background()
		return s.redisClient.Set(ctx, key, data, ttl).Err()
		
	case TierL3:
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		
		return s.memcacheClient.Set(&memcache.Item{
			Key:        key,
			Value:      data,
			Expiration: int32(ttl.Seconds()),
		})
		
	default:
		return fmt.Errorf("unsupported cache tier: %s", tier)
	}
}

func (s *CachingService) deleteCacheValue(key, tier string) error {
	switch tier {
	case TierL1:
		delete(s.l1Cache, key)
		return nil
		
	case TierL2:
		ctx := context.Background()
		return s.redisClient.Del(ctx, key).Err()
		
	case TierL3:
		return s.memcacheClient.Delete(key)
		
	default:
		return fmt.Errorf("unsupported cache tier: %s", tier)
	}
}

// L1 cache operations
func (s *CachingService) getL1Cache(key string) (*CacheEntry, bool) {
	entry, found := s.l1Cache[key]
	if !found {
		return nil, false
	}
	
	// Check expiration
	if time.Now().After(entry.ExpiresAt) {
		delete(s.l1Cache, key)
		return nil, false
	}
	
	return entry, true
}

func (s *CachingService) setL1Cache(key string, value interface{}, ttl time.Duration) {
	s.l1Cache[key] = &CacheEntry{
		Key:       key,
		Value:     value,
		TTL:       int64(ttl.Seconds()),
		Tier:      TierL1,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(ttl),
	}
}

// Background workers
func (s *CachingService) startL1CacheEviction() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.evictExpiredL1Entries()
		}
	}
}

func (s *CachingService) evictExpiredL1Entries() {
	now := time.Now()
	for key, entry := range s.l1Cache {
		if now.After(entry.ExpiresAt) {
			delete(s.l1Cache, key)
		}
	}
}

func (s *CachingService) startMetricsUpdater() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.updateCacheMetrics()
		}
	}
}

func (s *CachingService) updateCacheMetrics() {
	// Update L1 cache metrics
	cacheKeys.WithLabelValues(TierL1).Set(float64(len(s.l1Cache)))
	
	// Update Redis metrics
	ctx := context.Background()
	if info, err := s.redisClient.Info(ctx, "memory").Result(); err == nil {
		// Parse Redis memory info (simplified)
		lines := strings.Split(info, "\r\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "used_memory:") {
				if memStr := strings.TrimPrefix(line, "used_memory:"); memStr != "" {
					if mem, err := strconv.ParseInt(memStr, 10, 64); err == nil {
						cacheSize.WithLabelValues(TierL2).Set(float64(mem))
					}
				}
			}
		}
	}
}

func (s *CachingService) startHealthChecker() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.checkCacheHealth()
		}
	}
}

func (s *CachingService) checkCacheHealth() {
	// Check Redis health
	ctx := context.Background()
	if err := s.redisClient.Ping(ctx).Err(); err != nil {
		log.Printf("Redis health check failed: %v", err)
	}
	
	// Check Memcached health
	if err := s.memcacheClient.Ping(); err != nil {
		log.Printf("Memcached health check failed: %v", err)
	}
}

// Utility functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseInt(s string) int {
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	return 0
}

func parseInt64(s string) int64 {
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	return 0
}

func getBool(s string) bool {
	return strings.ToLower(s) == "true"
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Requested-With")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func loggingMiddleware() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("%s - [%s] \"%s %s %s %d %s \"%s\" %s\"\n",
			param.ClientIP,
			param.TimeStamp.Format(time.RFC1123),
			param.Method,
			param.Path,
			param.Request.Proto,
			param.StatusCode,
			param.Latency,
			param.Request.UserAgent(),
			param.ErrorMessage,
		)
	})
}
