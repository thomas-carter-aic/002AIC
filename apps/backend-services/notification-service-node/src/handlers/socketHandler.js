const logger = require('../utils/logger');
const { websocketConnectionsTotal } = require('../utils/metrics');

let connectedClients = new Map();

const socketHandler = (io) => {
  io.on('connection', (socket) => {
    logger.info(`Client connected: ${socket.id}`);
    connectedClients.set(socket.id, {
      id: socket.id,
      connectedAt: new Date(),
      userId: null
    });
    
    websocketConnectionsTotal.set(connectedClients.size);

    // Handle user authentication
    socket.on('authenticate', (data) => {
      try {
        const { userId, token } = data;
        // In production, verify the token here
        if (userId) {
          connectedClients.set(socket.id, {
            ...connectedClients.get(socket.id),
            userId
          });
          socket.join(`user:${userId}`);
          socket.emit('authenticated', { success: true, userId });
          logger.info(`User ${userId} authenticated on socket ${socket.id}`);
        }
      } catch (error) {
        logger.error('Socket authentication error:', error);
        socket.emit('authenticated', { success: false, error: 'Authentication failed' });
      }
    });

    // Handle joining notification channels
    socket.on('join-channel', (channel) => {
      socket.join(channel);
      socket.emit('joined-channel', { channel });
      logger.info(`Socket ${socket.id} joined channel: ${channel}`);
    });

    // Handle leaving notification channels
    socket.on('leave-channel', (channel) => {
      socket.leave(channel);
      socket.emit('left-channel', { channel });
      logger.info(`Socket ${socket.id} left channel: ${channel}`);
    });

    // Handle disconnect
    socket.on('disconnect', () => {
      logger.info(`Client disconnected: ${socket.id}`);
      connectedClients.delete(socket.id);
      websocketConnectionsTotal.set(connectedClients.size);
    });

    // Handle errors
    socket.on('error', (error) => {
      logger.error(`Socket error for ${socket.id}:`, error);
    });
  });

  // Broadcast notification to specific user
  const sendToUser = (userId, notification) => {
    io.to(`user:${userId}`).emit('notification', notification);
    logger.info(`Sent notification to user ${userId}:`, notification);
  };

  // Broadcast notification to channel
  const sendToChannel = (channel, notification) => {
    io.to(channel).emit('notification', notification);
    logger.info(`Sent notification to channel ${channel}:`, notification);
  };

  // Broadcast to all connected clients
  const broadcast = (notification) => {
    io.emit('notification', notification);
    logger.info('Broadcasted notification to all clients:', notification);
  };

  return {
    sendToUser,
    sendToChannel,
    broadcast,
    getConnectedClients: () => Array.from(connectedClients.values())
  };
};

module.exports = socketHandler;
