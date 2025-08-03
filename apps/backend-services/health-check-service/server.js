/**
 * Health Check Service - Node.js
 * System health monitoring and service discovery for the 002AIC platform
 * Handles health checks, service status monitoring, and system diagnostics
 */

const express = require('express');
const cors = require('cors');
const helmet = require('helmet');
const axios = require('axios');
const { v4: uuidv4 } = require('uuid');
const Redis = require('redis');
const { Pool } = require('pg');
const prometheus = require('prom-client');
const cron = require('node-cron');

// Initialize Express app
const app = express();
const PORT = process.env.PORT || 8080;

// Configuration
const config = {
    port: PORT,
    databaseUrl: process.env.DATABASE_URL || 'postgresql://postgres:password@localhost:5432/health_checks',
    redisUrl: process.env.REDIS_URL || 'redis://localhost:6379',
    checkInterval: parseInt(process.env.CHECK_INTERVAL) || 30, // seconds
    timeout: parseInt(process.env.TIMEOUT) || 5000, // ms
    retries: parseInt(process.env.RETRIES) || 3,
    environment: process.env.ENVIRONMENT || 'development'
};

// Initialize external services
const redis = Redis.createClient({ url: config.redisUrl });
const pgPool = new Pool({ connectionString: config.databaseUrl });

// Prometheus metrics
const register = new prometheus.Registry();
prometheus.collectDefaultMetrics({ register });

const healthChecksTotal = new prometheus.Counter({
    name: 'health_checks_total',
    help: 'Total health checks performed',
    labelNames: ['service', 'status'],
    registers: [register]
});

const serviceStatus = new prometheus.Gauge({
    name: 'service_status',
    help: 'Service health status (1=healthy, 0=unhealthy)',
    labelNames: ['service', 'endpoint'],
    registers: [register]
});

const responseTime = new prometheus.Histogram({
    name: 'health_check_response_time_seconds',
    help: 'Health check response time',
    labelNames: ['service'],
    registers: [register]
});

// Service registry
const services = new Map();
const healthStatus = new Map();

// Default services to monitor (from our 36-service architecture)
const DEFAULT_SERVICES = [
    { name: 'configuration-service', url: 'http://localhost:8001/health', critical: true },
    { name: 'service-discovery', url: 'http://localhost:8002/health', critical: true },
    { name: 'user-management-service', url: 'http://localhost:8003/health', critical: true },
    { name: 'data-management-service', url: 'http://localhost:8004/health', critical: true },
    { name: 'analytics-service', url: 'http://localhost:8005/health', critical: false },
    { name: 'runtime-management-service', url: 'http://localhost:8006/health', critical: true },
    { name: 'notification-service', url: 'http://localhost:8007/health', critical: false },
    { name: 'audit-service', url: 'http://localhost:8008/health', critical: true },
    { name: 'agent-orchestration-service', url: 'http://localhost:8009/health', critical: false },
    { name: 'model-training-service', url: 'http://localhost:8010/health', critical: false },
    { name: 'model-tuning-service', url: 'http://localhost:8011/health', critical: false },
    { name: 'model-deployment-service', url: 'http://localhost:8012/health', critical: false },
    { name: 'data-integration-service', url: 'http://localhost:8013/health', critical: false },
    { name: 'pipeline-execution-service', url: 'http://localhost:8014/health', critical: false },
    { name: 'monitoring-service', url: 'http://localhost:8015/health', critical: true },
    { name: 'file-storage-service', url: 'http://localhost:8016/health', critical: false },
    { name: 'search-service', url: 'http://localhost:8017/health', critical: false },
    { name: 'billing-service', url: 'http://localhost:8018/health', critical: true },
    { name: 'workflow-service', url: 'http://localhost:8019/health', critical: false },
    { name: 'integration-service', url: 'http://localhost:8020/health', critical: false },
    { name: 'deployment-service', url: 'http://localhost:8021/health', critical: true },
    { name: 'support-service', url: 'http://localhost:8022/health', critical: false },
    { name: 'marketplace-service', url: 'http://localhost:8023/health', critical: false },
    { name: 'model-registry-service', url: 'http://localhost:8024/health', critical: false },
    { name: 'api-gateway-service', url: 'http://localhost:8025/health', critical: true },
    { name: 'event-streaming-service', url: 'http://localhost:8026/health', critical: false },
    { name: 'recommendation-engine-service', url: 'http://localhost:8027/health', critical: false },
    { name: 'content-management-service', url: 'http://localhost:8028/health', critical: false },
    { name: 'security-service', url: 'http://localhost:8029/health', critical: true },
    { name: 'backup-service', url: 'http://localhost:8030/health', critical: true },
    { name: 'logging-service', url: 'http://localhost:8031/health', critical: true },
    { name: 'feature-flag-service', url: 'http://localhost:8032/health', critical: false },
    { name: 'caching-service', url: 'http://localhost:8033/health', critical: true },
    { name: 'queue-service', url: 'http://localhost:8034/health', critical: true },
    { name: 'metrics-service', url: 'http://localhost:8035/health', critical: true }
];

