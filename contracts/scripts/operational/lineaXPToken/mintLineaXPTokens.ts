import { config } from "dotenv";
import fs from "fs";
import { setTimeout } from "node:timers/promises";
import path from "path";
import {
  type Address,
  type Hash,
  type Hex,
  type PrivateKeyAccount,
  type PublicClient,
  type TransactionReceipt,
  type WalletClient,
  TransactionReceiptNotFoundError,
  concat,
  createPublicClient,
  createWalletClient,
  encodeAbiParameters,
  http,
  isHex,
  parseAbiParameters,
  parseEther,
  parseGwei,
} from "viem";
import { privateKeyToAccount } from "viem/accounts";

import { parseChecksummedAddress } from "../../../common/helpers/addressRegistry.js";
import { getRequiredEnvVar } from "../../../common/helpers/environment.js";
import { parsePrivateKey } from "../../cli.js";

config();

const processedBatchIds: number[] = [];

// *********************************************************************************
// ********************************* CONFIGURATION *********************************
// *********************************************************************************

const DEFAULT_GAS_PRICE_CAP = parseGwei("5").toString();

type Config = {
  inputFile: string;
  destinationAddress: Address;
  zodiacRolesModifierAddress: Address;
  providerUrl: string;
  signerPrivateKey: Hex;
  waitTimeInSeconds: number;
  gasPriceCap: string;
};

type Batch = {
  id: number;
  recipients: string[];
  amount: number;
};

enum BatchStatuses {
  Failed = "Failed",
  Success = "Success",
  Pending = "Pending",
}

type TrackingData = {
  recipients: string[];
  tokenAmount: number;
  status: BatchStatuses;
  transactionHash?: string;
  error?: unknown;
};

function isValidUrl(urlString: string): boolean {
  try {
    return Boolean(new URL(urlString));
  } catch {
    return false;
  }
}

function getConfig(): Config {
  const inputFile = getRequiredEnvVar("INPUT_FILE");
  const destinationAddress = parseChecksummedAddress(getRequiredEnvVar("DESTINATION_ADDRESS"), "DESTINATION_ADDRESS");
  const zodiacRolesModifierAddress = parseChecksummedAddress(
    getRequiredEnvVar("ZODIAC_ROLES_MODIFIER_ADDRESS"),
    "ZODIAC_ROLES_MODIFIER_ADDRESS",
  );
  const providerUrl = getRequiredEnvVar("PROVIDER_URL");
  const signerPrivateKey = parsePrivateKey(getRequiredEnvVar("SIGNER_PRIVATE_KEY"), "SIGNER_PRIVATE_KEY");
  const waitTimeInSeconds = getRequiredEnvVar("WAIT_TIME_IN_SECONDS");

  if (!isValidUrl(providerUrl)) {
    throw new Error(`Invalid provider URL.`);
  }

  if (path.extname(inputFile) !== ".json") {
    throw new Error(`File ${inputFile} is not a JSON file.`);
  }

  if (!fs.existsSync(inputFile)) {
    throw new Error(`File ${inputFile} does not exist.`);
  }

  if (waitTimeInSeconds == "0") {
    throw new Error(`WAIT_TIME_IN_SECONDS cannot be zero`);
  }

  return {
    inputFile,
    destinationAddress,
    zodiacRolesModifierAddress,
    providerUrl,
    signerPrivateKey,
    waitTimeInSeconds: parseInt(waitTimeInSeconds),
    gasPriceCap: process.env.GAS_PRICE_CAP ?? DEFAULT_GAS_PRICE_CAP,
  };
}

// *********************************************************************************
// ********************************* UTILS FUNCTIONS *******************************
// *********************************************************************************

type MintTransactionRequest = {
  to: Address;
  value: bigint;
  data: Hex;
  maxFeePerGas: bigint;
  maxPriorityFeePerGas: bigint;
  nonce: number;
  gas?: bigint;
};

async function estimateTransactionGas(
  provider: PublicClient,
  signer: PrivateKeyAccount,
  transaction: MintTransactionRequest,
): Promise<bigint> {
  try {
    return await provider.estimateGas({ account: signer, ...transaction });
  } catch (error: unknown) {
    throw new Error(`GasEstimationError: ${JSON.stringify(error)}`);
  }
}

