# Plan 4: Enhanced Platform Dashboard with Real-Time Visualization

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build an interactive dashboard with animated D3.js service topology showing real-time event flow, event log, saga status panel, and trigger buttons.

**Architecture:** The platform-dashboard Go service serves static HTML/JS/CSS. The browser connects to `/api/events` (SSE proxy to notification-service) for real-time events and uses `/api/trigger` to initiate payment flows. D3.js renders the animated topology. No build step.

**Tech Stack:** D3.js v7 (CDN), vanilla JS, CSS, Go (platform-dashboard server)

**Spec:** `docs/superpowers/specs/2026-03-21-local-test-deploy-visualization-design.md`

**Depends on:** Plan 3 (SSE migration) must be completed first.

---

### Task 1: Add proxy endpoints to platform-dashboard Go server

**Files:**
- Modify: `apps/platform-dashboard/main.go`

- [ ] **Step 1: Add /api/trigger endpoint**

This endpoint proxies POST requests to payment-service at `http://payment-service:8080/payments` (internal Docker network). The environment variable `PAYMENT_SERVICE_URL` configures the target (default: `http://payment-service:8080`).

Add to `main.go`:

```go
func triggerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	paymentURL := os.Getenv("PAYMENT_SERVICE_URL")
	if paymentURL == "" {
		paymentURL = "http://payment-service:8080"
	}

	resp, err := http.Post(paymentURL+"/payments", "application/json", r.Body)
	if err != nil {
		http.Error(w, "Failed to reach payment service: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
```

Register: `http.HandleFunc("/api/trigger", triggerHandler)`

Import `"io"` and `"os"`.

- [ ] **Step 2: Add /api/events SSE proxy endpoint**

This proxies the SSE stream from notification-service at `http://notification-service:8080/events`. The environment variable `NOTIFICATION_SERVICE_URL` configures the target.

```go
func eventsHandler(w http.ResponseWriter, r *http.Request) {
	notifURL := os.Getenv("NOTIFICATION_SERVICE_URL")
	if notifURL == "" {
		notifURL = "http://notification-service:8080"
	}

	resp, err := http.Get(notifURL + "/events")
	if err != nil {
		http.Error(w, "Failed to reach notification service", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	buf := make([]byte, 1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
			flusher.Flush()
		}
		if err != nil {
			break
		}
	}
}
```

Register: `http.HandleFunc("/api/events", eventsHandler)`

- [ ] **Step 3: Update docker-compose.yml**

Add environment variables to platform-dashboard service:

```yaml
environment:
  - PAYMENT_SERVICE_URL=http://payment-service:8080
  - NOTIFICATION_SERVICE_URL=http://notification-service:8080
```

- [ ] **Step 4: Verify Go build**

Run: `cd apps/platform-dashboard && go build -o server .`
Expected: Compiles successfully

- [ ] **Step 5: Commit**

```bash
git add apps/platform-dashboard/main.go docker-compose.yml
git commit -m "add SSE proxy and payment trigger endpoints to dashboard"
```

---

### Task 2: Build the dashboard HTML structure

**Files:**
- Rewrite: `apps/platform-dashboard/static/index.html`

- [ ] **Step 1: Create the HTML skeleton**

Replace `static/index.html` with the dashboard layout. Include D3.js v7 from CDN. Structure:

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>SaaS Payment Platform — Live Event Flow</title>
  <script src="https://d3js.org/d3.v7.min.js"></script>
  <link rel="stylesheet" href="style.css">
</head>
<body>
  <header>
    <h1>SaaS Payment Platform</h1>
    <span id="connection-status" class="status-disconnected">Disconnected</span>
  </header>

  <main>
    <section id="topology">
      <svg id="topology-svg" width="100%" height="350"></svg>
    </section>

    <div id="panels">
      <section id="event-log">
        <h2>Event Log</h2>
        <div id="events-container"></div>
      </section>

      <section id="saga-status">
        <h2>Active Sagas</h2>
        <div id="sagas-container"></div>
      </section>
    </div>

    <section id="controls">
      <button id="btn-trigger-payment" onclick="triggerPayment()">▶ Trigger Payment</button>
      <button id="btn-trigger-failure" onclick="triggerFailure()">▶ Trigger Failure Demo</button>
    </section>
  </main>

  <script src="topology.js"></script>
  <script src="events.js"></script>
  <script src="app.js"></script>
