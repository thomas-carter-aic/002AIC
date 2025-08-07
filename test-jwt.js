const jwt = require('jsonwebtoken');

const token = jwt.sign(
  { 
    id: 'test-user-123', 
    username: 'testuser', 
    roles: ['admin'],
    iat: Math.floor(Date.now() / 1000)
  },
  'nexus-platform-secret-key',
  { expiresIn: '1h' }
);

console.log(token);
