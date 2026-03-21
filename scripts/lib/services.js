const SERVICES = [
  { name: "localstack", url: "http://localhost:4566/_localstack/health", port: 4566 },
  { name: "payment-service", url: "http://localhost:8081/healthz", port: 8081 },
  { name: "ledger-service", url: "http://localhost:8082/healthz", port: 8082 },
  { name: "wallet-service", url: "http://localhost:8083/healthz", port: 8083 },
  { name: "gateway-service", url: "http://localhost:8084/healthz", port: 8084 },
  { name: "notification-service", url: "http://localhost:8085/healthz", port: 8085 },
  { name: "platform-dashboard", url: "http://localhost:3000/healthz", port: 3000 },
];

module.exports = { SERVICES };
