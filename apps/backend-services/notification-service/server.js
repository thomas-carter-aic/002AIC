/**
 * Notification Service - Node.js
 * Communication hub for the 002AIC platform
 * Handles email, SMS, push notifications, and real-time messaging
 */

const express = require('express');
const http = require('http');
const socketIo = require('socket.io');
const cors = require('cors');
const helmet = require('helmet');
const rateLimit = require('express-rate-limit');
const { v4: uuidv4 } = require('uuid');
const nodemailer = require('nodemailer');
const twilio = require('twilio');
const webpush = require('web-push');
const Redis = require('redis');
const { Pool } = require('pg');
const amqp = require('amqplib');
const prometheus = require('prom-client');

// Initialize Express app
const app = express();
const server = http.createServer(app);
const io = socketIo(server, {
    cors: {
        origin: "*",
        methods: ["GET", "POST"]
    }
});

// Configuration
const PORT = process.env.PORT || 8080;
const REDIS_URL = process.env.REDIS_URL || 'redis://localhost:6379';
const DATABASE_URL = process.env.DATABASE_URL || 'postgresql://postgres:password@localhost:5432/notifications';
const RABBITMQ_URL = process.env.RABBITMQ_URL || 'amqp://localhost';

// Initialize external services
const redis = Redis.createClient({ url: REDIS_URL });
const pgPool = new Pool({ connectionString: DATABASE_URL });

// Email configuration
const emailTransporter = nodemailer.createTransporter({
    host: process.env.SMTP_HOST || 'localhost',
    port: process.env.SMTP_PORT || 587,
    secure: false,
    auth: {
        user: process.env.SMTP_USER || 'notifications@002aic.com',
        pass: process.env.SMTP_PASS || 'password'
    }
});

// SMS configuration (Twilio)
const twilioClient = twilio(
    process.env.TWILIO_ACCOUNT_SID || 'demo',
    process.env.TWILIO_AUTH_TOKEN || 'demo'
);

// Push notification configuration
webpush.setVapidDetails(
    'mailto:notifications@002aic.com',
    process.env.VAPID_PUBLIC_KEY || 'demo-public-key',
    process.env.VAPID_PRIVATE_KEY || 'demo-private-key'
);

// Prometheus metrics
const register = new prometheus.Registry();
prometheus.collectDefaultMetrics({ register });

const notificationsSent = new prometheus.Counter({
    name: 'notifications_sent_total',
    help: 'Total number of notifications sent',
    labelNames: ['type', 'channel', 'status'],
    registers: [register]
});

const activeConnections = new prometheus.Gauge({
    name: 'websocket_connections_active',
    help: 'Number of active WebSocket connections',
    registers: [register]
});

const notificationLatency = new prometheus.Histogram({
    name: 'notification_processing_duration_seconds',
    help: 'Time taken to process notifications',
    labelNames: ['type', 'channel'],
    registers: [register]
});

// Middleware
app.use(helmet());
app.use(cors());
app.use(express.json({ limit: '10mb' }));
app.use(express.urlencoded({ extended: true }));

// Rate limiting
const limiter = rateLimit({
    windowMs: 15 * 60 * 1000, // 15 minutes
    max: 1000, // limit each IP to 1000 requests per windowMs
    message: 'Too many requests from this IP, please try again later.'
});
app.use(limiter);

// Storage for active connections and subscriptions
const activeConnections_map = new Map();
const userSubscriptions = new Map();

// Notification types and channels
const NOTIFICATION_TYPES = {
    ALERT: 'alert',
    INFO: 'info',
    WARNING: 'warning',
    SUCCESS: 'success',
    MARKETING: 'marketing',
    SYSTEM: 'system'
};

const CHANNELS = {
    EMAIL: 'email',
    SMS: 'sms',
    PUSH: 'push',
    WEBSOCKET: 'websocket',
    SLACK: 'slack',
    WEBHOOK: 'webhook'
};

// Health check endpoint
app.get('/health', (req, res) => {
    res.json({
        status: 'healthy',
        service: 'notification-service',
        timestamp: new Date().toISOString(),
        version: '1.0.0',
        connections: activeConnections_map.size
    });
});

