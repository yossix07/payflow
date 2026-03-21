const { execSync } = require("child_process");

console.log("=== Running Unit Tests ===\n");

// Go services
console.log("--- Go unit tests ---");
try {
  execSync("go test ./apps/payment-service/... ./apps/wallet-service/... ./apps/ledger-service/... ./apps/platform-dashboard/...", {
    stdio: "inherit",
    cwd: process.cwd(),
  });
} catch (err) {
  console.error("Go unit tests failed");
  process.exit(1);
}

// Node services
console.log("\n--- Node unit tests ---");
const nodeServices = ["gateway-service", "notification-service"];
for (const svc of nodeServices) {
  console.log(`\nTesting ${svc}...`);
  try {
    execSync("npm test", {
      stdio: "inherit",
      cwd: `apps/${svc}`,
    });
  } catch (err) {
    console.error(`${svc} unit tests failed`);
    process.exit(1);
  }
}

console.log("\n=== All unit tests passed ===");
