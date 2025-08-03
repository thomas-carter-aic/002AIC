/**
 * Integration Service - Node.js
 * Third-party API management and integration hub for the 002AIC platform
 * Handles API connections, webhooks, data synchronization, and external service integrations
 */

const express = require('express');
const cors = require('cors');
const helmet = require('helmet');
const rateLimit = require('express-rate-limit');
const { v4: uuidv4 } = require('uuid');
const axios = require('axios');
const crypto = require('crypto');
const Redis = require('redis');
const { Pool } = require('pg');
const amqp = require('amqplib');
const prometheus = require('prom-client');
const cron = require('node-cron');
const jwt = require('jsonwebtoken');
const OAuth = require('oauth-1.0a');

// Initialize Express app
const app = express();
const PORT = process.env.PORT || 8080;

// Configuration
const config = {
    port: PORT,
    databaseUrl: process.env.DATABASE_URL || 'postgresql://postgres:password@localhost:5432/integrations',
    redisUrl: process.env.REDIS_URL || 'redis://localhost:6379',
    rabbitmqUrl: process.env.RABBITMQ_URL || 'amqp://localhost',
    jwtSecret: process.env.JWT_SECRET || 'your-secret-key',
    webhookSecret: process.env.WEBHOOK_SECRET || 'webhook-secret',
    environment: process.env.ENVIRONMENT || 'development',
    maxRetries: parseInt(process.env.MAX_RETRIES) || 3,
    requestTimeout: parseInt(process.env.REQUEST_TIMEOUT) || 30000,
    rateLimitWindow: parseInt(process.env.RATE_LIMIT_WINDOW) || 900000, // 15 minutes
    rateLimitMax: parseInt(process.env.RATE_LIMIT_MAX) || 1000
};

// Initialize external services
const redis = Redis.createClient({ url: config.redisUrl });
const pgPool = new Pool({ connectionString: config.databaseUrl });

// Prometheus metrics
const register = new prometheus.Registry();
prometheus.collectDefaultMetrics({ register });

const integrationsTotal = new prometheus.Gauge({
    name: 'integrations_total',
    help: 'Total number of integrations',
    labelNames: ['provider', 'status'],
    registers: [register]
});

const apiRequestsTotal = new prometheus.Counter({
    name: 'api_requests_total',
    help: 'Total number of API requests',
    labelNames: ['integration_id', 'method', 'status'],
    registers: [register]
});

const webhookEventsTotal = new prometheus.Counter({
    name: 'webhook_events_total',
    help: 'Total number of webhook events received',
    labelNames: ['provider', 'event_type'],
    registers: [register]
});

const syncOperationsTotal = new prometheus.Counter({
    name: 'sync_operations_total',
    help: 'Total number of sync operations',
    labelNames: ['integration_id', 'direction', 'status'],
    registers: [register]
});

const requestDuration = new prometheus.Histogram({
    name: 'api_request_duration_seconds',
    help: 'Duration of API requests',
    labelNames: ['integration_id', 'method'],
    registers: [register]
});

// Middleware
app.use(helmet());
app.use(cors());
app.use(express.json({ limit: '10mb' }));
app.use(express.urlencoded({ extended: true }));

// Rate limiting
const limiter = rateLimit({
    windowMs: config.rateLimitWindow,
    max: config.rateLimitMax,
    message: 'Too many requests from this IP, please try again later.'
});
app.use(limiter);

// Integration providers configuration
const PROVIDERS = {
    SALESFORCE: {
        name: 'Salesforce',
        authType: 'oauth2',
        baseUrl: 'https://api.salesforce.com',
        scopes: ['api', 'refresh_token'],
        webhookSupport: true
    },
    SLACK: {
        name: 'Slack',
        authType: 'oauth2',
        baseUrl: 'https://slack.com/api',
        scopes: ['channels:read', 'chat:write', 'users:read'],
        webhookSupport: true
    },
    GITHUB: {
        name: 'GitHub',
        authType: 'oauth2',
        baseUrl: 'https://api.github.com',
        scopes: ['repo', 'user'],
        webhookSupport: true
    },
    STRIPE: {
        name: 'Stripe',
        authType: 'api_key',
        baseUrl: 'https://api.stripe.com/v1',
        webhookSupport: true
    },
    HUBSPOT: {
        name: 'HubSpot',
        authType: 'oauth2',
        baseUrl: 'https://api.hubapi.com',
        scopes: ['contacts', 'content'],
        webhookSupport: true
    },
    ZAPIER: {
        name: 'Zapier',
        authType: 'api_key',
        baseUrl: 'https://hooks.zapier.com',
        webhookSupport: false
    },
    CUSTOM: {
        name: 'Custom API',
        authType: 'custom',
        webhookSupport: true
    }
};

