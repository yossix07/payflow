const { execSync } = require("child_process");
const http = require("http");

const SERVICES = [
  { name: "localstack", url: "http://localhost:4566/_localstack/health", port: 4566 },
  { name: "payment-service", url: "http://localhost:8081/healthz", port: 8081 },
  { name: "ledger-service", url: "http://localhost:8082/healthz", port: 8082 },
  { name: "wallet-service", url: "http://localhost:8083/healthz", port: 8083 },
  { name: "gateway-service", url: "http://localhost:8084/healthz", port: 8084 },
  { name: "notification-service", url: "http://localhost:8085/healthz", port: 8085 },
  { name: "platform-dashboard", url: "http://localhost:3000/healthz", port: 3000 },
];

function checkHealth(url) {
  return new Promise((resolve) => {
    const req = http.get(url, { timeout: 2000 }, (res) => {
      resolve(res.statusCode >= 200 && res.statusCode < 300 ? "healthy" : `HTTP ${res.statusCode}`);
    });
    req.on("error", () => resolve("unreachable"));
    req.on("timeout", () => { req.destroy(); resolve("timeout"); });
  });
}

async function main() {
  console.log("\nService Status:\n");
  console.log("  Service                  Port    Health");
  console.log("  " + "-".repeat(50));

  for (const svc of SERVICES) {
    const status = await checkHealth(svc.url);
    const icon = status === "healthy" ? "✓" : "✗";
    console.log(`  ${icon} ${svc.name.padEnd(24)} ${String(svc.port).padEnd(7)} ${status}`);
  }

  console.log("\nDocker Containers:\n");
  try {
    execSync("docker compose ps", { stdio: "inherit" });
  } catch {
    console.log("  No containers running");
  }
}

main();