// Metrics endpoint
app.get('/metrics', (req, res) => {
    res.set('Content-Type', register.contentType);
    res.end(register.metrics());
});

// Notification API endpoints
app.post('/v1/notifications/send', async (req, res) => {
    try {
        const {
            recipients,
            type = NOTIFICATION_TYPES.INFO,
            channels = [CHANNELS.EMAIL],
            subject,
            message,
            template,
            data = {},
            priority = 'normal',
            schedule_at,
            expires_at
        } = req.body;

        // Validate request
        if (!recipients || !Array.isArray(recipients) || recipients.length === 0) {
            return res.status(400).json({ error: 'Recipients are required' });
        }

        if (!message && !template) {
            return res.status(400).json({ error: 'Message or template is required' });
        }

        const notificationId = uuidv4();
        const startTime = Date.now();

        // Create notification record
        const notification = {
            id: notificationId,
            type,
            channels,
            subject,
            message,
            template,
            data,
            priority,
            schedule_at: schedule_at ? new Date(schedule_at) : new Date(),
            expires_at: expires_at ? new Date(expires_at) : null,
            status: 'pending',
            created_at: new Date(),
            recipients: recipients.length,
            sent_count: 0,
            failed_count: 0
        };

        // Store notification in database
        await storeNotification(notification);

        // Process notification
        if (schedule_at && new Date(schedule_at) > new Date()) {
            // Schedule for later
            await scheduleNotification(notification);
            res.json({
                notification_id: notificationId,
                status: 'scheduled',
                scheduled_at: schedule_at,
                recipients: recipients.length
            });
        } else {
            // Send immediately
            const results = await processNotification(notification, recipients);
            
            // Update metrics
            const duration = (Date.now() - startTime) / 1000;
            channels.forEach(channel => {
                notificationLatency.labels(type, channel).observe(duration);
            });

            res.json({
                notification_id: notificationId,
                status: 'sent',
                results,
                recipients: recipients.length,
                processing_time_ms: Date.now() - startTime
            });
        }

    } catch (error) {
        console.error('Error sending notification:', error);
        res.status(500).json({ error: 'Failed to send notification' });
    }
});

app.get('/v1/notifications/:id', async (req, res) => {
    try {
        const { id } = req.params;
        const notification = await getNotification(id);
        
        if (!notification) {
            return res.status(404).json({ error: 'Notification not found' });
        }

        res.json(notification);
    } catch (error) {
        console.error('Error getting notification:', error);
        res.status(500).json({ error: 'Failed to get notification' });
    }
});

app.get('/v1/notifications', async (req, res) => {
    try {
        const {
            type,
            status,
            channel,
            limit = 50,
            offset = 0,
            start_date,
            end_date
        } = req.query;

        const notifications = await getNotifications({
            type,
            status,
            channel,
            limit: parseInt(limit),
            offset: parseInt(offset),
            start_date,
            end_date
        });

        res.json({
            notifications,
            total: notifications.length,
            limit: parseInt(limit),
            offset: parseInt(offset)
        });
    } catch (error) {
        console.error('Error listing notifications:', error);
        res.status(500).json({ error: 'Failed to list notifications' });
    }
});

// Template management
app.post('/v1/notifications/templates', async (req, res) => {
    try {
        const {
            name,
            type,
            channels,
            subject_template,
            body_template,
            variables = [],
            description
        } = req.body;

        const templateId = uuidv4();
        const template = {
            id: templateId,
            name,
            type,
            channels,
            subject_template,
            body_template,
            variables,
            description,
            created_at: new Date(),
            updated_at: new Date()
        };

        await storeTemplate(template);
        res.json(template);
    } catch (error) {
        console.error('Error creating template:', error);
        res.status(500).json({ error: 'Failed to create template' });
    }
});

app.get('/v1/notifications/templates', async (req, res) => {
    try {
        const templates = await getTemplates();
        res.json({ templates });
    } catch (error) {
        console.error('Error listing templates:', error);
        res.status(500).json({ error: 'Failed to list templates' });
    }
});