// Authentication types
const AUTH_TYPES = {
    API_KEY: 'api_key',
    OAUTH2: 'oauth2',
    OAUTH1: 'oauth1',
    BASIC: 'basic',
    BEARER: 'bearer',
    CUSTOM: 'custom'
};

// Integration status
const INTEGRATION_STATUS = {
    ACTIVE: 'active',
    INACTIVE: 'inactive',
    ERROR: 'error',
    PENDING: 'pending',
    EXPIRED: 'expired'
};

// Sync directions
const SYNC_DIRECTIONS = {
    INBOUND: 'inbound',
    OUTBOUND: 'outbound',
    BIDIRECTIONAL: 'bidirectional'
};

// Storage for active connections and webhooks
const activeIntegrations = new Map();
const webhookHandlers = new Map();

// Health check endpoint
app.get('/health', async (req, res) => {
    const status = {
        status: 'healthy',
        service: 'integration-service',
        timestamp: new Date().toISOString(),
        version: '1.0.0'
    };

    try {
        // Check database
        await pgPool.query('SELECT 1');
        status.database = 'connected';
    } catch (error) {
        status.status = 'unhealthy';
        status.database = 'disconnected';
    }

    try {
        // Check Redis
        await redis.ping();
        status.redis = 'connected';
    } catch (error) {
        status.status = 'unhealthy';
        status.redis = 'disconnected';
    }

    status.active_integrations = activeIntegrations.size;
    status.webhook_handlers = webhookHandlers.size;

    const statusCode = status.status === 'healthy' ? 200 : 503;
    res.status(statusCode).json(status);
});

// Metrics endpoint
app.get('/metrics', (req, res) => {
    res.set('Content-Type', register.contentType);
    res.end(register.metrics());
});

// Integration management endpoints
app.post('/v1/integrations', async (req, res) => {
    try {
        const {
            name,
            provider,
            auth_config,
            api_config = {},
            webhook_config = {},
            sync_config = {},
            user_id,
            project_id,
            tags = []
        } = req.body;

        // Validate provider
        if (!PROVIDERS[provider]) {
            return res.status(400).json({ error: 'Invalid provider' });
        }

        const integrationId = uuidv4();
        const now = new Date();

        // Create integration record
        const integration = {
            id: integrationId,
            name,
            provider,
            auth_config: JSON.stringify(auth_config),
            api_config: JSON.stringify(api_config),
            webhook_config: JSON.stringify(webhook_config),
            sync_config: JSON.stringify(sync_config),
            status: INTEGRATION_STATUS.PENDING,
            user_id,
            project_id,
            tags,
            created_at: now,
            updated_at: now,
            last_sync_at: null,
            error_message: null
        };

        // Store in database
        await storeIntegration(integration);

        // Initialize integration
        await initializeIntegration(integrationId);

        // Update metrics
        integrationsTotal.labels(provider, INTEGRATION_STATUS.PENDING).inc();

        res.status(201).json({
            integration_id: integrationId,
            status: INTEGRATION_STATUS.PENDING,
            message: 'Integration created successfully'
        });

    } catch (error) {
        console.error('Error creating integration:', error);
        res.status(500).json({ error: 'Failed to create integration' });
    }
});

app.get('/v1/integrations/:id', async (req, res) => {
    try {
        const { id } = req.params;
        const integration = await getIntegration(id);

        if (!integration) {
            return res.status(404).json({ error: 'Integration not found' });
        }

        res.json(integration);
    } catch (error) {
        console.error('Error getting integration:', error);
        res.status(500).json({ error: 'Failed to get integration' });
    }
});

