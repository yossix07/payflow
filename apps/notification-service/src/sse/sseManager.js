const MAX_CONNECTIONS = parseInt(process.env.SSE_MAX_CONNECTIONS || '1000', 10);
const HEARTBEAT_INTERVAL = 30000; // 30s
const IDLE_TIMEOUT = parseInt(process.env.SSE_IDLE_TIMEOUT_MS || '900000', 10); // 15 min

const clients = new Map(); // res -> { lastWrite: timestamp }
let heartbeatTimer = null;

function addClient(res) {
  if (clients.size >= MAX_CONNECTIONS) {
    console.warn(`SSE connection limit reached (${MAX_CONNECTIONS}), rejecting`);
    res.writeHead(503);
    res.end('Connection limit reached');
    return false;
  }

  clients.set(res, { lastWrite: Date.now() });
  console.log(`SSE client connected. Total clients: ${clients.size}`);

  res.on('close', () => {
    clients.delete(res);
    console.log(`SSE client disconnected. Total clients: ${clients.size}`);
  });

  if (clients.size === 1 && !heartbeatTimer) {
    startHeartbeat();
  }

  return true;
}

function broadcastEvent(notification) {
  const data = JSON.stringify(notification);
  const total = clients.size;
  let sent = 0;
  const dead = [];

  for (const [client, meta] of clients) {
    try {
      client.write(`data: ${data}\n\n`);
      meta.lastWrite = Date.now();
      sent++;
    } catch (err) {
      dead.push(client);
    }
  }

  for (const client of dead) {
    clients.delete(client);
  }

  console.log(`Broadcast event to ${sent}/${total} SSE clients`);
}

function startHeartbeat() {
  heartbeatTimer = setInterval(() => {
    const now = Date.now();
    const dead = [];

    for (const [client, meta] of clients) {
      try {
        client.write(': ping\n\n');
        meta.lastWrite = now;
      } catch (err) {
        dead.push(client);
      }

      if (now - meta.lastWrite > IDLE_TIMEOUT) {
        dead.push(client);
      }
    }

    for (const client of dead) {
      try { client.end(); } catch (e) { /* ignore */ }
      clients.delete(client);
    }

    if (clients.size === 0) {
      clearInterval(heartbeatTimer);
      heartbeatTimer = null;
    }
  }, HEARTBEAT_INTERVAL);
}

function getClientCount() {
  return clients.size;
}

module.exports = { addClient, broadcastEvent, getClientCount };
