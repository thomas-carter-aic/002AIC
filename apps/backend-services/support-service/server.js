/**
 * Support Service - Node.js
 * Customer support and ticketing system for the 002AIC platform
 * Handles support tickets, knowledge base, live chat, and customer feedback
 */

const express = require('express');
const http = require('http');
const socketIo = require('socket.io');
const cors = require('cors');
const helmet = require('helmet');
const rateLimit = require('express-rate-limit');
const { v4: uuidv4 } = require('uuid');
const Redis = require('redis');
const { Pool } = require('pg');
const prometheus = require('prom-client');
const nodemailer = require('nodemailer');
const multer = require('multer');
const path = require('path');
const fs = require('fs').promises;

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
const DATABASE_URL = process.env.DATABASE_URL || 'postgresql://postgres:password@localhost:5432/support';
const REDIS_URL = process.env.REDIS_URL || 'redis://localhost:6379';
const SMTP_HOST = process.env.SMTP_HOST || 'localhost';
const SMTP_PORT = process.env.SMTP_PORT || 587;
const SMTP_USER = process.env.SMTP_USER || 'support@002aic.com';
const SMTP_PASS = process.env.SMTP_PASS || 'password';
const UPLOAD_PATH = process.env.UPLOAD_PATH || './uploads';
const MAX_FILE_SIZE = parseInt(process.env.MAX_FILE_SIZE) || 10 * 1024 * 1024; // 10MB

// Initialize external services
const redis = Redis.createClient({ url: REDIS_URL });
const pgPool = new Pool({ connectionString: DATABASE_URL });

// Email configuration
const emailTransporter = nodemailer.createTransporter({
    host: SMTP_HOST,
    port: SMTP_PORT,
    secure: false,
    auth: {
        user: SMTP_USER,
        pass: SMTP_PASS
    }
});

// File upload configuration
const storage = multer.diskStorage({
    destination: async (req, file, cb) => {
        const uploadDir = path.join(UPLOAD_PATH, 'attachments');
        await fs.mkdir(uploadDir, { recursive: true });
        cb(null, uploadDir);
    },
    filename: (req, file, cb) => {
        const uniqueName = `${uuidv4()}-${file.originalname}`;
        cb(null, uniqueName);
    }
});

const upload = multer({
    storage: storage,
    limits: { fileSize: MAX_FILE_SIZE },
    fileFilter: (req, file, cb) => {
        // Allow common file types
        const allowedTypes = /jpeg|jpg|png|gif|pdf|doc|docx|txt|zip/;
        const extname = allowedTypes.test(path.extname(file.originalname).toLowerCase());
        const mimetype = allowedTypes.test(file.mimetype);

        if (mimetype && extname) {
            return cb(null, true);
        } else {
            cb(new Error('Invalid file type'));
        }
    }
});

// Prometheus metrics
const register = new prometheus.Registry();
prometheus.collectDefaultMetrics({ register });

const ticketsTotal = new prometheus.Gauge({
    name: 'support_tickets_total',
    help: 'Total number of support tickets',
    labelNames: ['status', 'priority', 'category'],
    registers: [register]
});

const ticketResponseTime = new prometheus.Histogram({
    name: 'support_ticket_response_time_hours',
    help: 'Time to first response for support tickets',
    labelNames: ['priority'],
    registers: [register]
});

const ticketResolutionTime = new prometheus.Histogram({
    name: 'support_ticket_resolution_time_hours',
    help: 'Time to resolution for support tickets',
    labelNames: ['priority', 'category'],
    registers: [register]
});

const chatSessionsActive = new prometheus.Gauge({
    name: 'support_chat_sessions_active',
    help: 'Number of active chat sessions',
    registers: [register]
});

const knowledgeBaseViews = new prometheus.Counter({
    name: 'knowledge_base_views_total',
    help: 'Total knowledge base article views',
    labelNames: ['article_id'],
    registers: [register]
});

const customerSatisfactionScore = new prometheus.Gauge({
    name: 'customer_satisfaction_score',
    help: 'Average customer satisfaction score',
    labelNames: ['period'],
    registers: [register]
});

// Constants
const TICKET_STATUS = {
    OPEN: 'open',
    IN_PROGRESS: 'in_progress',
    PENDING: 'pending',
    RESOLVED: 'resolved',
    CLOSED: 'closed'
};