app.get('/v1/integrations', async (req, res) => {
    try {
        const {
            provider,
            status,
            user_id,
            project_id,
            limit = 50,
            offset = 0
        } = req.query;

        const integrations = await listIntegrations({
            provider,
            status,
            user_id,
            project_id,
            limit: parseInt(limit),
            offset: parseInt(offset)
        });

        res.json({
            integrations,
            total: integrations.length,
            limit: parseInt(limit),
            offset: parseInt(offset)
        });
    } catch (error) {
        console.error('Error listing integrations:', error);
        res.status(500).json({ error: 'Failed to list integrations' });
    }
});

app.put('/v1/integrations/:id', async (req, res) => {
    try {
        const { id } = req.params;
        const updates = req.body;

        const integration = await updateIntegration(id, updates);

        if (!integration) {
            return res.status(404).json({ error: 'Integration not found' });
        }

        // Reinitialize if configuration changed
        if (updates.auth_config || updates.api_config) {
            await initializeIntegration(id);
        }

        res.json({
            message: 'Integration updated successfully',
            integration
        });
    } catch (error) {
        console.error('Error updating integration:', error);
        res.status(500).json({ error: 'Failed to update integration' });
    }
});

app.delete('/v1/integrations/:id', async (req, res) => {
    try {
        const { id } = req.params;

        // Remove from active integrations
        activeIntegrations.delete(id);

        // Delete from database
        await deleteIntegration(id);

        res.json({ message: 'Integration deleted successfully' });
    } catch (error) {
        console.error('Error deleting integration:', error);
        res.status(500).json({ error: 'Failed to delete integration' });
    }
});

// API request endpoints
app.post('/v1/integrations/:id/request', async (req, res) => {
    try {
        const { id } = req.params;
        const { method, endpoint, data, headers = {} } = req.body;

        const integration = activeIntegrations.get(id);
        if (!integration) {
            return res.status(404).json({ error: 'Integration not found or inactive' });
        }

        const startTime = Date.now();

        // Make API request
        const response = await makeApiRequest(integration, {
            method,
            endpoint,
            data,
            headers
        });

        // Update metrics
        const duration = (Date.now() - startTime) / 1000;
        requestDuration.labels(id, method.toUpperCase()).observe(duration);
        apiRequestsTotal.labels(id, method.toUpperCase(), 'success').inc();

        res.json({
            status: 'success',
            data: response.data,
            headers: response.headers,
            status_code: response.status,
            duration_ms: Date.now() - startTime
        });

    } catch (error) {
        apiRequestsTotal.labels(req.params.id, req.body.method?.toUpperCase() || 'UNKNOWN', 'error').inc();
        console.error('Error making API request:', error);
        res.status(500).json({ 
            error: 'API request failed',
            message: error.message 
        });
    }
});

// Webhook endpoints
app.post('/v1/webhooks/:provider/:integration_id', async (req, res) => {
    try {
        const { provider, integration_id } = req.params;
        const payload = req.body;
        const headers = req.headers;

        // Verify webhook signature if configured
        const integration = await getIntegration(integration_id);
        if (integration && integration.webhook_config.secret) {
            const isValid = verifyWebhookSignature(provider, payload, headers, integration.webhook_config.secret);
            if (!isValid) {
                return res.status(401).json({ error: 'Invalid webhook signature' });
            }
        }

        // Process webhook
        await processWebhook(provider, integration_id, payload, headers);

        // Update metrics
        const eventType = extractEventType(provider, payload);
        webhookEventsTotal.labels(provider, eventType).inc();

        res.json({ status: 'received' });

    } catch (error) {
        console.error('Error processing webhook:', error);
        res.status(500).json({ error: 'Failed to process webhook' });
    }
});

// Sync endpoints
app.post('/v1/integrations/:id/sync', async (req, res) => {
    try {
        const { id } = req.params;
        const { direction = SYNC_DIRECTIONS.BIDIRECTIONAL, entity_type, filters = {} } = req.body;

        const integration = activeIntegrations.get(id);
        if (!integration) {
            return res.status(404).json({ error: 'Integration not found or inactive' });
        }

        // Start sync operation
        const syncId = await startSyncOperation(id, direction, entity_type, filters);

        res.json({
            sync_id: syncId,
            status: 'started',
            message: 'Sync operation started'
        });

    } catch (error) {
        console.error('Error starting sync:', error);
        res.status(500).json({ error: 'Failed to start sync operation' });
    }
});

