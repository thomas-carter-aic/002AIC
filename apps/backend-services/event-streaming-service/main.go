/**
 * Event Streaming Service - Go
 * Real-time event processing and streaming for the 002AIC platform
 * Handles event ingestion, processing, routing, and real-time analytics
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
	"github.com/gorilla/websocket"
	"github.com/segmentio/kafka-go"
	"github.com/nats-io/nats.go"
	"github.com/confluentinc/confluent-kafka-go/kafka"
)

// Configuration
type Config struct {
	Port            string
	DatabaseURL     string
	RedisURL        string
	KafkaBrokers    []string
	NATSUrl         string
	Environment     string
	MaxEventSize    int64
	RetentionPeriod time.Duration
	BatchSize       int
	FlushInterval   time.Duration
}

// Event types
const (
	EventTypeUserAction     = "user_action"
	EventTypeSystemEvent    = "system_event"
	EventTypeBusinessEvent  = "business_event"
	EventTypeMetricEvent    = "metric_event"
	EventTypeAuditEvent     = "audit_event"
	EventTypeNotification   = "notification"
	EventTypeWorkflowEvent  = "workflow_event"
	EventTypeModelEvent     = "model_event"
)

// Event priorities
const (
	PriorityLow      = "low"
	PriorityNormal   = "normal"
	PriorityHigh     = "high"
	PriorityCritical = "critical"
)

// Models
type Event struct {
	ID          string                 `json:"id" gorm:"primaryKey"`
	Type        string                 `json:"type" gorm:"index;not null"`
	Source      string                 `json:"source" gorm:"index;not null"`
	Subject     string                 `json:"subject" gorm:"index"`
	Priority    string                 `json:"priority" gorm:"default:normal"`
	Data        map[string]interface{} `json:"data" gorm:"type:jsonb"`
	Metadata    map[string]interface{} `json:"metadata" gorm:"type:jsonb"`
	UserID      string                 `json:"user_id" gorm:"index"`
	SessionID   string                 `json:"session_id" gorm:"index"`
	TraceID     string                 `json:"trace_id" gorm:"index"`
	SpanID      string                 `json:"span_id"`
	Timestamp   time.Time              `json:"timestamp" gorm:"index"`
	ProcessedAt *time.Time             `json:"processed_at"`
	CreatedAt   time.Time              `json:"created_at"`
}

type EventStream struct {
	ID          string                 `json:"id" gorm:"primaryKey"`
	Name        string                 `json:"name" gorm:"uniqueIndex;not null"`
	Description string                 `json:"description"`
	EventTypes  []string               `json:"event_types" gorm:"type:text[]"`
	Filters     map[string]interface{} `json:"filters" gorm:"type:jsonb"`
	IsActive    bool                   `json:"is_active" gorm:"default:true"`
	Config      map[string]interface{} `json:"config" gorm:"type:jsonb"`
	CreatedBy   string                 `json:"created_by"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

type EventSubscription struct {
	ID            string                 `json:"id" gorm:"primaryKey"`
	StreamID      string                 `json:"stream_id" gorm:"index"`
	Stream        EventStream            `json:"stream" gorm:"foreignKey:StreamID"`
	SubscriberID  string                 `json:"subscriber_id" gorm:"index"`
	WebhookURL    string                 `json:"webhook_url"`
	EventTypes    []string               `json:"event_types" gorm:"type:text[]"`
	Filters       map[string]interface{} `json:"filters" gorm:"type:jsonb"`
	IsActive      bool                   `json:"is_active" gorm:"default:true"`
	RetryPolicy   map[string]interface{} `json:"retry_policy" gorm:"type:jsonb"`
	LastEventAt   *time.Time             `json:"last_event_at"`
	EventCount    int64                  `json:"event_count" gorm:"default:0"`
	ErrorCount    int64                  `json:"error_count" gorm:"default:0"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

// Service struct
type EventStreamingService struct {
	db              *gorm.DB
	redis           *redis.Client
	config          *Config
	router          *gin.Engine
	httpServer      *http.Server
	kafkaProducer   *kafka.Producer
	kafkaConsumer   *kafka.Consumer
	natsConn        *nats.Conn
	upgrader        websocket.Upgrader
	wsConnections   map[string]*websocket.Conn
	wsConnectionsMu sync.RWMutex
	eventBuffer     chan *Event
	subscribers     map[string][]*EventSubscription
	subscribersMu   sync.RWMutex
}

// Prometheus metrics
var (
	eventsIngested = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "events_ingested_total",
			Help: "Total number of events ingested",
		},
		[]string{"type", "source", "priority"},
	)

	eventsProcessed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "events_processed_total",
			Help: "Total number of events processed",
		},
		[]string{"type", "status"},
	)

	eventProcessingDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "event_processing_duration_seconds",
			Help: "Time taken to process events",
		},
		[]string{"type"},
	)

	activeStreams = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_streams_total",
			Help: "Number of active event streams",
		},
	)

	activeSubscriptions = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_subscriptions_total",
			Help: "Number of active event subscriptions",
		},
	)

	wsConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "websocket_connections_active",
			Help: "Number of active WebSocket connections",
		},
	)

	eventBufferSize = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "event_buffer_size",
			Help: "Current size of event buffer",
		},
	)
)

func init() {
	prometheus.MustRegister(eventsIngested)
	prometheus.MustRegister(eventsProcessed)
	prometheus.MustRegister(eventProcessingDuration)
	prometheus.MustRegister(activeStreams)
	prometheus.MustRegister(activeSubscriptions)
	prometheus.MustRegister(wsConnections)
	prometheus.MustRegister(eventBufferSize)
}

func main() {
	config := &Config{
		Port:            getEnv("PORT", "8080"),
		DatabaseURL:     getEnv("DATABASE_URL", "postgres://postgres:password@localhost:5432/event_streaming?sslmode=disable"),
		RedisURL:        getEnv("REDIS_URL", "redis://localhost:6379"),
		KafkaBrokers:    strings.Split(getEnv("KAFKA_BROKERS", "localhost:9092"), ","),
		NATSUrl:         getEnv("NATS_URL", "nats://localhost:4222"),
		Environment:     getEnv("ENVIRONMENT", "development"),
		MaxEventSize:    parseInt64(getEnv("MAX_EVENT_SIZE", "1048576")), // 1MB
		RetentionPeriod: time.Duration(parseInt(getEnv("RETENTION_DAYS", "30"))) * 24 * time.Hour,
		BatchSize:       parseInt(getEnv("BATCH_SIZE", "100")),
		FlushInterval:   time.Duration(parseInt(getEnv("FLUSH_INTERVAL", "1000"))) * time.Millisecond,
	}

	service, err := NewEventStreamingService(config)
	if err != nil {
		log.Fatal("Failed to create event streaming service:", err)
	}

	if err := service.Start(); err != nil {
		log.Fatal("Failed to start event streaming service:", err)
	}
}

func NewEventStreamingService(config *Config) (*EventStreamingService, error) {
	// Initialize database
	db, err := gorm.Open(postgres.Open(config.DatabaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto-migrate tables
	if err := db.AutoMigrate(&Event{}, &EventStream{}, &EventSubscription{}); err != nil {
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

	// Initialize Kafka producer
	kafkaProducer, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": strings.Join(config.KafkaBrokers, ","),
		"client.id":         "event-streaming-service",
		"acks":              "all",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka producer: %w", err)
	}

	// Initialize Kafka consumer
	kafkaConsumer, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": strings.Join(config.KafkaBrokers, ","),
		"group.id":          "event-streaming-service",
		"auto.offset.reset": "earliest",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka consumer: %w", err)
	}

	// Initialize NATS connection
	natsConn, err := nats.Connect(config.NATSUrl)
	if err != nil {
		log.Printf("Warning: Failed to connect to NATS: %v", err)
	}

	// Initialize WebSocket upgrader
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins in development
		},
	}

	service := &EventStreamingService{
		db:            db,
		redis:         redisClient,
		config:        config,
		kafkaProducer: kafkaProducer,
		kafkaConsumer: kafkaConsumer,
		natsConn:      natsConn,
		upgrader:      upgrader,
		wsConnections: make(map[string]*websocket.Conn),
		eventBuffer:   make(chan *Event, config.BatchSize*10),
		subscribers:   make(map[string][]*EventSubscription),
	}

	service.setupRoutes()
	return service, nil
}

func (s *EventStreamingService) setupRoutes() {
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
		// Event ingestion
		v1.POST("/events", s.ingestEvent)
		v1.POST("/events/batch", s.ingestBatchEvents)
		v1.GET("/events", s.queryEvents)
		v1.GET("/events/:id", s.getEvent)

		// Event streams
		v1.POST("/streams", s.createStream)
		v1.GET("/streams", s.listStreams)
		v1.GET("/streams/:id", s.getStream)
		v1.PUT("/streams/:id", s.updateStream)
		v1.DELETE("/streams/:id", s.deleteStream)

		// Event subscriptions
		v1.POST("/subscriptions", s.createSubscription)
		v1.GET("/subscriptions", s.listSubscriptions)
		v1.GET("/subscriptions/:id", s.getSubscription)
		v1.PUT("/subscriptions/:id", s.updateSubscription)
		v1.DELETE("/subscriptions/:id", s.deleteSubscription)

		// Real-time streaming
		v1.GET("/stream/:stream_id/ws", s.handleWebSocket)
		v1.GET("/events/live", s.handleLiveEvents)

		// Analytics
		v1.GET("/analytics/events", s.getEventAnalytics)
		v1.GET("/analytics/streams", s.getStreamAnalytics)
		v1.GET("/analytics/performance", s.getPerformanceAnalytics)
	}
}

func (s *EventStreamingService) Start() error {
	// Load subscriptions
	if err := s.loadSubscriptions(); err != nil {
		return fmt.Errorf("failed to load subscriptions: %w", err)
	}

	// Start background workers
	go s.startEventProcessor()
	go s.startKafkaConsumer()
	go s.startEventDispatcher()
	go s.startMetricsUpdater()
	go s.startCleanupWorker()

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

		log.Println("Shutting down event streaming service...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := s.httpServer.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}

		s.cleanup()
	}()

	log.Printf("🚀 Event Streaming Service starting on port %s", s.config.Port)
	log.Printf("📊 Health check: http://localhost:%s/health", s.config.Port)
	log.Printf("📈 Metrics: http://localhost:%s/metrics", s.config.Port)
	log.Printf("🔄 Kafka brokers: %v", s.config.KafkaBrokers)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	return nil
}

func (s *EventStreamingService) cleanup() {
	// Close WebSocket connections
	s.wsConnectionsMu.Lock()
	for _, conn := range s.wsConnections {
		conn.Close()
	}
	s.wsConnectionsMu.Unlock()

	// Close Kafka connections
	if s.kafkaProducer != nil {
		s.kafkaProducer.Close()
	}
	if s.kafkaConsumer != nil {
		s.kafkaConsumer.Close()
	}

	// Close NATS connection
	if s.natsConn != nil {
		s.natsConn.Close()
	}

	// Close other connections
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
func (s *EventStreamingService) healthCheck(c *gin.Context) {
	status := gin.H{
		"status":    "healthy",
		"service":   "event-streaming-service",
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

	// Check Kafka connection
	metadata, err := s.kafkaProducer.GetMetadata(nil, false, 5000)
	if err != nil {
		status["status"] = "unhealthy"
		status["kafka"] = "disconnected"
	} else {
		status["kafka"] = "connected"
		status["kafka_brokers"] = len(metadata.Brokers)
	}

	// Check NATS connection
	if s.natsConn != nil && s.natsConn.IsConnected() {
		status["nats"] = "connected"
	} else {
		status["nats"] = "disconnected"
	}

	// Add metrics
	s.wsConnectionsMu.RLock()
	status["websocket_connections"] = len(s.wsConnections)
	s.wsConnectionsMu.RUnlock()

	status["event_buffer_size"] = len(s.eventBuffer)

	if status["status"] == "unhealthy" {
		c.JSON(http.StatusServiceUnavailable, status)
		return
	}

	c.JSON(http.StatusOK, status)
}

// Event ingestion endpoint
func (s *EventStreamingService) ingestEvent(c *gin.Context) {
	var eventData map[string]interface{}
	if err := c.ShouldBindJSON(&eventData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event data"})
		return
	}

	// Create event
	event := &Event{
		ID:        uuid.New().String(),
		Type:      getString(eventData, "type", EventTypeSystemEvent),
		Source:    getString(eventData, "source", "unknown"),
		Subject:   getString(eventData, "subject", ""),
		Priority:  getString(eventData, "priority", PriorityNormal),
		Data:      getMap(eventData, "data"),
		Metadata:  getMap(eventData, "metadata"),
		UserID:    getString(eventData, "user_id", ""),
		SessionID: getString(eventData, "session_id", ""),
		TraceID:   getString(eventData, "trace_id", ""),
		SpanID:    getString(eventData, "span_id", ""),
		Timestamp: time.Now().UTC(),
		CreatedAt: time.Now().UTC(),
	}

	// Validate event
	if err := s.validateEvent(event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Add to buffer for processing
	select {
	case s.eventBuffer <- event:
		// Update metrics
		eventsIngested.WithLabelValues(event.Type, event.Source, event.Priority).Inc()
		eventBufferSize.Set(float64(len(s.eventBuffer)))

		c.JSON(http.StatusAccepted, gin.H{
			"event_id": event.ID,
			"status":   "accepted",
		})
	default:
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Event buffer full, please try again later",
		})
	}
}

// Batch event ingestion
func (s *EventStreamingService) ingestBatchEvents(c *gin.Context) {
	var batchData struct {
		Events []map[string]interface{} `json:"events"`
	}

	if err := c.ShouldBindJSON(&batchData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid batch data"})
		return
	}

	if len(batchData.Events) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No events provided"})
		return
	}

	if len(batchData.Events) > s.config.BatchSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":     "Batch size too large",
			"max_size":  s.config.BatchSize,
			"provided":  len(batchData.Events),
		})
		return
	}

	var events []*Event
	var eventIDs []string

	for _, eventData := range batchData.Events {
		event := &Event{
			ID:        uuid.New().String(),
			Type:      getString(eventData, "type", EventTypeSystemEvent),
			Source:    getString(eventData, "source", "unknown"),
			Subject:   getString(eventData, "subject", ""),
			Priority:  getString(eventData, "priority", PriorityNormal),
			Data:      getMap(eventData, "data"),
			Metadata:  getMap(eventData, "metadata"),
			UserID:    getString(eventData, "user_id", ""),
			SessionID: getString(eventData, "session_id", ""),
			TraceID:   getString(eventData, "trace_id", ""),
			SpanID:    getString(eventData, "span_id", ""),
			Timestamp: time.Now().UTC(),
			CreatedAt: time.Now().UTC(),
		}

		if err := s.validateEvent(event); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":    "Invalid event in batch",
				"event_id": event.ID,
				"details":  err.Error(),
			})
			return
		}

		events = append(events, event)
		eventIDs = append(eventIDs, event.ID)
	}

	// Add events to buffer
	accepted := 0
	for _, event := range events {
		select {
		case s.eventBuffer <- event:
			accepted++
			eventsIngested.WithLabelValues(event.Type, event.Source, event.Priority).Inc()
		default:
			break
		}
	}

	eventBufferSize.Set(float64(len(s.eventBuffer)))

	c.JSON(http.StatusAccepted, gin.H{
		"total_events":    len(events),
		"accepted_events": accepted,
		"event_ids":       eventIDs[:accepted],
		"status":          "accepted",
	})
}

// WebSocket handler for real-time event streaming
func (s *EventStreamingService) handleWebSocket(c *gin.Context) {
	streamID := c.Param("stream_id")
	
	// Upgrade connection
	conn, err := s.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	// Add connection to map
	connectionID := uuid.New().String()
	s.wsConnectionsMu.Lock()
	s.wsConnections[connectionID] = conn
	s.wsConnectionsMu.Unlock()

	// Update metrics
	wsConnections.Inc()

	// Remove connection on exit
	defer func() {
		s.wsConnectionsMu.Lock()
		delete(s.wsConnections, connectionID)
		s.wsConnectionsMu.Unlock()
		wsConnections.Dec()
	}()

	// Get stream configuration
	var stream EventStream
	if err := s.db.First(&stream, "id = ? AND is_active = true", streamID).Error; err != nil {
		conn.WriteJSON(map[string]interface{}{
			"error": "Stream not found or inactive",
		})
		return
	}

	// Send confirmation
	conn.WriteJSON(map[string]interface{}{
		"type":      "connection_established",
		"stream_id": streamID,
		"timestamp": time.Now().UTC(),
	})

	// Handle incoming messages and keep connection alive
	for {
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			log.Printf("WebSocket read error: %v", err)
			break
		}

		// Handle ping/pong
		if msgType, ok := msg["type"].(string); ok && msgType == "ping" {
			conn.WriteJSON(map[string]interface{}{
				"type":      "pong",
				"timestamp": time.Now().UTC(),
			})
		}
	}
}

// Utility functions
func getString(data map[string]interface{}, key, defaultValue string) string {
	if value, ok := data[key].(string); ok {
		return value
	}
	return defaultValue
}

func getMap(data map[string]interface{}, key string) map[string]interface{} {
	if value, ok := data[key].(map[string]interface{}); ok {
		return value
	}
	return make(map[string]interface{})
}

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
