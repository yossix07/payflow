const express = require('express');
const { startEventConsumer } = require('./consumers/eventConsumer');
const { addClient } = require('./sse/sseManager');
const { shutdown } = require('./utils/shutdown');
const logger = require('./utils/logger');

const app = express();
app.use(express.json());

// Health check endpoint
app.get('/healthz', (req, res) => {
  res.json({
    service: 'Notification Service',
    status: 'healthy',
    timestamp: new Date().toISOString()
  });
});

// Readiness check endpoint
app.get('/readyz', (req, res) => {
  const { isShuttingDown } = require('./utils/shutdown');
  if (isShuttingDown()) {
    return res.status(503).json({ status: 'draining' });
  }
  res.json({ status: 'ready' });
});

// SSE endpoint for real-time payment events
app.get('/events', (req, res) => {
  const accepted = addClient(res);
  if (!accepted) return; // 503 already sent by addClient

  res.writeHead(200, {
    'Content-Type': 'text/event-stream',
    'Cache-Control': 'no-cache',
    'Connection': 'keep-alive',
    'Access-Control-Allow-Origin': '*',
  });

  res.write(
    `data: ${JSON.stringify({ type: 'connected', message: 'Connected to payment event stream', timestamp: new Date().toISOString() })}\n\n`
  );
});

const PORT = process.env.PORT || 8080;

async function main() {
  logger.info('Starting Notification Service');

  // Validate environment variables
  const required = ['QUEUE_URL', 'AWS_REGION'];
  for (const envVar of required) {
    if (!process.env[envVar]) {
      throw new Error(`Missing required environment variable: ${envVar}`);
    }
  }

  // Start event consumer
  startEventConsumer();

  // Start HTTP server
  app.listen(PORT, () => {
    logger.info('Notification Service listening', { port: PORT });
    logger.info('SSE endpoint available', { url: `http://localhost:${PORT}/events` });
  });
}

// Graceful shutdown
process.on('SIGTERM', () => {
  logger.info('SIGTERM received, shutting down gracefully');
  shutdown();
  setTimeout(() => process.exit(0), 10000);
});

process.on('SIGINT', () => {
  logger.info('SIGINT received, shutting down gracefully');
  shutdown();
  setTimeout(() => process.exit(0), 10000);
});

main().catch(err => {
  logger.error('Failed to start Notification Service', { error: err.message });
  process.exit(1);
});
