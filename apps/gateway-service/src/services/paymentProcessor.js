const { v4: uuidv4 } = require('uuid');
const { saveTransaction } = require('../repository/transactionRepository');
const { publishEvent } = require('../repository/outboxRepository');

/**
 * Mock payment processor
 * Simulates calling an external payment gateway (Stripe, PayPal, etc.)
 * 
 * Success rate: 80% (configurable via SUCCESS_RATE env var)
 */
async function processPayment(payload) {
  const { payment_id, user_id, amount } = payload;

  console.log(`Processing payment ${payment_id} for user ${user_id}: $${amount}`);

  // Simulate processing delay
  await sleep(Math.random() * 2000 + 500); // 500-2500ms

  // Mock success/failure logic
  const successRate = parseFloat(process.env.SUCCESS_RATE || '0.8');
  const isSuccess = Math.random() < successRate;

  const transactionId = uuidv4();
  const timestamp = new Date().toISOString();

  // Save transaction record
  const transaction = {
    transaction_id: transactionId,
    payment_id,
    user_id,
    amount,
    status: isSuccess ? 'SUCCESS' : 'FAILED',
    gateway_response: isSuccess 
      ? { code: '00', message: 'Approved' }
      : { code: 'E01', message: 'Declined by issuer' },
    created_at: timestamp,
  };

  await saveTransaction(transaction);

  // Publish event via outbox
  if (isSuccess) {
    console.log(`Payment ${payment_id} succeeded (transaction: ${transactionId})`);
    await publishEvent('PaymentSucceeded', {
      payment_id,
      transaction_id: transactionId,
      timestamp,
    });
  } else {
    console.log(`Payment ${payment_id} failed`);
    await publishEvent('PaymentFailed', {
      payment_id,
      reason: 'Gateway declined transaction',
      timestamp,
    });
  }
}

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

module.exports = { processPayment };
