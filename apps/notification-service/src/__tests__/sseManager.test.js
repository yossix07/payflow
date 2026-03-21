const { addClient, broadcastEvent } = require("../sse/sseManager");

function createMockResponse() {
  const res = {
    write: jest.fn(),
    on: jest.fn(),
  };
  return res;
}

describe("sseManager", () => {
  beforeEach(() => {
    jest.resetModules();
  });

  test("addClient registers a client and listens for close", () => {
    const { addClient } = require("../sse/sseManager");
    const res = createMockResponse();

    addClient(res);

    expect(res.on).toHaveBeenCalledWith("close", expect.any(Function));
  });

  test("broadcastEvent sends to all connected clients", () => {
    const { addClient, broadcastEvent } = require("../sse/sseManager");
    const res1 = createMockResponse();
    const res2 = createMockResponse();

    addClient(res1);
    addClient(res2);

    broadcastEvent({ event_type: "PaymentStarted", message: "test" });

    expect(res1.write).toHaveBeenCalledTimes(1);
    expect(res2.write).toHaveBeenCalledTimes(1);

    const sentData = JSON.parse(res1.write.mock.calls[0][0].replace("data: ", "").trim());
    expect(sentData.event_type).toBe("PaymentStarted");
  });

  test("close event removes client from broadcast list", () => {
    const { addClient, broadcastEvent } = require("../sse/sseManager");
    const res = createMockResponse();

    addClient(res);

    // Simulate close
    const closeHandler = res.on.mock.calls.find((c) => c[0] === "close")[1];
    closeHandler();

    // Reset mock to check no further writes
    res.write.mockClear();
    broadcastEvent({ event_type: "test" });
    expect(res.write).not.toHaveBeenCalled();
  });
});
