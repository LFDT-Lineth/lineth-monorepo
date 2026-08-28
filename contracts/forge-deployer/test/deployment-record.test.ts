import assert from "node:assert/strict";
import test from "node:test";

import {
  awaitParentCheckpoint,
  awaitParentDeploymentIntent,
  formatDeploymentRecord,
  parseDeploymentRecord,
} from "../../common/helpers/deploymentRecord";

const ADDRESS = "0x1000000000000000000000000000000000000001";
const TX_HASH = `0x${"ab".repeat(32)}`;

// Temporarily makes `process` look like a connected IPC child (as it is when
// spawned by process-runner.ts), and fully restores the original
// process.send/process.connected descriptors afterwards (including "was
// unset", the normal non-IPC-child state), so awaitParent*'s ack/reject/
// disconnect branches can be exercised without spawning a real child process.
async function withMockedIpcParent<T>(
  run: (sent: { message: unknown; callback?: (error: Error | null) => void }[]) => Promise<T>,
): Promise<T> {
  const originalSend = Object.getOwnPropertyDescriptor(process, "send");
  const originalConnected = Object.getOwnPropertyDescriptor(process, "connected");
  const sent: { message: unknown; callback?: (error: Error | null) => void }[] = [];

  Object.defineProperties(process, {
    connected: { configurable: true, value: true },
    send: {
      configurable: true,
      value: (message: unknown, callback?: (error: Error | null) => void) => {
        sent.push(callback ? { message, callback } : { message });
        return true;
      },
    },
  });

  try {
    return await run(sent);
  } finally {
    if (originalSend) Object.defineProperty(process, "send", originalSend);
    else delete (process as unknown as { send?: typeof process.send }).send;
    if (originalConnected) Object.defineProperty(process, "connected", originalConnected);
    else delete (process as unknown as { connected?: boolean }).connected;
  }
}

test("formats and parses a durable deployment record", () => {
  const line = formatDeploymentRecord({
    contractName: "LinethRollupV8",
    address: ADDRESS,
    transactionHash: TX_HASH,
    blockNumber: 42,
    chainId: 1337n,
  });

  assert.equal(
    line,
    `contract=LinethRollupV8 deployed: address=${ADDRESS} blockNumber=42 chainId=1337 txHash=${TX_HASH}`,
  );
  assert.deepEqual(parseDeploymentRecord(line), {
    contractName: "LinethRollupV8",
    address: ADDRESS,
    transactionHash: TX_HASH,
    blockNumber: 42,
    chainId: "1337",
  });
});

test("ignores ordinary logs and legacy records without transaction hashes", () => {
  assert.equal(parseDeploymentRecord("Deploying LinethRollupV8..."), undefined);
  assert.equal(
    parseDeploymentRecord(`contract=LinethRollupV8 deployed: address=${ADDRESS} blockNumber=42 chainId=1337`),
    undefined,
  );
});

test("deployment intent is a no-op without an IPC parent", async () => {
  await assert.doesNotReject(awaitParentDeploymentIntent("LinethRollupV8"));
});

test("deployment intent waits for its matching parent acknowledgement", async () => {
  const originalSend = Object.getOwnPropertyDescriptor(process, "send");
  const originalConnected = Object.getOwnPropertyDescriptor(process, "connected");
  let request: { type: string; id: string; contractName: string } | undefined;
  let resolved = false;
  const emitMessage = (message: unknown) => {
    for (const listener of process.listeners("message")) listener(message, undefined);
  };

  Object.defineProperties(process, {
    connected: { configurable: true, value: true },
    send: {
      configurable: true,
      value: (message: typeof request, callback?: (error: Error | null) => void) => {
        request = message;
        callback?.(null);
        return true;
      },
    },
  });

  try {
    const acknowledgement = awaitParentDeploymentIntent("LinethRollupV8").then(() => {
      resolved = true;
    });
    await new Promise((resolve) => setImmediate(resolve));

    assert.equal(request?.type, "lineth-deployment-intent");
    assert.equal(request?.contractName, "LinethRollupV8");
    assert.equal(resolved, false);

    emitMessage({ type: "lineth-deployment-intent-ack", id: "not-the-request" });
    await new Promise((resolve) => setImmediate(resolve));
    assert.equal(resolved, false);

    emitMessage({ type: "lineth-deployment-intent-ack", id: request?.id });
    await acknowledgement;
    assert.equal(resolved, true);
  } finally {
    if (originalSend) Object.defineProperty(process, "send", originalSend);
    else delete (process as unknown as { send?: typeof process.send }).send;
    if (originalConnected) Object.defineProperty(process, "connected", originalConnected);
    else delete (process as unknown as { connected?: boolean }).connected;
  }
});

test("awaitParentCheckpoint rejects when the parent acknowledges with an error", () =>
  withMockedIpcParent(async (sent) => {
    const pending = awaitParentCheckpoint({
      contractName: "LinethRollupV8",
      address: ADDRESS,
      transactionHash: TX_HASH,
      blockNumber: 42,
      chainId: "1337",
    });
    await new Promise((resolve) => setImmediate(resolve));
    const message = sent[0]!.message as { type: string; id: string };
    assert.equal(message.type, "lineth-deployment-record");
    process.emit("message", { type: "lineth-deployment-record-ack", id: message.id, error: "boom" }, undefined);
    await assert.rejects(pending, /boom/);
  }));

test("awaitParentCheckpoint rejects when the parent disconnects before acknowledging", () =>
  withMockedIpcParent(async () => {
    const pending = awaitParentCheckpoint({
      contractName: "LinethRollupV8",
      address: ADDRESS,
      transactionHash: TX_HASH,
      blockNumber: 42,
      chainId: "1337",
    });
    await new Promise((resolve) => setImmediate(resolve));
    process.emit("disconnect");
    await assert.rejects(pending, /disconnected before acknowledging/);
  }));