// Subscription management
app.post('/v1/notifications/subscriptions', async (req, res) => {
    try {
        const {
            user_id,
            channels,
            preferences = {},
            topics = []
        } = req.body;

        const subscription = {
            id: uuidv4(),
            user_id,
            channels,
            preferences,
            topics,
            created_at: new Date(),
            updated_at: new Date()
        };

        await storeSubscription(subscription);
        userSubscriptions.set(user_id, subscription);

        res.json(subscription);
    } catch (error) {
        console.error('Error creating subscription:', error);
        res.status(500).json({ error: 'Failed to create subscription' });
    }
});

// WebSocket connection handling
io.on('connection', (socket) => {
    console.log('New WebSocket connection:', socket.id);
    activeConnections.inc();
    activeConnections_map.set(socket.id, {
        socket,
        user_id: null,
        connected_at: new Date()
    });

    socket.on('authenticate', (data) => {
        const { user_id, token } = data;
        // In production, validate the token
        if (user_id) {
            const connection = activeConnections_map.get(socket.id);
            if (connection) {
                connection.user_id = user_id;
                socket.join(`user:${user_id}`);
                console.log(`User ${user_id} authenticated on socket ${socket.id}`);
            }
        }
    });

    socket.on('subscribe', (data) => {
        const { topics } = data;
        if (Array.isArray(topics)) {
            topics.forEach(topic => {
                socket.join(`topic:${topic}`);
            });
            console.log(`Socket ${socket.id} subscribed to topics:`, topics);
        }
    });

    socket.on('disconnect', () => {
        console.log('WebSocket disconnected:', socket.id);
        activeConnections.dec();
        activeConnections_map.delete(socket.id);
    });
});

// Notification processing functions
async function processNotification(notification, recipients) {
    const results = {
        total: recipients.length,
        sent: 0,
        failed: 0,
        errors: []
    };

    for (const recipient of recipients) {
        for (const channel of notification.channels) {
            try {
                await sendNotificationByChannel(notification, recipient, channel);
                results.sent++;
                notificationsSent.labels(notification.type, channel, 'success').inc();
            } catch (error) {
                results.failed++;
                results.errors.push({
                    recipient: recipient.id || recipient.email || recipient.phone,
                    channel,
                    error: error.message
                });
                notificationsSent.labels(notification.type, channel, 'failed').inc();
                console.error(`Failed to send ${channel} notification:`, error);
            }
        }
    }

    // Update notification status
    await updateNotificationStatus(notification.id, {
        status: results.failed === 0 ? 'sent' : 'partial',
        sent_count: results.sent,
        failed_count: results.failed,
        completed_at: new Date()
    });

    return results;
}

async function sendNotificationByChannel(notification, recipient, channel) {
    const { message, subject, template, data } = notification;
    
    // Render message if using template
    let renderedMessage = message;
    let renderedSubject = subject;
    
    if (template) {
        const templateData = await getTemplate(template);
        if (templateData) {
            renderedMessage = renderTemplate(templateData.body_template, { ...data, ...recipient });
            renderedSubject = renderTemplate(templateData.subject_template, { ...data, ...recipient });
        }
    }

    switch (channel) {
        case CHANNELS.EMAIL:
            await sendEmail(recipient, renderedSubject, renderedMessage);
            break;
        case CHANNELS.SMS:
            await sendSMS(recipient, renderedMessage);
            break;
        case CHANNELS.PUSH:
            await sendPushNotification(recipient, renderedSubject, renderedMessage);
            break;
        case CHANNELS.WEBSOCKET:
            await sendWebSocketNotification(recipient, {
                type: notification.type,
                subject: renderedSubject,
                message: renderedMessage,
                data
            });
            break;
        case CHANNELS.WEBHOOK:
            await sendWebhook(recipient, {
                type: notification.type,
                subject: renderedSubject,
                message: renderedMessage,
                data
            });
            break;
        default:
            throw new Error(`Unsupported channel: ${channel}`);
    }
}

