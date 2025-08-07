const express = require('express');
const cors = require('cors');
const helmet = require('helmet');
const compression = require('compression');
const morgan = require('morgan');
const rateLimit = require('express-rate-limit');
const slowDown = require('express-slow-down');
const { createProxyMiddleware } = require('http-proxy-middleware');
const jwt = require('jsonwebtoken');
const redis = require('redis');
const NodeCache = require('node-cache');
const winston = require('winston');
const promClient = require('prom-client');
const axios = require('axios');
const CircuitBreaker = require('circuit-breaker-js');
require('dotenv').config();

const app = express();
const PORT = process.env.PORT || 3000;

// Initialize Redis client
let redisClient;
if (process.env.REDIS_URL) {
  try {
    redisClient = redis.createClient({
      url: process.env.REDIS_URL
    });
    redisClient.connect().catch(error => {
      console.warn('Redis connection failed, using in-memory cache:', error.message);
      redisClient = null;
    });
  } catch (error) {
    console.warn('Redis not available, using in-memory cache:', error.message);
    redisClient = null;
  }
} else {
  console.info('No Redis URL provided, using in-memory cache');
}

// Initialize in-memory cache as fallback
const cache = new NodeCache({ stdTTL: 300 }); // 5 minutes default TTL

// Configure Winston logger
const logger = winston.createLogger({
  level: 'info',
  format: winston.format.combine(
    winston.format.timestamp(),
    winston.format.errors({ stack: true }),
    winston.format.json()
  ),
  transports: [
    new winston.transports.Console(),
    new winston.transports.File({ filename: 'logs/gateway.log' })
  ]
});

// Prometheus metrics
const register = new promClient.Registry();
promClient.collectDefaultMetrics({ register });

const httpRequestsTotal = new promClient.Counter({
  name: 'http_requests_total',
  help: 'Total number of HTTP requests',
  labelNames: ['method', 'route', 'status_code', 'service'],
  registers: [register]
});

const httpRequestDuration = new promClient.Histogram({
  name: 'http_request_duration_seconds',
  help: 'Duration of HTTP requests in seconds',
  labelNames: ['method', 'route', 'service'],
  buckets: [0.1, 0.5, 1, 2, 5],
  registers: [register]
});

const activeConnections = new promClient.Gauge({
  name: 'active_connections',
  help: 'Number of active connections',
  registers: [register]
});

// Service registry with health checking
class ServiceRegistry {
  constructor() {
    this.services = new Map();
    this.circuitBreakers = new Map();
    this.healthCheckInterval = 30000; // 30 seconds
    this.startHealthChecks();
  }

  register(name, config) {
    this.services.set(name, {
      ...config,
      healthy: true,
      lastHealthCheck: Date.now(),
      failureCount: 0
    });

    // Create circuit breaker for service
    const breaker = new CircuitBreaker({
      timeout: config.timeout || 5000,
      maxFailures: config.maxFailures || 5,
      resetTimeout: config.resetTimeout || 60000
    });
    this.circuitBreakers.set(name, breaker);

    logger.info(`Service registered: ${name}`, config);
  }

  getService(name) {
    return this.services.get(name);
  }

  getAllServices() {
    return Array.from(this.services.entries()).map(([name, config]) => ({
      name,
      ...config
    }));
  }

  async healthCheck(name, service) {
    try {
      const response = await axios.get(`${service.url}/health`, {
        timeout: 5000
      });
      
      if (response.status === 200) {
        service.healthy = true;
        service.failureCount = 0;
        service.lastHealthCheck = Date.now();
        logger.debug(`Health check passed for ${name}`);
      }
    } catch (error) {
      service.healthy = false;
      service.failureCount++;
      service.lastHealthCheck = Date.now();
      logger.warn(`Health check failed for ${name}:`, error.message);
    }
  }

  startHealthChecks() {
    setInterval(() => {
      for (const [name, service] of this.services) {
        this.healthCheck(name, service);
      }
    }, this.healthCheckInterval);
  }
}

const serviceRegistry = new ServiceRegistry();

// Register default services
serviceRegistry.register('auth', {
  url: process.env.AUTH_SERVICE_URL || 'http://localhost:3001',
  path: '/api/auth',
  timeout: 5000
});

serviceRegistry.register('config', {
  url: process.env.CONFIG_SERVICE_URL || 'http://localhost:3002',
  path: '/api/v1/config',
  timeout: 5000
});

serviceRegistry.register('notification', {
  url: process.env.NOTIFICATION_SERVICE_URL || 'http://localhost:3003',
  path: '/api/v1/notifications',
  timeout: 5000
});

