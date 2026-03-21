const express = require('express');
const { startEventConsumer } = require('./consumers/eventConsumer');
const { addClient } = require('./sse/sseManager');

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

// SSE endpoint for real-time payment events
app.get('/events', (req, res) => {
  res.writeHead(200, {
    'Content-Type': 'text/event-stream',
    'Cache-Control': 'no-cache',
    'Connection': 'keep-alive',
    'Access-Control-Allow-Origin': '*',
  });

  // Send initial connection event
  res.write(
    `data: ${JSON.stringify({ type: 'connected', message: 'Connected to payment event stream', timestamp: new Date().toISOString() })}\n\n`
  );

  addClient(res);
});

const PORT = process.env.PORT || 8080;

async function main() {
  console.log('Starting Notification Service...');

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
    console.log(`Notification Service listening on port ${PORT}`);
    console.log(`SSE endpoint: http://localhost:${PORT}/events`);
  });
}

// Graceful shutdown
process.on('SIGTERM', () => {
  console.log('SIGTERM received, shutting down gracefully...');
  process.exit(0);
});

process.on('SIGINT', () => {
  console.log('SIGINT received, shutting down gracefully...');
  process.exit(0);
});

main().catch(err => {
  console.error('Failed to start Notification Service:', err);
  process.exit(1);
});
