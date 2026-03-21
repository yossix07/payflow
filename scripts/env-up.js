const { execSync } = require("child_process");
const http = require("http");
const { SERVICES } = require("./lib/services");

function checkHealth(url) {
  return new Promise((resolve) => {
    const req = http.get(url, { timeout: 2000 }, (res) => {
      resolve(res.statusCode >= 200 && res.statusCode < 300);
    });
    req.on("error", () => resolve(false));
    req.on("timeout", () => {
      req.destroy();
      resolve(false);
    });
  });
}

async function waitForServices(maxWaitMs = 120000) {
  const start = Date.now();
  const pending = new Set(SERVICES.map((s) => s.name));

  console.log("Waiting for services to become healthy...\n");

  while (pending.size > 0 && Date.now() - start < maxWaitMs) {
    for (const svc of SERVICES) {
      if (!pending.has(svc.name)) continue;
      const healthy = await checkHealth(svc.url);
      if (healthy) {
        console.log(`  ✓ ${svc.name} is healthy`);
        pending.delete(svc.name);
      }
    }
    if (pending.size > 0) {
      await new Promise((r) => setTimeout(r, 2000));
    }
  }

  if (pending.size > 0) {
    console.error(`\nTimed out waiting for: ${[...pending].join(", ")}`);
    process.exit(1);
  }

  console.log("\nAll services are healthy!");
}

async function main() {
  console.log("Starting environment...\n");
  execSync("docker compose up --build -d", { stdio: "inherit" });

  await waitForServices();

  console.log("\nEnvironment is ready!");
  console.log("  Dashboard: http://localhost:3000");
  console.log("  Payment API: http://localhost:8081");
  console.log("  Wallet API: http://localhost:8083");
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
