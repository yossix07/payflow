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
