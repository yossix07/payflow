# Plan 6: AWS Deploy-on-Demand & Cost Protection

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create `infra:up`, `infra:down`, and `deploy` npm scripts that enforce the deploy-on-demand workflow: test locally first, deploy to AWS only when needed, tear down to $0 when done.

**Architecture:** Node.js scripts that wrap Terraform and kubectl commands. The `deploy` script gates on passing tests before building and pushing Docker images to ECR and applying Kubernetes manifests.

**Tech Stack:** Node.js scripts, Terraform CLI, AWS CLI, Docker, kubectl

**Spec:** `docs/superpowers/specs/2026-03-21-local-test-deploy-visualization-design.md`

---

### Task 1: Create infra:up script

**Files:**
- Create: `scripts/infra-up.js`

- [ ] **Step 1: Write infra-up.js**

```javascript
const { execSync } = require("child_process");
const readline = require("readline");

const COST_ESTIMATE = `
Estimated hourly cost while infrastructure is running:
  EKS Control Plane:  ~$0.10/hr
  NAT Gateway:        ~$0.045/hr
  EC2 Nodes (t3.medium x2): ~$0.084/hr
  Load Balancer:      ~$0.025/hr
  ─────────────────────────────
  Total:              ~$0.25/hr (~$6/day)

Remember to run 'npm run infra:down' when done!
`;

function ask(question) {
  const rl = readline.createInterface({ input: process.stdin, output: process.stdout });
  return new Promise((resolve) => {
    rl.question(question, (answer) => {
      rl.close();
      resolve(answer.toLowerCase());
    });
  });
}

async function main() {
  console.log(COST_ESTIMATE);
  const answer = await ask("Proceed with terraform apply? (yes/no): ");
  if (answer !== "yes" && answer !== "y") {
    console.log("Cancelled.");
    process.exit(0);
  }

  console.log("\nInitializing Terraform...");
  execSync("terraform init", { stdio: "inherit", cwd: "envs/dev" });

  console.log("\nRunning terraform plan...");
  execSync("terraform plan", { stdio: "inherit", cwd: "envs/dev" });

  const confirm = await ask("\nApply this plan? (yes/no): ");
  if (confirm !== "yes" && confirm !== "y") {
    console.log("Cancelled.");
    process.exit(0);
  }

  console.log("\nApplying infrastructure...");
  execSync("terraform apply -auto-approve", { stdio: "inherit", cwd: "envs/dev" });

  console.log("\nInfrastructure is up!");
  console.log("Run 'npm run deploy' to deploy services.");
  console.log("Run 'npm run infra:down' when done to avoid charges.");
}

main().catch((err) => {
  console.error(err.message);
  process.exit(1);
});
```

- [ ] **Step 2: Commit**

```bash
git add scripts/infra-up.js
git commit -m "add infra:up script with cost estimate and confirmation"
```

---

### Task 2: Create deploy script

**Files:**
- Create: `scripts/deploy.js`

- [ ] **Step 1: Write deploy.js**

```javascript
const { execSync } = require("child_process");

const SERVICES = [
  { name: "payment-service", dir: "apps/payment-service" },
  { name: "wallet-service", dir: "apps/wallet-service" },
  { name: "ledger-service", dir: "apps/ledger-service" },
  { name: "gateway-service", dir: "apps/gateway-service" },
  { name: "notification-service", dir: "apps/notification-service" },
  { name: "platform-dashboard", dir: "apps/platform-dashboard" },
];

function run(cmd, opts = {}) {
  console.log(`> ${cmd}`);
  execSync(cmd, { stdio: "inherit", ...opts });
}

async function main() {
  // Step 1: Run all tests
  console.log("=== Step 1: Running tests ===\n");
  try {
    run("npm run test:unit");
  } catch (err) {
    console.error("\nTests failed. Fix tests before deploying.");
    process.exit(1);
  }

  // Step 2: Get ECR registry URL
  console.log("\n=== Step 2: Getting ECR registry ===\n");
  const region = process.env.AWS_REGION || "us-east-1";
  const accountId = execSync("aws sts get-caller-identity --query Account --output text").toString().trim();
  const registry = `${accountId}.dkr.ecr.${region}.amazonaws.com`;

  // ECR login
  console.log("Logging into ECR...");
  run(`aws ecr get-login-password --region ${region} | docker login --username AWS --password-stdin ${registry}`);

  // Step 3: Build, tag, push each service
  console.log("\n=== Step 3: Building and pushing images ===\n");
  for (const svc of SERVICES) {
    const image = `${registry}/${svc.name}:latest`;
    console.log(`\n--- ${svc.name} ---`);
    run(`docker build -t ${svc.name} ${svc.dir}`);
    run(`docker tag ${svc.name} ${image}`);
    run(`docker push ${image}`);
  }

  // Step 4: Apply Kubernetes manifests
  console.log("\n=== Step 4: Applying Kubernetes manifests ===\n");
  for (const svc of SERVICES) {
    const k8sDir = `${svc.dir}/k8s`;
    console.log(`Applying ${svc.name}...`);
    run(`kubectl apply -f ${k8sDir}/deployment.yaml`);
  }

  // Step 5: Restart deployments to pull new images
  console.log("\n=== Step 5: Rolling restart ===\n");
  for (const svc of SERVICES) {
    run(`kubectl rollout restart deployment/${svc.name}`);
  }

  console.log("\n=== Deploy complete! ===");
  console.log("Run 'kubectl get pods' to check status.");
  console.log("Run 'npm run infra:down' when done to avoid charges.");
}

main().catch((err) => {
  console.error(err.message);
  process.exit(1);
});
```

