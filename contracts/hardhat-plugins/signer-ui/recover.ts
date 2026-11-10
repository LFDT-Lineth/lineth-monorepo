import { readFileSync, readdirSync } from "node:fs";
import path from "node:path";
import { type Address, getAddress, isAddress } from "viem";

import { findSignerUiBroadcast, getSignerUiBroadcastLogPath, readSignerUiBroadcasts } from "./broadcast-log.js";

import type { HardhatRuntimeEnvironment } from "hardhat/types/hre";

type DeployRecoverTaskArguments = {
  /** Ignition deployment id; by default the one deployment of this chain that ends on an unsent transaction. */
  deploymentId: string | undefined;
};

function isFileNotFound(error: unknown): boolean {
  return error instanceof Error && "code" in error && error.code === "ENOENT";
}

type JournalRecord = Record<string, unknown> & { type: string };

function parseJournalLine(line: string, index: number, journalPath: string): JournalRecord {
  const parsed: unknown = JSON.parse(line);
  if (typeof parsed !== "object" || parsed === null || !("type" in parsed) || typeof parsed.type !== "string") {
    throw new Error(`${journalPath}: record ${index + 1} is not an Ignition journal message.`);
  }
  return parsed as JournalRecord;
}

type Journal = { records: JournalRecord[] };

/** Reads and parses a journal once (no separate existence check: CodeQL js/file-system-race); missing → `undefined`. */
function readJournal(journalPath: string): Journal | undefined {
  let content: string;
  try {
    content = readFileSync(journalPath, "utf8");
  } catch (error: unknown) {
    if (isFileNotFound(error)) {
      return undefined;
    }
    throw error;
  }
  // Ignition writes each record as "\n" + JSON, so the file starts with a blank line.
  const lines = content.split("\n").filter((line) => line.trim() !== "");
  return { records: lines.map((line, index) => parseJournalLine(line, index, journalPath)) };
}

/** Deployments of `chainId` (`chain-<id>`, or `<Module>-<id>-<hash>` for upgrade modules) ending on an unsent tx. */
function findStuckDeployments(deploymentsDir: string, chainId: number) {
  let entries: string[];
  try {
    entries = readdirSync(deploymentsDir);
  } catch (error: unknown) {
    if (isFileNotFound(error)) {
      return [];
    }
    throw error;
  }
  const upgradeId = new RegExp(`-${chainId}-[0-9a-f]{8}$`);
  return entries
    .filter((deploymentId) => deploymentId === `chain-${chainId}` || upgradeId.test(deploymentId))
    .flatMap((deploymentId) => {
      const journalPath = path.join(deploymentsDir, deploymentId, "journal.jsonl");
      let journal: Journal | undefined;
      try {
        journal = readJournal(journalPath);
      } catch (error: unknown) {
        // One unreadable journal (e.g. a record cut short by a crash) must not hide the stuck deployment.
        console.warn(
          `Skipping ${deploymentId}: ${journalPath} cannot be read (${error instanceof Error ? error.message : String(error)}). Pass --deployment-id to inspect it.`,
        );
        return [];
      }
      return journal?.records.at(-1)?.type === "TRANSACTION_PREPARE_SEND"
        ? [{ deploymentId, journalPath, journal }]
        : [];
    });
}

/** Sender of the future that prepared the transaction (its `*_EXECUTION_STATE_INITIALIZE` record carries `from`). */
function findFutureSender(records: readonly JournalRecord[], futureId: string): Address | undefined {
  const initialize = records.findLast(
    (record) => record.futureId === futureId && typeof record.from === "string" && isAddress(record.from),
  );
  return typeof initialize?.from === "string" ? getAddress(initialize.from) : undefined;
}

/**
 * `hardhat deploy:recover`: resolves an Ignition deployment whose journal ends on a `TRANSACTION_PREPARE_SEND` that
 * Ignition never saw sent (wallet rejection or Ctrl-C at the wallet prompt), which Ignition refuses to resume (HHE10410).
 *
 * - the signer-UI broadcast log has that `(sender, nonce)`: the transaction was broadcast, so it is handed to
 *   `ignition track-tx` with the logged hash;
 * - the sender's pending nonce still equals the prepared nonce: nothing was broadcast, so the future is reset with
 *   `ignition wipe` (Ignition appends a `WIPE_APPLY` record; the journal is never rewritten);
 * - otherwise the nonce was used by a transaction the bridge did not record: stop and explain.
 *
 * Re-run the same `ignition deploy` afterwards.
 */
