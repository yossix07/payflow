const { SQSClient, SendMessageCommand } = require('@aws-sdk/client-sqs');
const { getUnpublishedMessages, markAsPublished } = require('../repository/outboxRepository');

const sqsClient = new SQSClient({ region: process.env.AWS_REGION });
const QUEUE_URL = process.env.QUEUE_URL;
const BROADCAST_QUEUE_URLS = process.env.BROADCAST_QUEUE_URLS
  ? process.env.BROADCAST_QUEUE_URLS.split(',').map(u => u.trim()).filter(Boolean)
  : [QUEUE_URL];
const POLL_INTERVAL = 100; // 100ms

async function startOutboxWorker() {
  console.log('Starting outbox worker...');

  while (true) {
    try {
      await processOutbox();
      await sleep(POLL_INTERVAL);
    } catch (error) {
      console.error('Error processing outbox:', error);
      await sleep(5000); // Wait before retrying
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

      // Broadcast to all queues
      for (const queueUrl of BROADCAST_QUEUE_URLS) {
        const command = new SendMessageCommand({
          QueueUrl: queueUrl,
          MessageBody: JSON.stringify(wrapper),
        });
        await sqsClient.send(command);
      }

      // Mark as published
      await markAsPublished(msg.message_id);
      console.log(`Published event: ${msg.event_type} (${msg.message_id})`);
    } catch (error) {
      console.error(`Failed to publish message ${msg.message_id}:`, error);
    }
  }
}

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

module.exports = { startOutboxWorker };
