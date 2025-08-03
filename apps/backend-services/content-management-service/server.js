/**
 * Content Management Service - Node.js
 * Digital asset and content management for the 002AIC platform
 * Handles content creation, versioning, publishing, and digital asset management
 */

const express = require('express');
const cors = require('cors');
const helmet = require('helmet');
const rateLimit = require('express-rate-limit');
const { v4: uuidv4 } = require('uuid');
const multer = require('multer');
const path = require('path');
const fs = require('fs').promises;
const sharp = require('sharp');
const Redis = require('redis');
const { Pool } = require('pg');
const prometheus = require('prom-client');
const slugify = require('slugify');
const marked = require('marked');
const DOMPurify = require('isomorphic-dompurify');

// Initialize Express app
const app = express();
const PORT = process.env.PORT || 8080;

// Configuration
const config = {
    port: PORT,
    databaseUrl: process.env.DATABASE_URL || 'postgresql://postgres:password@localhost:5432/content_management',
    redisUrl: process.env.REDIS_URL || 'redis://localhost:6379',
    uploadPath: process.env.UPLOAD_PATH || './uploads',
    maxFileSize: parseInt(process.env.MAX_FILE_SIZE) || 50 * 1024 * 1024, // 50MB
    allowedImageTypes: ['image/jpeg', 'image/png', 'image/gif', 'image/webp'],
    allowedDocumentTypes: ['application/pdf', 'text/plain', 'application/msword'],
    environment: process.env.ENVIRONMENT || 'development'
};

// Initialize external services
const redis = Redis.createClient({ url: config.redisUrl });
const pgPool = new Pool({ connectionString: config.databaseUrl });

// Prometheus metrics
const register = new prometheus.Registry();
prometheus.collectDefaultMetrics({ register });

const contentItemsTotal = new prometheus.Gauge({
    name: 'content_items_total',
    help: 'Total number of content items',
    labelNames: ['type', 'status'],
    registers: [register]
});

const contentOperations = new prometheus.Counter({
    name: 'content_operations_total',
    help: 'Total content operations',
    labelNames: ['operation', 'type', 'status'],
    registers: [register]
});

const assetUploads = new prometheus.Counter({
    name: 'asset_uploads_total',
    help: 'Total asset uploads',
    labelNames: ['type', 'status'],
    registers: [register]
});

const contentViews = new prometheus.Counter({
    name: 'content_views_total',
    help: 'Total content views',
    labelNames: ['content_id', 'type'],
    registers: [register]
});

const assetStorage = new prometheus.Gauge({
    name: 'asset_storage_bytes',
    help: 'Total asset storage in bytes',
    labelNames: ['type'],
    registers: [register]
});

// Content types
const CONTENT_TYPES = {
    ARTICLE: 'article',
    PAGE: 'page',
    BLOG_POST: 'blog_post',
    DOCUMENTATION: 'documentation',
    TUTORIAL: 'tutorial',
    FAQ: 'faq',
    ANNOUNCEMENT: 'announcement',
    TEMPLATE: 'template'
};

// Content status
const CONTENT_STATUS = {
    DRAFT: 'draft',
    REVIEW: 'review',
    PUBLISHED: 'published',
    ARCHIVED: 'archived',
    DELETED: 'deleted'
};

// Asset types
const ASSET_TYPES = {
    IMAGE: 'image',
    DOCUMENT: 'document',
    VIDEO: 'video',
    AUDIO: 'audio',
    ARCHIVE: 'archive'
};

// File upload configuration
const storage = multer.diskStorage({
    destination: async (req, file, cb) => {
        const uploadDir = path.join(config.uploadPath, 'assets');
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
    limits: { fileSize: config.maxFileSize },
    fileFilter: (req, file, cb) => {
        const allowedTypes = [
            ...config.allowedImageTypes,
            ...config.allowedDocumentTypes,
            'video/mp4',
            'audio/mpeg',
            'application/zip'
        ];
        
        if (allowedTypes.includes(file.mimetype)) {
            cb(null, true);
        } else {
            cb(new Error('Invalid file type'), false);
        }
    }
});

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
        service: 'content-management-service',
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

    const statusCode = status.status === 'healthy' ? 200 : 503;
    res.status(statusCode).json(status);
});

// Metrics endpoint
app.get('/metrics', (req, res) => {
    res.set('Content-Type', register.contentType);
    res.end(register.metrics());
});

