const express = require('express');
const { body, param, query, validationResult } = require('express-validator');
const router = express.Router();
const logger = require('../utils/logger');
const { configOperationsTotal } = require('../utils/metrics');

// In-memory configuration store (replace with database in production)
let configStore = {
  'app.name': 'Nexus Platform',
  'app.version': '2.0.0',
  'app.environment': process.env.NODE_ENV || 'development',
  'database.host': process.env.DB_HOST || 'localhost',
  'database.port': process.env.DB_PORT || '5432',
  'redis.host': process.env.REDIS_HOST || 'localhost',
  'redis.port': process.env.REDIS_PORT || '6379',
  'auth.jwt.expiry': '24h',
  'auth.session.timeout': '30m',
  'api.rate.limit': '1000',
  'logging.level': 'info',
  'monitoring.enabled': 'true',
  'features.ai.enabled': 'true',
  'features.analytics.enabled': 'true'
};

// Validation middleware
const validateConfigKey = [
  param('key').notEmpty().withMessage('Configuration key is required')
];

const validateConfigValue = [
  body('value').notEmpty().withMessage('Configuration value is required'),
  body('description').optional().isString()
];

const validateBulkConfig = [
  body('configs').isArray().withMessage('Configs must be an array'),
  body('configs.*.key').notEmpty().withMessage('Each config must have a key'),
  body('configs.*.value').notEmpty().withMessage('Each config must have a value')
];

// Helper function to handle validation errors
const handleValidationErrors = (req, res, next) => {
  const errors = validationResult(req);
  if (!errors.isEmpty()) {
    return res.status(400).json({
      error: 'Validation Error',
      details: errors.array()
    });
  }
  next();
};

// GET /api/v1/config - Get all configurations
router.get('/', [
  query('filter').optional().isString(),
  query('page').optional().isInt({ min: 1 }),
  query('limit').optional().isInt({ min: 1, max: 100 })
], handleValidationErrors, (req, res) => {
  try {
    const { filter, page = 1, limit = 50 } = req.query;
    let configs = Object.entries(configStore);

    // Apply filter if provided
    if (filter) {
      configs = configs.filter(([key]) => 
        key.toLowerCase().includes(filter.toLowerCase())
      );
    }

    // Apply pagination
    const startIndex = (page - 1) * limit;
    const endIndex = startIndex + parseInt(limit);
    const paginatedConfigs = configs.slice(startIndex, endIndex);

    const result = {
      configs: paginatedConfigs.map(([key, value]) => ({ key, value })),
      pagination: {
        page: parseInt(page),
        limit: parseInt(limit),
        total: configs.length,
        totalPages: Math.ceil(configs.length / limit)
      }
    };

    configOperationsTotal.inc({ operation: 'get_all', status: 'success' });
    logger.info(`Retrieved ${paginatedConfigs.length} configurations`);
    res.json(result);
  } catch (error) {
    configOperationsTotal.inc({ operation: 'get_all', status: 'error' });
    logger.error('Error retrieving configurations:', error);
    res.status(500).json({ error: 'Failed to retrieve configurations' });
  }
});

// GET /api/v1/config/:key - Get specific configuration
router.get('/:key', validateConfigKey, handleValidationErrors, (req, res) => {
  try {
    const { key } = req.params;
    const value = configStore[key];

    if (value === undefined) {
      configOperationsTotal.inc({ operation: 'get', status: 'not_found' });
      return res.status(404).json({
        error: 'Configuration not found',
        key
      });
    }

    configOperationsTotal.inc({ operation: 'get', status: 'success' });
    logger.info(`Retrieved configuration: ${key}`);
    res.json({ key, value });
  } catch (error) {
    configOperationsTotal.inc({ operation: 'get', status: 'error' });
    logger.error(`Error retrieving configuration ${req.params.key}:`, error);
    res.status(500).json({ error: 'Failed to retrieve configuration' });
  }
});

