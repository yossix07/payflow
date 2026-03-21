const { SQSClient, ReceiveMessageCommand, DeleteMessageCommand } = require('@aws-sdk/client-sqs');
const { processPayment } = require('../services/paymentProcessor');
const { claimPayment } = require('../repository/idempotencyRepository');
const { isShuttingDown } = require('../utils/shutdown');
const logger = require('../utils/logger');

const sqsClient = new SQSClient({ region: process.env.AWS_REGION });
const QUEUE_URL = process.env.QUEUE_URL;

async function startEventConsumer() {
  logger.info('Starting event consumer');

  while (!isShuttingDown()) {
    try {
      const messages = await receiveMessages();
      
      for (const message of messages) {
        try {
          await handleMessage(message);
          await deleteMessage(message.ReceiptHandle);
        } catch (error) {
          logger.error('Error handling message', { error: error.message });
        }
      }
    } catch (error) {
      logger.error('Error receiving messages', { error: error.message });
      await sleep(5000); // Wait before retrying
    }
  }
}

async function receiveMessages() {
  const command = new ReceiveMessageCommand({
    QueueUrl: QUEUE_URL,
    MaxNumberOfMessages: 10,
    WaitTimeSeconds: 20, // Long polling
  });

  const response = await sqsClient.send(command);
  return response.Messages || [];
}

async function handleMessage(message) {
  const body = JSON.parse(message.Body);
  const eventType = body.event_type;
  const payload = body.payload;

  logger.info('Received event', { event_type: eventType });

  if (eventType === 'ProcessPayment') {
    // Idempotency check: skip if already processed
    const claimed = await claimPayment(payload.payment_id);
    if (!claimed) {
      logger.info('Duplicate ProcessPayment, skipping', { payment_id: payload.payment_id });
      return;
    }
    await processPayment(payload);
  } else {
    logger.info('Ignoring event type', { event_type: eventType });
  }
}

async function deleteMessage(receiptHandle) {
  const command = new DeleteMessageCommand({
    QueueUrl: QUEUE_URL,
    ReceiptHandle: receiptHandle,
  });

  await sqsClient.send(command);
}

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

module.exports = { startEventConsumer };