async function executeTransaction(
  walletClient: WalletClient,
  signer: PrivateKeyAccount,
  transaction: MintTransactionRequest,
  batch: Batch,
): Promise<{ transactionHash: Hash; batch: Batch }> {
  try {
    return {
      transactionHash: await walletClient.sendTransaction({
        account: signer,
        chain: null,
        type: "eip1559",
        ...transaction,
      }),
      batch,
    };
  } catch (error: unknown) {
    throw new Error(`TransactionError: ${JSON.stringify(error)}`);
  }
}

async function getTransactionReceiptOrNull(provider: PublicClient, hash: Hash): Promise<TransactionReceipt | null> {
  try {
    return await provider.getTransactionReceipt({ hash });
  } catch (error: unknown) {
    if (error instanceof TransactionReceiptNotFoundError) {
      return null;
    }
    throw error;
  }
}

function createTrackingFile(path: string): Map<number, TrackingData> {
  if (fs.existsSync(path)) {
    const mapAsArray = fs.readFileSync(path, "utf-8");
    return new Map(JSON.parse(mapAsArray));
  }

  fs.writeFileSync(path, JSON.stringify(Array.from(new Map<number, TrackingData>().entries())));
  return new Map<number, TrackingData>();
}

function updateTrackingFile(trackingData: Map<number, TrackingData>) {
  fs.writeFileSync("tracking.json", JSON.stringify(Array.from(trackingData.entries()), null, 2));
}

async function processPendingBatches(
  provider: PublicClient,
  batches: Batch[],
  trackingData: Map<number, TrackingData>,
): Promise<(Batch & { transactionHash?: string })[]> {
  const pendingBatches = batches
    .filter((batch) => trackingData.get(batch.id)?.status === BatchStatuses.Pending)
    .map((batch) => ({
      ...batch,
      transactionHash: trackingData.get(batch.id)?.transactionHash,
    }));

  const remainingPendingBatches: (Batch & { transactionHash?: string })[] = [];

  for (const { transactionHash, id, recipients, amount } of pendingBatches) {
    if (!transactionHash) {
      remainingPendingBatches.push({ id, recipients, amount });
      continue;
    }

    if (!isHex(transactionHash)) {
      throw new Error(`Invalid transaction hash ${transactionHash} in tracking data for batch ${id}`);
    }
    const receipt = await getTransactionReceiptOrNull(provider, transactionHash);

    if (!receipt) {
      remainingPendingBatches.push({ id, recipients, amount, transactionHash });
      continue;
    }

    if (receipt.status === "reverted") {
      // track failing batches
      trackingData.set(id, {
        recipients,
        tokenAmount: amount,
        status: BatchStatuses.Failed,
        transactionHash,
      });

      console.log(`Transaction reverted. Hash: ${transactionHash}, batchId: ${id}`);
      updateTrackingFile(trackingData);

      // continue the batch loop
      continue;
    }
    // track succeded batches
    trackingData.set(id, {
      recipients,
      tokenAmount: amount,
      status: BatchStatuses.Success,
      transactionHash: transactionHash,
    });

    updateTrackingFile(trackingData);
    console.log(`Transaction succeed. Hash: ${transactionHash}, batchId: ${id}`);
  }

  return remainingPendingBatches;
}

// *********************************************************************************
// ********************************* MAIN FUNCTION *********************************
// *********************************************************************************