app.get('/v1/integrations/:id/sync/:sync_id', async (req, res) => {
    try {
        const { id, sync_id } = req.params;
        const syncStatus = await getSyncStatus(sync_id);

        if (!syncStatus) {
            return res.status(404).json({ error: 'Sync operation not found' });
        }

        res.json(syncStatus);
    } catch (error) {
        console.error('Error getting sync status:', error);
        res.status(500).json({ error: 'Failed to get sync status' });
    }
});

// OAuth flow endpoints
app.get('/v1/integrations/:id/oauth/authorize', async (req, res) => {
    try {
        const { id } = req.params;
        const integration = await getIntegration(id);

        if (!integration) {
            return res.status(404).json({ error: 'Integration not found' });
        }

        const provider = PROVIDERS[integration.provider];
        if (provider.authType !== AUTH_TYPES.OAUTH2) {
            return res.status(400).json({ error: 'Integration does not support OAuth2' });
        }

        const authUrl = generateOAuthUrl(integration);
        res.json({ auth_url: authUrl });

    } catch (error) {
        console.error('Error generating OAuth URL:', error);
        res.status(500).json({ error: 'Failed to generate OAuth URL' });
    }
});

app.post('/v1/integrations/:id/oauth/callback', async (req, res) => {
    try {
        const { id } = req.params;
        const { code, state } = req.body;

        const integration = await getIntegration(id);
        if (!integration) {
            return res.status(404).json({ error: 'Integration not found' });
        }

        // Exchange code for tokens
        const tokens = await exchangeOAuthCode(integration, code);

        // Update integration with tokens
        await updateIntegration(id, {
            auth_config: {
                ...integration.auth_config,
                access_token: tokens.access_token,
                refresh_token: tokens.refresh_token,
                expires_at: new Date(Date.now() + tokens.expires_in * 1000)
            },
            status: INTEGRATION_STATUS.ACTIVE
        });

        // Initialize integration
        await initializeIntegration(id);

        res.json({
            status: 'success',
            message: 'OAuth authorization completed'
        });

    } catch (error) {
        console.error('Error handling OAuth callback:', error);
        res.status(500).json({ error: 'Failed to complete OAuth authorization' });
    }
});

// Provider-specific endpoints
app.get('/v1/providers', (req, res) => {
    const providers = Object.entries(PROVIDERS).map(([key, config]) => ({
        id: key,
        name: config.name,
        auth_type: config.authType,
        webhook_support: config.webhookSupport,
        scopes: config.scopes || []
    }));

    res.json({ providers });
});

app.get('/v1/providers/:provider/schema', async (req, res) => {
    try {
        const { provider } = req.params;
        const schema = await getProviderSchema(provider);

        if (!schema) {
            return res.status(404).json({ error: 'Provider schema not found' });
        }

        res.json(schema);
    } catch (error) {
        console.error('Error getting provider schema:', error);
        res.status(500).json({ error: 'Failed to get provider schema' });
    }
});

// Database functions
async function storeIntegration(integration) {
    const query = `
        INSERT INTO integrations (
            id, name, provider, auth_config, api_config, webhook_config, 
            sync_config, status, user_id, project_id, tags, created_at, updated_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
    `;

    await pgPool.query(query, [
        integration.id,
        integration.name,
        integration.provider,
        integration.auth_config,
        integration.api_config,
        integration.webhook_config,
        integration.sync_config,
        integration.status,
        integration.user_id,
        integration.project_id,
        integration.tags,
        integration.created_at,
        integration.updated_at
    ]);
}

async function getIntegration(id) {
    const query = 'SELECT * FROM integrations WHERE id = $1';
    const result = await pgPool.query(query, [id]);
    
    if (result.rows.length === 0) {
        return null;
    }

    const row = result.rows[0];
    return {
        ...row,
        auth_config: JSON.parse(row.auth_config),
        api_config: JSON.parse(row.api_config),
        webhook_config: JSON.parse(row.webhook_config),
        sync_config: JSON.parse(row.sync_config)
    };
}

