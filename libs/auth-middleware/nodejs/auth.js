/**
 * AI Services Authentication and Authorization Middleware for Node.js
 * Supports Express.js and other Node.js frameworks
 */

const jwt = require('jsonwebtoken');
const axios = require('axios');

class AuthConfig {
    constructor({
        authorizationServiceUrl,
        jwtPublicKeyUrl,
        jwtIssuer,
        jwtAudience,
        serviceName,
        timeout = 10000
    }) {
        this.authorizationServiceUrl = authorizationServiceUrl;
        this.jwtPublicKeyUrl = jwtPublicKeyUrl;
        this.jwtIssuer = jwtIssuer;
        this.jwtAudience = jwtAudience;
        this.serviceName = serviceName;
        this.timeout = timeout;
    }
}

class UserContext {
    constructor({ userId, username, email, roles = [] }) {
        this.userId = userId;
        this.username = username;
        this.email = email;
        this.roles = roles;
    }
}

class AuthorizationRequest {
    constructor({ userId, resource, action, context = {} }) {
        this.user_id = userId;
        this.resource = resource;
        this.action = action;
        this.context = context;
    }
}

class AuthorizationResponse {
    constructor({ allowed, reason = null }) {
        this.allowed = allowed;
        this.reason = reason;
    }
}

class AuthMiddleware {
    constructor(config) {
        this.config = config;
        this.httpClient = axios.create({
            timeout: config.timeout,
            headers: {
                'Content-Type': 'application/json',
                'X-Service-Name': config.serviceName
            }
        });
    }

    /**
     * Validate JWT token and extract user context
     * @param {string} token - JWT token
     * @returns {UserContext} User context
     */
    validateJWT(token) {
        // Remove Bearer prefix if present
        token = token.replace('Bearer ', '');

        try {
            // In production, verify with Keycloak's public key
            // For now, decode without verification for demo
            const decoded = jwt.decode(token);
            
            if (!decoded) {
                throw new Error('Invalid JWT token');
            }

            const userContext = new UserContext({
                userId: decoded.sub || '',
                username: decoded.preferred_username || '',
                email: decoded.email || '',
                roles: decoded.realm_access?.roles || []
            });

            return userContext;
        } catch (error) {
            throw new Error(`Invalid JWT token: ${error.message}`);
        }
    }

    /**
     * Check if user has permission for resource and action
     * @param {string} userId - User ID
     * @param {string} resource - Resource name
     * @param {string} action - Action name
     * @param {Object} context - Additional context
     * @returns {Promise<AuthorizationResponse>} Authorization response
     */
    async checkPermission(userId, resource, action, context = {}) {
        const request = new AuthorizationRequest({
            userId,
            resource,
            action,
            context
        });

        try {
            const response = await this.httpClient.post(
                `${this.config.authorizationServiceUrl}/v1/auth/check`,
                request
            );

            return new AuthorizationResponse({
                allowed: response.data.allowed || false,
                reason: response.data.reason
            });
        } catch (error) {
            return new AuthorizationResponse({
                allowed: false,
                reason: `Authorization service error: ${error.message}`
            });
        }
    }

    /**
     * Express.js middleware for authentication and authorization
     * @param {string} resource - Resource name
     * @param {string} action - Action name
     * @returns {Function} Express middleware function
     */
    requireAuth(resource, action) {
        return async (req, res, next) => {
            try {
                // Extract JWT from Authorization header
                const authHeader = req.headers.authorization;
                if (!authHeader) {
                    return res.status(401).json({ error: 'Authorization header required' });
                }

                // Validate JWT and extract user context
                const userContext = this.validateJWT(authHeader);

                // Check permission
                const authResponse = await this.checkPermission(
                    userContext.userId,
                    resource,
                    action
                );

                if (!authResponse.allowed) {
                    return res.status(403).json({
                        error: `Access denied: ${authResponse.reason}`
                    });
                }

                // Add user context to request
                req.user = userContext;

                // Add user headers for downstream services
                req.headers['x-user-id'] = userContext.userId;
                req.headers['x-user-email'] = userContext.email;
                req.headers['x-user-roles'] = userContext.roles.join(',');

                next();
            } catch (error) {
                if (error.message.includes('Invalid JWT')) {
                    return res.status(401).json({ error: error.message });
                }
                return res.status(500).json({ error: `Authentication error: ${error.message}` });
            }
        };
    }

    /**
     * Create headers for service-to-service communication
     * @param {UserContext} userContext - User context
     * @returns {Object} Headers object
     */
    getAuthHeaders(userContext) {
        return {
            'X-User-ID': userContext.userId,
            'X-User-Email': userContext.email,
            'X-User-Roles': userContext.roles.join(','),
            'X-Service-Auth': 'internal'
        };
    }

    /**
     * Create HTTP client for calling other AI services
     * @param {string} serviceUrl - Target service URL
     * @param {UserContext} userContext - User context
     * @returns {Object} Axios instance
     */
    createServiceClient(serviceUrl, userContext) {
        return axios.create({
            baseURL: serviceUrl,
            timeout: this.config.timeout,
            headers: this.getAuthHeaders(userContext)
        });
    }
}

// Example Express.js service implementation
function createExpressExample() {
    const express = require('express');
    const app = express();

    app.use(express.json());

    // Initialize auth middleware
    const authConfig = new AuthConfig({
        authorizationServiceUrl: 'http://localhost:8081',
        jwtPublicKeyUrl: 'http://localhost:8080/realms/002aic/protocol/openid-connect/certs',
        jwtIssuer: 'http://localhost:8080/realms/002aic',
        jwtAudience: '002aic-api',
        serviceName: 'api-gateway-service'
    });

    const auth = new AuthMiddleware(authConfig);

    // Protected routes
    app.get('/api/models', auth.requireAuth('model', 'read'), (req, res) => {
        res.json({
            models: ['model1', 'model2'],
            user: req.user.username
        });
    });

    app.post('/api/models', auth.requireAuth('model', 'create'), (req, res) => {
        res.json({
            message: 'Model created',
            created_by: req.user.username
        });
    });

    app.get('/api/datasets', auth.requireAuth('dataset', 'read'), (req, res) => {
        res.json({
            datasets: ['dataset1', 'dataset2'],
            user: req.user.username
        });
    });

    app.post('/api/pipelines', auth.requireAuth('pipeline', 'create'), (req, res) => {
        res.json({
            message: 'Pipeline created',
            created_by: req.user.username
        });
    });

    return app;
}

// Health check middleware (no auth required)
function healthCheck() {
    return (req, res, next) => {
        if (req.path === '/health' || req.path === '/ready') {
            return res.json({ status: 'healthy' });
        }
        next();
    };
}

module.exports = {
    AuthConfig,
    UserContext,
    AuthorizationRequest,
    AuthorizationResponse,
    AuthMiddleware,
    createExpressExample,
    healthCheck
};

// Example usage:
/*
const { AuthMiddleware, AuthConfig } = require('./auth');

const authConfig = new AuthConfig({
    authorizationServiceUrl: 'http://localhost:8081',
    jwtPublicKeyUrl: 'http://localhost:8080/realms/002aic/protocol/openid-connect/certs',
    jwtIssuer: 'http://localhost:8080/realms/002aic',
    jwtAudience: '002aic-api',
    serviceName: 'my-ai-service'
});

const auth = new AuthMiddleware(authConfig);

// Use in Express routes
app.get('/protected-endpoint', auth.requireAuth('resource', 'action'), (req, res) => {
    // Access user context via req.user
    res.json({ user: req.user.username });
});
*/
