jest.mock("../repository/outboxRepository", () => ({
  publishEvent: jest.fn().mockResolvedValue(undefined),
}));
jest.mock("../repository/transactionRepository", () => ({
  saveTransaction: jest.fn().mockResolvedValue(undefined),
}));

const { processPayment } = require("../services/paymentProcessor");
const { publishEvent } = require("../repository/outboxRepository");
const { saveTransaction } = require("../repository/transactionRepository");

beforeEach(() => {
  jest.clearAllMocks();
  jest.useFakeTimers();
});

afterEach(() => {
  jest.useRealTimers();
  delete process.env.SUCCESS_RATE;
});

function flushSleep() {
  return jest.advanceTimersByTimeAsync(3000);
}

describe("processPayment", () => {
  test("saves transaction record", async () => {
    process.env.SUCCESS_RATE = "1.0";
    const promise = processPayment({
      payment_id: "pay-1",
      user_id: "user-1",
      amount: 100,
    });
    await flushSleep();
    await promise;

    expect(saveTransaction).toHaveBeenCalledTimes(1);
    const saved = saveTransaction.mock.calls[0][0];
    expect(saved.payment_id).toBe("pay-1");
    expect(saved.amount).toBe(100);
    expect(saved.transaction_id).toBeDefined();
    expect(saved.status).toBe("SUCCESS");
  });

  test("publishes PaymentSucceeded on success", async () => {
    process.env.SUCCESS_RATE = "1.0";
    const promise = processPayment({
      payment_id: "pay-1",
      user_id: "user-1",
      amount: 50,
    });
    await flushSleep();
    await promise;

    expect(publishEvent).toHaveBeenCalledWith(
      "PaymentSucceeded",
      expect.objectContaining({ payment_id: "pay-1" })
    );
  });

  test("publishes PaymentFailed on failure", async () => {
    process.env.SUCCESS_RATE = "0.0";
    const promise = processPayment({
      payment_id: "pay-2",
      user_id: "user-1",
      amount: 50,
    });
    await flushSleep();
    await promise;

    expect(publishEvent).toHaveBeenCalledWith(
      "PaymentFailed",
      expect.objectContaining({
        payment_id: "pay-2",
        reason: expect.any(String),
      })
    );
  });

  test("transaction status matches success/failure", async () => {
    process.env.SUCCESS_RATE = "0.0";
    const promise = processPayment({
      payment_id: "pay-3",
      user_id: "user-1",
      amount: 25,
    });
    await flushSleep();
    await promise;

    const saved = saveTransaction.mock.calls[0][0];
    expect(saved.status).toBe("FAILED");
    expect(saved.gateway_response.code).toBe("E01");
  });
});
