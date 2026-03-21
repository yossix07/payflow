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
  // Step 1: Run unit tests
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

  console.log("Logging into ECR...");
  run(`aws ecr get-login-password --region ${region} | docker login --username AWS --password-stdin ${registry}`);

  // Step 3: Build, tag, push
  console.log("\n=== Step 3: Building and pushing images ===\n");
  for (const svc of SERVICES) {
    const image = `${registry}/${svc.name}:latest`;
    console.log(`\n--- ${svc.name} ---`);
    run(`docker build -t ${svc.name} ${svc.dir}`);
    run(`docker tag ${svc.name} ${image}`);
    run(`docker push ${image}`);
  }

  // Step 4: Apply K8s manifests
  console.log("\n=== Step 4: Applying Kubernetes manifests ===\n");
  for (const svc of SERVICES) {
    console.log(`Applying ${svc.name}...`);
    run(`kubectl apply -f ${svc.dir}/k8s/deployment.yaml`);
  }

  // Step 5: Rolling restart
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
