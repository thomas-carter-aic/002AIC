/**
 * API Gateway Service - Go
 * Centralized API management and routing for the 002AIC platform
 * Handles request routing, authentication, rate limiting, load balancing, and API analytics
 */

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/time/rate"
	"github.com/gorilla/websocket"
)

// Configuration
type Config struct {
	Port         string
	DatabaseURL  string
	RedisURL     string
	JWTSecret    string
	Environment  string
	DefaultRateLimit int
	MaxRequestSize   int64
	RequestTimeout   time.Duration
}

// Rate limiting
type RateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
}

// Models
type APIRoute struct {
	ID              string                 `json:"id" gorm:"primaryKey"`
	Path            string                 `json:"path" gorm:"uniqueIndex;not null"`
	Method          string                 `json:"method" gorm:"not null"`
	ServiceName     string                 `json:"service_name" gorm:"not null"`
	ServiceURL      string                 `json:"service_url" gorm:"not null"`
	IsActive        bool                   `json:"is_active" gorm:"default:true"`
	RequireAuth     bool                   `json:"require_auth" gorm:"default:true"`
	RateLimit       int                    `json:"rate_limit" gorm:"default:1000"`
	Timeout         int                    `json:"timeout" gorm:"default:30"`
	RetryCount      int                    `json:"retry_count" gorm:"default:3"`
	LoadBalancing   string                 `json:"load_balancing" gorm:"default:round_robin"`
	HealthCheckURL  string                 `json:"health_check_url"`
	Metadata        map[string]interface{} `json:"metadata" gorm:"type:jsonb"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

type APIKey struct {
	ID          string    `json:"id" gorm:"primaryKey"`
	Key         string    `json:"key" gorm:"uniqueIndex;not null"`
	Name        string    `json:"name" gorm:"not null"`
	UserID      string    `json:"user_id" gorm:"index"`
	Scopes      []string  `json:"scopes" gorm:"type:text[]"`
	RateLimit   int       `json:"rate_limit" gorm:"default:1000"`
	IsActive    bool      `json:"is_active" gorm:"default:true"`
	ExpiresAt   *time.Time `json:"expires_at"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type RequestLog struct {
	ID            string                 `json:"id" gorm:"primaryKey"`
	RequestID     string                 `json:"request_id" gorm:"index"`
	Method        string                 `json:"method"`
	Path          string                 `json:"path"`
	ServiceName   string                 `json:"service_name"`
	UserID        string                 `json:"user_id" gorm:"index"`
	APIKeyID      string                 `json:"api_key_id" gorm:"index"`
	IPAddress     string                 `json:"ip_address"`
	UserAgent     string                 `json:"user_agent"`
	StatusCode    int                    `json:"status_code"`
	ResponseTime  int64                  `json:"response_time_ms"`
	RequestSize   int64                  `json:"request_size"`
	ResponseSize  int64                  `json:"response_size"`
	ErrorMessage  string                 `json:"error_message"`
	Metadata      map[string]interface{} `json:"metadata" gorm:"type:jsonb"`
	CreatedAt     time.Time              `json:"created_at"`
}

// Service struct
type APIGatewayService struct {
	db           *gorm.DB
	redis        *redis.Client
	config       *Config
	router       *gin.Engine
	httpServer   *http.Server
	rateLimiter  *RateLimiter
	routes       map[string]*APIRoute
	routesMutex  sync.RWMutex
	upgrader     websocket.Upgrader
}

// Prometheus metrics
var (
	requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_gateway_requests_total",
			Help: "Total number of API requests",
		},
		[]string{"method", "path", "service", "status_code"},
	)

	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "api_gateway_request_duration_seconds",
			Help: "Request duration in seconds",
		},
		[]string{"method", "path", "service"},
	)

	activeConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "api_gateway_active_connections",
			Help: "Number of active connections",
		},
	)

	rateLimitHits = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_gateway_rate_limit_hits_total",
			Help: "Total number of rate limit hits",
		},
		[]string{"user_id", "api_key_id"},
	)

	routesTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "api_gateway_routes_total",
			Help: "Total number of configured routes",
		},
	)
)

func init() {
	prometheus.MustRegister(requestsTotal)
	prometheus.MustRegister(requestDuration)
	prometheus.MustRegister(activeConnections)
	prometheus.MustRegister(rateLimitHits)
	prometheus.MustRegister(routesTotal)
}

