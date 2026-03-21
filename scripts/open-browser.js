const { execFile } = require("child_process");

const url = process.argv[2] || "http://localhost:3000";

// Cross-platform browser open (uses execFile to avoid shell injection)
const openers = {
  win32: ["cmd", ["/c", "start", url]],
  darwin: ["open", [url]],
  linux: ["xdg-open", [url]],
};

const [bin, args] = openers[process.platform] || openers.linux;

execFile(bin, args, (err) => {
  if (err) console.log(`Open ${url} in your browser`);
});