const TICKET_PRIORITY = {
    LOW: 'low',
    MEDIUM: 'medium',
    HIGH: 'high',
    URGENT: 'urgent'
};

const TICKET_CATEGORY = {
    TECHNICAL: 'technical',
    BILLING: 'billing',
    FEATURE_REQUEST: 'feature_request',
    BUG_REPORT: 'bug_report',
    GENERAL: 'general'
};

const CHAT_STATUS = {
    WAITING: 'waiting',
    ACTIVE: 'active',
    ENDED: 'ended'
};

// Storage for active connections
const activeConnections = new Map();
const activeChatSessions = new Map();
const supportAgents = new Map();

// Middleware
app.use(helmet());
app.use(cors());
app.use(express.json({ limit: '10mb' }));
app.use(express.urlencoded({ extended: true }));

// Rate limiting
const limiter = rateLimit({
    windowMs: 15 * 60 * 1000, // 15 minutes
    max: 1000,
    message: 'Too many requests from this IP, please try again later.'
});
app.use(limiter);

// Health check endpoint
app.get('/health', async (req, res) => {
    const status = {
        status: 'healthy',
        service: 'support-service',
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

    status.active_connections = activeConnections.size;
    status.active_chat_sessions = activeChatSessions.size;
    status.support_agents = supportAgents.size;

    const statusCode = status.status === 'healthy' ? 200 : 503;
    res.status(statusCode).json(status);
});

// Metrics endpoint
app.get('/metrics', (req, res) => {
    res.set('Content-Type', register.contentType);
    res.end(register.metrics());
});

// Ticket management endpoints
app.post('/v1/tickets', upload.array('attachments', 5), async (req, res) => {
    try {
        const {
            subject,
            description,
            category = TICKET_CATEGORY.GENERAL,
            priority = TICKET_PRIORITY.MEDIUM,
            customer_id,
            customer_email,
            customer_name,
            metadata = '{}'
        } = req.body;

        const ticketId = uuidv4();
        const ticketNumber = await generateTicketNumber();

        // Process attachments
        const attachments = req.files ? req.files.map(file => ({
            id: uuidv4(),
            filename: file.originalname,
            path: file.path,
            size: file.size,
            mimetype: file.mimetype
        })) : [];

        // Create ticket
        const ticket = {
            id: ticketId,
            number: ticketNumber,
            subject,
            description,
            category,
            priority,
            status: TICKET_STATUS.OPEN,
            customer_id,
            customer_email,
            customer_name,
            attachments: JSON.stringify(attachments),
            metadata,
            created_at: new Date(),
            updated_at: new Date()
        };

        await storeTicket(ticket);

        // Send confirmation email
        await sendTicketConfirmationEmail(ticket);

        // Notify support agents
        await notifySupportAgents('new_ticket', ticket);

        // Update metrics
        ticketsTotal.labels(TICKET_STATUS.OPEN, priority, category).inc();

        res.status(201).json({
            ticket_id: ticketId,
            ticket_number: ticketNumber,
            status: TICKET_STATUS.OPEN,
            message: 'Support ticket created successfully'
        });

    } catch (error) {
        console.error('Error creating ticket:', error);
        res.status(500).json({ error: 'Failed to create support ticket' });
    }
});

app.get('/v1/tickets/:id', async (req, res) => {
    try {
        const { id } = req.params;
        const ticket = await getTicket(id);

        if (!ticket) {
            return res.status(404).json({ error: 'Ticket not found' });
        }

        res.json(ticket);
    } catch (error) {
        console.error('Error getting ticket:', error);
        res.status(500).json({ error: 'Failed to get ticket' });
    }
});

app.get('/v1/tickets', async (req, res) => {
    try {
        const {
            status,
            priority,
            category,
            customer_id,
            assigned_to,
            limit = 50,
            offset = 0,
            sort_by = 'created_at',
            sort_order = 'desc'
        } = req.query;

        const tickets = await listTickets({
            status,
            priority,
            category,
            customer_id,
            assigned_to,
            limit: parseInt(limit),
            offset: parseInt(offset),
            sort_by,
            sort_order
        });

        res.json({
            tickets,
            total: tickets.length,
            limit: parseInt(limit),
            offset: parseInt(offset)
        });
    } catch (error) {
        console.error('Error listing tickets:', error);
        res.status(500).json({ error: 'Failed to list tickets' });
    }
});

app.put('/v1/tickets/:id', async (req, res) => {
    try {
        const { id } = req.params;
        const updates = req.body;

        const ticket = await updateTicket(id, updates);

        if (!ticket) {
            return res.status(404).json({ error: 'Ticket not found' });
        }

        // Send update notification
        await sendTicketUpdateEmail(ticket, updates);

        // Notify relevant parties
        await notifyTicketUpdate(ticket, updates);

        res.json({
            message: 'Ticket updated successfully',
            ticket
        });
    } catch (error) {
        console.error('Error updating ticket:', error);
        res.status(500).json({ error: 'Failed to update ticket' });
    }
});

// Ticket comments/responses
app.post('/v1/tickets/:id/comments', upload.array('attachments', 3), async (req, res) => {
    try {
        const { id } = req.params;
        const { content, is_internal = false, author_id, author_name, author_email } = req.body;

        const ticket = await getTicket(id);
        if (!ticket) {
            return res.status(404).json({ error: 'Ticket not found' });
        }

        // Process attachments
        const attachments = req.files ? req.files.map(file => ({
            id: uuidv4(),
            filename: file.originalname,
            path: file.path,
            size: file.size,
            mimetype: file.mimetype
        })) : [];

        const comment = {
            id: uuidv4(),
            ticket_id: id,
            content,
            is_internal: is_internal === 'true',
            author_id,
            author_name,
            author_email,
            attachments: JSON.stringify(attachments),
            created_at: new Date()
        };

        await storeTicketComment(comment);

        // Update ticket status if needed
        if (ticket.status === TICKET_STATUS.PENDING && !comment.is_internal) {
            await updateTicket(id, { status: TICKET_STATUS.OPEN });
        }

        // Send notification email
        if (!comment.is_internal) {
            await sendCommentNotificationEmail(ticket, comment);
        }

        res.status(201).json({
            comment_id: comment.id,
            message: 'Comment added successfully'
        });

    } catch (error) {
        console.error('Error adding comment:', error);
        res.status(500).json({ error: 'Failed to add comment' });
    }
});

app.get('/v1/tickets/:id/comments', async (req, res) => {
    try {
        const { id } = req.params;
        const { include_internal = false } = req.query;

        const comments = await getTicketComments(id, include_internal === 'true');
        res.json({ comments });
    } catch (error) {
        console.error('Error getting comments:', error);
        res.status(500).json({ error: 'Failed to get comments' });
    }
});

// Knowledge base endpoints
app.post('/v1/knowledge-base/articles', async (req, res) => {
    try {
        const {
            title,
            content,
            category,
            tags = [],
            is_published = false,
            author_id
        } = req.body;

        const articleId = uuidv4();
        const article = {
            id: articleId,
            title,
            content,
            category,
            tags,
            is_published,
            author_id,
            view_count: 0,
            created_at: new Date(),
            updated_at: new Date()
        };

        await storeKnowledgeBaseArticle(article);

        res.status(201).json({
            article_id: articleId,
            message: 'Knowledge base article created successfully'
        });

    } catch (error) {
        console.error('Error creating article:', error);
        res.status(500).json({ error: 'Failed to create article' });
    }
});

app.get('/v1/knowledge-base/articles', async (req, res) => {
    try {
        const {
            category,
            tag,
            search,
            is_published = true,
            limit = 20,
            offset = 0
        } = req.query;

        const articles = await searchKnowledgeBaseArticles({
            category,
            tag,
            search,
            is_published: is_published === 'true',
            limit: parseInt(limit),
            offset: parseInt(offset)
        });

        res.json({
            articles,
            total: articles.length,
            limit: parseInt(limit),
            offset: parseInt(offset)
        });
    } catch (error) {
        console.error('Error searching articles:', error);
        res.status(500).json({ error: 'Failed to search articles' });
    }
});

app.get('/v1/knowledge-base/articles/:id', async (req, res) => {
    try {
        const { id } = req.params;
        const article = await getKnowledgeBaseArticle(id);

        if (!article) {
            return res.status(404).json({ error: 'Article not found' });
        }

        // Increment view count
        await incrementArticleViews(id);
        knowledgeBaseViews.labels(id).inc();

        res.json(article);
    } catch (error) {
        console.error('Error getting article:', error);
        res.status(500).json({ error: 'Failed to get article' });
    }
});

// Live chat endpoints
app.post('/v1/chat/sessions', async (req, res) => {
    try {
        const {
            customer_id,
            customer_name,
            customer_email,
            initial_message,
            metadata = '{}'
        } = req.body;

        const sessionId = uuidv4();
        const session = {
            id: sessionId,
            customer_id,
            customer_name,
            customer_email,
            status: CHAT_STATUS.WAITING,
            initial_message,
            metadata,
            started_at: new Date(),
            created_at: new Date()
        };

        await storeChatSession(session);

        // Add to active sessions
        activeChatSessions.set(sessionId, session);

        // Notify available agents
        await notifyAvailableAgents('new_chat_request', session);

        // Update metrics
        chatSessionsActive.inc();

        res.status(201).json({
            session_id: sessionId,
            status: CHAT_STATUS.WAITING,
            message: 'Chat session created successfully'
        });

    } catch (error) {
        console.error('Error creating chat session:', error);
        res.status(500).json({ error: 'Failed to create chat session' });
    }
});

app.get('/v1/chat/sessions/:id', async (req, res) => {
    try {
        const { id } = req.params;
        const session = await getChatSession(id);

        if (!session) {
            return res.status(404).json({ error: 'Chat session not found' });
        }

        res.json(session);
    } catch (error) {
        console.error('Error getting chat session:', error);
        res.status(500).json({ error: 'Failed to get chat session' });
    }
});

// Feedback endpoints
app.post('/v1/feedback', async (req, res) => {
    try {
        const {
            type,
            rating,
            comment,
            ticket_id,
            customer_id,
            customer_email,
            metadata = '{}'
        } = req.body;

        const feedbackId = uuidv4();
        const feedback = {
            id: feedbackId,
            type,
            rating,
            comment,
            ticket_id,
            customer_id,
            customer_email,
            metadata,
            created_at: new Date()
        };

        await storeFeedback(feedback);

        // Update satisfaction metrics
        if (rating) {
            await updateSatisfactionMetrics(rating);
        }

        res.status(201).json({
            feedback_id: feedbackId,
            message: 'Feedback submitted successfully'
        });

    } catch (error) {
        console.error('Error submitting feedback:', error);
        res.status(500).json({ error: 'Failed to submit feedback' });
    }
});

// WebSocket connection handling for live chat
io.on('connection', (socket) => {
    console.log('New WebSocket connection:', socket.id);
    activeConnections.set(socket.id, {
        socket,
        user_id: null,
        user_type: null, // 'customer' or 'agent'
        session_id: null,
        connected_at: new Date()
    });

    socket.on('authenticate', async (data) => {
        const { user_id, user_type, session_id, token } = data;
        
        // In production, validate the token
        const connection = activeConnections.get(socket.id);
        if (connection) {
            connection.user_id = user_id;
            connection.user_type = user_type;
            connection.session_id = session_id;

            if (user_type === 'agent') {
                supportAgents.set(user_id, {
                    socket_id: socket.id,
                    user_id,
                    status: 'available',
                    current_sessions: []
                });
                socket.join('support_agents');
            } else if (user_type === 'customer' && session_id) {
                socket.join(`chat_session_${session_id}`);
            }

            console.log(`User ${user_id} (${user_type}) authenticated on socket ${socket.id}`);
        }
    });

    socket.on('join_chat_session', async (data) => {
        const { session_id, user_type } = data;
        const connection = activeConnections.get(socket.id);
        
        if (connection && session_id) {
            socket.join(`chat_session_${session_id}`);
            connection.session_id = session_id;

            if (user_type === 'agent') {
                // Agent joining session
                await updateChatSession(session_id, { 
                    status: CHAT_STATUS.ACTIVE,
                    agent_id: connection.user_id,
                    agent_joined_at: new Date()
                });

                // Notify customer that agent joined
                socket.to(`chat_session_${session_id}`).emit('agent_joined', {
                    agent_id: connection.user_id,
                    timestamp: new Date()
                });
            }

            console.log(`User joined chat session: ${session_id}`);
        }
    });

    socket.on('send_message', async (data) => {
        const { session_id, message, message_type = 'text' } = data;
        const connection = activeConnections.get(socket.id);

        if (connection && session_id && message) {
            const chatMessage = {
                id: uuidv4(),
                session_id,
                sender_id: connection.user_id,
                sender_type: connection.user_type,
                message,
                message_type,
                timestamp: new Date()
            };

            // Store message
            await storeChatMessage(chatMessage);

            // Broadcast to session participants
            io.to(`chat_session_${session_id}`).emit('new_message', chatMessage);

            console.log(`Message sent in session ${session_id} by ${connection.user_id}`);
        }
    });

    socket.on('end_chat_session', async (data) => {
        const { session_id } = data;
        const connection = activeConnections.get(socket.id);

        if (connection && session_id) {
            await updateChatSession(session_id, {
                status: CHAT_STATUS.ENDED,
                ended_at: new Date(),
                ended_by: connection.user_id
            });

            // Notify all participants
            io.to(`chat_session_${session_id}`).emit('session_ended', {
                ended_by: connection.user_id,
                timestamp: new Date()
            });

            // Remove from active sessions
            activeChatSessions.delete(session_id);
            chatSessionsActive.dec();

            console.log(`Chat session ${session_id} ended by ${connection.user_id}`);
        }
    });

    socket.on('agent_status_update', (data) => {
        const { status } = data;
        const connection = activeConnections.get(socket.id);

        if (connection && connection.user_type === 'agent') {
            const agent = supportAgents.get(connection.user_id);
            if (agent) {
                agent.status = status;
                console.log(`Agent ${connection.user_id} status updated to: ${status}`);
            }
        }
    });

    socket.on('disconnect', () => {
        console.log('WebSocket disconnected:', socket.id);
        const connection = activeConnections.get(socket.id);
        
        if (connection) {
            if (connection.user_type === 'agent' && connection.user_id) {
                supportAgents.delete(connection.user_id);
            }
            activeConnections.delete(socket.id);
        }
    });
});

// Database functions
async function storeTicket(ticket) {
    const query = `
        INSERT INTO tickets (
            id, number, subject, description, category, priority, status,
            customer_id, customer_email, customer_name, attachments, metadata,
            created_at, updated_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
    `;

    await pgPool.query(query, [
        ticket.id, ticket.number, ticket.subject, ticket.description,
        ticket.category, ticket.priority, ticket.status, ticket.customer_id,
        ticket.customer_email, ticket.customer_name, ticket.attachments,
        ticket.metadata, ticket.created_at, ticket.updated_at
    ]);
}

async function getTicket(id) {
    const query = 'SELECT * FROM tickets WHERE id = $1';
    const result = await pgPool.query(query, [id]);
    return result.rows[0];
}

async function generateTicketNumber() {
    const today = new Date();
    const dateStr = today.toISOString().slice(0, 10).replace(/-/g, '');
    
    // Get count of tickets created today
    const countQuery = `
        SELECT COUNT(*) as count 
        FROM tickets 
        WHERE DATE(created_at) = CURRENT_DATE
    `;
    const result = await pgPool.query(countQuery);
    const count = parseInt(result.rows[0].count) + 1;
    
    return `TKT-${dateStr}-${count.toString().padStart(4, '0')}`;
}

// Initialize database tables
async function initializeDatabase() {
    const createTables = `
        CREATE TABLE IF NOT EXISTS tickets (
            id UUID PRIMARY KEY,
            number VARCHAR(50) UNIQUE NOT NULL,
            subject VARCHAR(500) NOT NULL,
            description TEXT NOT NULL,
            category VARCHAR(50) NOT NULL,
            priority VARCHAR(20) NOT NULL,
            status VARCHAR(20) NOT NULL,
            customer_id VARCHAR(255),
            customer_email VARCHAR(255) NOT NULL,
            customer_name VARCHAR(255),
            assigned_to VARCHAR(255),
            attachments JSONB DEFAULT '[]',
            metadata JSONB DEFAULT '{}',
            first_response_at TIMESTAMP,
            resolved_at TIMESTAMP,
            closed_at TIMESTAMP,
            created_at TIMESTAMP DEFAULT NOW(),
            updated_at TIMESTAMP DEFAULT NOW()
        );

        CREATE TABLE IF NOT EXISTS ticket_comments (
            id UUID PRIMARY KEY,
            ticket_id UUID NOT NULL REFERENCES tickets(id),
            content TEXT NOT NULL,
            is_internal BOOLEAN DEFAULT FALSE,
            author_id VARCHAR(255),
            author_name VARCHAR(255),
            author_email VARCHAR(255),
            attachments JSONB DEFAULT '[]',
            created_at TIMESTAMP DEFAULT NOW()
        );

        CREATE TABLE IF NOT EXISTS knowledge_base_articles (
            id UUID PRIMARY KEY,
            title VARCHAR(500) NOT NULL,
            content TEXT NOT NULL,
            category VARCHAR(100),
            tags TEXT[] DEFAULT '{}',
            is_published BOOLEAN DEFAULT FALSE,
            author_id VARCHAR(255) NOT NULL,
            view_count INTEGER DEFAULT 0,
            created_at TIMESTAMP DEFAULT NOW(),
            updated_at TIMESTAMP DEFAULT NOW()
        );

        CREATE TABLE IF NOT EXISTS chat_sessions (
            id UUID PRIMARY KEY,
            customer_id VARCHAR(255),
            customer_name VARCHAR(255),
            customer_email VARCHAR(255),
            agent_id VARCHAR(255),
            status VARCHAR(20) NOT NULL,
            initial_message TEXT,
            metadata JSONB DEFAULT '{}',
            started_at TIMESTAMP,
            agent_joined_at TIMESTAMP,
            ended_at TIMESTAMP,
            ended_by VARCHAR(255),
            created_at TIMESTAMP DEFAULT NOW()
        );

        CREATE TABLE IF NOT EXISTS chat_messages (
            id UUID PRIMARY KEY,
            session_id UUID NOT NULL REFERENCES chat_sessions(id),
            sender_id VARCHAR(255) NOT NULL,
            sender_type VARCHAR(20) NOT NULL,
            message TEXT NOT NULL,
            message_type VARCHAR(20) DEFAULT 'text',
            timestamp TIMESTAMP DEFAULT NOW()
        );

        CREATE TABLE IF NOT EXISTS feedback (
            id UUID PRIMARY KEY,
            type VARCHAR(50),
            rating INTEGER,
            comment TEXT,
            ticket_id UUID REFERENCES tickets(id),
            customer_id VARCHAR(255),
            customer_email VARCHAR(255),
            metadata JSONB DEFAULT '{}',
            created_at TIMESTAMP DEFAULT NOW()
        );

        CREATE INDEX IF NOT EXISTS idx_tickets_status ON tickets(status);
        CREATE INDEX IF NOT EXISTS idx_tickets_priority ON tickets(priority);
        CREATE INDEX IF NOT EXISTS idx_tickets_customer_id ON tickets(customer_id);
        CREATE INDEX IF NOT EXISTS idx_tickets_created_at ON tickets(created_at);
        CREATE INDEX IF NOT EXISTS idx_ticket_comments_ticket_id ON ticket_comments(ticket_id);
        CREATE INDEX IF NOT EXISTS idx_knowledge_base_published ON knowledge_base_articles(is_published);
        CREATE INDEX IF NOT EXISTS idx_chat_sessions_status ON chat_sessions(status);
        CREATE INDEX IF NOT EXISTS idx_chat_messages_session_id ON chat_messages(session_id);
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
        
        // Start periodic tasks
        startPeriodicTasks();
        
        // Start server
        server.listen(PORT, () => {
            console.log(`🚀 Support Service running on port ${PORT}`);
            console.log(`📊 Health check: http://localhost:${PORT}/health`);
            console.log(`📈 Metrics: http://localhost:${PORT}/metrics`);
            console.log(`💬 WebSocket server ready for live chat`);
        });
    } catch (error) {
        console.error('Failed to start server:', error);
        process.exit(1);
    }
}

// Start periodic tasks
function startPeriodicTasks() {
    // Update metrics every 5 minutes
    setInterval(async () => {
        await updateSupportMetrics();
    }, 5 * 60 * 1000);

    // Clean up old chat sessions every hour
    setInterval(async () => {
        await cleanupOldChatSessions();
    }, 60 * 60 * 1000);
}

// Graceful shutdown
process.on('SIGTERM', async () => {
    console.log('Shutting down support service...');
    server.close();
    await redis.quit();
    await pgPool.end();
    process.exit(0);
});

startServer();