// Content management endpoints
app.post('/v1/content', async (req, res) => {
    try {
        const {
            title,
            content,
            type = CONTENT_TYPES.ARTICLE,
            status = CONTENT_STATUS.DRAFT,
            author_id,
            category,
            tags = [],
            metadata = {},
            seo_title,
            seo_description,
            featured_image,
            publish_at
        } = req.body;

        // Validate required fields
        if (!title || !content || !author_id) {
            return res.status(400).json({ 
                error: 'Title, content, and author_id are required' 
            });
        }

        const contentId = uuidv4();
        const slug = slugify(title, { lower: true, strict: true });

        // Process content based on type
        let processedContent = content;
        if (type === CONTENT_TYPES.ARTICLE || type === CONTENT_TYPES.BLOG_POST) {
            // Convert markdown to HTML and sanitize
            processedContent = DOMPurify.sanitize(marked.parse(content));
        }

        // Create content item
        const contentItem = {
            id: contentId,
            title,
            slug,
            content: processedContent,
            raw_content: content,
            type,
            status,
            author_id,
            category,
            tags,
            metadata: JSON.stringify(metadata),
            seo_title: seo_title || title,
            seo_description,
            featured_image,
            publish_at: publish_at ? new Date(publish_at) : null,
            version: 1,
            view_count: 0,
            created_at: new Date(),
            updated_at: new Date()
        };

        await storeContentItem(contentItem);

        // Update metrics
        contentItemsTotal.labels(type, status).inc();
        contentOperations.labels('create', type, 'success').inc();

        res.status(201).json({
            content_id: contentId,
            slug,
            status,
            message: 'Content created successfully'
        });

    } catch (error) {
        console.error('Error creating content:', error);
        contentOperations.labels('create', 'unknown', 'error').inc();
        res.status(500).json({ error: 'Failed to create content' });
    }
});

app.get('/v1/content/:id', async (req, res) => {
    try {
        const { id } = req.params;
        const { increment_views = false } = req.query;

        const content = await getContentItem(id);
        if (!content) {
            return res.status(404).json({ error: 'Content not found' });
        }

        // Increment view count if requested
        if (increment_views === 'true') {
            await incrementViewCount(id);
            contentViews.labels(id, content.type).inc();
            content.view_count += 1;
        }

        res.json(content);
    } catch (error) {
        console.error('Error getting content:', error);
        res.status(500).json({ error: 'Failed to get content' });
    }
});

app.get('/v1/content', async (req, res) => {
    try {
        const {
            type,
            status,
            category,
            author_id,
            tag,
            search,
            limit = 20,
            offset = 0,
            sort_by = 'created_at',
            sort_order = 'desc'
        } = req.query;

        const content = await listContent({
            type,
            status,
            category,
            author_id,
            tag,
            search,
            limit: parseInt(limit),
            offset: parseInt(offset),
            sort_by,
            sort_order
        });

        res.json({
            content,
            total: content.length,
            limit: parseInt(limit),
            offset: parseInt(offset)
        });
    } catch (error) {
        console.error('Error listing content:', error);
        res.status(500).json({ error: 'Failed to list content' });
    }
});

app.put('/v1/content/:id', async (req, res) => {
    try {
        const { id } = req.params;
        const updates = req.body;

        const content = await getContentItem(id);
        if (!content) {
            return res.status(404).json({ error: 'Content not found' });
        }

        // Process content if updated
        if (updates.content) {
            if (content.type === CONTENT_TYPES.ARTICLE || content.type === CONTENT_TYPES.BLOG_POST) {
                updates.processed_content = DOMPurify.sanitize(marked.parse(updates.content));
                updates.raw_content = updates.content;
                updates.content = updates.processed_content;
            }
        }

        // Update slug if title changed
        if (updates.title && updates.title !== content.title) {
            updates.slug = slugify(updates.title, { lower: true, strict: true });
        }

        // Increment version
        updates.version = content.version + 1;
        updates.updated_at = new Date();

        const updatedContent = await updateContentItem(id, updates);

        contentOperations.labels('update', content.type, 'success').inc();

        res.json({
            message: 'Content updated successfully',
            content: updatedContent
        });
    } catch (error) {
        console.error('Error updating content:', error);
        contentOperations.labels('update', 'unknown', 'error').inc();
        res.status(500).json({ error: 'Failed to update content' });
    }
});

