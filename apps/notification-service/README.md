# Notification Service

Real-time event broadcasting service using WebSockets for live payment updates.

## Architecture

- **Pattern**: Event-driven real-time broadcaster
- **Protocol**: WebSocket (ws://)
- **Message Queue**: AWS SQS
- **Language**: Node.js 18

## Features

- ✅ Real-time WebSocket event streaming
- ✅ Broadcasts all payment events to connected clients
- ✅ Human-readable event messages
- ✅ Multiple client support
- ✅ Auto-reconnection friendly

## API Endpoints

### Health Check

```bash
GET /healthz
```

### WebSocket Connection

```text
ws://localhost:8080/ws
```

## Environment Variables

| Variable     | Description         | Example          |
| ------------ | ------------------- | ---------------- |
| `AWS_REGION` | AWS region          | `us-east-1`      |
| `QUEUE_URL`  | SQS queue URL       | `https://sqs...` |
| `PORT`       | HTTP/WebSocket port | `8080`           |

## Event Handling

**Consumes (all payment events):**

- `PaymentStarted`
- `FundsReserved`
- `InsufficientFunds`
- `PaymentSucceeded`
- `PaymentFailed`
- `SendNotification`

**Produces:** WebSocket messages to connected clients

## WebSocket Message Format

```json
{
  "event_type": "PaymentSucceeded",
  "timestamp": "2024-01-01T00:00:00.000Z",
  "data": {
    "payment_id": "payment-123",
    "transaction_id": "txn-456"
  },
  "message": "✅ Payment payment-123 succeeded!"
}
```

## Usage Example

### JavaScript Client

```javascript
const ws = new WebSocket("ws://localhost:8080/ws");

ws.onopen = () => {
  console.log("Connected to payment event stream");
};

ws.onmessage = (event) => {
  const notification = JSON.parse(event.data);
  console.log(notification.message);

  // Update UI based on event type
  switch (notification.event_type) {
    case "PaymentSucceeded":
      showSuccessNotification(notification.data.payment_id);
      break;
    case "PaymentFailed":
      showErrorNotification(notification.data.payment_id);
      break;
  }
};

ws.onerror = (error) => {
  console.error("WebSocket error:", error);
};

ws.onclose = () => {
  console.log("Disconnected from payment event stream");
};
```

### HTML Client

```html
<!DOCTYPE html>
<html>
  <head>
    <title>Payment Events</title>
  </head>
  <body>
    <h1>Live Payment Events</h1>
    <div id="events"></div>

    <script>
      const ws = new WebSocket("ws://localhost:8080/ws");
      const eventsDiv = document.getElementById("events");

      ws.onmessage = (event) => {
        const notification = JSON.parse(event.data);
        const eventDiv = document.createElement("div");
        eventDiv.textContent = notification.message;
        eventsDiv.prepend(eventDiv);
      };
    </script>
  </body>
</html>
```

## Event Messages

| Event Type        | Message                                             |
| ----------------- | --------------------------------------------------- |
| PaymentStarted    | 💳 Payment {id} started for user {user} (${amount}) |
| FundsReserved     | 🔒 Funds reserved for payment {id}                  |
| InsufficientFunds | ❌ Insufficient funds for payment {id}              |
| PaymentSucceeded  | ✅ Payment {id} succeeded!                          |
| PaymentFailed     | ❌ Payment {id} failed: {reason}                    |
| SendNotification  | 📧 Notification sent for payment {id} ({status})    |

## Local Development

```bash
npm install

export AWS_REGION=us-east-1
export QUEUE_URL=https://sqs.us-east-1.amazonaws.com/xxx/dev-payment-events

npm start
```

## Docker Build

```bash
docker build -t notification-service .
```

## Kubernetes Deployment

```bash
kubectl apply -f k8s/deployment.yaml
```

## Client Recommendations

- Implement auto-reconnection logic
- Handle connection errors gracefully
- Parse JSON before processing messages
- Consider using WebSocket libraries (socket.io-client, etc.)
