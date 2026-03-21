const { SQSClient, ReceiveMessageCommand, DeleteMessageCommand } = require('@aws-sdk/client-sqs');
const { broadcastEvent } = require('../sse/sseManager');

const sqsClient = new SQSClient({ region: process.env.AWS_REGION });
const QUEUE_URL = process.env.QUEUE_URL;

async function startEventConsumer() {
  console.log('Starting notification event consumer...');

  while (true) {
    try {
      const messages = await receiveMessages();
      
      for (const message of messages) {
        try {
          await handleMessage(message);
          await deleteMessage(message.ReceiptHandle);
        } catch (error) {
          console.error('Error handling message:', error);
        }
      }
    } catch (error) {
      console.error('Error receiving messages:', error);
      await sleep(5000);
    }
  }
}

async function receiveMessages() {
  const command = new ReceiveMessageCommand({
    QueueUrl: QUEUE_URL,
    MaxNumberOfMessages: 10,
    WaitTimeSeconds: 20,
  });

  const response = await sqsClient.send(command);
  return response.Messages || [];
}

async function handleMessage(message) {
  const body = JSON.parse(message.Body);
  const eventType = body.event_type;
  const payload = body.payload;

  console.log(`Broadcasting event: ${eventType}`);

  // Format notification based on event type
  const notification = formatNotification(eventType, payload);
  
  // Broadcast to all connected WebSocket clients
  broadcastEvent(notification);
}

function formatNotification(eventType, payload) {
  const notification = {
    event_type: eventType,
    timestamp: new Date().toISOString(),
    data: payload,
    message: getHumanReadableMessage(eventType, payload)
  };

  return notification;
}

function getHumanReadableMessage(eventType, payload) {
  const paymentId = payload.payment_id || 'N/A';
  const userId = payload.user_id || 'N/A';
  const amount = payload.amount ? `$${payload.amount.toFixed(2)}` : '';

  switch (eventType) {
    case 'PaymentStarted':
      return `💳 Payment ${paymentId} started for user ${userId} (${amount})`;
    case 'FundsReserved':
      return `🔒 Funds reserved for payment ${paymentId}`;
    case 'InsufficientFunds':
      return `❌ Insufficient funds for payment ${paymentId}`;
    case 'PaymentSucceeded':
      return `✅ Payment ${paymentId} succeeded!`;
    case 'PaymentFailed':
      return `❌ Payment ${paymentId} failed: ${payload.reason}`;
    case 'SendNotification':
      return `📧 Notification sent for payment ${paymentId} (${payload.status})`;
    default:
      return `📡 Event: ${eventType}`;
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
