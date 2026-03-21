const { SQSClient, SendMessageCommand } = require('@aws-sdk/client-sqs');
const { getUnpublishedMessages, markAsPublished, markAsFailed, incrementRetryCount } = require('../repository/outboxRepository');

const sqsClient = new SQSClient({ region: process.env.AWS_REGION });
const QUEUE_URL = process.env.QUEUE_URL;
const BROADCAST_QUEUE_URLS = process.env.BROADCAST_QUEUE_URLS
  ? process.env.BROADCAST_QUEUE_URLS.split(',').map(u => u.trim()).filter(Boolean)
  : [QUEUE_URL];
const BASE_INTERVAL = 500;
const MAX_INTERVAL = 5000;
const MAX_OUTBOX_RETRIES = 5;

async function startOutboxWorker() {
  console.log('Starting outbox worker...');
  let currentInterval = BASE_INTERVAL;

  while (true) {
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
      console.error('Error processing outbox:', error);
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
          console.error(`Failed to publish message ${msg.message_id} to queue ${queueUrl} after retries:`, err);
          allSent = false;
        }
      }

      if (allSent) {
        await markAsPublished(msg.message_id);
        console.log(`Published event: ${msg.event_type} (${msg.message_id})`);
      } else {
        await incrementRetryCount(msg.message_id);
        const currentRetries = (msg.retry_count || 0) + 1;
        if (currentRetries >= MAX_OUTBOX_RETRIES) {
          console.error(`Marking message ${msg.message_id} as failed after ${currentRetries} retries`);
          await markAsFailed(msg.message_id);
        }
      }
    } catch (error) {
      console.error(`Failed to process message ${msg.message_id}:`, error);
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
