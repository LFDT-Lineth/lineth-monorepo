import { config } from "dotenv";
import { ethers } from "ethers";
import fs from "fs";
import path from "path";
import { get1559Fees } from "../../utils";

config();

const processedBatchIds: number[] = [];

// *********************************************************************************
// ********************************* CONFIGURATION *********************************
// *********************************************************************************

const DEFAULT_GAS_PRICE_CAP = ethers.parseUnits("5", "gwei").toString();

type Config = {
  inputFile: string;
  destinationAddress: string;
  zodiacRolesModifierAddress: string;
  providerUrl: string;
  signerPrivateKey: string;
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

function requireEnv(name: string): string {
  const envVariable = process.env[name];
  if (!envVariable) {
    throw new Error(`Missing ${name} environment variable.`);
  }
  return envVariable;
}

function getConfig(): Config {
  const inputFile = requireEnv("INPUT_FILE");
  const destinationAddress = requireEnv("DESTINATION_ADDRESS");
  const zodiacRolesModifierAddress = requireEnv("ZODIAC_ROLES_MODIFIER_ADDRESS");
  const providerUrl = requireEnv("PROVIDER_URL");
  const signerPrivateKey = requireEnv("SIGNER_PRIVATE_KEY");
  const waitTimeInSeconds = requireEnv("WAIT_TIME_IN_SECONDS");

  if (!ethers.isAddress(destinationAddress)) {
    throw new Error(`Destination address is not a valid Ethereum address.`);
  }

  if (!ethers.isAddress(zodiacRolesModifierAddress)) {
    throw new Error(`Zodiac roles modifier address is not a valid Ethereum address.`);
  }

  if (!isValidUrl(providerUrl)) {
    throw new Error(`Invalid provider URL.`);
  }

  if (!ethers.isHexString(signerPrivateKey, 32)) {
    throw new Error(`Signer private key must be hexadecimal string of length 64`);
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

export const wait = (timeout: number) => new Promise((resolve) => setTimeout(resolve, timeout));

async function estimateTransactionGas(signer: ethers.Wallet, transaction: ethers.TransactionRequest): Promise<bigint> {
  try {
    return signer.estimateGas(transaction);
  } catch (error: unknown) {
    throw new Error(`GasEstimationError: ${JSON.stringify(error)}`);
  }
}

async function executeTransaction(
  signer: ethers.Wallet,
  transaction: ethers.TransactionRequest,
  batch: Batch,
): Promise<{ transactionResponse: ethers.TransactionResponse; batch: Batch }> {
  try {
    return {
      transactionResponse: await signer.sendTransaction(transaction),
      batch,
    };
  } catch (error: unknown) {
    throw new Error(`TransactionError: ${JSON.stringify(error)}`);
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
  provider: ethers.JsonRpcProvider,
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

    const receipt = await provider.getTransactionReceipt(transactionHash);

    if (!receipt) {
      remainingPendingBatches.push({ id, recipients, amount, transactionHash });
      continue;
    }

    if (receipt.status == 0) {
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

  const provider = new ethers.JsonRpcProvider(providerUrl);
  const { chainId } = await provider.getNetwork();
  const signer = new ethers.Wallet(signerPrivateKey, provider);

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

  let nonce = await provider.getTransactionCount(signer.address);

  const pendingTransactions = [];

  console.log(`Total number of batches to process: ${filteredBatches.length}.`);

  for (const batch of filteredBatches) {
    try {
      const encodedBatchMintCall = ethers.concat([
        "0x83b74baa",
        ethers.AbiCoder.defaultAbiCoder().encode(
          ["address[]", "uint256"],
          [batch.recipients, ethers.parseUnits(batch.amount.toString())],
        ),
      ]);

      const encodedExecuteTransactionWithRole = ethers.concat([
        "0x6928e74b",
        ethers.AbiCoder.defaultAbiCoder().encode(
          ["address", "uint256", "bytes", "uint8", "uint16", "bool"],
          [destinationAddress, 0, encodedBatchMintCall, 0, 1, true],
        ),
      ]);

      let fees = await get1559Fees(provider);

      while (fees.maxFeePerGas && fees.maxFeePerGas > BigInt(gasPriceCap)) {
        console.warn(`Max fee per gas (${fees.maxFeePerGas.toString()}) exceeds gas price cap (${gasPriceCap})`);

        const currentBlockNumber = await provider.getBlockNumber();
        while ((await provider.getBlockNumber()) === currentBlockNumber) {
          console.warn(`Waiting for next block: ${currentBlockNumber}`);
          await wait(4_000);
        }

        fees = await get1559Fees(provider);
      }

      const transactionRequest: ethers.TransactionRequest = {
        to: zodiacRolesModifierAddress,
        value: 0,
        type: 2,
        data: encodedExecuteTransactionWithRole,
        chainId,
        maxFeePerGas: fees.maxFeePerGas!,
        maxPriorityFeePerGas: fees.maxPriorityFeePerGas!,
        nonce,
      };

      const transactionGasLimit = await estimateTransactionGas(signer, transactionRequest);

      const transaction: ethers.TransactionRequest = {
        ...transactionRequest,
        gasLimit: transactionGasLimit,
      };

      const transactionInfo = await executeTransaction(signer, transaction, batch);
      pendingTransactions.push(transactionInfo);

      trackingData.set(batch.id, {
        recipients: batch.recipients,
        tokenAmount: batch.amount,
        status: BatchStatuses.Pending,
        ...(transactionInfo.transactionResponse.hash
          ? { transactionHash: transactionInfo.transactionResponse.hash }
          : {}),
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
    await wait(waitTimeInSeconds * 1000);
  }

  if (pendingTransactions.length !== 0) {
    console.log(`Waiting for all receipts...`);
  }

  const transactionsInfos = await Promise.all(
    pendingTransactions.map(async ({ transactionResponse, batch }) => {
      return {
        transactionReceipt: await transactionResponse.wait(),
        batch,
      };
    }),
  );

  for (const { batch, transactionReceipt } of transactionsInfos) {
    if (transactionReceipt && transactionReceipt.status == 0) {
      trackingData.set(batch.id, {
        recipients: batch.recipients,
        tokenAmount: batch.amount,
        status: BatchStatuses.Failed,
        transactionHash: transactionReceipt.hash,
      });

      console.log(`Transaction reverted. Hash: ${transactionReceipt.hash}, batchId: ${batch.id}`);
      updateTrackingFile(trackingData);
      continue;
    }

    trackingData.set(batch.id, {
      recipients: batch.recipients,
      tokenAmount: batch.amount,
      status: BatchStatuses.Success,
      ...(transactionReceipt?.hash ? { transactionHash: transactionReceipt?.hash } : {}),
    });

    updateTrackingFile(trackingData);
    console.log(`Transaction succeed. Hash: ${transactionReceipt?.hash}, batchId: ${batch.id}`);
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
