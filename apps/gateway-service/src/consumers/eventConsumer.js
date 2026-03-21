const { SQSClient, ReceiveMessageCommand, DeleteMessageCommand } = require('@aws-sdk/client-sqs');
const { processPayment } = require('../services/paymentProcessor');

const sqsClient = new SQSClient({ region: process.env.AWS_REGION });
const QUEUE_URL = process.env.QUEUE_URL;

async function startEventConsumer() {
  console.log('Starting event consumer...');

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

  console.log(`Received event: ${eventType}`);

  if (eventType === 'ProcessPayment') {
    await processPayment(payload);
  } else {
    console.log(`Ignoring event type: ${eventType}`);
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
