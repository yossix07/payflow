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
