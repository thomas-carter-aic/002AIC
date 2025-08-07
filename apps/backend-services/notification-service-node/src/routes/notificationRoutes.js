const express = require('express');
const { body, param, query, validationResult } = require('express-validator');
const router = express.Router();
const logger = require('../utils/logger');
const { notificationsSentTotal, notificationDeliveryDuration } = require('../utils/metrics');

// In-memory notification store (replace with database in production)
let notifications = [];
let notificationId = 1;

// Mock email service
const sendEmail = async (to, subject, body, options = {}) => {
  const start = Date.now();
  
  // Simulate email sending delay
  await new Promise(resolve => setTimeout(resolve, Math.random() * 1000 + 500));
  
  const duration = (Date.now() - start) / 1000;
  notificationDeliveryDuration.observe({ type: 'email' }, duration);
  
  // Simulate 95% success rate
  const success = Math.random() > 0.05;
  
  if (success) {
    notificationsSentTotal.inc({ type: 'email', status: 'success' });
    logger.info(`Email sent to ${to}: ${subject}`);
    return { success: true, messageId: `email_${Date.now()}` };
  } else {
    notificationsSentTotal.inc({ type: 'email', status: 'failed' });
    throw new Error('Failed to send email');
  }
};

// Mock SMS service
const sendSMS = async (to, message, options = {}) => {
  const start = Date.now();
  
  // Simulate SMS sending delay
  await new Promise(resolve => setTimeout(resolve, Math.random() * 500 + 200));
  
  const duration = (Date.now() - start) / 1000;
  notificationDeliveryDuration.observe({ type: 'sms' }, duration);
  
  // Simulate 98% success rate
  const success = Math.random() > 0.02;
  
  if (success) {
    notificationsSentTotal.inc({ type: 'sms', status: 'success' });
    logger.info(`SMS sent to ${to}: ${message}`);
    return { success: true, messageId: `sms_${Date.now()}` };
  } else {
    notificationsSentTotal.inc({ type: 'sms', status: 'failed' });
    throw new Error('Failed to send SMS');
  }
};

// Validation middleware
const validateNotification = [
  body('type').isIn(['email', 'sms', 'websocket', 'push']).withMessage('Invalid notification type'),
  body('recipient').notEmpty().withMessage('Recipient is required'),
  body('subject').optional().isString(),
  body('message').notEmpty().withMessage('Message is required'),
  body('priority').optional().isIn(['low', 'normal', 'high', 'urgent']),
  body('scheduledAt').optional().isISO8601()
];

