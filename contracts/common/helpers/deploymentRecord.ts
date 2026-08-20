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

interface DeploymentRecordAcknowledgement {
  type: "lineth-deployment-record-ack";
  id: string;
  error?: string;
}

const DEPLOYMENT_RECORD_PATTERN =
  /^contract=(\S+) deployed: address=(0x[0-9a-fA-F]{40}) blockNumber=(\d+) chainId=(\d+) txHash=(0x[0-9a-fA-F]{64})$/;
let recordSequence = 0;

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

function isAcknowledgement(message: unknown, id: string): message is DeploymentRecordAcknowledgement {
  if (typeof message !== "object" || message === null) return false;
  const candidate = message as Partial<DeploymentRecordAcknowledgement>;
  return candidate.type === "lineth-deployment-record-ack" && candidate.id === id;
}

export async function awaitParentCheckpoint(record: ParsedDeploymentRecord): Promise<void> {
  if (!process.send || !process.connected) return;

  const id = `${process.pid}-${recordSequence++}`;
  await new Promise<void>((resolve, reject) => {
    const cleanup = () => {
      process.off("message", onMessage);
      process.off("disconnect", onDisconnect);
    };
    const onMessage = (message: unknown) => {
      if (!isAcknowledgement(message, id)) return;
      cleanup();
      if (message.error) reject(new Error(message.error));
      else resolve();
    };
    const onDisconnect = () => {
      cleanup();
      reject(new Error("checkpoint parent disconnected before acknowledging deployment"));
    };

    process.on("message", onMessage);
    process.once("disconnect", onDisconnect);
    process.send?.({ type: "lineth-deployment-record", id, record }, (error) => {
      if (!error) return;
      cleanup();
      reject(error);
    });
  });
}
