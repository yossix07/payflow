let shuttingDown = false;

function shutdown() {
  shuttingDown = true;
}

function isShuttingDown() {
  return shuttingDown;
}

module.exports = { shutdown, isShuttingDown };
