const express = require("express");
const { startEventConsumer } = require("./consumers/eventConsumer");
const { startOutboxWorker } = require("./workers/outboxWorker");
const { shutdown } = require("./utils/shutdown");
const logger = require("./utils/logger");

const app = express();
app.use(express.json());

// Health check endpoint
app.get("/healthz", (req, res) => {
  res.json({
    service: "Gateway Service",
    status: "healthy",
    timestamp: new Date().toISOString(),
  });
});

const PORT = process.env.PORT || 8080;

async function main() {
  logger.info("Starting Gateway Service");

  // Validate environment variables
  const required = [
    "QUEUE_URL",
    "TRANSACTIONS_TABLE",
    "OUTBOX_TABLE",
    "IDEMPOTENCY_TABLE",
    "AWS_REGION",
  ];
  for (const envVar of required) {
    if (!process.env[envVar]) {
      throw new Error(`Missing required environment variable: ${envVar}`);
    }
  }

  // Start background workers
  startEventConsumer();
  startOutboxWorker();

  // Start HTTP server
  app.listen(PORT, () => {
    logger.info("Gateway Service listening", { port: PORT });
  });
}

// Graceful shutdown
process.on("SIGTERM", () => {
  logger.info("SIGTERM received, shutting down gracefully");
  shutdown();
  setTimeout(() => process.exit(0), 10000);
});

process.on("SIGINT", () => {
  logger.info("SIGINT received, shutting down gracefully");
  shutdown();
  setTimeout(() => process.exit(0), 10000);
});

main().catch((err) => {
  logger.error("Failed to start Gateway Service", { error: err.message });
  process.exit(1);
});
