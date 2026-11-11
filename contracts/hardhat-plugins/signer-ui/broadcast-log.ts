import { appendFileSync, mkdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { type Address, type Hash, getAddress, isAddress, isHash } from "viem";

/**
 * Append-only record of every transaction the signer-UI bridge handed to the browser wallet and saw on chain.
 * `deploy:recover` reads it to resolve an Ignition deployment stuck on an unsent `TRANSACTION_PREPARE_SEND`.
 */
export type SignerUiBroadcastRecord = {
  sender: Address;
  nonce: number;
  hash: Hash;
  /** ISO timestamp of the broadcast confirmation. */
  at: string;
};

export function getSignerUiBroadcastLogPath(cacheDir: string, chainId: number): string {
  return join(cacheDir, "signer-ui", `broadcasts-${chainId}.jsonl`);
}

export function appendSignerUiBroadcast(logPath: string, record: SignerUiBroadcastRecord): void {
  mkdirSync(join(logPath, ".."), { recursive: true });
  appendFileSync(logPath, `${JSON.stringify(record)}\n`);
}

function parseBroadcastRecord(line: string): SignerUiBroadcastRecord | undefined {
  let parsed: unknown;
  try {
    parsed = JSON.parse(line);
  } catch {
    return undefined;
  }
  if (typeof parsed !== "object" || parsed === null) {
    return undefined;
  }
  const { sender, nonce, hash, at } = parsed as Record<string, unknown>;
  if (
    typeof sender !== "string" ||
    !isAddress(sender, { strict: false }) ||
    typeof nonce !== "number" ||
    !Number.isSafeInteger(nonce) ||
    typeof hash !== "string" ||
    !isHash(hash) ||
    typeof at !== "string"
  ) {
    return undefined;
  }
  return { sender: getAddress(sender), nonce, hash, at };
}

/** Reads the log, skipping malformed lines (e.g. a line truncated by a crash mid-write). */
export function readSignerUiBroadcasts(logPath: string): SignerUiBroadcastRecord[] {
  let content: string;
  try {
    content = readFileSync(logPath, "utf8");
  } catch (error: unknown) {
    if (error instanceof Error && "code" in error && error.code === "ENOENT") {
      return [];
    }
    throw error;
  }
  return content
    .split("\n")
    .filter((line) => line.trim() !== "")
    .map(parseBroadcastRecord)
    .filter((record): record is SignerUiBroadcastRecord => record !== undefined);
}

/** Latest broadcast for `(sender, nonce)`, or `undefined` if the bridge never broadcast that nonce. */
export function findSignerUiBroadcast(
  records: readonly SignerUiBroadcastRecord[],
  sender: Address,
  nonce: number,
): SignerUiBroadcastRecord | undefined {
  const normalizedSender = getAddress(sender);
  return records.findLast((record) => record.sender === normalizedSender && record.nonce === nonce);
}