async function listIntegrations(filters) {
    let query = 'SELECT * FROM integrations WHERE 1=1';
    const params = [];
    let paramCount = 0;

    if (filters.provider) {
        query += ` AND provider = $${++paramCount}`;
        params.push(filters.provider);
    }

    if (filters.status) {
        query += ` AND status = $${++paramCount}`;
        params.push(filters.status);
    }

    if (filters.user_id) {
        query += ` AND user_id = $${++paramCount}`;
        params.push(filters.user_id);
    }

    if (filters.project_id) {
        query += ` AND project_id = $${++paramCount}`;
        params.push(filters.project_id);
    }

    query += ` ORDER BY created_at DESC LIMIT $${++paramCount} OFFSET $${++paramCount}`;
    params.push(filters.limit, filters.offset);

    const result = await pgPool.query(query, params);
    
    return result.rows.map(row => ({
        ...row,
        auth_config: JSON.parse(row.auth_config),
        api_config: JSON.parse(row.api_config),
        webhook_config: JSON.parse(row.webhook_config),
        sync_config: JSON.parse(row.sync_config)
    }));
}

async function updateIntegration(id, updates) {
    const setClause = [];
    const params = [id];
    let paramCount = 1;

    Object.entries(updates).forEach(([key, value]) => {
        if (key !== 'id') {
            setClause.push(`${key} = $${++paramCount}`);
            if (typeof value === 'object' && value !== null) {
                params.push(JSON.stringify(value));
            } else {
                params.push(value);
            }
        }
    });

    setClause.push(`updated_at = $${++paramCount}`);
    params.push(new Date());

    const query = `UPDATE integrations SET ${setClause.join(', ')} WHERE id = $1 RETURNING *`;
    const result = await pgPool.query(query, params);

    if (result.rows.length === 0) {
        return null;
    }

    const row = result.rows[0];
    return {
        ...row,
        auth_config: JSON.parse(row.auth_config),
        api_config: JSON.parse(row.api_config),
        webhook_config: JSON.parse(row.webhook_config),
        sync_config: JSON.parse(row.sync_config)
    };
}

async function deleteIntegration(id) {
    const query = 'DELETE FROM integrations WHERE id = $1';
    await pgPool.query(query, [id]);
}

// Integration management functions
async function initializeIntegration(integrationId) {
    try {
        const integration = await getIntegration(integrationId);
        if (!integration) {
            throw new Error('Integration not found');
        }

        // Validate authentication
        const isValid = await validateAuthentication(integration);
        if (!isValid) {
            await updateIntegration(integrationId, { 
                status: INTEGRATION_STATUS.ERROR,
                error_message: 'Authentication failed'
            });
            return;
        }

        // Store in active integrations
        activeIntegrations.set(integrationId, integration);

        // Set up webhook handler if configured
        if (integration.webhook_config.enabled) {
            setupWebhookHandler(integration);
        }

        // Update status
        await updateIntegration(integrationId, { 
            status: INTEGRATION_STATUS.ACTIVE,
            error_message: null
        });

        console.log(`Integration ${integrationId} initialized successfully`);

    } catch (error) {
        console.error(`Error initializing integration ${integrationId}:`, error);
        await updateIntegration(integrationId, { 
            status: INTEGRATION_STATUS.ERROR,
            error_message: error.message
        });
    }
}

async function validateAuthentication(integration) {
    try {
        const provider = PROVIDERS[integration.provider];
        
        switch (provider.authType) {
            case AUTH_TYPES.API_KEY:
                return integration.auth_config.api_key ? true : false;
            
            case AUTH_TYPES.OAUTH2:
                if (!integration.auth_config.access_token) {
                    return false;
                }
                
                // Check if token is expired
                if (integration.auth_config.expires_at && 
                    new Date(integration.auth_config.expires_at) < new Date()) {
                    // Try to refresh token
                    return await refreshOAuthToken(integration);
                }
                return true;
            
            case AUTH_TYPES.BASIC:
                return integration.auth_config.username && integration.auth_config.password;
            
            case AUTH_TYPES.BEARER:
                return integration.auth_config.token ? true : false;
            
            default:
                return true;
        }
    } catch (error) {
        console.error('Error validating authentication:', error);
        return false;
    }
}