app.delete('/v1/content/:id', async (req, res) => {
    try {
        const { id } = req.params;
        const { permanent = false } = req.query;

        const content = await getContentItem(id);
        if (!content) {
            return res.status(404).json({ error: 'Content not found' });
        }

        if (permanent === 'true') {
            await deleteContentItem(id);
        } else {
            await updateContentItem(id, { 
                status: CONTENT_STATUS.DELETED,
                updated_at: new Date()
            });
        }

        contentOperations.labels('delete', content.type, 'success').inc();

        res.json({
            message: permanent === 'true' ? 'Content permanently deleted' : 'Content moved to trash'
        });
    } catch (error) {
        console.error('Error deleting content:', error);
        contentOperations.labels('delete', 'unknown', 'error').inc();
        res.status(500).json({ error: 'Failed to delete content' });
    }
});

// Content publishing
app.post('/v1/content/:id/publish', async (req, res) => {
    try {
        const { id } = req.params;
        const { publish_at } = req.body;

        const content = await getContentItem(id);
        if (!content) {
            return res.status(404).json({ error: 'Content not found' });
        }

        const updates = {
            status: CONTENT_STATUS.PUBLISHED,
            published_at: publish_at ? new Date(publish_at) : new Date(),
            updated_at: new Date()
        };

        await updateContentItem(id, updates);

        contentOperations.labels('publish', content.type, 'success').inc();

        res.json({
            message: 'Content published successfully',
            published_at: updates.published_at
        });
    } catch (error) {
        console.error('Error publishing content:', error);
        contentOperations.labels('publish', 'unknown', 'error').inc();
        res.status(500).json({ error: 'Failed to publish content' });
    }
});

// Asset management endpoints
app.post('/v1/assets/upload', upload.single('file'), async (req, res) => {
    try {
        if (!req.file) {
            return res.status(400).json({ error: 'No file uploaded' });
        }

        const {
            title,
            description,
            alt_text,
            tags = [],
            metadata = {}
        } = req.body;

        const assetId = uuidv4();
        const assetType = getAssetType(req.file.mimetype);

        // Process image if it's an image file
        let processedPath = req.file.path;
        let thumbnailPath = null;

        if (assetType === ASSET_TYPES.IMAGE) {
            // Generate thumbnail
            thumbnailPath = path.join(path.dirname(req.file.path), `thumb_${req.file.filename}`);
            await sharp(req.file.path)
                .resize(300, 300, { fit: 'inside', withoutEnlargement: true })
                .jpeg({ quality: 80 })
                .toFile(thumbnailPath);

            // Optimize original image
            await sharp(req.file.path)
                .jpeg({ quality: 85 })
                .toFile(processedPath + '_optimized');
            
            // Replace original with optimized version
            await fs.rename(processedPath + '_optimized', processedPath);
        }

        // Create asset record
        const asset = {
            id: assetId,
            title: title || req.file.originalname,
            description,
            filename: req.file.filename,
            original_filename: req.file.originalname,
            file_path: processedPath,
            thumbnail_path: thumbnailPath,
            mime_type: req.file.mimetype,
            file_size: req.file.size,
            type: assetType,
            alt_text,
            tags: Array.isArray(tags) ? tags : tags.split(','),
            metadata: typeof metadata === 'string' ? JSON.parse(metadata) : metadata,
            upload_date: new Date(),
            created_at: new Date()
        };

        await storeAsset(asset);

        // Update metrics
        assetUploads.labels(assetType, 'success').inc();
        assetStorage.labels(assetType).add(req.file.size);

        res.status(201).json({
            asset_id: assetId,
            filename: req.file.filename,
            type: assetType,
            size: req.file.size,
            url: `/v1/assets/${assetId}/download`,
            thumbnail_url: thumbnailPath ? `/v1/assets/${assetId}/thumbnail` : null,
            message: 'Asset uploaded successfully'
        });

    } catch (error) {
        console.error('Error uploading asset:', error);
        assetUploads.labels('unknown', 'error').inc();
        res.status(500).json({ error: 'Failed to upload asset' });
    }
});

app.get('/v1/assets/:id', async (req, res) => {
    try {
        const { id } = req.params;
        const asset = await getAsset(id);

        if (!asset) {
            return res.status(404).json({ error: 'Asset not found' });
        }

        res.json(asset);
    } catch (error) {
        console.error('Error getting asset:', error);
        res.status(500).json({ error: 'Failed to get asset' });
    }
});