func main() {
	config := &Config{
		Port:             getEnv("PORT", "8080"),
		DatabaseURL:      getEnv("DATABASE_URL", "postgres://postgres:password@localhost:5432/api_gateway?sslmode=disable"),
		RedisURL:         getEnv("REDIS_URL", "redis://localhost:6379"),
		JWTSecret:        getEnv("JWT_SECRET", "your-secret-key"),
		Environment:      getEnv("ENVIRONMENT", "development"),
		DefaultRateLimit: parseInt(getEnv("DEFAULT_RATE_LIMIT", "1000")),
		MaxRequestSize:   parseInt64(getEnv("MAX_REQUEST_SIZE", "10485760")), // 10MB
		RequestTimeout:   time.Duration(parseInt(getEnv("REQUEST_TIMEOUT", "30"))) * time.Second,
	}

	service, err := NewAPIGatewayService(config)
	if err != nil {
		log.Fatal("Failed to create API gateway service:", err)
	}

	if err := service.Start(); err != nil {
		log.Fatal("Failed to start API gateway service:", err)
	}
}

func NewAPIGatewayService(config *Config) (*APIGatewayService, error) {
	// Initialize database
	db, err := gorm.Open(postgres.Open(config.DatabaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto-migrate tables
	if err := db.AutoMigrate(&APIRoute{}, &APIKey{}, &RequestLog{}); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	// Initialize Redis
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

	// Initialize rate limiter
	rateLimiter := &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(config.DefaultRateLimit),
		burst:    config.DefaultRateLimit,
	}

	// Initialize WebSocket upgrader
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins in development
		},
	}

	service := &APIGatewayService{
		db:          db,
		redis:       redisClient,
		config:      config,
		rateLimiter: rateLimiter,
		routes:      make(map[string]*APIRoute),
		upgrader:    upgrader,
	}

	service.setupRoutes()
	return service, nil
}

func (s *APIGatewayService) setupRoutes() {
	if s.config.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	s.router = gin.Default()

	// Middleware
	s.router.Use(gin.Recovery())
	s.router.Use(corsMiddleware())
	s.router.Use(requestLoggingMiddleware())
	s.router.Use(s.rateLimitMiddleware())

	// Set max request size
	s.router.MaxMultipartMemory = s.config.MaxRequestSize

	// Health check
	s.router.GET("/health", s.healthCheck)
	s.router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Admin API routes
	admin := s.router.Group("/admin/v1")
	admin.Use(s.adminAuthMiddleware())
	{
		// Route management
		admin.POST("/routes", s.createRoute)
		admin.GET("/routes", s.listRoutes)
		admin.GET("/routes/:id", s.getRoute)
		admin.PUT("/routes/:id", s.updateRoute)
		admin.DELETE("/routes/:id", s.deleteRoute)

		// API Key management
		admin.POST("/api-keys", s.createAPIKey)
		admin.GET("/api-keys", s.listAPIKeys)
		admin.GET("/api-keys/:id", s.getAPIKey)
		admin.PUT("/api-keys/:id", s.updateAPIKey)
		admin.DELETE("/api-keys/:id", s.deleteAPIKey)

		// Analytics
		admin.GET("/analytics/requests", s.getRequestAnalytics)
		admin.GET("/analytics/performance", s.getPerformanceAnalytics)
		admin.GET("/analytics/errors", s.getErrorAnalytics)
	}

	// WebSocket endpoint
	s.router.GET("/ws", s.handleWebSocket)

	// Catch-all route for API proxying
	s.router.NoRoute(s.proxyHandler)
}

func (s *APIGatewayService) Start() error {
	// Load routes from database
	if err := s.loadRoutes(); err != nil {
		return fmt.Errorf("failed to load routes: %w", err)
	}

	// Start background workers
	go s.startMetricsUpdater()
	go s.startHealthChecker()
	go s.startLogCleaner()

	// Start HTTP server
	s.httpServer = &http.Server{
		Addr:         ":" + s.config.Port,
		Handler:      s.router,
		ReadTimeout:  s.config.RequestTimeout,
		WriteTimeout: s.config.RequestTimeout,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down API gateway service...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := s.httpServer.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}

		s.cleanup()
	}()

	log.Printf("🚀 API Gateway Service starting on port %s", s.config.Port)
	log.Printf("📊 Health check: http://localhost:%s/health", s.config.Port)
	log.Printf("📈 Metrics: http://localhost:%s/metrics", s.config.Port)
	log.Printf("🔧 Admin API: http://localhost:%s/admin/v1", s.config.Port)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	return nil
}

func (s *APIGatewayService) cleanup() {
	if s.redis != nil {
		s.redis.Close()
	}
	if s.db != nil {
		sqlDB, _ := s.db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	}
}