</body>
</html>
```

- [ ] **Step 2: Commit**

```bash
git add apps/platform-dashboard/static/index.html
git commit -m "add dashboard HTML skeleton with topology, event log, and controls"
```

---

### Task 3: Create CSS styles

**Files:**
- Create: `apps/platform-dashboard/static/style.css`

- [ ] **Step 1: Write dashboard styles**

Dark theme with glowing effects for the topology. Grid layout for panels. Scrollable event log. Color coding for event types.

Key styles:
- Body: dark background (#0d1117), light text
- Topology SVG: centered, dark card background
- Service nodes: rounded rectangles with subtle glow
- Active node: pulsing glow animation (green for success, red for failure)
- Event log: max-height with overflow-y scroll, newest on top
- Event entries: color-coded left border (green/red/yellow)
- Buttons: accent color, hover effect
- Connection status indicator: green dot when connected, red when disconnected

- [ ] **Step 2: Commit**

```bash
git add apps/platform-dashboard/static/style.css
git commit -m "add dashboard dark theme CSS with glow animations"
```

---

### Task 4: Build D3.js animated topology

**Files:**
- Create: `apps/platform-dashboard/static/topology.js`

- [ ] **Step 1: Write topology renderer**

Define service nodes and their connections as data:

```javascript
const services = [
  { id: "payment", label: "Payment Service", x: 150, y: 80 },
  { id: "wallet", label: "Wallet Service", x: 400, y: 80 },
  { id: "gateway", label: "Gateway Service", x: 650, y: 80 },
  { id: "ledger", label: "Ledger Service", x: 150, y: 250 },
  { id: "notification", label: "Notification", x: 650, y: 250 },
];

const connections = [
  { source: "payment", target: "wallet", label: "ReserveFunds" },
  { source: "wallet", target: "payment", label: "FundsReserved" },
  { source: "payment", target: "gateway", label: "ProcessPayment" },
  { source: "gateway", target: "payment", label: "PaymentSucceeded" },
  { source: "payment", target: "ledger", label: "record" },
  { source: "payment", target: "notification", label: "SendNotification" },
];
```

Use D3 to render:
- Rectangles for service nodes with rounded corners
- Arrows (SVG markers) for connections
- A `highlightService(serviceId, color)` function that triggers a CSS animation (glow pulse)
- An `animateMessage(sourceId, targetId, color)` function that moves a circle along the connection path

Export functions: `initTopology()`, `highlightService()`, `animateMessage()`

- [ ] **Step 2: Commit**

```bash
git add apps/platform-dashboard/static/topology.js
git commit -m "add D3.js animated service topology"
```

---

### Task 5: Build event handling and SSE connection

**Files:**
- Create: `apps/platform-dashboard/static/events.js`

- [ ] **Step 1: Write SSE event handler**

```javascript
let eventSource = null;
const sagas = new Map(); // paymentId -> saga state

function connectSSE() {
  eventSource = new EventSource("/api/events");

  eventSource.onopen = () => {
    document.getElementById("connection-status").className = "status-connected";
    document.getElementById("connection-status").textContent = "Connected";
  };

  eventSource.onmessage = (event) => {
    const data = JSON.parse(event.data);
    if (data.type === "connected") return; // Initial handshake

    handleEvent(data);
  };

  eventSource.onerror = () => {
    document.getElementById("connection-status").className = "status-disconnected";
    document.getElementById("connection-status").textContent = "Reconnecting...";
  };
}

function handleEvent(data) {
  addEventToLog(data);
  updateSagaStatus(data);
  animateTopology(data);
}

function addEventToLog(data) {
  const container = document.getElementById("events-container");
  const entry = document.createElement("div");
  entry.className = `event-entry event-${getEventClass(data.event_type)}`;

  const time = new Date(data.timestamp).toLocaleTimeString();
  entry.innerHTML = `<span class="event-time">${time}</span> <span class="event-message">${data.message || data.event_type}</span>`;

  container.insertBefore(entry, container.firstChild);

  // Keep max 100 entries
  while (container.children.length > 100) {
    container.removeChild(container.lastChild);
  }
}

function updateSagaStatus(data) {
  // Extract payment_id from event data
  const paymentId = data.data?.payment_id;
  if (!paymentId) return;

  let saga = sagas.get(paymentId);
  if (!saga) {
    saga = { paymentId, state: "STARTED", events: [] };
    sagas.set(paymentId, saga);
  }

  saga.events.push(data.event_type);
  saga.state = mapEventToState(data.event_type);

  renderSagas();
}

function mapEventToState(eventType) {
  const stateMap = {
    PaymentStarted: "STARTED",
    FundsReserved: "FUNDS RESERVED",
    InsufficientFunds: "FAILED",
    ProcessPayment: "PROCESSING",
    PaymentSucceeded: "COMPLETED",
    PaymentFailed: "FAILED",
    SendNotification: "NOTIFIED",
  };
  return stateMap[eventType] || eventType;
}

