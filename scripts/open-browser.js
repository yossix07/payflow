const { exec } = require("child_process");

const url = process.argv[2] || "http://localhost:3000";

// Cross-platform browser open
const cmd = process.platform === "win32" ? `start ${url}`
  : process.platform === "darwin" ? `open ${url}`
  : `xdg-open ${url}`;

exec(cmd, (err) => {
  if (err) console.log(`Open ${url} in your browser`);
});
