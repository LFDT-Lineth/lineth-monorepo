// Emits a bootstrap intent, waits for its ack, then emits the matching record —
// the well-formed flow (and what the presigned already-deployed skip now does).
const intent = JSON.parse(process.env.TEST_BOOTSTRAP_INTENT);
const record = JSON.parse(process.env.TEST_BOOTSTRAP_RECORD);

function send(type, id, payload) {
  if (!process.send) throw new Error("checkpoint acknowledgement IPC is unavailable");
  const acknowledgement = new Promise((resolve, reject) => {
    const onMessage = (message) => {
      if (!message || message.type !== `${type}-ack` || message.id !== id) return;
      process.off("message", onMessage);
      if (message.error) reject(new Error(message.error));
      else resolve();
    };
    process.on("message", onMessage);
  });
  process.send({ type, id, ...payload });
  return acknowledgement;
}

async function main() {
  await send("lineth-bootstrap-intent", "bootstrap-intent-0", { intent });
  await send("lineth-bootstrap-record", "bootstrap-record-0", { record });
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