app.get('/v1/assets/:id/download', async (req, res) => {
    try {
        const { id } = req.params;
        const asset = await getAsset(id);

        if (!asset) {
            return res.status(404).json({ error: 'Asset not found' });
        }

        // Check if file exists
        try {
            await fs.access(asset.file_path);
        } catch {
            return res.status(404).json({ error: 'Asset file not found' });
        }

        res.setHeader('Content-Type', asset.mime_type);
        res.setHeader('Content-Disposition', `attachment; filename="${asset.original_filename}"`);
        res.sendFile(path.resolve(asset.file_path));

    } catch (error) {
        console.error('Error downloading asset:', error);
        res.status(500).json({ error: 'Failed to download asset' });
    }
});

app.get('/v1/assets/:id/thumbnail', async (req, res) => {
    try {
        const { id } = req.params;
        const asset = await getAsset(id);

        if (!asset || !asset.thumbnail_path) {
            return res.status(404).json({ error: 'Thumbnail not found' });
        }

        // Check if thumbnail exists
        try {
            await fs.access(asset.thumbnail_path);
        } catch {
            return res.status(404).json({ error: 'Thumbnail file not found' });
        }

        res.setHeader('Content-Type', 'image/jpeg');
        res.sendFile(path.resolve(asset.thumbnail_path));

    } catch (error) {
        console.error('Error getting thumbnail:', error);
        res.status(500).json({ error: 'Failed to get thumbnail' });
    }
});

app.get('/v1/assets', async (req, res) => {
    try {
        const {
            type,
            tag,
            search,
            limit = 20,
            offset = 0,
            sort_by = 'created_at',
            sort_order = 'desc'
        } = req.query;

        const assets = await listAssets({
            type,
            tag,
            search,
            limit: parseInt(limit),
            offset: parseInt(offset),
            sort_by,
            sort_order
        });

        res.json({
            assets,
            total: assets.length,
            limit: parseInt(limit),
            offset: parseInt(offset)
        });
    } catch (error) {
        console.error('Error listing assets:', error);
        res.status(500).json({ error: 'Failed to list assets' });
    }
});

// Content versioning
app.get('/v1/content/:id/versions', async (req, res) => {
    try {
        const { id } = req.params;
        const versions = await getContentVersions(id);

        res.json({
            content_id: id,
            versions,
            count: versions.length
        });
    } catch (error) {
        console.error('Error getting content versions:', error);
        res.status(500).json({ error: 'Failed to get content versions' });
    }
});

// Content analytics
app.get('/v1/analytics/content', async (req, res) => {
    try {
        const {
            start_date,
            end_date,
            type,
            author_id
        } = req.query;

        const analytics = await getContentAnalytics({
            start_date,
            end_date,
            type,
            author_id
        });

        res.json(analytics);
    } catch (error) {
        console.error('Error getting content analytics:', error);
        res.status(500).json({ error: 'Failed to get analytics' });
    }
});

// Database functions
async function storeContentItem(contentItem) {
    const query = `
        INSERT INTO content_items (
            id, title, slug, content, raw_content, type, status, author_id,
            category, tags, metadata, seo_title, seo_description, featured_image,
            publish_at, version, view_count, created_at, updated_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
    `;

    await pgPool.query(query, [
        contentItem.id, contentItem.title, contentItem.slug, contentItem.content,
        contentItem.raw_content, contentItem.type, contentItem.status, contentItem.author_id,
        contentItem.category, contentItem.tags, contentItem.metadata, contentItem.seo_title,
        contentItem.seo_description, contentItem.featured_image, contentItem.publish_at,
        contentItem.version, contentItem.view_count, contentItem.created_at, contentItem.updated_at
    ]);
}

async function getContentItem(id) {
    const query = 'SELECT * FROM content_items WHERE id = $1 AND status != $2';
    const result = await pgPool.query(query, [id, CONTENT_STATUS.DELETED]);
    return result.rows[0];
}

async function storeAsset(asset) {
    const query = `
        INSERT INTO assets (
            id, title, description, filename, original_filename, file_path,
            thumbnail_path, mime_type, file_size, type, alt_text, tags,
            metadata, upload_date, created_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
    `;

    await pgPool.query(query, [
        asset.id, asset.title, asset.description, asset.filename,
        asset.original_filename, asset.file_path, asset.thumbnail_path,
        asset.mime_type, asset.file_size, asset.type, asset.alt_text,
        asset.tags, JSON.stringify(asset.metadata), asset.upload_date, asset.created_at
    ]);
}

async function getAsset(id) {
    const query = 'SELECT * FROM assets WHERE id = $1';
    const result = await pgPool.query(query, [id]);
    return result.rows[0];
}

