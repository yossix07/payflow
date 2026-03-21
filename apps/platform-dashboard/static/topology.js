/* topology.js — D3.js animated service topology graph */

const services = [
  { id: "payment",      label: "Payment Service",  port: ":8081", x: 150, y: 80  },
  { id: "wallet",       label: "Wallet Service",   port: ":8083", x: 400, y: 80  },
  { id: "gateway",      label: "Gateway Service",  port: ":8084", x: 650, y: 80  },
  { id: "ledger",       label: "Ledger Service",   port: ":8082", x: 150, y: 250 },
  { id: "notification", label: "Notification",      port: ":8085", x: 650, y: 250 },
];

const connections = [
  { source: "payment", target: "wallet",       label: "ReserveFunds"  },
  { source: "wallet",  target: "payment",      label: "FundsReserved" },
  { source: "payment", target: "gateway",      label: "ProcessPayment"},
  { source: "gateway", target: "payment",      label: "PaymentResult" },
  { source: "payment", target: "ledger",       label: "record"        },
  { source: "payment", target: "notification", label: "notify"        },
];

const NODE_W = 140;
const NODE_H = 48;

/* Map for quick node lookup */
const serviceMap = new Map();
services.forEach(s => serviceMap.set(s.id, s));

/* Compute a path between two nodes, slightly offset for bidirectional arrows */
function computePath(conn) {
  const s = serviceMap.get(conn.source);
  const t = serviceMap.get(conn.target);

  /* Determine exit/entry edges */
  const dx = t.x - s.x;
  const dy = t.y - s.y;

  let x1, y1, x2, y2;

  if (Math.abs(dx) > Math.abs(dy)) {
    /* Horizontal dominant */
    x1 = dx > 0 ? s.x + NODE_W / 2 : s.x - NODE_W / 2;
    y1 = s.y;
    x2 = dx > 0 ? t.x - NODE_W / 2 : t.x + NODE_W / 2;
    y2 = t.y;
  } else {
    /* Vertical dominant */
    x1 = s.x;
    y1 = dy > 0 ? s.y + NODE_H / 2 : s.y - NODE_H / 2;
    x2 = t.x;
    y2 = dy > 0 ? t.y - NODE_H / 2 : t.y + NODE_H / 2;
  }

  /* Small perpendicular offset so bidirectional arrows don't overlap */
  const len = Math.sqrt((x2 - x1) ** 2 + (y2 - y1) ** 2) || 1;
  const nx = -(y2 - y1) / len;
  const ny = (x2 - x1) / len;
  const offset = 6;
  x1 += nx * offset; y1 += ny * offset;
  x2 += nx * offset; y2 += ny * offset;

  return { x1, y1, x2, y2 };
}

function initTopology() {
  const svg = d3.select("#topology-svg");

  /* Responsive viewBox */
  svg.attr("viewBox", "0 0 800 330").attr("preserveAspectRatio", "xMidYMid meet");

  /* Arrow marker definition */
  const defs = svg.append("defs");

  defs.append("marker")
    .attr("id", "arrowhead")
    .attr("viewBox", "0 0 10 6")
    .attr("refX", 9)
    .attr("refY", 3)
    .attr("markerWidth", 8)
    .attr("markerHeight", 6)
    .attr("orient", "auto")
    .append("path")
    .attr("d", "M0,0 L10,3 L0,6 Z")
    .attr("fill", "#484f58");

  /* Draw connections */
  const connGroup = svg.append("g").attr("class", "connections");

  connections.forEach(conn => {
    const p = computePath(conn);

    connGroup.append("line")
      .attr("class", "connection-line")
      .attr("data-source", conn.source)
      .attr("data-target", conn.target)
      .attr("x1", p.x1).attr("y1", p.y1)
      .attr("x2", p.x2).attr("y2", p.y2)
      .attr("marker-end", "url(#arrowhead)");

    /* Label at midpoint */
    connGroup.append("text")
      .attr("class", "connection-label")
      .attr("x", (p.x1 + p.x2) / 2)
      .attr("y", (p.y1 + p.y2) / 2 - 8)
      .attr("text-anchor", "middle")
      .text(conn.label);
  });

  /* Draw service nodes */
  const nodeGroup = svg.append("g").attr("class", "nodes");

  services.forEach(s => {
    const g = nodeGroup.append("g")
      .attr("class", "service-node")
      .attr("data-id", s.id)
      .attr("transform", `translate(${s.x}, ${s.y})`);

    g.append("rect")
      .attr("class", "node-rect")
      .attr("x", -NODE_W / 2)
      .attr("y", -NODE_H / 2)
      .attr("width", NODE_W)
      .attr("height", NODE_H);

    g.append("text")
      .attr("class", "node-label")
      .attr("y", -4)
      .text(s.label);

    g.append("text")
      .attr("class", "node-sublabel")
      .attr("y", 14)
      .text(s.port);
  });
}

/**
 * Highlight a service node with a colored glow.
 */
function highlightService(serviceId, color) {
  const node = d3.select(`.service-node[data-id="${serviceId}"]`);
  if (node.empty()) return;

  const rect = node.select(".node-rect");

  rect
    .style("stroke", color)
    .style("--glow-color", color);

  node.classed("node-active", false);
  /* Force reflow to restart animation */
  void node.node().offsetWidth;
  node.style("--glow-color", color);
  node.classed("node-active", true);

  /* Reset after animation */
  setTimeout(() => {
    node.classed("node-active", false);
    rect.style("stroke", null);
  }, 900);
}

/**
 * Animate a circle travelling from source to target node.
 */
function animateMessage(sourceId, targetId, color) {
  const svg = d3.select("#topology-svg");
  const s = serviceMap.get(sourceId);
  const t = serviceMap.get(targetId);
  if (!s || !t) return;

  /* Find the matching connection path coords */
  const conn = connections.find(c => c.source === sourceId && c.target === targetId);
  let p;
  if (conn) {
    p = computePath(conn);
  } else {
    /* Fallback: compute a direct path */
    p = computePath({ source: sourceId, target: targetId });
  }

  const dot = svg.append("circle")
    .attr("class", "message-dot")
    .attr("r", 5)
    .attr("cx", p.x1)
    .attr("cy", p.y1)
    .attr("fill", color)
    .style("color", color);

  dot.transition()
    .duration(600)
    .ease(d3.easeCubicInOut)
    .attr("cx", p.x2)
    .attr("cy", p.y2)
    .on("end", () => {
      highlightService(targetId, color);
      dot.transition()
        .duration(200)
        .attr("r", 0)
        .remove();
    });

  /* Also highlight the source immediately */
  highlightService(sourceId, color);
}
