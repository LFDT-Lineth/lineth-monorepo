import { Command, Flags } from "@oclif/core";
import { readFileSync } from "fs";
import { type Hex } from "viem";
import { sendRawTransaction } from "viem/actions";

import {
  createSynctxClient,
  getPendingTransactions,
  getTransactionPool,
  getTransactionsFromPool,
  hasPendingTransactions,
  isLocalPort,
  isValidNodeTarget,
  parseTransactionsFileContent,
  serializeVerifiedTxpoolTransaction,
  type Transaction,
  type TransactionPool,
  getClientType,
} from "../utils/synctx/index.js";

export default class Synctx extends Command {
  static examples = [
    "<%= config.bin %> <%= command.id %> --source=8500 --target=8501 --local",
    "<%= config.bin %> <%= command.id %> --source=http://geth-archive-1:8545 --target=http://geth-validator-1:8545 --concurrency=10",
  ];

  static flags = {
    source: Flags.string({
      char: "s",
      description: "source node to sync from, mutually exclusive with file flag",
      helpGroup: "Node",
      multiple: false,
      required: false,
      default: "",
      env: "SYNCTX_SOURCE",
    }),
    target: Flags.string({
      char: "t",
      description: "target node to sync to",
      helpGroup: "Node",
      multiple: false,
      required: true,
      env: "SYNCTX_TARGET",
    }),
    "dry-run": Flags.boolean({
      description: "enable dry run",
      helpGroup: "Config",
      required: false,
      default: false,
      env: "SYNCTX_DRYRUN",
    }),
    local: Flags.boolean({
      description: "enable local run, provide only forwarded ports",
      helpGroup: "Config",
      required: false,
      default: false,
      env: "SYNCTX_LOCAL",
    }),
    concurrency: Flags.integer({
      char: "c",
      description: "number of concurrent batch requests",
      helpGroup: "Config",
      multiple: false,
      required: false,
      default: 10,
      env: "SYNCTX_CONCURRENCY",
    }),
    file: Flags.string({
      char: "f",
      description: "local txs file to read from, mutually exclusive with source flag",
      helpGroup: "Config",
      multiple: false,
      required: false,
      default: "",
      env: "SYNCTX_FILE",
    }),
  };