function animateTopology(data) {
  const flowMap = {
    PaymentStarted: { source: "payment", target: "wallet", color: "#58a6ff" },
    ReserveFunds: { source: "payment", target: "wallet", color: "#58a6ff" },
    FundsReserved: { source: "wallet", target: "payment", color: "#3fb950" },
    InsufficientFunds: { source: "wallet", target: "payment", color: "#f85149" },
    ProcessPayment: { source: "payment", target: "gateway", color: "#d2a8ff" },
    PaymentSucceeded: { source: "gateway", target: "payment", color: "#3fb950" },
    PaymentFailed: { source: "gateway", target: "payment", color: "#f85149" },
    SendNotification: { source: "payment", target: "notification", color: "#3fb950" },
  };

  const flow = flowMap[data.event_type];
  if (flow) {
    highlightService(flow.source, flow.color);
    animateMessage(flow.source, flow.target, flow.color);
    setTimeout(() => highlightService(flow.target, flow.color), 500);
  }
}

function getEventClass(eventType) {
  if (["PaymentSucceeded", "FundsReserved", "SendNotification"].includes(eventType)) return "success";
  if (["PaymentFailed", "InsufficientFunds"].includes(eventType)) return "error";
  return "info";
}

function renderSagas() {
  const container = document.getElementById("sagas-container");
  container.innerHTML = "";

  // Show most recent 10 sagas
  const entries = [...sagas.entries()].slice(-10).reverse();
  for (const [id, saga] of entries) {
    const div = document.createElement("div");
    div.className = `saga-entry saga-${saga.state === "COMPLETED" || saga.state === "NOTIFIED" ? "success" : saga.state === "FAILED" ? "error" : "active"}`;
    div.innerHTML = `<strong>${id.substring(0, 8)}...</strong> ${saga.state}`;
    container.appendChild(div);
  }
}
```

- [ ] **Step 2: Commit**

```bash
git add apps/platform-dashboard/static/events.js
git commit -m "add SSE event handling with saga tracking"
```

---

### Task 6: Build app.js — trigger functions and initialization

**Files:**
- Create: `apps/platform-dashboard/static/app.js`

- [ ] **Step 1: Write app.js**

```javascript
// Initialize on page load
document.addEventListener("DOMContentLoaded", () => {
  initTopology();
  connectSSE();
});

async function triggerPayment() {
  const btn = document.getElementById("btn-trigger-payment");
  btn.disabled = true;
  btn.textContent = "Sending...";

  try {
    const resp = await fetch("/api/trigger", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Idempotency-Key": crypto.randomUUID(),
      },
      body: JSON.stringify({
        user_id: "demo-user-001",
        amount: Math.floor(Math.random() * 100) + 10,
      }),
    });

    if (!resp.ok) {
      const err = await resp.text();
      console.error("Trigger failed:", err);
    }
  } catch (err) {
    console.error("Trigger error:", err);
  } finally {
    btn.disabled = false;
    btn.textContent = "▶ Trigger Payment";
  }
}

async function triggerFailure() {
  const btn = document.getElementById("btn-trigger-failure");
  btn.disabled = true;
  btn.textContent = "Sending...";

  try {
    // Trigger with a user that has no wallet balance — will cause InsufficientFunds
    const resp = await fetch("/api/trigger", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Idempotency-Key": crypto.randomUUID(),
      },
      body: JSON.stringify({
        user_id: "empty-wallet-user",
        amount: 99999,
      }),
    });

    if (!resp.ok) {
      const err = await resp.text();
      console.error("Trigger failed:", err);
    }
  } catch (err) {
    console.error("Trigger error:", err);
  } finally {
    btn.disabled = false;
    btn.textContent = "▶ Trigger Failure Demo";
  }
}
```

- [ ] **Step 2: Commit**

```bash
git add apps/platform-dashboard/static/app.js
git commit -m "add trigger functions and app initialization"
```

---

### Task 7: End-to-end dashboard test

- [ ] **Step 1: Start the environment**

Run: `npm run env:up`
Expected: All services healthy

- [ ] **Step 2: Seed a demo wallet**

Run: `curl -X POST http://localhost:8083/wallets/demo-user-001/credit -H "Content-Type: application/json" -d '{"amount": 1000}'`
Expected: 200 OK with updated balance

- [ ] **Step 3: Open dashboard and test**

Open `http://localhost:3000` in browser. Verify:
- Topology SVG renders with 5 service nodes
- Connection status shows "Connected" (green)
- Click "Trigger Payment" — see animated dots flowing between nodes
- Event log populates with PaymentStarted → FundsReserved → ProcessPayment → PaymentSucceeded/PaymentFailed
- Saga status panel shows the payment progression

- [ ] **Step 4: Test failure scenario**

Click "Trigger Failure Demo". Verify:
- Event shows InsufficientFunds (red)
- Saga status shows FAILED

- [ ] **Step 5: Tear down**

Run: `npm run env:down`

- [ ] **Step 6: Commit any fixes**

If any adjustments were needed during testing, commit them.
