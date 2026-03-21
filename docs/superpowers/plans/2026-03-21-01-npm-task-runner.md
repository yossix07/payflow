# Plan 1: Root npm Task Runner & Local Environment Orchestration

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create a root `package.json` as the centralized command center for local dev, testing, and deployment workflows.

**Architecture:** A scripts-only root `package.json` with `concurrently` for parallel execution. Shell scripts under `scripts/` handle the orchestration logic (health-check polling, LocalStack init wait). All services are managed via `docker-compose`.

**Tech Stack:** npm scripts, Docker Compose, Node.js scripts for cross-platform compatibility

**Spec:** `docs/superpowers/specs/2026-03-21-local-test-deploy-visualization-design.md`

---

### Task 1: Create root package.json

**Files:**
- Create: `package.json`

- [ ] **Step 1: Create root package.json with all script stubs**

```json
{
  "name": "saas-payment-platform",
  "version": "1.0.0",
  "private": true,
  "description": "Distributed SaaS payment processing platform with event-driven microservices",
  "scripts": {
    "env:up": "node scripts/env-up.js",
    "env:down": "docker compose down -v",
    "env:status": "node scripts/env-status.js",
    "test:unit": "node scripts/test-unit.js",
    "test:integration": "node scripts/test-integration.js",
    "test": "npm run test:unit && npm run test:integration",
    "demo": "npm run env:up && node scripts/open-browser.js http://localhost:3000",
    "demo:trigger": "node scripts/demo-flow.js",
    "infra:up": "node scripts/infra-up.js",
    "infra:down": "cd envs/dev && terraform destroy -auto-approve",
    "deploy": "npm run test && node scripts/deploy.js"
  },
  "devDependencies": {
    "concurrently": "^9.0.0"
  }
}
```

- [ ] **Step 2: Install dependencies**

Run: `npm install`
Expected: `node_modules/` created, `package-lock.json` generated

- [ ] **Step 3: Commit**

```bash
git add package.json package-lock.json
git commit -m "add root package.json as centralized task runner"
```

---

### Task 2: Create env:up script with health-check polling

**Files:**
- Create: `scripts/env-up.js`

- [ ] **Step 1: Write env-up.js**

```javascript
const { execSync, spawn } = require("child_process");
const http = require("http");

const SERVICES = [
  { name: "localstack", url: "http://localhost:4566/_localstack/health" },
  { name: "payment-service", url: "http://localhost:8081/healthz" },
  { name: "wallet-service", url: "http://localhost:8083/healthz" },
  { name: "ledger-service", url: "http://localhost:8082/healthz" },
  { name: "gateway-service", url: "http://localhost:8084/healthz" },
  { name: "notification-service", url: "http://localhost:8085/healthz" },
  { name: "platform-dashboard", url: "http://localhost:3000/healthz" },
];

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
```

- [ ] **Step 2: Test env:up manually**

Run: `npm run env:up`
Expected: Docker containers start, health checks pass one by one, "All services are healthy!" printed

- [ ] **Step 3: Test env:down**

Run: `npm run env:down`
Expected: All containers stopped and volumes removed

- [ ] **Step 4: Commit**

```bash
git add scripts/env-up.js
git commit -m "add env:up script with health-check polling"
```

---

### Task 3: Create env:status script

**Files:**
- Create: `scripts/env-status.js`

- [ ] **Step 1: Write env-status.js**

```javascript
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
```

- [ ] **Step 2: Test**

Run: `npm run env:status`
Expected: Table showing each service's health status

- [ ] **Step 3: Commit**

```bash
git add scripts/env-status.js
git commit -m "add env:status script for service health overview"
```

---

### Task 4: Create test runner scripts (stubs)

**Files:**
- Create: `scripts/test-unit.js`
- Create: `scripts/test-integration.js`

- [ ] **Step 1: Write test-unit.js stub**

```javascript
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
```

- [ ] **Step 2: Write test-integration.js stub**

```javascript
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
```

- [ ] **Step 3: Commit**

```bash
git add scripts/test-unit.js scripts/test-integration.js
git commit -m "add test runner scripts for unit and integration tests"
```

---

### Task 5: Create open-browser utility

**Files:**
- Create: `scripts/open-browser.js`

- [ ] **Step 1: Write open-browser.js**

```javascript
const { exec } = require("child_process");

const url = process.argv[2] || "http://localhost:3000";

// Cross-platform browser open
const cmd = process.platform === "win32" ? `start ${url}`
  : process.platform === "darwin" ? `open ${url}`
  : `xdg-open ${url}`;

exec(cmd, (err) => {
  if (err) console.log(`Open ${url} in your browser`);
});
```

- [ ] **Step 2: Commit**

```bash
git add scripts/open-browser.js
git commit -m "add cross-platform browser open utility"
```

---

### Task 6: Add .gitignore for node_modules

**Files:**
- Modify: `.gitignore`

- [ ] **Step 1: Ensure node_modules is in root .gitignore**

Check if `.gitignore` exists at root. If not, create it. Ensure it contains `node_modules/`.

- [ ] **Step 2: Commit if changed**

```bash
git add .gitignore
git commit -m "add node_modules to gitignore"
```
