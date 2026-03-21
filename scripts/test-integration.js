const { execSync } = require("child_process");

console.log("=== Running Integration Tests ===\n");

// Start environment
console.log("Starting test environment...");
try {
  execSync("node scripts/env-up.js", { stdio: "inherit" });
} catch (err) {
  console.error("Failed to start environment");
  process.exit(1);
}

// Run integration tests
let testFailed = false;
try {
  console.log("\nRunning saga flow tests...");
  execSync("go test -v -timeout 60s ./tests/integration/...", {
    stdio: "inherit",
    cwd: process.cwd(),
  });
} catch (err) {
  console.error("Integration tests failed");
  testFailed = true;
}

// Tear down environment
console.log("\nTearing down test environment...");
execSync("docker compose down -v", { stdio: "inherit" });

if (testFailed) {
  process.exit(1);
}

console.log("\n=== All integration tests passed ===");
