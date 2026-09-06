const { writeFileSync } = require("node:fs");

const records = JSON.parse(process.env.TEST_DEPLOYMENT_RECORDS);
const firstBroadcastMarker = process.env.TEST_FIRST_BROADCAST_MARKER;
const markerFile = process.env.TEST_SECOND_TRANSACTION_MARKER;
const safetyTimer = setTimeout(() => {
  console.error("test fixture timed out waiting for checkpoint acknowledgement");
  process.exit(2);
}, Number(process.env.TEST_ACK_TIMEOUT_MS ?? "2000"));

async function sendIntent(contractName, index) {
  if (!process.send) throw new Error("checkpoint acknowledgement IPC is unavailable");

  const id = `intent-${index}`;
  const acknowledgement = new Promise((resolve, reject) => {
    const onMessage = (message) => {
      if (!message || message.type !== "lineth-deployment-intent-ack" || message.id !== id) return;
      process.off("message", onMessage);
      if (message.error) reject(new Error(message.error));
      else resolve();
    };
    process.on("message", onMessage);
  });

  process.send({ type: "lineth-deployment-intent", id, contractName });
  await acknowledgement;
}

async function sendRecord(record, index) {
  if (!process.send) throw new Error("checkpoint acknowledgement IPC is unavailable");

  const id = `record-${index}`;
  const acknowledgement = new Promise((resolve, reject) => {
    const onMessage = (message) => {
      if (!message || message.type !== "lineth-deployment-record-ack" || message.id !== id) return;
      process.off("message", onMessage);
      if (message.error) reject(new Error(message.error));
      else resolve();
    };
    process.on("message", onMessage);
  });

  process.send({ type: "lineth-deployment-record", id, record });
  await acknowledgement;
}

async function main() {
  await sendIntent(records[0].contractName, 0);
  writeFileSync(firstBroadcastMarker, "first transaction submitted");
  await sendRecord(records[0], 0);
  writeFileSync(markerFile, "second transaction submitted");
  await sendIntent(records[1].contractName, 1);
  await sendRecord(records[1], 1);
  clearTimeout(safetyTimer);
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
