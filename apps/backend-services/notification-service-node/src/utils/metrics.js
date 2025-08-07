const client = require('prom-client');

// Create a Registry to register the metrics
const register = new client.Registry();

// Add a default label which is added to all metrics
register.setDefaultLabels({
  app: 'notification-service'
});

// Enable the collection of default metrics
client.collectDefaultMetrics({ register });

// Create custom metrics
const httpRequestDuration = new client.Histogram({
  name: 'http_request_duration_seconds',
  help: 'Duration of HTTP requests in seconds',
  labelNames: ['method', 'route', 'status_code'],
  buckets: [0.1, 0.3, 0.5, 0.7, 1, 3, 5, 7, 10]
});

const httpRequestsTotal = new client.Counter({
  name: 'http_requests_total',
  help: 'Total number of HTTP requests',
  labelNames: ['method', 'route', 'status_code']
});

const notificationsSentTotal = new client.Counter({
  name: 'notifications_sent_total',
  help: 'Total number of notifications sent',
  labelNames: ['type', 'status']
});

const notificationDeliveryDuration = new client.Histogram({
  name: 'notification_delivery_duration_seconds',
  help: 'Duration of notification delivery in seconds',
  labelNames: ['type'],
  buckets: [0.1, 0.5, 1, 2, 5, 10, 30]
});

const websocketConnectionsTotal = new client.Gauge({
  name: 'websocket_connections_total',
  help: 'Total number of active WebSocket connections'
});

// Register custom metrics
register.registerMetric(httpRequestDuration);
register.registerMetric(httpRequestsTotal);
register.registerMetric(notificationsSentTotal);
register.registerMetric(notificationDeliveryDuration);
register.registerMetric(websocketConnectionsTotal);

module.exports = {
  register,
  httpRequestDuration,
  httpRequestsTotal,
  notificationsSentTotal,
  notificationDeliveryDuration,
  websocketConnectionsTotal
};
