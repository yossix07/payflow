# Plan 3: Notification-Service SSE Migration

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace WebSocket with Server-Sent Events (SSE) in notification-service. Remove `ws` dependency. This is a prerequisite for the enhanced dashboard (Plan 4).

**Architecture:** Replace `WebSocketServer` with a standard Express `GET /events` SSE endpoint. Clients use the browser-native `EventSource` API. The `broadcastEvent` function writes to all connected SSE response streams instead of WebSocket clients.

**Tech Stack:** Express.js, native SSE (no library needed)

**Spec:** `docs/superpowers/specs/2026-03-21-local-test-deploy-visualization-design.md`

---

### Task 1: Replace WebSocket manager with SSE manager

**Files:**
- Delete: `apps/notification-service/src/websocket/wsManager.js`
- Create: `apps/notification-service/src/sse/sseManager.js`

- [ ] **Step 1: Write SSE manager**

File: `apps/notification-service/src/sse/sseManager.js`

```javascript
const clients = new Set();

function addClient(res) {
  clients.add(res);
  console.log(`SSE client connected. Total clients: ${clients.size}`);

  res.on("close", () => {
    clients.delete(res);
    console.log(`SSE client disconnected. Total clients: ${clients.size}`);
  });
}

function broadcastEvent(notification) {
  const data = JSON.stringify(notification);
  let sent = 0;

  for (const client of clients) {
    try {
      client.write(`data: ${data}\n\n`);
      sent++;
    } catch (err) {
      clients.delete(client);
    }
  }

  console.log(`Broadcast event to ${sent}/${clients.size} SSE clients`);
}

module.exports = { addClient, broadcastEvent };
```

- [ ] **Step 2: Commit**

```bash
git add apps/notification-service/src/sse/sseManager.js
git commit -m "add SSE manager to replace WebSocket manager"
```

---

### Task 2: Update index.js to use SSE instead of WebSocket

**Files:**
- Modify: `apps/notification-service/src/index.js`

- [ ] **Step 1: Rewrite index.js**

Replace the WebSocket server setup with an SSE endpoint. Key changes:
- Remove `require('ws')` and `WebSocketServer` creation
- Remove `global.wss` assignment
- Add `GET /events` SSE endpoint using `sseManager.addClient(res)`
- Keep health check and event consumer unchanged

The new `/events` endpoint:

```javascript
const { addClient } = require("./sse/sseManager");

app.get("/events", (req, res) => {
  res.writeHead(200, {
    "Content-Type": "text/event-stream",
    "Cache-Control": "no-cache",
    Connection: "keep-alive",
    "Access-Control-Allow-Origin": "*",
  });

  // Send initial connection event
  res.write(
    `data: ${JSON.stringify({ type: "connected", message: "Connected to payment event stream", timestamp: new Date().toISOString() })}\n\n`
  );

  addClient(res);
});
```

The server creation changes from:
```javascript
const server = app.listen(PORT, ...);
const wss = new WebSocketServer({ server, path: '/ws' });
```
To simply:
```javascript
app.listen(PORT, ...);
```

- [ ] **Step 2: Update event consumer import**

Modify: `apps/notification-service/src/consumers/eventConsumer.js`

Change the import from:
```javascript
const { broadcastEvent } = require("../websocket/wsManager");
```
To:
```javascript
const { broadcastEvent } = require("../sse/sseManager");
```

- [ ] **Step 3: Remove ws dependency from package.json**

Modify: `apps/notification-service/package.json`

Remove `"ws": "^8.16.0"` from dependencies.

Run: `cd apps/notification-service && npm install`
Expected: `ws` removed from node_modules

- [ ] **Step 4: Delete old WebSocket manager**

Delete: `apps/notification-service/src/websocket/wsManager.js`

Also delete the `websocket/` directory if it's now empty.

- [ ] **Step 5: Verify service starts**

Run: `cd apps/notification-service && node src/index.js`
Expected: Service starts without errors, logs "Notification Service running on port 8080"
(Stop with Ctrl+C after verifying)

- [ ] **Step 6: Commit**

```bash
git add -A apps/notification-service/
git commit -m "migrate notification-service from WebSocket to SSE"
```

---

### Task 3: Update tests for SSE

**Files:**
- Delete: `apps/notification-service/src/__tests__/wsManager.test.js`
- Create: `apps/notification-service/src/__tests__/sseManager.test.js`

- [ ] **Step 1: Write SSE manager tests**

File: `apps/notification-service/src/__tests__/sseManager.test.js`

```javascript
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
    // Reset clients by broadcasting to flush — we need access to the Set.
    // Alternative: require fresh module each test.
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
```

- [ ] **Step 2: Delete old WebSocket test**

Delete: `apps/notification-service/src/__tests__/wsManager.test.js`

- [ ] **Step 3: Run tests**

Run: `cd apps/notification-service && npm test`
Expected: All 3 SSE tests pass

- [ ] **Step 4: Commit**

```bash
git add -A apps/notification-service/src/__tests__/
git commit -m "update notification-service tests for SSE migration"
```

---

### Task 4: Update CLAUDE.md and docker-compose.yml

**Files:**
- Modify: `CLAUDE.md`
- Modify: `docker-compose.yml` (if WebSocket-specific config exists)

- [ ] **Step 1: Update CLAUDE.md**

In `CLAUDE.md`, change:
- `notification-service (Node.js/Express+WebSocket, :8085)` → `notification-service (Node.js/Express+SSE, :8085)`
- `Real-time notifications via WebSocket` → `Real-time notifications via SSE`

- [ ] **Step 2: Check docker-compose.yml for WebSocket references**

The current `docker-compose.yml` doesn't have WebSocket-specific config (no special ports or protocols). No changes needed.

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md
git commit -m "update CLAUDE.md to reflect SSE migration"
```