export default async function deployRecover(
  taskArgs: DeployRecoverTaskArguments,
  hre: HardhatRuntimeEnvironment,
): Promise<void> {
  const connection = await hre.network.getOrCreate();
  const publicClient = await connection.viem.getPublicClient();
  const chainId = await publicClient.getChainId();
  const deploymentsDir = path.join(hre.config.paths.ignition, "deployments");

  let deploymentId: string;
  let journalPath: string;
  let journal: Journal;
  if (taskArgs.deploymentId !== undefined) {
    deploymentId = taskArgs.deploymentId;
    journalPath = path.join(deploymentsDir, deploymentId, "journal.jsonl");
    const read = readJournal(journalPath);
    if (read === undefined) {
      throw new Error(`No Ignition journal at ${journalPath}. Check --deployment-id and --network.`);
    }
    journal = read;
  } else {
    // Ignition's `chain-<chainId>`, or an upgrade module's `<Module>-<chainId>-<hash>` (deploy plugin).
    const stuck = findStuckDeployments(deploymentsDir, chainId);
    if (stuck.length === 0) {
      console.log(
        `Nothing to recover on chain ${chainId}: no deployment in ${deploymentsDir} ends on an unsent transaction.`,
      );
      return;
    }
    if (stuck.length > 1) {
      throw new Error(
        `Several deployments on chain ${chainId} end on an unsent transaction (${stuck.map((s) => s.deploymentId).join(", ")}). Pass --deployment-id.`,
      );
    }
    [{ deploymentId, journalPath, journal }] = stuck;
  }

  const { records } = journal;
  const last = records.at(-1);
  if (last === undefined || last.type !== "TRANSACTION_PREPARE_SEND") {
    console.log(`Nothing to recover in ${deploymentId}: the last journal record is ${last?.type ?? "missing"}.`);
    return;
  }

  const { futureId, nonce } = last;
  if (typeof futureId !== "string" || typeof nonce !== "number") {
    throw new Error(`${journalPath}: the trailing TRANSACTION_PREPARE_SEND has no futureId/nonce.`);
  }
  const sender = findFutureSender(records, futureId);
  if (sender === undefined) {
    throw new Error(`${journalPath}: cannot find the sender of ${futureId}.`);
  }

  const broadcastLogPath = getSignerUiBroadcastLogPath(hre.config.paths.cache, chainId);
  const broadcast = findSignerUiBroadcast(readSignerUiBroadcasts(broadcastLogPath), sender, nonce);
  if (broadcast !== undefined) {
    console.log(
      `${futureId}: the signer UI broadcast nonce ${nonce} of ${sender} as ${broadcast.hash} (${broadcast.at}); ` +
        "tracking it with `ignition track-tx`.",
    );
    await hre.tasks.getTask(["ignition", "track-tx"]).run({ txHash: broadcast.hash, deploymentId });
    return;
  }

  const pendingNonce = await publicClient.getTransactionCount({ address: sender, blockTag: "pending" });
  if (pendingNonce === nonce) {
    await hre.tasks.getTask(["ignition", "wipe"]).run({ deploymentId, futureId });
    console.log(
      `Recovered ${deploymentId}: reset ${futureId}, whose transaction (nonce ${nonce} of ${sender}) was never sent. ` +
        "Re-run the same ignition deploy to continue.",
    );
    return;
  }

  throw new Error(
    `Cannot recover ${deploymentId} automatically: ${futureId} prepared nonce ${nonce} for ${sender}, the signer-UI ` +
      `broadcast log (${broadcastLogPath}) has no transaction for it, and the pending nonce on chain is ${pendingNonce}. ` +
      (pendingNonce > nonce
        ? "The nonce was used by a transaction the bridge did not record (sent from another tool, or Hardhat stopped " +
          "before logging it). If that transaction is the intended one, find its hash in the wallet or explorer and run " +
          `\`hardhat ignition track-tx <hash> ${deploymentId} --network ${connection.networkName}\`; otherwise inspect ` +
          "the deployment before using `ignition deploy --reset`."
        : "The account's pending nonce is below the prepared nonce; wait for its pending transactions to be mined, " +
          "then run deploy:recover again."),
  );
}