- [ ] **Step 2: Commit**

```bash
git add scripts/deploy.js
git commit -m "add deploy script with test gate and ECR push"
```

---

### Task 3: Create demo-flow CLI script

**Files:**
- Create: `scripts/demo-flow.js`

- [ ] **Step 1: Write demo-flow.js**

```javascript
const http = require("http");

const PAYMENT_URL = process.env.PAYMENT_SERVICE_URL || "http://localhost:8081";
const WALLET_URL = process.env.WALLET_SERVICE_URL || "http://localhost:8083";
const LEDGER_URL = process.env.LEDGER_SERVICE_URL || "http://localhost:8082";

function post(url, data, headers = {}) {
  return new Promise((resolve, reject) => {
    const body = JSON.stringify(data);
    const parsed = new URL(url);
    const req = http.request(
      {
        hostname: parsed.hostname,
        port: parsed.port,
        path: parsed.pathname,
        method: "POST",
        headers: { "Content-Type": "application/json", ...headers },
      },
      (res) => {
        let buf = "";
        res.on("data", (d) => (buf += d));
        res.on("end", () => resolve({ status: res.statusCode, body: JSON.parse(buf || "{}") }));
      }
    );
    req.on("error", reject);
    req.write(body);
    req.end();
  });
}

function get(url) {
  return new Promise((resolve, reject) => {
    http.get(url, (res) => {
      let buf = "";
      res.on("data", (d) => (buf += d));
      res.on("end", () => resolve(JSON.parse(buf || "[]")));
    }).on("error", reject);
  });
}

function sleep(ms) {
  return new Promise((r) => setTimeout(r, ms));
}

async function main() {
  const mode = process.argv[2] || "success";
  const userID = mode === "fail" ? "demo-empty-wallet" : "demo-user-001";
  const amount = mode === "fail" ? 99999 : Math.floor(Math.random() * 100) + 10;

  console.log(`\n=== Demo Flow (${mode} scenario) ===\n`);

  // Step 1: Seed wallet (only for success scenario)
  if (mode !== "fail") {
    console.log("1. Seeding wallet with $1000...");
    await post(`${WALLET_URL}/wallets/${userID}/credit`, { amount: 1000 });
    console.log("   Done.\n");
  }

  // Step 2: Create payment
  console.log(`2. Creating payment: $${amount} for ${userID}...`);
  const { body: payment } = await post(`${PAYMENT_URL}/payments`, {
    user_id: userID,
    amount: amount,
  }, { "Idempotency-Key": `demo-${Date.now()}` });

  console.log(`   Payment ID: ${payment.payment_id}`);
  console.log(`   Initial state: ${payment.state}\n`);

  // Step 3: Poll for completion
  console.log("3. Tracking saga progression...");
  let finalState = payment.state;
  for (let i = 0; i < 30; i++) {
    await sleep(1000);
    const current = await get(`${PAYMENT_URL}/payments/${payment.payment_id}`);
    if (current.state !== finalState) {
      finalState = current.state;
      const icon = finalState === "COMPLETED" ? "[OK]" : finalState === "FAILED" ? "[FAIL]" : "[..]";
      console.log(`   ${icon} State: ${finalState}`);
    }
    if (["COMPLETED", "FAILED"].includes(finalState)) break;
  }

  // Step 4: Final summary
  console.log("\n4. Final Summary:");
  const wallet = await get(`${WALLET_URL}/wallets/${userID}`);
  const ledger = await get(`${LEDGER_URL}/ledger/payment/${payment.payment_id}`);

  console.log(`   Payment: ${payment.payment_id} -> ${finalState}`);
  console.log(`   Wallet balance: $${wallet.balance || 0}`);
  console.log(`   Ledger entries: ${ledger.length}`);
  console.log("");
}

main().catch((err) => {
  console.error("Error:", err.message);
  process.exit(1);
});
```

- [ ] **Step 2: Test demo script**

Run: `npm run env:up` (if not already running)
Run: `npm run demo:trigger`
Expected: Prints saga progression with state changes

Run: `node scripts/demo-flow.js fail`
Expected: Shows failure scenario with InsufficientFunds

- [ ] **Step 3: Commit**

```bash
git add scripts/demo-flow.js
git commit -m "add CLI demo flow script for success and failure scenarios"
```