async function sendEmail(recipient, subject, message) {
    const mailOptions = {
        from: process.env.FROM_EMAIL || 'notifications@002aic.com',
        to: recipient.email,
        subject: subject,
        html: message,
        text: message.replace(/<[^>]*>/g, '') // Strip HTML for text version
    };

    await emailTransporter.sendMail(mailOptions);
}

async function sendSMS(recipient, message) {
    if (process.env.TWILIO_ACCOUNT_SID === 'demo') {
        console.log(`SMS (demo): ${recipient.phone} - ${message}`);
        return;
    }

    await twilioClient.messages.create({
        body: message,
        from: process.env.TWILIO_PHONE_NUMBER,
        to: recipient.phone
    });
}

async function sendPushNotification(recipient, title, message) {
    if (!recipient.push_subscription) {
        throw new Error('No push subscription found for recipient');
    }

    const payload = JSON.stringify({
        title,
        body: message,
        icon: '/icon-192x192.png',
        badge: '/badge-72x72.png'
    });

    await webpush.sendNotification(recipient.push_subscription, payload);
}

async function sendWebSocketNotification(recipient, notification) {
    if (recipient.user_id) {
        io.to(`user:${recipient.user_id}`).emit('notification', {
            id: uuidv4(),
            ...notification,
            timestamp: new Date().toISOString()
        });
    }
}

async function sendWebhook(recipient, notification) {
    if (!recipient.webhook_url) {
        throw new Error('No webhook URL found for recipient');
    }

    const response = await fetch(recipient.webhook_url, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'User-Agent': '002AIC-Notification-Service/1.0'
        },
        body: JSON.stringify(notification)
    });

    if (!response.ok) {
        throw new Error(`Webhook failed with status: ${response.status}`);
    }
}

function renderTemplate(template, data) {
    return template.replace(/\{\{(\w+)\}\}/g, (match, key) => {
        return data[key] || match;
    });
}

// Database functions
async function storeNotification(notification) {
    const query = `
        INSERT INTO notifications (id, type, channels, subject, message, template, data, priority, schedule_at, expires_at, status, created_at, recipients, sent_count, failed_count)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
    `;
    
    await pgPool.query(query, [
        notification.id,
        notification.type,
        JSON.stringify(notification.channels),
        notification.subject,
        notification.message,
        notification.template,
        JSON.stringify(notification.data),
        notification.priority,
        notification.schedule_at,
        notification.expires_at,
        notification.status,
        notification.created_at,
        notification.recipients,
        notification.sent_count,
        notification.failed_count
    ]);
}

async function getNotification(id) {
    const query = 'SELECT * FROM notifications WHERE id = $1';
    const result = await pgPool.query(query, [id]);
    return result.rows[0];
}

async function getNotifications(filters) {
    let query = 'SELECT * FROM notifications WHERE 1=1';
    const params = [];
    let paramCount = 0;

    if (filters.type) {
        query += ` AND type = $${++paramCount}`;
        params.push(filters.type);
    }

    if (filters.status) {
        query += ` AND status = $${++paramCount}`;
        params.push(filters.status);
    }

    query += ` ORDER BY created_at DESC LIMIT $${++paramCount} OFFSET $${++paramCount}`;
    params.push(filters.limit, filters.offset);

    const result = await pgPool.query(query, params);
    return result.rows;
}

async function updateNotificationStatus(id, updates) {
    const query = `
        UPDATE notifications 
        SET status = $2, sent_count = $3, failed_count = $4, completed_at = $5, updated_at = NOW()
        WHERE id = $1
    `;
    
    await pgPool.query(query, [
        id,
        updates.status,
        updates.sent_count,
        updates.failed_count,
        updates.completed_at
    ]);
}

async function storeTemplate(template) {
    const query = `
        INSERT INTO notification_templates (id, name, type, channels, subject_template, body_template, variables, description, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
    `;
    
    await pgPool.query(query, [
        template.id,
        template.name,
        template.type,
        JSON.stringify(template.channels),
        template.subject_template,
        template.body_template,
        JSON.stringify(template.variables),
        template.description,
        template.created_at,
        template.updated_at
    ]);
}