// Initialize database tables
async function initializeDatabase() {
    const createTables = `
        CREATE TABLE IF NOT EXISTS integrations (
            id UUID PRIMARY KEY,
            name VARCHAR(255) NOT NULL,
            provider VARCHAR(100) NOT NULL,
            auth_config JSONB NOT NULL DEFAULT '{}',
            api_config JSONB NOT NULL DEFAULT '{}',
            webhook_config JSONB NOT NULL DEFAULT '{}',
            sync_config JSONB NOT NULL DEFAULT '{}',
            status VARCHAR(50) NOT NULL DEFAULT 'pending',
            user_id VARCHAR(255),
            project_id VARCHAR(255),
            tags TEXT[] DEFAULT '{}',
            last_sync_at TIMESTAMP,
            error_message TEXT,
            created_at TIMESTAMP DEFAULT NOW(),
            updated_at TIMESTAMP DEFAULT NOW()
        );

        CREATE TABLE IF NOT EXISTS sync_operations (
            id UUID PRIMARY KEY,
            integration_id UUID NOT NULL REFERENCES integrations(id),
            direction VARCHAR(50) NOT NULL,
            entity_type VARCHAR(100),
            status VARCHAR(50) NOT NULL DEFAULT 'running',
            records_processed INTEGER DEFAULT 0,
            records_total INTEGER DEFAULT 0,
            errors JSONB DEFAULT '[]',
            started_at TIMESTAMP DEFAULT NOW(),
            completed_at TIMESTAMP,
            created_at TIMESTAMP DEFAULT NOW()
        );

        CREATE TABLE IF NOT EXISTS webhook_events (
            id UUID PRIMARY KEY,
            integration_id UUID NOT NULL REFERENCES integrations(id),
            provider VARCHAR(100) NOT NULL,
            event_type VARCHAR(100),
            payload JSONB NOT NULL,
            headers JSONB DEFAULT '{}',
            processed BOOLEAN DEFAULT FALSE,
            created_at TIMESTAMP DEFAULT NOW()
        );

        CREATE INDEX IF NOT EXISTS idx_integrations_provider ON integrations(provider);
        CREATE INDEX IF NOT EXISTS idx_integrations_status ON integrations(status);
        CREATE INDEX IF NOT EXISTS idx_integrations_user_id ON integrations(user_id);
        CREATE INDEX IF NOT EXISTS idx_sync_operations_integration_id ON sync_operations(integration_id);
        CREATE INDEX IF NOT EXISTS idx_webhook_events_integration_id ON webhook_events(integration_id);
        CREATE INDEX IF NOT EXISTS idx_webhook_events_processed ON webhook_events(processed);
    `;

    await pgPool.query(createTables);
}

// Start server
async function startServer() {
    try {
        // Initialize database
        await initializeDatabase();
        
        // Connect to Redis
        await redis.connect();
        
        // Load active integrations
        await loadActiveIntegrations();
        
        // Start periodic tasks
        startPeriodicTasks();
        
        // Start server
        app.listen(PORT, () => {
            console.log(`🚀 Integration Service running on port ${PORT}`);
            console.log(`📊 Health check: http://localhost:${PORT}/health`);
            console.log(`📈 Metrics: http://localhost:${PORT}/metrics`);
            console.log(`🔗 Active integrations: ${activeIntegrations.size}`);
        });
    } catch (error) {
        console.error('Failed to start server:', error);
        process.exit(1);
    }
}

// Load active integrations on startup
async function loadActiveIntegrations() {
    try {
        const integrations = await listIntegrations({ 
            status: INTEGRATION_STATUS.ACTIVE,
            limit: 1000,
            offset: 0
        });

        for (const integration of integrations) {
            activeIntegrations.set(integration.id, integration);
            
            if (integration.webhook_config.enabled) {
                setupWebhookHandler(integration);
            }
        }

        console.log(`Loaded ${integrations.length} active integrations`);
    } catch (error) {
        console.error('Error loading active integrations:', error);
    }
}

// Start periodic tasks
function startPeriodicTasks() {
    // Refresh OAuth tokens every hour
    cron.schedule('0 * * * *', async () => {
        console.log('Running OAuth token refresh...');
        await refreshExpiredTokens();
    });

    // Update metrics every 5 minutes
    cron.schedule('*/5 * * * *', async () => {
        await updateMetrics();
    });

    // Clean up old webhook events daily
    cron.schedule('0 2 * * *', async () => {
        await cleanupWebhookEvents();
    });
}

// Graceful shutdown
process.on('SIGTERM', async () => {
    console.log('Shutting down integration service...');
    app.close();
    await redis.quit();
    await pgPool.end();
    process.exit(0);
});

startServer();
