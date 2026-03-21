const { execSync } = require("child_process");
const http = require("http");
const { SERVICES } = require("./lib/services");

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
