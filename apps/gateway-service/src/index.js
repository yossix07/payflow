const express = require("express");
const { startEventConsumer } = require("./consumers/eventConsumer");
const { startOutboxWorker } = require("./workers/outboxWorker");

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
  console.log("Starting Gateway Service...");

  // Validate environment variables
  const required = [
    "QUEUE_URL",
    "TRANSACTIONS_TABLE",
    "OUTBOX_TABLE",
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
    console.log(`Gateway Service listening on port ${PORT}`);
  });
}

// Graceful shutdown
process.on("SIGTERM", () => {
  console.log("SIGTERM received, shutting down gracefully...");
  process.exit(0);
});

process.on("SIGINT", () => {
  console.log("SIGINT received, shutting down gracefully...");
  process.exit(0);
});

main().catch((err) => {
  console.error("Failed to start Gateway Service:", err);
  process.exit(1);
});
