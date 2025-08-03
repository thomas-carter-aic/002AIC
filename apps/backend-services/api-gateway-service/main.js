/**
 * API Gateway Service - Node.js
 * Central entry point for all 002AIC platform APIs
 * Handles routing, authentication, rate limiting, and request transformation
 */

const express = require('express');
const { createProxyMiddleware } = require('http-proxy-middleware');
const rateLimit = require('express-rate-limit');
const helmet = require('helmet');
const cors = require('cors');
const { AuthMiddleware, AuthConfig } = require('../../../libs/auth-middleware/nodejs/auth');

const app = express();
const PORT = process.env.PORT || 8080;

// Security middleware
app.use(helmet());
app.use(cors({
  origin: process.env.ALLOWED_ORIGINS?.split(',') || ['http://localhost:3000'],
  credentials: true
}));

// Rate limiting
const limiter = rateLimit({
  windowMs: 15 * 60 * 1000, // 15 minutes
  max: 1000, // limit each IP to 1000 requests per windowMs
  message: 'Too many requests from this IP, please try again later.'
});
app.use(limiter);

// Body parsing
app.use(express.json({ limit: '10mb' }));
app.use(express.urlencoded({ extended: true }));

// Initialize auth middleware
const authConfig = new AuthConfig({
  authorizationServiceUrl: process.env.AUTHORIZATION_SERVICE_URL || 'http://authorization-service:8080',
  jwtPublicKeyUrl: process.env.JWT_PUBLIC_KEY_URL || 'http://keycloak:8080/realms/002aic/protocol/openid-connect/certs',
  jwtIssuer: process.env.JWT_ISSUER || 'http://keycloak:8080/realms/002aic',
  jwtAudience: process.env.JWT_AUDIENCE || '002aic-api',
  serviceName: 'api-gateway-service'
});

const auth = new AuthMiddleware(authConfig);

// Health check endpoint
app.get('/health', (req, res) => {
  res.json({ 
    status: 'healthy', 
    service: 'api-gateway-service',
    timestamp: new Date().toISOString(),
    version: process.env.SERVICE_VERSION || '1.0.0'
  });
});

// Service discovery endpoint
app.get('/services', (req, res) => {
  res.json({
    services: [
      { name: 'authentication-authorization-service', url: '/v1/auth', status: 'active' },
      { name: 'model-management-service', url: '/v1/models', status: 'active' },
      { name: 'data-management-service', url: '/v1/data', status: 'active' },
      { name: 'analytics-service', url: '/v1/analytics', status: 'active' },
      // Add all 36 services here
    ]
  });
});

// Microservice proxy configurations
const services = {
  // Core Services
  '/v1/auth': {
    target: process.env.AUTH_SERVICE_URL || 'http://authorization-service:8080',
    pathRewrite: { '^/v1/auth': '/v1/auth' }
  },
  
  // AI/ML Services
  '/v1/models': {
    target: process.env.MODEL_MANAGEMENT_SERVICE_URL || 'http://model-management-service:8080',
    pathRewrite: { '^/v1/models': '/v1/models' },
    auth: true
  },
  
  '/v1/training': {
    target: process.env.MODEL_TRAINING_SERVICE_URL || 'http://model-training-service:8080',
    pathRewrite: { '^/v1/training': '/v1/training' },
    auth: true
  },
  
  // Data Services
  '/v1/data': {
    target: process.env.DATA_MANAGEMENT_SERVICE_URL || 'http://data-management-service:8080',
    pathRewrite: { '^/v1/data': '/v1/data' },
    auth: true
  },
  
  // Analytics Services
  '/v1/analytics': {
    target: process.env.ANALYTICS_SERVICE_URL || 'http://analytics-service:8080',
    pathRewrite: { '^/v1/analytics': '/v1/analytics' },
    auth: true
  },
  
  // Add all other services...
};

// Create proxy middleware for each service
Object.entries(services).forEach(([path, config]) => {
  const middleware = [];
  
  // Add authentication if required
  if (config.auth) {
    middleware.push(auth.requireAuth('api', 'access'));
  }
  
  // Add proxy middleware
  middleware.push(createProxyMiddleware({
    target: config.target,
    changeOrigin: true,
    pathRewrite: config.pathRewrite,
    onProxyReq: (proxyReq, req, res) => {
      // Add user context headers for downstream services
      if (req.user) {
        proxyReq.setHeader('X-User-ID', req.user.userId);
        proxyReq.setHeader('X-User-Email', req.user.email);
        proxyReq.setHeader('X-User-Roles', req.user.roles.join(','));
      }
      proxyReq.setHeader('X-Gateway-Service', 'api-gateway-service');
      proxyReq.setHeader('X-Request-ID', req.headers['x-request-id'] || generateRequestId());
    },
    onError: (err, req, res) => {
      console.error(`Proxy error for ${req.url}:`, err);
      res.status(502).json({
        error: 'Bad Gateway',
        message: 'Service temporarily unavailable',
        requestId: req.headers['x-request-id']
      });
    }
  }));
  
  app.use(path, ...middleware);
});

// Request ID generator
function generateRequestId() {
  return `req_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
}

// Global error handler
app.use((err, req, res, next) => {
  console.error('Gateway error:', err);
  res.status(500).json({
    error: 'Internal Server Error',
    message: 'An unexpected error occurred',
    requestId: req.headers['x-request-id']
  });
});

// 404 handler
app.use('*', (req, res) => {
  res.status(404).json({
    error: 'Not Found',
    message: `Route ${req.originalUrl} not found`,
    availableRoutes: Object.keys(services)
  });
});

// Start server
app.listen(PORT, () => {
  console.log(`🚀 API Gateway Service running on port ${PORT}`);
  console.log(`📊 Health check: http://localhost:${PORT}/health`);
  console.log(`🔍 Services: http://localhost:${PORT}/services`);
  console.log(`🛡️  Authentication: ${authConfig.authorizationServiceUrl}`);
});

module.exports = app;