// Health check endpoint
func (s *APIGatewayService) healthCheck(c *gin.Context) {
	status := gin.H{
		"status":    "healthy",
		"service":   "api-gateway-service",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   "1.0.0",
	}

	// Check database connection
	sqlDB, err := s.db.DB()
	if err != nil || sqlDB.Ping() != nil {
		status["status"] = "unhealthy"
		status["database"] = "disconnected"
		c.JSON(http.StatusServiceUnavailable, status)
		return
	}
	status["database"] = "connected"

	// Check Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.redis.Ping(ctx).Err(); err != nil {
		status["status"] = "unhealthy"
		status["redis"] = "disconnected"
		c.JSON(http.StatusServiceUnavailable, status)
		return
	}
	status["redis"] = "connected"

	// Add route count
	s.routesMutex.RLock()
	status["active_routes"] = len(s.routes)
	s.routesMutex.RUnlock()

	c.JSON(http.StatusOK, status)
}

// Proxy handler - main request routing logic
func (s *APIGatewayService) proxyHandler(c *gin.Context) {
	startTime := time.Now()
	requestID := uuid.New().String()
	c.Set("request_id", requestID)

	// Find matching route
	route := s.findRoute(c.Request.Method, c.Request.URL.Path)
	if route == nil {
		s.logRequest(c, requestID, "", http.StatusNotFound, time.Since(startTime), "Route not found")
		c.JSON(http.StatusNotFound, gin.H{"error": "Route not found"})
		return
	}

	// Check if route is active
	if !route.IsActive {
		s.logRequest(c, requestID, route.ServiceName, http.StatusServiceUnavailable, time.Since(startTime), "Route inactive")
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable"})
		return
	}

	// Authentication check
	if route.RequireAuth {
		if !s.authenticateRequest(c) {
			s.logRequest(c, requestID, route.ServiceName, http.StatusUnauthorized, time.Since(startTime), "Authentication failed")
			return
		}
	}

	// Rate limiting
	if !s.checkRateLimit(c, route) {
		s.logRequest(c, requestID, route.ServiceName, http.StatusTooManyRequests, time.Since(startTime), "Rate limit exceeded")
		return
	}

	// Proxy the request
	s.proxyRequest(c, route, requestID, startTime)
}

// Find matching route
func (s *APIGatewayService) findRoute(method, path string) *APIRoute {
	s.routesMutex.RLock()
	defer s.routesMutex.RUnlock()

	// Exact match first
	key := method + ":" + path
	if route, exists := s.routes[key]; exists {
		return route
	}

	// Pattern matching (simplified)
	for routeKey, route := range s.routes {
		if strings.HasPrefix(routeKey, method+":") {
			routePath := strings.TrimPrefix(routeKey, method+":")
			if s.matchPath(routePath, path) {
				return route
			}
		}
	}

	return nil
}

// Simple path matching (can be enhanced with more sophisticated patterns)
func (s *APIGatewayService) matchPath(pattern, path string) bool {
	// Handle wildcard patterns
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return strings.HasPrefix(path, prefix)
	}

	// Handle parameter patterns like /users/:id
	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")

	if len(patternParts) != len(pathParts) {
		return false
	}

	for i, part := range patternParts {
		if strings.HasPrefix(part, ":") {
			// Parameter match
			continue
		}
		if part != pathParts[i] {
			return false
		}
	}

	return true
}

// Authenticate request
func (s *APIGatewayService) authenticateRequest(c *gin.Context) bool {
	// Check for API key
	apiKey := c.GetHeader("X-API-Key")
	if apiKey != "" {
		return s.validateAPIKey(c, apiKey)
	}

	// Check for JWT token
	authHeader := c.GetHeader("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		return s.validateJWT(c, token)
	}

	c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
	return false
}

// Validate API key
func (s *APIGatewayService) validateAPIKey(c *gin.Context, keyValue string) bool {
	var apiKey APIKey
	if err := s.db.Where("key = ? AND is_active = true", keyValue).First(&apiKey).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
		return false
	}

	// Check expiration
	if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "API key expired"})
		return false
	}

	// Update last used
	go func() {
		now := time.Now()
		s.db.Model(&apiKey).Update("last_used_at", now)
	}()

	// Set context
	c.Set("user_id", apiKey.UserID)
	c.Set("api_key_id", apiKey.ID)
	c.Set("scopes", apiKey.Scopes)

	return true
}

