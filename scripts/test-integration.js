const { execSync } = require("child_process");

console.log("=== Running Integration Tests ===\n");

let testFailed = false;
try {
  // Start environment
  console.log("Starting test environment...");
  execSync("node scripts/env-up.js", { stdio: "inherit" });

  // Run integration tests
  console.log("\nRunning saga flow tests...");
  execSync("go test -v -timeout 60s ./tests/integration/...", {
    stdio: "inherit",
    cwd: process.cwd(),
  });
} catch (err) {
  console.error("Test run failed:", err.message);
  testFailed = true;
} finally {
  // Always tear down, even on failure
  console.log("\nTearing down test environment...");
  try {
    execSync("docker compose down -v", { stdio: "inherit" });
  } catch (e) {
    console.error("Teardown failed:", e.message);
  }
}

if (testFailed) {
  process.exit(1);
}

console.log("\n=== All integration tests passed ===");
