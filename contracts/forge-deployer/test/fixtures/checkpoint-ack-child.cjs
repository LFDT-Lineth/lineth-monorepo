const { writeFileSync } = require("node:fs");

const records = JSON.parse(process.env.TEST_DEPLOYMENT_RECORDS);
const markerFile = process.env.TEST_SECOND_TRANSACTION_MARKER;

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
  await sendRecord(records[0], 0);
  writeFileSync(markerFile, "second transaction submitted");
  await sendRecord(records[1], 1);
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
