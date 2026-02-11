/**
 * Simple HTTP server with health endpoint for Kubernetes testing.
 * Minimal Node.js server - no dependencies required.
 *
 * Endpoints:
 * - GET / - Returns welcome message
 * - GET /health - Returns health status as JSON
 * - GET /ready - Returns readiness status as JSON
 */

const http = require('http');

const PORT = process.env.PORT || 8080;
const APP_NAME = process.env.APP_NAME || 'test-app';
const VERSION = process.env.VERSION || '1.0.0';

// Track startup time for readiness probe
const startTime = Date.now();
const STARTUP_DELAY_MS = 2000; // 2 second startup delay

const server = http.createServer((req, res) => {
  const uptime = Math.floor((Date.now() - startTime) / 1000);
  
  if (req.url === '/health' || req.url === '/healthz') {
    // Liveness probe - always healthy once server is running
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({
      status: 'healthy',
      timestamp: new Date().toISOString(),
      uptime: uptime,
      app: APP_NAME,
      version: VERSION
    }));
  } else if (req.url === '/ready' || req.url === '/readyz') {
    // Readiness probe - ready after startup delay
    const isReady = (Date.now() - startTime) > STARTUP_DELAY_MS;
    res.writeHead(isReady ? 200 : 503, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({
      status: isReady ? 'ready' : 'starting',
      timestamp: new Date().toISOString(),
      uptime: uptime
    }));
  } else {
    // Default endpoint
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({
      message: `Hello from ${APP_NAME}!`,
      version: VERSION,
      timestamp: new Date().toISOString()
    }));
  }
});

server.listen(PORT, '0.0.0.0', () => {
  console.log(`${APP_NAME} v${VERSION} running on port ${PORT}`);
  console.log(`Health endpoint: http://localhost:${PORT}/health`);
  console.log(`Readiness endpoint: http://localhost:${PORT}/ready`);
});

// Graceful shutdown
process.on('SIGTERM', () => {
  console.log('Received SIGTERM, shutting down gracefully...');
  server.close(() => {
    console.log('Server closed');
    process.exit(0);
  });
});
