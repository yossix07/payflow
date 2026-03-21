/* app.js — Initialization and trigger functions */

document.addEventListener("DOMContentLoaded", function () {
  initTopology();
  connectSSE();

  /* Seed empty-state placeholders */
  var eventsEl = document.getElementById("events-container");
  if (eventsEl && eventsEl.children.length === 0) {
    eventsEl.innerHTML = '<div class="empty-state">Waiting for events&hellip;</div>';
  }
  var sagasEl = document.getElementById("sagas-container");
  if (sagasEl && sagasEl.children.length === 0) {
    sagasEl.innerHTML = '<div class="empty-state">No active sagas</div>';
  }
});

/**
 * Trigger a normal payment flow.
 */
async function triggerPayment() {
  var btn = document.getElementById("btn-trigger-payment");
  btn.disabled = true;
  btn.textContent = "Sending...";

  try {
    var amount = Math.floor(Math.random() * 100) + 10 + Math.random();
    amount = Math.round(amount * 100) / 100;

    var res = await fetch("/api/trigger", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Idempotency-Key": crypto.randomUUID(),
      },
      body: JSON.stringify({
        user_id: "demo-user-001",
        amount: amount,
      }),
    });

    if (!res.ok) {
      console.error("Trigger failed:", res.status, await res.text());
    }
  } catch (err) {
    console.error("Trigger error:", err);
  } finally {
    btn.disabled = false;
    btn.textContent = "\u25B6 Trigger Payment";
  }
}

/**
 * Trigger a payment that will fail (insufficient funds).
 */
async function triggerFailure() {
  var btn = document.getElementById("btn-trigger-failure");
  btn.disabled = true;
  btn.textContent = "Sending...";

  try {
    var res = await fetch("/api/trigger", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Idempotency-Key": crypto.randomUUID(),
      },
      body: JSON.stringify({
        user_id: "empty-wallet-user",
        amount: 99999,
      }),
    });

    if (!res.ok) {
      console.error("Failure trigger failed:", res.status, await res.text());
    }
  } catch (err) {
    console.error("Failure trigger error:", err);
  } finally {
    btn.disabled = false;
    btn.textContent = "\u25B6 Trigger Failure Demo";
  }
}