const validateBulkNotification = [
  body('notifications').isArray().withMessage('Notifications must be an array'),
  body('notifications.*.type').isIn(['email', 'sms', 'websocket', 'push']),
  body('notifications.*.recipient').notEmpty(),
  body('notifications.*.message').notEmpty()
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

// GET /api/v1/notifications - Get all notifications
router.get('/', [
  query('type').optional().isIn(['email', 'sms', 'websocket', 'push']),
  query('status').optional().isIn(['pending', 'sent', 'failed', 'delivered']),
  query('page').optional().isInt({ min: 1 }),
  query('limit').optional().isInt({ min: 1, max: 100 })
], handleValidationErrors, (req, res) => {
  try {
    const { type, status, page = 1, limit = 50 } = req.query;
    let filteredNotifications = [...notifications];

    // Apply filters
    if (type) {
      filteredNotifications = filteredNotifications.filter(n => n.type === type);
    }
    if (status) {
      filteredNotifications = filteredNotifications.filter(n => n.status === status);
    }

    // Apply pagination
    const startIndex = (page - 1) * limit;
    const endIndex = startIndex + parseInt(limit);
    const paginatedNotifications = filteredNotifications.slice(startIndex, endIndex);

    const result = {
      notifications: paginatedNotifications,
      pagination: {
        page: parseInt(page),
        limit: parseInt(limit),
        total: filteredNotifications.length,
        totalPages: Math.ceil(filteredNotifications.length / limit)
      }
    };

    logger.info(`Retrieved ${paginatedNotifications.length} notifications`);
    res.json(result);
  } catch (error) {
    logger.error('Error retrieving notifications:', error);
    res.status(500).json({ error: 'Failed to retrieve notifications' });
  }
});

// GET /api/v1/notifications/stats - Get notification statistics
router.get('/stats', (req, res) => {
  try {
    const stats = {
      total: notifications.length,
      byStatus: {
        pending: notifications.filter(n => n.status === 'pending').length,
        sent: notifications.filter(n => n.status === 'sent').length,
        failed: notifications.filter(n => n.status === 'failed').length
      },
      byType: {
        email: notifications.filter(n => n.type === 'email').length,
        sms: notifications.filter(n => n.type === 'sms').length,
        websocket: notifications.filter(n => n.type === 'websocket').length,
        push: notifications.filter(n => n.type === 'push').length
      },
      byPriority: {
        low: notifications.filter(n => n.priority === 'low').length,
        normal: notifications.filter(n => n.priority === 'normal').length,
        high: notifications.filter(n => n.priority === 'high').length,
        urgent: notifications.filter(n => n.priority === 'urgent').length
      }
    };

    logger.info('Retrieved notification statistics');
    res.json(stats);
  } catch (error) {
    logger.error('Error retrieving notification statistics:', error);
    res.status(500).json({ error: 'Failed to retrieve statistics' });
  }
});

// GET /api/v1/notifications/:id - Get specific notification
router.get('/:id', [
  param('id').isInt().withMessage('Invalid notification ID')
], handleValidationErrors, (req, res) => {
  try {
    const { id } = req.params;
    const notification = notifications.find(n => n.id === parseInt(id));

    if (!notification) {
      return res.status(404).json({
        error: 'Notification not found',
        id
      });
    }

    logger.info(`Retrieved notification: ${id}`);
    res.json(notification);
  } catch (error) {
    logger.error(`Error retrieving notification ${req.params.id}:`, error);
    res.status(500).json({ error: 'Failed to retrieve notification' });
  }
});

// POST /api/v1/notifications - Send notification
router.post('/', validateNotification, handleValidationErrors, async (req, res) => {
  try {
    const { type, recipient, subject, message, priority = 'normal', scheduledAt, metadata = {} } = req.body;
    
    const notification = {
      id: notificationId++,
      type,
      recipient,
      subject,
      message,
      priority,
      status: 'pending',
      scheduledAt: scheduledAt ? new Date(scheduledAt) : null,
      createdAt: new Date(),
      sentAt: null,
      deliveredAt: null,
      metadata,
      attempts: 0,
      lastError: null
    };

    notifications.push(notification);

    // If not scheduled, send immediately
    if (!scheduledAt) {
      try {
        let result;
        
        switch (type) {
          case 'email':
            result = await sendEmail(recipient, subject, message, metadata);
            break;
          case 'sms':
            result = await sendSMS(recipient, message, metadata);
            break;
          case 'websocket':
            // WebSocket notifications are handled differently
            notification.status = 'sent';
            notification.sentAt = new Date();
            notificationsSentTotal.inc({ type: 'websocket', status: 'success' });
            result = { success: true, messageId: `ws_${Date.now()}` };
            break;
          case 'push':
            // Mock push notification
            notification.status = 'sent';
            notification.sentAt = new Date();
            notificationsSentTotal.inc({ type: 'push', status: 'success' });
            result = { success: true, messageId: `push_${Date.now()}` };
            break;
          default:
            throw new Error(`Unsupported notification type: ${type}`);
        }

        notification.status = 'sent';
        notification.sentAt = new Date();
        notification.metadata.messageId = result.messageId;
        
      } catch (error) {
        notification.status = 'failed';
        notification.lastError = error.message;
        notification.attempts = 1;
        logger.error(`Failed to send notification ${notification.id}:`, error);
      }
    }

    logger.info(`Created notification: ${notification.id}`);
    res.status(201).json({
      message: 'Notification created successfully',
      notification
    });
  } catch (error) {
    logger.error('Error creating notification:', error);
    res.status(500).json({ error: 'Failed to create notification' });
  }
});

// POST /api/v1/notifications/bulk - Send bulk notifications
router.post('/bulk', validateBulkNotification, handleValidationErrors, async (req, res) => {
  try {
    const { notifications: bulkNotifications } = req.body;
    const results = [];

    for (const notifData of bulkNotifications) {
      const notification = {
        id: notificationId++,
        type: notifData.type,
        recipient: notifData.recipient,
        subject: notifData.subject,
        message: notifData.message,
        priority: notifData.priority || 'normal',
        status: 'pending',
        createdAt: new Date(),
        sentAt: null,
        metadata: notifData.metadata || {},
        attempts: 0,
        lastError: null
      };

      notifications.push(notification);

      try {
        let result;
        
        switch (notification.type) {
          case 'email':
            result = await sendEmail(notification.recipient, notification.subject, notification.message);
            break;
          case 'sms':
            result = await sendSMS(notification.recipient, notification.message);
            break;
          case 'websocket':
          case 'push':
            notification.status = 'sent';
            notification.sentAt = new Date();
            notificationsSentTotal.inc({ type: notification.type, status: 'success' });
            result = { success: true, messageId: `${notification.type}_${Date.now()}` };
            break;
        }

        notification.status = 'sent';
        notification.sentAt = new Date();
        notification.metadata.messageId = result.messageId;
        
        results.push({
          id: notification.id,
          status: 'sent',
          recipient: notification.recipient
        });
        
      } catch (error) {
        notification.status = 'failed';
        notification.lastError = error.message;
        notification.attempts = 1;
        
        results.push({
          id: notification.id,
          status: 'failed',
          recipient: notification.recipient,
          error: error.message
        });
      }
    }

    logger.info(`Bulk notification operation completed for ${bulkNotifications.length} notifications`);
    res.json({
      message: 'Bulk notification operation completed',
      results,
      summary: {
        total: results.length,
        sent: results.filter(r => r.status === 'sent').length,
        failed: results.filter(r => r.status === 'failed').length
      }
    });
  } catch (error) {
    logger.error('Error in bulk notification operation:', error);
    res.status(500).json({ error: 'Failed to perform bulk notification operation' });
  }
});

// POST /api/v1/notifications/:id/retry - Retry failed notification
router.post('/:id/retry', [
  param('id').isInt().withMessage('Invalid notification ID')
], handleValidationErrors, async (req, res) => {
  try {
    const { id } = req.params;
    const notification = notifications.find(n => n.id === parseInt(id));

    if (!notification) {
      return res.status(404).json({
        error: 'Notification not found',
        id
      });
    }

    if (notification.status === 'sent') {
      return res.status(400).json({
        error: 'Notification already sent',
        id
      });
    }

    try {
      let result;
      
      switch (notification.type) {
        case 'email':
          result = await sendEmail(notification.recipient, notification.subject, notification.message);
          break;
        case 'sms':
          result = await sendSMS(notification.recipient, notification.message);
          break;
        case 'websocket':
        case 'push':
          notification.status = 'sent';
          notification.sentAt = new Date();
          notificationsSentTotal.inc({ type: notification.type, status: 'success' });
          result = { success: true, messageId: `${notification.type}_${Date.now()}` };
          break;
      }

      notification.status = 'sent';
      notification.sentAt = new Date();
      notification.attempts += 1;
      notification.lastError = null;
      notification.metadata.messageId = result.messageId;
      
      logger.info(`Retried notification ${id} successfully`);
      res.json({
        message: 'Notification retried successfully',
        notification
      });
      
    } catch (error) {
      notification.attempts += 1;
      notification.lastError = error.message;
      
      logger.error(`Failed to retry notification ${id}:`, error);
      res.status(500).json({
        error: 'Failed to retry notification',
        details: error.message
      });
    }
  } catch (error) {
    logger.error(`Error retrying notification ${req.params.id}:`, error);
    res.status(500).json({ error: 'Failed to retry notification' });
  }
});

// GET /api/v1/notifications/stats - Get notification statistics
router.get('/stats', (req, res) => {
  try {
    const stats = {
      total: notifications.length,
      byStatus: {
        pending: notifications.filter(n => n.status === 'pending').length,
        sent: notifications.filter(n => n.status === 'sent').length,
        failed: notifications.filter(n => n.status === 'failed').length
      },
      byType: {
        email: notifications.filter(n => n.type === 'email').length,
        sms: notifications.filter(n => n.type === 'sms').length,
        websocket: notifications.filter(n => n.type === 'websocket').length,
        push: notifications.filter(n => n.type === 'push').length
      },
      byPriority: {
        low: notifications.filter(n => n.priority === 'low').length,
        normal: notifications.filter(n => n.priority === 'normal').length,
        high: notifications.filter(n => n.priority === 'high').length,
        urgent: notifications.filter(n => n.priority === 'urgent').length
      }
    };

    logger.info('Retrieved notification statistics');
    res.json(stats);
  } catch (error) {
    logger.error('Error retrieving notification statistics:', error);
    res.status(500).json({ error: 'Failed to retrieve statistics' });
  }
});

module.exports = router;