// Validate JWT token
func (s *APIGatewayService) validateJWT(c *gin.Context, tokenString string) bool {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.config.JWTSecret), nil
	})

	if err != nil || !token.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return false
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		c.Set("user_id", claims["user_id"])
		c.Set("scopes", claims["scopes"])
		return true
	}

	c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
	return false
}

// Check rate limit
func (s *APIGatewayService) checkRateLimit(c *gin.Context, route *APIRoute) bool {
	// Get identifier for rate limiting
	identifier := s.getRateLimitIdentifier(c)
	
	// Get rate limit for this route/user
	limit := route.RateLimit
	if apiKeyID, exists := c.Get("api_key_id"); exists {
		var apiKey APIKey
		if err := s.db.First(&apiKey, "id = ?", apiKeyID).Error; err == nil {
			limit = apiKey.RateLimit
		}
	}

	// Check rate limit
	limiter := s.rateLimiter.getLimiter(identifier, rate.Limit(limit), limit)
	if !limiter.Allow() {
		rateLimitHits.WithLabelValues(
			c.GetString("user_id"),
			c.GetString("api_key_id"),
		).Inc()
		
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error": "Rate limit exceeded",
			"limit": limit,
		})
		return false
	}

	return true
}

// Get rate limit identifier
func (s *APIGatewayService) getRateLimitIdentifier(c *gin.Context) string {
	if userID := c.GetString("user_id"); userID != "" {
		return "user:" + userID
	}
	if apiKeyID := c.GetString("api_key_id"); apiKeyID != "" {
		return "api_key:" + apiKeyID
	}
	return "ip:" + c.ClientIP()
}

// Proxy request to backend service
func (s *APIGatewayService) proxyRequest(c *gin.Context, route *APIRoute, requestID string, startTime time.Time) {
	// Parse target URL
	target, err := url.Parse(route.ServiceURL)
	if err != nil {
		s.logRequest(c, requestID, route.ServiceName, http.StatusInternalServerError, time.Since(startTime), "Invalid service URL")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Service configuration error"})
		return
	}

	// Create reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(target)
	
	// Customize the director to modify the request
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		
		// Add headers
		req.Header.Set("X-Request-ID", requestID)
		req.Header.Set("X-Forwarded-For", c.ClientIP())
		req.Header.Set("X-Gateway-Service", "002aic-api-gateway")
		
		// Add user context
		if userID := c.GetString("user_id"); userID != "" {
			req.Header.Set("X-User-ID", userID)
		}
	}

	// Handle response
	proxy.ModifyResponse = func(resp *http.Response) error {
		// Add response headers
		resp.Header.Set("X-Request-ID", requestID)
		return nil
	}

	// Handle errors
	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		s.logRequest(c, requestID, route.ServiceName, http.StatusBadGateway, time.Since(startTime), err.Error())
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(gin.H{"error": "Service unavailable"})
	}

	// Set timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(route.Timeout)*time.Second)
	defer cancel()
	c.Request = c.Request.WithContext(ctx)

	// Proxy the request
	proxy.ServeHTTP(c.Writer, c.Request)

	// Log the request
	duration := time.Since(startTime)
	statusCode := c.Writer.Status()
	
	s.logRequest(c, requestID, route.ServiceName, statusCode, duration, "")

	// Update metrics
	requestsTotal.WithLabelValues(
		c.Request.Method,
		c.Request.URL.Path,
		route.ServiceName,
		strconv.Itoa(statusCode),
	).Inc()

	requestDuration.WithLabelValues(
		c.Request.Method,
		c.Request.URL.Path,
		route.ServiceName,
	).Observe(duration.Seconds())
}

// Rate limiter methods
func (rl *RateLimiter) getLimiter(key string, r rate.Limit, burst int) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.limiters[key]
	if !exists {
		limiter = rate.NewLimiter(r, burst)
		rl.limiters[key] = limiter
	}

	return limiter
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

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Requested-With, X-API-Key")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func requestLoggingMiddleware() gin.HandlerFunc {
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

func (s *APIGatewayService) rateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip rate limiting for health check and admin endpoints
		if strings.HasPrefix(c.Request.URL.Path, "/health") ||
		   strings.HasPrefix(c.Request.URL.Path, "/metrics") ||
		   strings.HasPrefix(c.Request.URL.Path, "/admin") {
			c.Next()
			return
		}

		c.Next()
	}
}

func (s *APIGatewayService) adminAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Simple admin authentication - in production, use proper admin tokens
		adminToken := c.GetHeader("X-Admin-Token")
		if adminToken != "admin-secret-token" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Admin access required"})
			c.Abort()
			return
		}
		c.Next()
	}
}
