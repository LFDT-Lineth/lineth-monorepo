import { copyFileSync, existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import path from "node:path";
import { type Address, getAddress, isAddress } from "viem";

import { findSignerUiBroadcast, getSignerUiBroadcastLogPath, readSignerUiBroadcasts } from "./broadcast-log.js";

import type { HardhatRuntimeEnvironment } from "hardhat/types/hre";

type DeployRecoverTaskArguments = {
  /** Ignition deployment id; defaults to Ignition's `chain-<chainId>`. */
  deploymentId: string | undefined;
};

type JournalRecord = Record<string, unknown> & { type: string };

function parseJournalLine(line: string, index: number, journalPath: string): JournalRecord {
  const parsed: unknown = JSON.parse(line);
  if (typeof parsed !== "object" || parsed === null || !("type" in parsed) || typeof parsed.type !== "string") {
    throw new Error(`${journalPath}: record ${index + 1} is not an Ignition journal message.`);
  }
  return parsed as JournalRecord;
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
 * - the sender's pending nonce still equals the prepared nonce: nothing was broadcast, so the record is dropped
 *   (a backup of the journal is kept under `<cache>/signer-ui/journal-backups/`);
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
  const deploymentId = taskArgs.deploymentId ?? `chain-${chainId}`;
  const journalPath = path.join(hre.config.paths.ignition, "deployments", deploymentId, "journal.jsonl");

  if (!existsSync(journalPath)) {
    throw new Error(`No Ignition journal at ${journalPath}. Check --deployment-id and --network.`);
  }

  // Ignition writes each record as "\n" + JSON, so the file starts with a blank line.
  const lines = readFileSync(journalPath, "utf8")
    .split("\n")
    .filter((line) => line.trim() !== "");
  const records = lines.map((line, index) => parseJournalLine(line, index, journalPath));
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
    const backupDir = path.join(hre.config.paths.cache, "signer-ui", "journal-backups");
    mkdirSync(backupDir, { recursive: true });
    const backupPath = path.join(backupDir, `${deploymentId}-${new Date().toISOString().replace(/[:.]/g, "-")}.jsonl`);
    copyFileSync(journalPath, backupPath);
    writeFileSync(
      journalPath,
      lines
        .slice(0, -1)
        .map((line) => `\n${line}`)
        .join(""),
    );
    console.log(
      `Recovered ${deploymentId}: dropped the unsent ${futureId} transaction (nonce ${nonce} of ${sender}). ` +
        `Journal backup: ${backupPath}. Re-run the same ignition deploy to continue.`,
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