serviceRegistry.register('user-management', {
  url: process.env.USER_MANAGEMENT_SERVICE_URL || 'http://localhost:3004',
  path: '/api/users',
  timeout: 10000
});

// Middleware setup
app.use(helmet());
app.use(compression());
app.use(cors({
  origin: process.env.ALLOWED_ORIGINS?.split(',') || ['http://localhost:3000'],
  credentials: true
}));

// Custom Morgan format for structured logging
morgan.token('user-id', (req) => req.user?.id || 'anonymous');
morgan.token('request-id', (req) => req.requestId);

app.use(morgan(':method :url :status :res[content-length] - :response-time ms :user-id :request-id', {
  stream: {
    write: (message) => logger.info(message.trim())
  }
}));

// Request ID middleware
app.use((req, res, next) => {
  req.requestId = require('uuid').v4();
  res.setHeader('X-Request-ID', req.requestId);
  next();
});

// Connection tracking
app.use((req, res, next) => {
  activeConnections.inc();
  res.on('finish', () => {
    activeConnections.dec();
  });
  next();
});

// Rate limiting
const limiter = rateLimit({
  windowMs: 15 * 60 * 1000, // 15 minutes
  max: process.env.RATE_LIMIT_MAX || 1000,
  message: {
    error: 'Too many requests from this IP',
    retryAfter: '15 minutes'
  },
  standardHeaders: true,
  legacyHeaders: false,
  skip: (req) => {
    // Skip rate limiting for health checks
    return req.path === '/health' || req.path === '/metrics';
  }
});

// Slow down middleware for additional protection
const speedLimiter = slowDown({
  windowMs: 15 * 60 * 1000, // 15 minutes
  delayAfter: 100, // allow 100 requests per 15 minutes at full speed
  delayMs: 500 // slow down subsequent requests by 500ms per request
});

app.use(limiter);
app.use(speedLimiter);

// Body parsing
app.use(express.json({ limit: '10mb' }));
app.use(express.urlencoded({ extended: true, limit: '10mb' }));

// JWT Authentication middleware
const authenticateToken = async (req, res, next) => {
  const authHeader = req.headers['authorization'];
  const token = authHeader && authHeader.split(' ')[1];

  if (!token) {
    return res.status(401).json({ error: 'Access token required' });
  }

  try {
    // Check cache first
    const cacheKey = `token:${token}`;
    let decoded;

    if (redisClient) {
      const cached = await redisClient.get(cacheKey);
      if (cached) {
        decoded = JSON.parse(cached);
      }
    } else {
      decoded = cache.get(cacheKey);
    }

    if (!decoded) {
      decoded = jwt.verify(token, process.env.JWT_SECRET || 'nexus-secret');
      
      // Cache the decoded token
      if (redisClient) {
        await redisClient.setEx(cacheKey, 300, JSON.stringify(decoded));
      } else {
        cache.set(cacheKey, decoded, 300);
      }
    }

    req.user = decoded;
    next();
  } catch (error) {
    logger.warn('Token verification failed:', error.message);
    return res.status(403).json({ error: 'Invalid or expired token' });
  }
};

// Metrics middleware
app.use((req, res, next) => {
  const start = Date.now();
  
  res.on('finish', () => {
    const duration = (Date.now() - start) / 1000;
    const service = req.route?.path?.split('/')[2] || 'unknown';
    
    httpRequestsTotal.inc({
      method: req.method,
      route: req.route?.path || req.path,
      status_code: res.statusCode,
      service
    });
    
    httpRequestDuration.observe({
      method: req.method,
      route: req.route?.path || req.path,
      service
    }, duration);
  });
  
  next();
});

// Health check endpoint
app.get('/health', (req, res) => {
  const services = serviceRegistry.getAllServices();
  const healthyServices = services.filter(s => s.healthy).length;
  const totalServices = services.length;
  
  const health = {
    status: healthyServices === totalServices ? 'healthy' : 'degraded',
    timestamp: new Date().toISOString(),
    uptime: process.uptime(),
    services: {
      total: totalServices,
      healthy: healthyServices,
      unhealthy: totalServices - healthyServices
    },
    memory: process.memoryUsage(),
    version: process.env.npm_package_version || '1.0.0'
  };

  const statusCode = health.status === 'healthy' ? 200 : 503;
  res.status(statusCode).json(health);
});

// Metrics endpoint
app.get('/metrics', async (req, res) => {
  res.set('Content-Type', register.contentType);
  res.end(await register.metrics());
});

// Service discovery endpoint
app.get('/api/services', authenticateToken, (req, res) => {
  const services = serviceRegistry.getAllServices();
  res.json({
    services: services.map(service => ({
      name: service.name,
      healthy: service.healthy,
      lastHealthCheck: service.lastHealthCheck,
      path: service.path
    }))
  });
});

