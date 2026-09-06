// Emits a bootstrap record with no prior bootstrap intent. The parent must
// reject it ("without a durable bootstrap intent"), which this child surfaces
// as a non-zero exit so the test can assert the failure.
const record = JSON.parse(process.env.TEST_BOOTSTRAP_RECORD);

async function main() {
  if (!process.send) throw new Error("checkpoint acknowledgement IPC is unavailable");

  const id = "bootstrap-record-0";
  const acknowledgement = new Promise((resolve, reject) => {
    const onMessage = (message) => {
      if (!message || message.type !== "lineth-bootstrap-record-ack" || message.id !== id) return;
      process.off("message", onMessage);
      if (message.error) reject(new Error(message.error));
      else resolve();
    };
    process.on("message", onMessage);
  });

  process.send({ type: "lineth-bootstrap-record", id, record });
  await acknowledgement;
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
