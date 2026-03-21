const { DynamoDBClient } = require('@aws-sdk/client-dynamodb');
const { DynamoDBDocumentClient, PutCommand, QueryCommand, UpdateCommand } = require('@aws-sdk/lib-dynamodb');
const { v4: uuidv4 } = require('uuid');

const client = new DynamoDBClient({ region: process.env.AWS_REGION });
const docClient = DynamoDBDocumentClient.from(client);

const OUTBOX_TABLE = process.env.OUTBOX_TABLE;

async function publishEvent(eventType, payload) {
  const message = {
    message_id: uuidv4(),
    event_type: eventType,
    payload: JSON.stringify(payload),
    published: 0, // false
    created_at: new Date().toISOString(),
  };

  const command = new PutCommand({
    TableName: OUTBOX_TABLE,
    Item: message,
  });

  await docClient.send(command);
  console.log(`Event written to outbox: ${eventType} (${message.message_id})`);
}

async function getUnpublishedMessages() {
  const command = new QueryCommand({
    TableName: OUTBOX_TABLE,
    IndexName: 'published-index',
    KeyConditionExpression: 'published = :published',
    ExpressionAttributeValues: {
      ':published': 0,
    },
    Limit: 10,
  });

  const response = await docClient.send(command);
  return response.Items || [];
}

async function markAsPublished(messageId) {
  const command = new UpdateCommand({
    TableName: OUTBOX_TABLE,
    Key: { message_id: messageId },
    UpdateExpression: 'SET published = :published',
    ExpressionAttributeValues: {
      ':published': 1,
    },
  });

  await docClient.send(command);
}

async function markAsFailed(messageId) {
  const command = new UpdateCommand({
    TableName: OUTBOX_TABLE,
    Key: { message_id: messageId },
    UpdateExpression: 'SET published = :failed',
    ExpressionAttributeValues: {
      ':failed': -1,
    },
  });

  await docClient.send(command);
}

async function incrementRetryCount(messageId) {
  const command = new UpdateCommand({
    TableName: OUTBOX_TABLE,
    Key: { message_id: messageId },
    UpdateExpression: 'SET retry_count = if_not_exists(retry_count, :zero) + :one',
    ExpressionAttributeValues: {
      ':zero': 0,
      ':one': 1,
    },
  });

  await docClient.send(command);
}

module.exports = {
  publishEvent,
  getUnpublishedMessages,
  markAsPublished,
  markAsFailed,
  incrementRetryCount,
};