// Dynamic proxy middleware with circuit breaker
const createServiceProxy = (serviceName) => {
  return async (req, res, next) => {
    const service = serviceRegistry.getService(serviceName);
    
    if (!service) {
      return res.status(404).json({ error: `Service ${serviceName} not found` });
    }

    if (!service.healthy) {
      return res.status(503).json({ 
        error: `Service ${serviceName} is currently unavailable`,
        retryAfter: 30
      });
    }

    const breaker = serviceRegistry.circuitBreakers.get(serviceName);
    
    try {
      await breaker.run(async () => {
        const proxy = createProxyMiddleware({
          target: service.url,
          changeOrigin: true,
          pathRewrite: {
            [`^/api/${serviceName}`]: service.path
          },
          timeout: service.timeout,
          onError: (err, req, res) => {
            logger.error(`Proxy error for ${serviceName}:`, err.message);
            if (!res.headersSent) {
              res.status(502).json({ 
                error: 'Bad Gateway',
                service: serviceName,
                requestId: req.requestId
              });
            }
          },
          onProxyReq: (proxyReq, req) => {
            // Add request tracking headers
            proxyReq.setHeader('X-Request-ID', req.requestId);
            proxyReq.setHeader('X-Forwarded-For', req.ip);
            if (req.user) {
              proxyReq.setHeader('X-User-ID', req.user.id);
            }
          }
        });

        return new Promise((resolve, reject) => {
          proxy(req, res, (err) => {
            if (err) reject(err);
            else resolve();
          });
        });
      });
    } catch (error) {
      logger.error(`Circuit breaker open for ${serviceName}:`, error.message);
      res.status(503).json({
        error: `Service ${serviceName} is temporarily unavailable`,
        retryAfter: 60
      });
    }
  };
};

// Route definitions with authentication
app.use('/api/auth', createServiceProxy('auth'));
app.use('/api/config', authenticateToken, createServiceProxy('config'));
app.use('/api/notifications', authenticateToken, createServiceProxy('notification'));
app.use('/api/users', authenticateToken, createServiceProxy('user-management'));

// API documentation endpoint
app.get('/api/docs', (req, res) => {
  res.json({
    name: 'Nexus Platform API Gateway',
    version: '1.0.0',
    description: 'Enhanced API Gateway with routing, authentication, and monitoring',
    endpoints: {
      '/health': 'Health check endpoint',
      '/metrics': 'Prometheus metrics',
      '/api/services': 'Service discovery (requires auth)',
      '/api/auth/*': 'Authentication service proxy',
      '/api/config/*': 'Configuration service proxy (requires auth)',
      '/api/notifications/*': 'Notification service proxy (requires auth)',
      '/api/users/*': 'User management service proxy (requires auth)'
    },
    features: [
      'JWT Authentication',
      'Rate Limiting',
      'Circuit Breaker Pattern',
      'Service Health Monitoring',
      'Request/Response Logging',
      'Prometheus Metrics',
      'Redis Caching',
      'CORS Support',
      'Request ID Tracking'
    ]
  });
});

// Catch-all route for undefined endpoints
app.use('*', (req, res) => {
  res.status(404).json({
    error: 'Endpoint not found',
    path: req.originalUrl,
    method: req.method,
    requestId: req.requestId,
    availableEndpoints: [
      '/health',
      '/metrics',
      '/api/docs',
      '/api/services',
      '/api/auth/*',
      '/api/config/*',
      '/api/notifications/*',
      '/api/users/*'
    ]
  });
});

// Global error handler
app.use((error, req, res, next) => {
  logger.error('Unhandled error:', {
    error: error.message,
    stack: error.stack,
    requestId: req.requestId,
    url: req.url,
    method: req.method
  });

  res.status(500).json({
    error: 'Internal server error',
    requestId: req.requestId,
    timestamp: new Date().toISOString()
  });
});

// Graceful shutdown
process.on('SIGTERM', async () => {
  logger.info('SIGTERM received, shutting down gracefully');
  
  if (redisClient) {
    await redisClient.quit();
  }
  
  process.exit(0);
});

process.on('SIGINT', async () => {
  logger.info('SIGINT received, shutting down gracefully');
  
  if (redisClient) {
    await redisClient.quit();
  }
  
  process.exit(0);
});

// Start server
app.listen(PORT, () => {
  logger.info(`API Gateway Service started on port ${PORT}`, {
    environment: process.env.NODE_ENV || 'development',
    services: serviceRegistry.getAllServices().length
  });
});

module.exports = app;