  public async run(): Promise<void> {
    const { flags } = await this.parse(Synctx);

    let sourceNode: string = flags.source ?? "";
    let targetNode: string = flags.target;
    const filePath: string = flags.file ?? "";
    let pendingTransactionsToSync: Transaction[] = [];
    const concurrentCount = flags.concurrency as number;

    if (concurrentCount <= 0) {
      this.error("Invalid concurrency supplied; must be greater than 0");
    }

    if ((filePath === "" && sourceNode === "") || (filePath !== "" && sourceNode !== "")) {
      this.error(
        "Invalid flag values are supplied; source and file are mutually exclusive, and at least one needs to be specified",
      );
    }

    if (flags.local) {
      sourceNode = sourceNode && isLocalPort(sourceNode) ? `http://localhost:${sourceNode}` : sourceNode;
      targetNode = isLocalPort(targetNode) ? `http://localhost:${targetNode}` : targetNode;
    }

    if (!isValidNodeTarget(sourceNode, targetNode)) {
      this.error("Invalid nodes supplied to source and/or target; must be valid URLs");
    }

    const sourceClient = sourceNode ? createSynctxClient(sourceNode) : undefined;
    const targetClient = createSynctxClient(targetNode);

    const sourceClientType = sourceClient ? await getClientType(sourceClient) : undefined;
    const targetClientType = await getClientType(targetClient);

    if (sourceNode) {
      this.log(`Source ${sourceClientType} node: ${sourceNode}`);
    } else {
      this.log(`Skip checking source node type as txs file is supplied`);
    }
    this.log(`Target ${targetClientType} node: ${targetNode}`);

    let sourceTransactionPool: TransactionPool = { pending: {}, queued: {} };
    let targetTransactionPool: TransactionPool = { pending: {}, queued: {} };
    try {
      if (sourceClient && sourceClientType) {
        this.log(`Fetching pending txs from txpool`);
        sourceTransactionPool = await getTransactionPool(sourceClient, sourceClientType);
        targetTransactionPool = await getTransactionPool(targetClient, targetClientType);
      } else {
        this.log(`Skip fetching txs from source node as txs file is supplied`);
      }
    } catch {
      this.error(`Failed to get transaction pool from source and target nodes.`);
    }

    let sourcePendingTransactions: Transaction[] = [];
    let targetPendingTransactions: Transaction[] = [];
    if (sourceNode) {
      if (!sourceClientType) {
        this.error("Failed to determine source node client type");
      }

      if (!hasPendingTransactions(sourceClientType, sourceTransactionPool)) {
        this.log("No pending transactions found on source node");
        return;
      }

      sourcePendingTransactions = getTransactionsFromPool(sourceClientType, sourceTransactionPool);
      targetPendingTransactions = getTransactionsFromPool(targetClientType, targetTransactionPool);
    }

    if (sourceNode) {
      this.log(`Source pending transactions: ${sourcePendingTransactions.length}`);
      this.log(`Target pending transactions: ${targetPendingTransactions.length}`);
    }

    try {
      pendingTransactionsToSync = sourceNode
        ? getPendingTransactions(sourcePendingTransactions, targetPendingTransactions)
        : parseTransactionsFileContent(readFileSync(filePath, "utf-8"), filePath);
    } catch (error) {
      this.error((error as Error).message);
    }

    if (pendingTransactionsToSync.length === 0) {
      if (sourceNode) {
        this.log(`Delta between source and target pending transactions is 0.`);
      } else {
        this.log(`No txs found in file ${filePath}`);
      }
      return;
    }

    this.log(`Pending transactions to process: ${pendingTransactionsToSync.length}`);

    // Track errors during serialization
    let errorSerialization = 0;
    const transactions: Hex[] = [];

    for (const tx of pendingTransactionsToSync) {
      try {
        transactions.push(serializeVerifiedTxpoolTransaction(tx));
      } catch (error) {
        errorSerialization++;
        this.warn(`Error serializing transaction ${tx.hash}: ${(error as Error).message}`);
      }
    }
    const totalBatchesToProcess = Math.ceil(transactions.length / concurrentCount);

    this.log(`Total serialization errors: ${errorSerialization}`);

    let totalSuccess = 0;
    let success = 0;
    let errors = 0;
    let totalErrors = 0;

    if (flags["dry-run"]) {
      this.log(`Total batches to process: ${totalBatchesToProcess}`);
      return;
    }

    for (let i = 0; i < totalBatchesToProcess; i++) {
      const batchIndex = concurrentCount * i;
      const transactionBatch = transactions.slice(batchIndex, batchIndex + concurrentCount);

      if (transactionBatch.length === 0) {
        break;
      }

      this.log(`Processing batch: ${i + 1} of ${totalBatchesToProcess}, size ${transactionBatch.length}`);

      const transactionPromises = transactionBatch.map((rawTransaction) => {
        return sendRawTransaction(targetClient, { serializedTransaction: rawTransaction });
      });

      const results = await Promise.allSettled(transactionPromises);
      success = results.filter((result) => result.status === "fulfilled").length;

      const resultErrors = results.filter((result): result is PromiseRejectedResult => result.status === "rejected");
      errors = resultErrors.length;

      resultErrors.forEach((result) => {
        this.log(`Error broadcasting transaction: ${(result.reason as Error).message}`);
      });

      totalSuccess += success;
      totalErrors += errors;

      this.log(
        `
        Total count: ${transactionBatch.length + batchIndex} - Success: ${success} - Errors: ${errors} - Total Success: ${totalSuccess} - Total Errors: ${totalErrors}
      `.trim(),
      );

      success = 0;
      errors = 0;
    }
  }
}