async function main() {
  const {
    inputFile,
    destinationAddress,
    zodiacRolesModifierAddress,
    providerUrl,
    signerPrivateKey,
    waitTimeInSeconds,
    gasPriceCap,
  } = getConfig();

  const provider = createPublicClient({ transport: http(providerUrl) });
  const signer = privateKeyToAccount(signerPrivateKey);
  const walletClient = createWalletClient({ account: signer, transport: http(providerUrl) });

  const trackingData = createTrackingFile("tracking.json");

  const readFile = fs.readFileSync(inputFile, "utf-8");
  const batches: Batch[] = JSON.parse(readFile);

  const filteredBatches = batches.filter(
    (batch) => trackingData.get(batch.id)?.status === BatchStatuses.Failed || !trackingData.has(batch.id),
  );

  console.log("Processing pending batches...");
  const remainingPendingBatches = await processPendingBatches(provider, batches, trackingData);

  if (remainingPendingBatches.length !== 0) {
    console.warn(`The following batches are still pending: ${JSON.stringify(remainingPendingBatches, null, 2)}`);
    return;
  }

  let nonce = await provider.getTransactionCount({ address: signer.address });

  const pendingTransactions = [];

  console.log(`Total number of batches to process: ${filteredBatches.length}.`);

  for (const batch of filteredBatches) {
    try {
      const recipients = batch.recipients.map((recipient) =>
        parseChecksummedAddress(recipient, `batch ${batch.id} recipient`),
      );

      const encodedBatchMintCall = concat([
        "0x83b74baa",
        encodeAbiParameters(parseAbiParameters("address[], uint256"), [
          recipients,
          parseEther(batch.amount.toString()),
        ]),
      ]);

      const encodedExecuteTransactionWithRole = concat([
        "0x6928e74b",
        encodeAbiParameters(parseAbiParameters("address, uint256, bytes, uint8, uint16, bool"), [
          destinationAddress,
          0n,
          encodedBatchMintCall,
          0,
          1,
          true,
        ]),
      ]);

      let fees = await provider.estimateFeesPerGas();

      while (fees.maxFeePerGas > BigInt(gasPriceCap)) {
        console.warn(`Max fee per gas (${fees.maxFeePerGas.toString()}) exceeds gas price cap (${gasPriceCap})`);

        const currentBlockNumber = await provider.getBlockNumber();
        while ((await provider.getBlockNumber()) === currentBlockNumber) {
          console.warn(`Waiting for next block: ${currentBlockNumber}`);
          await setTimeout(4_000);
        }

        fees = await provider.estimateFeesPerGas();
      }

      const transactionRequest: MintTransactionRequest = {
        to: zodiacRolesModifierAddress,
        value: 0n,
        data: encodedExecuteTransactionWithRole,
        maxFeePerGas: fees.maxFeePerGas,
        maxPriorityFeePerGas: fees.maxPriorityFeePerGas,
        nonce,
      };

      const transactionGasLimit = await estimateTransactionGas(provider, signer, transactionRequest);

      const transaction: MintTransactionRequest = {
        ...transactionRequest,
        gas: transactionGasLimit,
      };

      const transactionInfo = await executeTransaction(walletClient, signer, transaction, batch);
      pendingTransactions.push(transactionInfo);

      trackingData.set(batch.id, {
        recipients: batch.recipients,
        tokenAmount: batch.amount,
        status: BatchStatuses.Pending,
        ...(transactionInfo.transactionHash ? { transactionHash: transactionInfo.transactionHash } : {}),
      });

      updateTrackingFile(trackingData);

      processedBatchIds.push(batch.id);

      console.log(`Batch with ID=${batch.id} sent.\n`);
      nonce = nonce + 1;
    } catch (error) {
      trackingData.set(batch.id, {
        recipients: batch.recipients,
        tokenAmount: batch.amount,
        status: BatchStatuses.Failed,
        error,
      });
      updateTrackingFile(trackingData);
      console.error(`Batch with ID=${batch.id} failed.\n Stopping script execution.`);
      return;
    }

    console.log(`Pause the execution for ${waitTimeInSeconds} seconds...`);
    await setTimeout(waitTimeInSeconds * 1000);
  }

  if (pendingTransactions.length !== 0) {
    console.log(`Waiting for all receipts...`);
  }

  const transactionsInfos = await Promise.all(
    pendingTransactions.map(async ({ transactionHash, batch }) => {
      return {
        // No timeout (viem defaults to 180 s); a reverted batch is recorded as Failed below rather than thrown.
        transactionReceipt: await provider.waitForTransactionReceipt({ hash: transactionHash, timeout: 0 }),
        batch,
      };
    }),
  );

  for (const { batch, transactionReceipt } of transactionsInfos) {
    if (transactionReceipt.status === "reverted") {
      trackingData.set(batch.id, {
        recipients: batch.recipients,
        tokenAmount: batch.amount,
        status: BatchStatuses.Failed,
        transactionHash: transactionReceipt.transactionHash,
      });

      console.log(`Transaction reverted. Hash: ${transactionReceipt.transactionHash}, batchId: ${batch.id}`);
      updateTrackingFile(trackingData);
      continue;
    }

    trackingData.set(batch.id, {
      recipients: batch.recipients,
      tokenAmount: batch.amount,
      status: BatchStatuses.Success,
      transactionHash: transactionReceipt.transactionHash,
    });

    updateTrackingFile(trackingData);
    console.log(`Transaction succeed. Hash: ${transactionReceipt.transactionHash}, batchId: ${batch.id}`);
  }
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });

process.on("SIGINT", () => {
  console.log(`Processed batches: ${JSON.stringify(processedBatchIds, null, 2)}`);
  console.log("\nGracefully shutting down from SIGINT (Ctrl-C)");
  process.exit(1);
});
