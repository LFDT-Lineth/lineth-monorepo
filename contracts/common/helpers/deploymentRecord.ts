export interface DeploymentRecordInput {
  contractName: string;
  address: string;
  transactionHash: string;
  blockNumber: number;
  chainId: bigint | string;
}

export interface ParsedDeploymentRecord {
  contractName: string;
  address: string;
  transactionHash: string;
  blockNumber: number;
  chainId: string;
}

interface Acknowledgement {
  type: string;
  id: string;
  error?: string;
}

const DEPLOYMENT_RECORD_PATTERN =
  /^contract=(\S+) deployed: address=(0x[0-9a-fA-F]{40}) blockNumber=(\d+) chainId=(\d+) txHash=(0x[0-9a-fA-F]{64})$/;
let recordSequence = 0;
let intentSequence = 0;

export function formatDeploymentRecord(record: DeploymentRecordInput): string {
  return (
    `contract=${record.contractName} deployed: address=${record.address} ` +
    `blockNumber=${record.blockNumber} chainId=${record.chainId.toString()} txHash=${record.transactionHash}`
  );
}

export function parseDeploymentRecord(line: string): ParsedDeploymentRecord | undefined {
  const match = line.trim().match(DEPLOYMENT_RECORD_PATTERN);
  if (!match) return undefined;

  return {
    contractName: match[1]!,
    address: match[2]!,
    transactionHash: match[5]!,
    blockNumber: Number(match[3]),
    chainId: match[4]!,
  };
}

function isAcknowledgement(message: unknown, id: string, ackType: string): message is Acknowledgement {
  if (typeof message !== "object" || message === null) return false;
  const candidate = message as Partial<Acknowledgement>;
  return candidate.type === ackType && candidate.id === id;
}

/**
 * Sends an IPC message to the connected parent process (a no-op when there is
 * none) and waits for a matching acknowledgement of `ackType`, rejecting on
 * an error payload or a parent disconnect. Shared by
 * `awaitParentDeploymentIntent`/`awaitParentCheckpoint` (which differ only in
 * the message/ack type names, payload shape, and id generation) and by
 * `bootstrap.ts`'s `awaitBootstrapRecord`.
 */
export async function sendAndAwaitAck(
  buildMessage: (id: string) => { type: string; id: string } & Record<string, unknown>,
  ackType: string,
  generateId: () => string,
  disconnectErrorMessage: string,
): Promise<void> {
  if (!process.send || !process.connected) return;

  const id = generateId();
  const message = buildMessage(id);
  await new Promise<void>((resolve, reject) => {
    const cleanup = () => {
      process.off("message", onMessage);
      process.off("disconnect", onDisconnect);
    };
    const onMessage = (received: unknown) => {
      if (!isAcknowledgement(received, id, ackType)) return;
      cleanup();
      if (received.error) reject(new Error(received.error));
      else resolve();
    };
    const onDisconnect = () => {
      cleanup();
      reject(new Error(disconnectErrorMessage));
    };

    process.on("message", onMessage);
    process.once("disconnect", onDisconnect);
    process.send?.(message, (error) => {
      if (!error) return;
      cleanup();
      reject(error);
    });
  });
}

export async function awaitParentDeploymentIntent(contractName: string): Promise<void> {
  await sendAndAwaitAck(
    (id) => ({ type: "lineth-deployment-intent", id, contractName }),
    "lineth-deployment-intent-ack",
    () => `${process.pid}-intent-${intentSequence++}`,
    "checkpoint parent disconnected before authorizing deployment broadcast",
  );
}

export async function awaitParentCheckpoint(record: ParsedDeploymentRecord): Promise<void> {
  await sendAndAwaitAck(
    (id) => ({ type: "lineth-deployment-record", id, record }),
    "lineth-deployment-record-ack",
    () => `${process.pid}-${recordSequence++}`,
    "checkpoint parent disconnected before acknowledging deployment",
  );
}