async function getTemplate(id) {
    const query = 'SELECT * FROM notification_templates WHERE id = $1';
    const result = await pgPool.query(query, [id]);
    return result.rows[0];
}

async function getTemplates() {
    const query = 'SELECT * FROM notification_templates ORDER BY created_at DESC';
    const result = await pgPool.query(query);
    return result.rows;
}

async function storeSubscription(subscription) {
    const query = `
        INSERT INTO user_subscriptions (id, user_id, channels, preferences, topics, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        ON CONFLICT (user_id) DO UPDATE SET
        channels = $3, preferences = $4, topics = $5, updated_at = $7
    `;
    
    await pgPool.query(query, [
        subscription.id,
        subscription.user_id,
        JSON.stringify(subscription.channels),
        JSON.stringify(subscription.preferences),
        JSON.stringify(subscription.topics),
        subscription.created_at,
        subscription.updated_at
    ]);
}

async function scheduleNotification(notification) {
    // In production, use a proper job queue like Bull or Agenda
    console.log(`Notification ${notification.id} scheduled for ${notification.schedule_at}`);
}

// Initialize database tables
async function initializeDatabase() {
    const createTables = `
        CREATE TABLE IF NOT EXISTS notifications (
            id UUID PRIMARY KEY,
            type VARCHAR(50) NOT NULL,
            channels JSONB NOT NULL,
            subject TEXT,
            message TEXT,
            template UUID,
            data JSONB DEFAULT '{}',
            priority VARCHAR(20) DEFAULT 'normal',
            schedule_at TIMESTAMP,
            expires_at TIMESTAMP,
            status VARCHAR(20) DEFAULT 'pending',
            created_at TIMESTAMP DEFAULT NOW(),
            updated_at TIMESTAMP DEFAULT NOW(),
            completed_at TIMESTAMP,
            recipients INTEGER DEFAULT 0,
            sent_count INTEGER DEFAULT 0,
            failed_count INTEGER DEFAULT 0
        );

        CREATE TABLE IF NOT EXISTS notification_templates (
            id UUID PRIMARY KEY,
            name VARCHAR(255) UNIQUE NOT NULL,
            type VARCHAR(50) NOT NULL,
            channels JSONB NOT NULL,
            subject_template TEXT,
            body_template TEXT NOT NULL,
            variables JSONB DEFAULT '[]',
            description TEXT,
            created_at TIMESTAMP DEFAULT NOW(),
            updated_at TIMESTAMP DEFAULT NOW()
        );

        CREATE TABLE IF NOT EXISTS user_subscriptions (
            id UUID PRIMARY KEY,
            user_id VARCHAR(255) UNIQUE NOT NULL,
            channels JSONB NOT NULL,
            preferences JSONB DEFAULT '{}',
            topics JSONB DEFAULT '[]',
            created_at TIMESTAMP DEFAULT NOW(),
            updated_at TIMESTAMP DEFAULT NOW()
        );

        CREATE INDEX IF NOT EXISTS idx_notifications_type ON notifications(type);
        CREATE INDEX IF NOT EXISTS idx_notifications_status ON notifications(status);
        CREATE INDEX IF NOT EXISTS idx_notifications_created_at ON notifications(created_at);
        CREATE INDEX IF NOT EXISTS idx_user_subscriptions_user_id ON user_subscriptions(user_id);
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
        
        // Start server
        server.listen(PORT, () => {
            console.log(`🚀 Notification Service running on port ${PORT}`);
            console.log(`📊 Health check: http://localhost:${PORT}/health`);
            console.log(`📈 Metrics: http://localhost:${PORT}/metrics`);
            console.log(`🔌 WebSocket connections: ${activeConnections_map.size}`);
        });
    } catch (error) {
        console.error('Failed to start server:', error);
        process.exit(1);
    }
}

// Graceful shutdown
process.on('SIGTERM', async () => {
    console.log('Shutting down notification service...');
    server.close();
    await redis.quit();
    await pgPool.end();
    process.exit(0);
});

startServer();
