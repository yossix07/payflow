const { SQSClient, SendMessageCommand } = require('@aws-sdk/client-sqs');
const { getUnpublishedMessages, markAsPublished, markAsFailed, incrementRetryCount } = require('../repository/outboxRepository');
const { isShuttingDown } = require('../utils/shutdown');
const logger = require('../utils/logger');

const sqsClient = new SQSClient({ region: process.env.AWS_REGION });
const QUEUE_URL = process.env.QUEUE_URL;
const BROADCAST_QUEUE_URLS = process.env.BROADCAST_QUEUE_URLS
  ? process.env.BROADCAST_QUEUE_URLS.split(',').map(u => u.trim()).filter(Boolean)
  : [QUEUE_URL];
const BASE_INTERVAL = parseInt(process.env.OUTBOX_POLL_INTERVAL_MS || '100', 10);
const MAX_INTERVAL = parseInt(process.env.OUTBOX_MAX_INTERVAL_MS || '5000', 10);
const MAX_OUTBOX_RETRIES = 5;

async function startOutboxWorker() {
  logger.info('Starting outbox worker');
  let currentInterval = BASE_INTERVAL;

  while (!isShuttingDown()) {
    try {
      const count = await processOutbox();
      if (count >= 10) {
        currentInterval = 1;
      } else if (count > 0) {
        currentInterval = BASE_INTERVAL;
      } else {
        currentInterval = Math.min(currentInterval * 2, MAX_INTERVAL);
      }
      await sleep(currentInterval);
    } catch (error) {
      logger.error('Error processing outbox', { error: error.message });
      currentInterval = BASE_INTERVAL;
      await sleep(5000);
    }
  }
}

async function processOutbox() {
  const messages = await getUnpublishedMessages();

  for (const msg of messages) {
    try {
      // Wrap message with event_type
      const wrapper = {
        event_type: msg.event_type,
        payload: JSON.parse(msg.payload),
      };

      // Broadcast to all queues, tracking per-queue success
      let allSent = true;
      for (const queueUrl of BROADCAST_QUEUE_URLS) {
        try {
          await sendWithRetry(queueUrl, JSON.stringify(wrapper));
        } catch (err) {
          logger.error('Failed to publish message to queue after retries', { message_id: msg.message_id, queue_url: queueUrl, error: err.message });
          allSent = false;
        }
      }

      if (allSent) {
        await markAsPublished(msg.message_id);
        logger.info('Published event', { event_type: msg.event_type, message_id: msg.message_id });
      } else {
        await incrementRetryCount(msg.message_id);
        const currentRetries = (msg.retry_count || 0) + 1;
        if (currentRetries >= MAX_OUTBOX_RETRIES) {
          logger.error('Marking message as failed after max retries', { message_id: msg.message_id, retries: currentRetries });
          await markAsFailed(msg.message_id);
        }
      }
    } catch (error) {
      logger.error('Failed to process message', { message_id: msg.message_id, error: error.message });
    }
  }

  return messages.length;
}

async function sendWithRetry(queueUrl, messageBody) {
  let lastErr;
  for (let attempt = 0; attempt < 3; attempt++) {
    if (attempt > 0) {
      await sleep(attempt * 500);
    }
    try {
      const command = new SendMessageCommand({
        QueueUrl: queueUrl,
        MessageBody: messageBody,
      });
      await sqsClient.send(command);
      return;
    } catch (err) {
      lastErr = err;
    }
  }
  throw new Error(`All 3 send attempts failed: ${lastErr.message}`);
}

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

module.exports = { startOutboxWorker };
