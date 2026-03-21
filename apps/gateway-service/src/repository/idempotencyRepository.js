const { DynamoDBClient } = require('@aws-sdk/client-dynamodb');
const { DynamoDBDocumentClient, PutCommand } = require('@aws-sdk/lib-dynamodb');

const client = new DynamoDBClient({ region: process.env.AWS_REGION });
const docClient = DynamoDBDocumentClient.from(client);

const IDEMPOTENCY_TABLE = process.env.IDEMPOTENCY_TABLE;

/**
 * Attempts to claim a payment_id for processing.
 * Returns true if this is the first time (proceed), false if duplicate (skip).
 */
async function claimPayment(paymentId) {
  try {
    const command = new PutCommand({
      TableName: IDEMPOTENCY_TABLE,
      Item: {
        payment_id: paymentId,
        created_at: new Date().toISOString(),
        expires_at: Math.floor(Date.now() / 1000) + 86400, // 24h TTL
      },
      ConditionExpression: 'attribute_not_exists(payment_id)',
    });

    await docClient.send(command);
    return true; // First claim — proceed with processing
  } catch (error) {
    if (error.name === 'ConditionalCheckFailedException') {
      return false; // Duplicate — skip
    }
    throw error;
  }
}

module.exports = { claimPayment };
