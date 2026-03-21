/* events.js — SSE connection, event log, and saga tracking */

let eventSource = null;
const sagas = new Map();

/* ===== Flow map: event type -> topology animation ===== */
const flowMap = {
  PaymentStarted:    { source: "payment",  target: "wallet",       color: "#58a6ff" },
  ReserveFunds:      { source: "payment",  target: "wallet",       color: "#58a6ff" },
  FundsReserved:     { source: "wallet",   target: "payment",      color: "#3fb950" },
  InsufficientFunds: { source: "wallet",   target: "payment",      color: "#f85149" },
  ProcessPayment:    { source: "payment",  target: "gateway",      color: "#d2a8ff" },
  PaymentSucceeded:  { source: "gateway",  target: "payment",      color: "#3fb950" },
  PaymentFailed:     { source: "gateway",  target: "payment",      color: "#f85149" },
  SendNotification:  { source: "payment",  target: "notification", color: "#3fb950" },
  RecordTransaction: { source: "payment",  target: "ledger",       color: "#58a6ff" },
};

/* Event type -> visual category */
const categoryMap = {
  PaymentStarted:    "info",
  ReserveFunds:      "info",
  FundsReserved:     "success",
  InsufficientFunds: "error",
  ProcessPayment:    "process",
  PaymentSucceeded:  "success",
  PaymentFailed:     "error",
  SendNotification:  "success",
  RecordTransaction: "info",
  ReleaseFunds:      "error",
};

/* Event type -> saga state */
const sagaStateMap = {
  PaymentStarted:    "started",
  ReserveFunds:      "reserving",
  FundsReserved:     "processing",
  ProcessPayment:    "processing",
  PaymentSucceeded:  "completed",
  PaymentFailed:     "failed",
  InsufficientFunds: "failed",
  SendNotification:  "completed",
  ReleaseFunds:      "failed",
};

/* ===== SSE connection ===== */
function connectSSE() {
  const statusEl = document.getElementById("connection-status");

  eventSource = new EventSource("/api/events");

  eventSource.onopen = function () {
    statusEl.textContent = "Connected";
    statusEl.className = "status-connected";
  };

  eventSource.onmessage = function (e) {
    let data;
    try {
      data = JSON.parse(e.data);
    } catch (_) {
      return;
    }

    /* Skip the initial "connected" handshake event */
    if (data.type === "connected") return;

    handleEvent(data);
  };

  eventSource.onerror = function () {
    statusEl.textContent = "Reconnecting...";
    statusEl.className = "status-reconnecting";
  };
}

/* ===== Central event handler ===== */
function handleEvent(data) {
  addEventToLog(data);
  updateSagaStatus(data);
  animateTopology(data);
}

/* ===== Event log ===== */
function addEventToLog(data) {
  const container = document.getElementById("events-container");

  /* Remove empty-state placeholder if present */
  const placeholder = container.querySelector(".empty-state");
  if (placeholder) placeholder.remove();

  const eventType = data.event_type || data.type || "unknown";
  const category = categoryMap[eventType] || "info";
  const timestamp = data.timestamp ? new Date(data.timestamp) : new Date();
  const timeStr = timestamp.toLocaleTimeString("en-GB", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });

  const entry = document.createElement("div");
  entry.className = "event-entry " + category;

  const paymentId = data.payment_id || data.saga_id || "";
  const shortId = paymentId ? paymentId.slice(0, 8) : "";
  const detailText = shortId
    ? shortId + (data.amount ? " \u2014 $" + Number(data.amount).toFixed(2) : "")
    : data.message || "";

  const timeSpan = document.createElement("span");
  timeSpan.className = "event-time";
  timeSpan.textContent = timeStr;

  const typeDiv = document.createElement("div");
  typeDiv.className = "event-type";
  typeDiv.style.color = flowMap[eventType] ? flowMap[eventType].color : "var(--text-primary)";
  typeDiv.textContent = eventType;

  const detailDiv = document.createElement("div");
  detailDiv.className = "event-detail";
  detailDiv.textContent = detailText;

  const bodyDiv = document.createElement("div");
  bodyDiv.className = "event-body";
  bodyDiv.appendChild(typeDiv);
  bodyDiv.appendChild(detailDiv);

  entry.appendChild(timeSpan);
  entry.appendChild(bodyDiv);

  /* Newest on top */
  container.prepend(entry);

  /* Cap at 100 entries */
  while (container.children.length > 100) {
    container.removeChild(container.lastChild);
  }
}

/* ===== Saga tracking ===== */
function updateSagaStatus(data) {
  const eventType = data.event_type || data.type || "";
  const paymentId = data.payment_id || data.saga_id || "";
  if (!paymentId) return;

  const state = sagaStateMap[eventType];
  if (!state) return;

  const existing = sagas.get(paymentId);
  if (existing) {
    existing.state = state;
    existing.lastEvent = eventType;
    existing.updatedAt = Date.now();
  } else {
    sagas.set(paymentId, {
      paymentId: paymentId,
      state: state,
      amount: data.amount || null,
      lastEvent: eventType,
      updatedAt: Date.now(),
    });
  }

  renderSagas();
}

function renderSagas() {
  const container = document.getElementById("sagas-container");
  container.innerHTML = "";

  /* Show last 10 sagas, most recent first */
  const sorted = Array.from(sagas.values())
    .sort((a, b) => b.updatedAt - a.updatedAt)
    .slice(0, 10);

  if (sorted.length === 0) {
    container.innerHTML = '<div class="empty-state">No active sagas</div>';
    return;
  }

  sorted.forEach(saga => {
    const el = document.createElement("div");
    el.className = "saga-entry";

    const shortId = saga.paymentId.slice(0, 12);
    const amountStr = saga.amount ? "$" + Number(saga.amount).toFixed(2) : "";

    const idSpan = document.createElement("span");
    idSpan.className = "saga-id";
    idSpan.title = saga.paymentId;
    idSpan.textContent = shortId;

    const amountSpan = document.createElement("span");
    amountSpan.className = "saga-amount";
    amountSpan.textContent = amountStr;

    const badgeSpan = document.createElement("span");
    badgeSpan.className = "saga-badge " + saga.state;
    badgeSpan.textContent = saga.state;

    el.appendChild(idSpan);
    el.appendChild(amountSpan);
    el.appendChild(badgeSpan);

    container.appendChild(el);
  });
}

/* ===== Topology animation ===== */
function animateTopology(data) {
  const eventType = data.event_type || data.type || "";
  const flow = flowMap[eventType];
  if (!flow) return;

  animateMessage(flow.source, flow.target, flow.color);
}