// POST /api/v1/config/:key - Create or update configuration
router.post('/:key', [
  ...validateConfigKey,
  ...validateConfigValue
], handleValidationErrors, (req, res) => {
  try {
    const { key } = req.params;
    const { value, description } = req.body;
    const isUpdate = configStore.hasOwnProperty(key);

    configStore[key] = value;

    const operation = isUpdate ? 'update' : 'create';
    configOperationsTotal.inc({ operation, status: 'success' });
    
    logger.info(`${isUpdate ? 'Updated' : 'Created'} configuration: ${key}`);
    res.status(isUpdate ? 200 : 201).json({
      message: `Configuration ${isUpdate ? 'updated' : 'created'} successfully`,
      key,
      value,
      ...(description && { description })
    });
  } catch (error) {
    const operation = configStore.hasOwnProperty(req.params.key) ? 'update' : 'create';
    configOperationsTotal.inc({ operation, status: 'error' });
    logger.error(`Error ${operation} configuration ${req.params.key}:`, error);
    res.status(500).json({ error: `Failed to ${operation} configuration` });
  }
});

// PUT /api/v1/config/:key - Update configuration (same as POST for simplicity)
router.put('/:key', [
  ...validateConfigKey,
  ...validateConfigValue
], handleValidationErrors, (req, res) => {
  try {
    const { key } = req.params;
    const { value, description } = req.body;

    if (!configStore.hasOwnProperty(key)) {
      configOperationsTotal.inc({ operation: 'update', status: 'not_found' });
      return res.status(404).json({
        error: 'Configuration not found',
        key
      });
    }

    configStore[key] = value;

    configOperationsTotal.inc({ operation: 'update', status: 'success' });
    logger.info(`Updated configuration: ${key}`);
    res.json({
      message: 'Configuration updated successfully',
      key,
      value,
      ...(description && { description })
    });
  } catch (error) {
    configOperationsTotal.inc({ operation: 'update', status: 'error' });
    logger.error(`Error updating configuration ${req.params.key}:`, error);
    res.status(500).json({ error: 'Failed to update configuration' });
  }
});

// DELETE /api/v1/config/:key - Delete configuration
router.delete('/:key', validateConfigKey, handleValidationErrors, (req, res) => {
  try {
    const { key } = req.params;

    if (!configStore.hasOwnProperty(key)) {
      configOperationsTotal.inc({ operation: 'delete', status: 'not_found' });
      return res.status(404).json({
        error: 'Configuration not found',
        key
      });
    }

    delete configStore[key];

    configOperationsTotal.inc({ operation: 'delete', status: 'success' });
    logger.info(`Deleted configuration: ${key}`);
    res.json({
      message: 'Configuration deleted successfully',
      key
    });
  } catch (error) {
    configOperationsTotal.inc({ operation: 'delete', status: 'error' });
    logger.error(`Error deleting configuration ${req.params.key}:`, error);
    res.status(500).json({ error: 'Failed to delete configuration' });
  }
});

// POST /api/v1/config/bulk - Bulk create/update configurations
router.post('/bulk', validateBulkConfig, handleValidationErrors, (req, res) => {
  try {
    const { configs } = req.body;
    const results = [];

    configs.forEach(({ key, value, description }) => {
      const isUpdate = configStore.hasOwnProperty(key);
      configStore[key] = value;
      results.push({
        key,
        value,
        action: isUpdate ? 'updated' : 'created',
        ...(description && { description })
      });
    });

    configOperationsTotal.inc({ operation: 'bulk', status: 'success' });
    logger.info(`Bulk operation completed for ${configs.length} configurations`);
    res.json({
      message: 'Bulk operation completed successfully',
      results,
      count: results.length
    });
  } catch (error) {
    configOperationsTotal.inc({ operation: 'bulk', status: 'error' });
    logger.error('Error in bulk configuration operation:', error);
    res.status(500).json({ error: 'Failed to perform bulk operation' });
  }
});

// GET /api/v1/config/export - Export all configurations
router.get('/export', (req, res) => {
  try {
    const timestamp = new Date().toISOString();
    const exportData = {
      timestamp,
      service: 'configuration-service',
      version: '1.0.0',
      configurations: configStore
    };

    configOperationsTotal.inc({ operation: 'export', status: 'success' });
    logger.info('Configuration export completed');
    
    res.setHeader('Content-Disposition', `attachment; filename=config-export-${timestamp}.json`);
    res.json(exportData);
  } catch (error) {
    configOperationsTotal.inc({ operation: 'export', status: 'error' });
    logger.error('Error exporting configurations:', error);
    res.status(500).json({ error: 'Failed to export configurations' });
  }
});

module.exports = router;
