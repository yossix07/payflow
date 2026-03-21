const { DynamoDBClient } = require('@aws-sdk/client-dynamodb');
const { DynamoDBDocumentClient, PutCommand } = require('@aws-sdk/lib-dynamodb');
const logger = require('../utils/logger');

const client = new DynamoDBClient({ region: process.env.AWS_REGION });
const docClient = DynamoDBDocumentClient.from(client);

const TRANSACTIONS_TABLE = process.env.TRANSACTIONS_TABLE;

async function saveTransaction(transaction) {
  const command = new PutCommand({
    TableName: TRANSACTIONS_TABLE,
    Item: transaction,
  });

  await docClient.send(command);
  logger.info('Transaction saved', { transaction_id: transaction.transaction_id });
}

module.exports = { saveTransaction };