// Middleware
app.use(helmet());
app.use(cors());
app.use(express.json());

// Health check endpoint for this service
app.get('/health', async (req, res) => {
    const status = {
        status: 'healthy',
        service: 'health-check-service',
        timestamp: new Date().toISOString(),
        version: '1.0.0'
    };

    try {
        await pgPool.query('SELECT 1');
        status.database = 'connected';
    } catch (error) {
        status.status = 'unhealthy';
        status.database = 'disconnected';
    }

    try {
        await redis.ping();
        status.redis = 'connected';
    } catch (error) {
        status.status = 'unhealthy';
        status.redis = 'disconnected';
    }

    status.monitored_services = services.size;
    status.healthy_services = Array.from(healthStatus.values()).filter(s => s.status === 'healthy').length;

    const statusCode = status.status === 'healthy' ? 200 : 503;
    res.status(statusCode).json(status);
});

// Metrics endpoint
app.get('/metrics', (req, res) => {
    res.set('Content-Type', register.contentType);
    res.end(register.metrics());
});

// Service registration
app.post('/v1/services', async (req, res) => {
    try {
        const { name, url, critical = false, interval = 30, timeout = 5000, metadata = {} } = req.body;

        if (!name || !url) {
            return res.status(400).json({ error: 'Name and URL are required' });
        }

        const serviceId = uuidv4();
        const service = {
            id: serviceId,
            name,
            url,
            critical,
            interval,
            timeout,
            metadata,
            registered_at: new Date(),
            last_check: null,
            status: 'unknown'
        };

        services.set(serviceId, service);
        await storeService(service);

        res.status(201).json({
            service_id: serviceId,
            message: 'Service registered successfully'
        });
    } catch (error) {
        console.error('Error registering service:', error);
        res.status(500).json({ error: 'Failed to register service' });
    }
});

// Get all services and their health status
app.get('/v1/services', (req, res) => {
    const servicesArray = Array.from(services.values()).map(service => ({
        ...service,
        health_status: healthStatus.get(service.id) || { status: 'unknown' }
    }));

    res.json({
        services: servicesArray,
        total: servicesArray.length,
        healthy: servicesArray.filter(s => s.health_status.status === 'healthy').length,
        unhealthy: servicesArray.filter(s => s.health_status.status === 'unhealthy').length
    });
});

// Get system overview
app.get('/v1/system/overview', (req, res) => {
    const overview = {
        timestamp: new Date().toISOString(),
        total_services: services.size,
        healthy_services: 0,
        unhealthy_services: 0,
        critical_services_down: 0,
        system_status: 'healthy'
    };

    for (const [serviceId, service] of services) {
        const health = healthStatus.get(serviceId);
        if (health) {
            if (health.status === 'healthy') {
                overview.healthy_services++;
            } else {
                overview.unhealthy_services++;
                if (service.critical) {
                    overview.critical_services_down++;
                }
            }
        }
    }

    // Determine overall system status
    if (overview.critical_services_down > 0) {
        overview.system_status = 'critical';
    } else if (overview.unhealthy_services > 0) {
        overview.system_status = 'degraded';
    }

    res.json(overview);
});

// Perform health check on specific service
app.post('/v1/services/:id/check', async (req, res) => {
    try {
        const { id } = req.params;
        const service = services.get(id);

        if (!service) {
            return res.status(404).json({ error: 'Service not found' });
        }

        const result = await performHealthCheck(service);
        res.json(result);
    } catch (error) {
        console.error('Error performing health check:', error);
        res.status(500).json({ error: 'Failed to perform health check' });
    }
});

// Health check functions
async function performHealthCheck(service) {
    const startTime = Date.now();
    let result = {
        service_id: service.id,
        service_name: service.name,
        url: service.url,
        status: 'unhealthy',
        response_time: 0,
        error: null,
        timestamp: new Date().toISOString()
    };

    try {
        const response = await axios.get(service.url, {
            timeout: service.timeout || config.timeout,
            validateStatus: (status) => status < 500 // Accept 4xx as healthy
        });

        result.status = 'healthy';
        result.response_time = Date.now() - startTime;
        result.http_status = response.status;
        result.response_data = response.data;

        // Update metrics
        serviceStatus.labels(service.name, service.url).set(1);
        healthChecksTotal.labels(service.name, 'success').inc();
        responseTime.labels(service.name).observe(result.response_time / 1000);

    } catch (error) {
        result.error = error.message;
        result.response_time = Date.now() - startTime;

        // Update metrics
        serviceStatus.labels(service.name, service.url).set(0);
        healthChecksTotal.labels(service.name, 'failure').inc();
        responseTime.labels(service.name).observe(result.response_time / 1000);
    }

    // Store result
    healthStatus.set(service.id, result);
    service.last_check = new Date();

    return result;
}