function getAssetType(mimeType) {
    if (config.allowedImageTypes.includes(mimeType)) {
        return ASSET_TYPES.IMAGE;
    } else if (config.allowedDocumentTypes.includes(mimeType)) {
        return ASSET_TYPES.DOCUMENT;
    } else if (mimeType.startsWith('video/')) {
        return ASSET_TYPES.VIDEO;
    } else if (mimeType.startsWith('audio/')) {
        return ASSET_TYPES.AUDIO;
    } else {
        return ASSET_TYPES.ARCHIVE;
    }
}

// Initialize database tables
async function initializeDatabase() {
    const createTables = `
        CREATE TABLE IF NOT EXISTS content_items (
            id UUID PRIMARY KEY,
            title VARCHAR(500) NOT NULL,
            slug VARCHAR(500) UNIQUE NOT NULL,
            content TEXT NOT NULL,
            raw_content TEXT NOT NULL,
            type VARCHAR(50) NOT NULL,
            status VARCHAR(20) NOT NULL DEFAULT 'draft',
            author_id VARCHAR(255) NOT NULL,
            category VARCHAR(100),
            tags TEXT[] DEFAULT '{}',
            metadata JSONB DEFAULT '{}',
            seo_title VARCHAR(500),
            seo_description TEXT,
            featured_image VARCHAR(500),
            publish_at TIMESTAMP,
            published_at TIMESTAMP,
            version INTEGER DEFAULT 1,
            view_count INTEGER DEFAULT 0,
            created_at TIMESTAMP DEFAULT NOW(),
            updated_at TIMESTAMP DEFAULT NOW()
        );

        CREATE TABLE IF NOT EXISTS assets (
            id UUID PRIMARY KEY,
            title VARCHAR(500) NOT NULL,
            description TEXT,
            filename VARCHAR(500) NOT NULL,
            original_filename VARCHAR(500) NOT NULL,
            file_path VARCHAR(1000) NOT NULL,
            thumbnail_path VARCHAR(1000),
            mime_type VARCHAR(100) NOT NULL,
            file_size BIGINT NOT NULL,
            type VARCHAR(50) NOT NULL,
            alt_text VARCHAR(500),
            tags TEXT[] DEFAULT '{}',
            metadata JSONB DEFAULT '{}',
            upload_date TIMESTAMP DEFAULT NOW(),
            created_at TIMESTAMP DEFAULT NOW()
        );

        CREATE TABLE IF NOT EXISTS content_versions (
            id UUID PRIMARY KEY,
            content_id UUID NOT NULL REFERENCES content_items(id),
            version INTEGER NOT NULL,
            title VARCHAR(500) NOT NULL,
            content TEXT NOT NULL,
            raw_content TEXT NOT NULL,
            changes_summary TEXT,
            created_by VARCHAR(255) NOT NULL,
            created_at TIMESTAMP DEFAULT NOW(),
            UNIQUE(content_id, version)
        );

        CREATE INDEX IF NOT EXISTS idx_content_items_type ON content_items(type);
        CREATE INDEX IF NOT EXISTS idx_content_items_status ON content_items(status);
        CREATE INDEX IF NOT EXISTS idx_content_items_author_id ON content_items(author_id);
        CREATE INDEX IF NOT EXISTS idx_content_items_created_at ON content_items(created_at);
        CREATE INDEX IF NOT EXISTS idx_assets_type ON assets(type);
        CREATE INDEX IF NOT EXISTS idx_assets_tags ON assets USING GIN(tags);
        CREATE INDEX IF NOT EXISTS idx_content_versions_content_id ON content_versions(content_id);
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
        
        // Create upload directories
        await fs.mkdir(config.uploadPath, { recursive: true });
        
        // Start periodic tasks
        startPeriodicTasks();
        
        // Start server
        app.listen(PORT, () => {
            console.log(`🚀 Content Management Service running on port ${PORT}`);
            console.log(`📊 Health check: http://localhost:${PORT}/health`);
            console.log(`📈 Metrics: http://localhost:${PORT}/metrics`);
            console.log(`📁 Upload path: ${config.uploadPath}`);
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
        await updateContentMetrics();
    }, 5 * 60 * 1000);

    // Clean up temporary files every hour
    setInterval(async () => {
        await cleanupTempFiles();
    }, 60 * 60 * 1000);
}

// Graceful shutdown
process.on('SIGTERM', async () => {
    console.log('Shutting down content management service...');
    app.close();
    await redis.quit();
    await pgPool.end();
    process.exit(0);
});

startServer();
