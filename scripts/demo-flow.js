const http = require("http");

const PAYMENT_URL = process.env.PAYMENT_SERVICE_URL || "http://localhost:8081";
const WALLET_URL = process.env.WALLET_SERVICE_URL || "http://localhost:8083";
const LEDGER_URL = process.env.LEDGER_SERVICE_URL || "http://localhost:8082";

function post(url, data, headers = {}) {
  return new Promise((resolve, reject) => {
    const body = JSON.stringify(data);
    const parsed = new URL(url);
    const req = http.request({
      hostname: parsed.hostname,
      port: parsed.port,
      path: parsed.pathname,
      method: "POST",
      headers: { "Content-Type": "application/json", ...headers },
    }, (res) => {
      let buf = "";
      res.on("data", (d) => (buf += d));
      res.on("end", () => resolve({ status: res.statusCode, body: JSON.parse(buf || "{}") }));
    });
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

  if (mode !== "fail") {
    console.log("1. Seeding wallet with $1000...");
    await post(`${WALLET_URL}/wallets/${userID}/credit`, { amount: 1000 });
    console.log("   Done.\n");
  }

  console.log(`2. Creating payment: $${amount} for ${userID}...`);
  const { body: payment } = await post(`${PAYMENT_URL}/payments`, {
    user_id: userID,
    amount: amount,
  }, { "Idempotency-Key": `demo-${Date.now()}` });

  console.log(`   Payment ID: ${payment.payment_id}`);
  console.log(`   Initial state: ${payment.state}\n`);

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