// Background health checking
async function runHealthChecks() {
    console.log(`Running health checks for ${services.size} services...`);
    
    const promises = Array.from(services.values()).map(service => 
        performHealthCheck(service).catch(error => {
            console.error(`Health check failed for ${service.name}:`, error);
            return {
                service_id: service.id,
                service_name: service.name,
                status: 'unhealthy',
                error: error.message,
                timestamp: new Date().toISOString()
            };
        })
    );

    const results = await Promise.all(promises);
    
    // Log critical service failures
    results.forEach(result => {
        const service = services.get(result.service_id);
        if (service && service.critical && result.status === 'unhealthy') {
            console.error(`CRITICAL SERVICE DOWN: ${service.name} - ${result.error}`);
        }
    });

    return results;
}

// Database functions
async function storeService(service) {
    const query = `
        INSERT INTO services (id, name, url, critical, interval_seconds, timeout_ms, metadata, registered_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        ON CONFLICT (id) DO UPDATE SET
        name = $2, url = $3, critical = $4, interval_seconds = $5, timeout_ms = $6, metadata = $7
    `;

    await pgPool.query(query, [
        service.id,
        service.name,
        service.url,
        service.critical,
        service.interval,
        service.timeout,
        JSON.stringify(service.metadata),
        service.registered_at
    ]);
}

// Initialize database
async function initializeDatabase() {
    const createTables = `
        CREATE TABLE IF NOT EXISTS services (
            id UUID PRIMARY KEY,
            name VARCHAR(255) NOT NULL,
            url VARCHAR(500) NOT NULL,
            critical BOOLEAN DEFAULT FALSE,
            interval_seconds INTEGER DEFAULT 30,
            timeout_ms INTEGER DEFAULT 5000,
            metadata JSONB DEFAULT '{}',
            registered_at TIMESTAMP DEFAULT NOW(),
            last_check TIMESTAMP,
            UNIQUE(name)
        );

        CREATE TABLE IF NOT EXISTS health_check_history (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            service_id UUID NOT NULL REFERENCES services(id),
            status VARCHAR(20) NOT NULL,
            response_time_ms INTEGER,
            http_status INTEGER,
            error_message TEXT,
            checked_at TIMESTAMP DEFAULT NOW()
        );

        CREATE INDEX IF NOT EXISTS idx_services_name ON services(name);
        CREATE INDEX IF NOT EXISTS idx_health_history_service_id ON health_check_history(service_id);
        CREATE INDEX IF NOT EXISTS idx_health_history_checked_at ON health_check_history(checked_at);
    `;

    await pgPool.query(createTables);
}

// Initialize default services
async function initializeDefaultServices() {
    for (const serviceConfig of DEFAULT_SERVICES) {
        const serviceId = uuidv4();
        const service = {
            id: serviceId,
            name: serviceConfig.name,
            url: serviceConfig.url,
            critical: serviceConfig.critical,
            interval: config.checkInterval,
            timeout: config.timeout,
            metadata: {},
            registered_at: new Date(),
            last_check: null,
            status: 'unknown'
        };

        services.set(serviceId, service);
        await storeService(service);
    }

    console.log(`Initialized ${DEFAULT_SERVICES.length} default services`);
}

// Start server
async function startServer() {
    try {
        // Initialize database
        await initializeDatabase();
        
        // Connect to Redis
        await redis.connect();
        
        // Initialize default services
        await initializeDefaultServices();
        
        // Start health check scheduler
        cron.schedule(`*/${config.checkInterval} * * * * *`, runHealthChecks);
        
        // Start server
        app.listen(PORT, () => {
            console.log(`🚀 Health Check Service running on port ${PORT}`);
            console.log(`📊 Health check: http://localhost:${PORT}/health`);
            console.log(`📈 Metrics: http://localhost:${PORT}/metrics`);
            console.log(`🔍 System overview: http://localhost:${PORT}/v1/system/overview`);
            console.log(`⏱️  Check interval: ${config.checkInterval} seconds`);
        });
    } catch (error) {
        console.error('Failed to start server:', error);
        process.exit(1);
    }
}

// Graceful shutdown
process.on('SIGTERM', async () => {
    console.log('Shutting down health check service...');
    await redis.quit();
    await pgPool.end();
    process.exit(0);
});

startServer();
