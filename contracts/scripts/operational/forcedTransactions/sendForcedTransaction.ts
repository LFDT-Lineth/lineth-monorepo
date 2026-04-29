/**
 * sendForcedTransaction.ts
 *
 * Submits a forced transaction to the ForcedTransactionGateway contract on L1.
 * The script automatically resolves the current LastFinalizedState from on-chain
 * data (falling back to genesis defaults on fresh deployments) and supports two
 * modes for providing the L2 EIP-1559 transaction.
 *
 * ──────────────────────────────────────────────────────────────────────────────
 * REQUIRED ENVIRONMENT VARIABLES
 * ──────────────────────────────────────────────────────────────────────────────
 *   L1_RPC_URL                          L1 JSON-RPC endpoint
 *   PRIVATE_KEY                         Private key for the L1 sender wallet
 *   FORCED_TRANSACTION_GATEWAY_ADDRESS  Deployed ForcedTransactionGateway address
 *   LINEA_ROLLUP_ADDRESS               Deployed LineaRollup proxy address
 *
 * ──────────────────────────────────────────────────────────────────────────────
 * OPTIONAL ENVIRONMENT VARIABLES
 * ──────────────────────────────────────────────────────────────────────────────
 *   L2_PRIVATE_KEY                      Separate key for signing the L2 tx
 *                                       (defaults to PRIVATE_KEY)
 *
 * ──────────────────────────────────────────────────────────────────────────────
 * MODE A — FROM FILE  (set TX_DATA_FILE)
 * ──────────────────────────────────────────────────────────────────────────────
 *   TX_DATA_FILE          Path to a JSON file with a signed EIP-1559 transaction.
 *                         Expected fields: nonce, maxPriorityFeePerGas, maxFeePerGas,
 *                         gasLimit (or gas), to, value, input (or data),
 *                         accessList, yParity (or v), r, s
 *
 * ──────────────────────────────────────────────────────────────────────────────
 * MODE B — FROM PARAMS  (omit TX_DATA_FILE, set the vars below)
 * ──────────────────────────────────────────────────────────────────────────────
 *   TX_TO                               Destination address on L2
 *   TX_GAS_LIMIT                        Gas limit (min 21000, max per gateway config)
 *   TX_MAX_FEE_PER_GAS                  Max fee per gas (must be > 0)
 *   TX_MAX_PRIORITY_FEE_PER_GAS         Max priority fee per gas (must be <= max fee)
 *   TX_NONCE                            L2 nonce for the signer
 *   TX_VALUE                            Wei to transfer (optional, default "0")
 *   TX_DATA                             Calldata hex (optional, default "0x")
 *
 * ──────────────────────────────────────────────────────────────────────────────
 * USAGE EXAMPLES
 * ──────────────────────────────────────────────────────────────────────────────
 *
 *   # Mode B — sign a simple ETH transfer on the fly (local dev)
 *   L1_RPC_URL=http://localhost:8445 \
 *   PRIVATE_KEY=0x7c852118294e51e653712a81e05800f419141751be58f605c371e15141b007a6 \
 *   FORCED_TRANSACTION_GATEWAY_ADDRESS=0x0165878A594ca255338adfa4d48449f69242Eb8F \
 *   LINEA_ROLLUP_ADDRESS=0xDc64a140Aa3E981100a9becA4E685f962f0cF6C9 \
 *   TX_TO=0x70997970C51812dc3A010C7d01b50e0d17dc79C8 \
 *   TX_GAS_LIMIT=21000 \
 *   TX_MAX_FEE_PER_GAS=1000000000 \
 *   TX_MAX_PRIORITY_FEE_PER_GAS=100000000 \
 *   TX_NONCE=0 \
 *   npx ts-node scripts/operational/forcedTransactions/sendForcedTransaction.ts
 *
 *   # Mode A — submit a pre-signed transaction from a JSON file
 *   L1_RPC_URL=http://localhost:8445 \
 *   PRIVATE_KEY=0x7c852118294e51e653712a81e05800f419141751be58f605c371e15141b007a6 \
 *   FORCED_TRANSACTION_GATEWAY_ADDRESS=0x0165878A594ca255338adfa4d48449f69242Eb8F \
 *   LINEA_ROLLUP_ADDRESS=0xDc64a140Aa3E981100a9becA4E685f962f0cF6C9 \
 *   TX_DATA_FILE=./my-signed-tx.json \
 *   npx ts-node scripts/operational/forcedTransactions/sendForcedTransaction.ts
 */

import * as dotenv from "dotenv";
import { ethers } from "ethers";
import fs from "fs";
import path from "path";

import { getRequiredEnvVar } from "../../../common/helpers/environment";
import { abi as LineaRollupV8Abi } from "../../../local-deployments-artifacts/dynamic-artifacts/LineaRollupV8.json";
import { abi as ForcedTransactionGatewayAbi } from "../../../local-deployments-artifacts/static-artifacts/ForcedTransactionGateway.json";

dotenv.config();

// Default finalized state used on fresh deployments (genesis).
// This timestamp is specific to the local dev stack. Real-world deployments
// will have a different genesis timestamp — always verify against the chain's
// genesis configuration file before using this value.
const DEFAULT_LAST_FINALIZED_TIMESTAMP = 1770394477n;
const DEFAULT_FINALIZED_STATE = {
  timestamp: DEFAULT_LAST_FINALIZED_TIMESTAMP,
  messageNumber: 0n,
  messageRollingHash: ethers.ZeroHash,
  forcedTransactionNumber: 0n,
  forcedTransactionRollingHash: ethers.ZeroHash,
};

type LastFinalizedState = {
  timestamp: bigint;
  messageNumber: bigint;
  messageRollingHash: string;
  forcedTransactionNumber: bigint;
  forcedTransactionRollingHash: string;
};

type Eip1559TransactionStruct = {
  nonce: bigint;
  maxPriorityFeePerGas: bigint;
  maxFeePerGas: bigint;
  gasLimit: bigint;
  to: string;
  value: bigint;
  input: string;
  accessList: { contractAddress: string; storageKeys: string[] }[];
  yParity: number;
  r: bigint;
  s: bigint;
};

/**
 * Computes the finalized state hash identical to
 * FinalizedStateHashing._computeLastFinalizedState in Solidity.
 */
function computeFinalizedStateHash(state: LastFinalizedState): string {
  return ethers.keccak256(
    ethers.AbiCoder.defaultAbiCoder().encode(
      ["uint256", "bytes32", "uint256", "bytes32", "uint256"],
      [
        state.messageNumber,
        state.messageRollingHash,
        state.forcedTransactionNumber,
        state.forcedTransactionRollingHash,
        state.timestamp,
      ],
    ),
  );
}

/**
 * Resolves the current LastFinalizedState from on-chain data.
 *
 * 1. Checks if the on-chain hash matches the default (genesis) state.
 * 2. If not, reconstructs the state from the FinalizedStateUpdated event
 *    and the rollingHashes / forcedTransactionRollingHashes mappings.
 */
async function resolveLastFinalizedState(
  rollup: ethers.Contract,
  provider: ethers.JsonRpcProvider,
): Promise<LastFinalizedState> {
  const onChainStateHash: string = await rollup.currentFinalizedState();
  console.log(`On-chain currentFinalizedState: ${onChainStateHash}`);

  const defaultHash = computeFinalizedStateHash(DEFAULT_FINALIZED_STATE);
  if (onChainStateHash === defaultHash) {
    console.log("Finalized state matches default genesis state.");
    return { ...DEFAULT_FINALIZED_STATE };
  }

  console.log("Finalized state differs from default — reconstructing from on-chain events...");

  const currentL2BlockNumber: bigint = await rollup.currentL2BlockNumber();
  console.log(`currentL2BlockNumber: ${currentL2BlockNumber}`);

  const eventSignature = "FinalizedStateUpdated(uint256,uint256,uint256,uint256)";
  const eventTopic = ethers.id(eventSignature);
  const blockNumberTopic = ethers.AbiCoder.defaultAbiCoder().encode(["uint256"], [currentL2BlockNumber]);

  const logs = await provider.getLogs({
    address: await rollup.getAddress(),
    topics: [eventTopic, blockNumberTopic],
    fromBlock: 0,
    toBlock: "latest",
  });

  if (logs.length === 0) {
    throw new Error(
      `No FinalizedStateUpdated event found for blockNumber=${currentL2BlockNumber}. ` +
        `Cannot reconstruct the last finalized state.`,
    );
  }

  const latestLog = logs[logs.length - 1];
  const iface = new ethers.Interface(LineaRollupV8Abi);
  const parsed = iface.parseLog({ topics: latestLog.topics as string[], data: latestLog.data });

  if (!parsed) {
    throw new Error("Failed to parse FinalizedStateUpdated event.");
  }

  const timestamp: bigint = parsed.args.timestamp;
  const messageNumber: bigint = parsed.args.messageNumber;
  const forcedTransactionNumber: bigint = parsed.args.forcedTransactionNumber;

  console.log(
    `Event data — timestamp=${timestamp}, messageNumber=${messageNumber}, forcedTransactionNumber=${forcedTransactionNumber}`,
  );

  const [messageRollingHash, forcedTransactionRollingHash]: [string, string] = await Promise.all([
    rollup.rollingHashes(messageNumber),
    rollup.forcedTransactionRollingHashes(forcedTransactionNumber),
  ]);

  console.log(`messageRollingHash=${messageRollingHash}, forcedTransactionRollingHash=${forcedTransactionRollingHash}`);

  const reconstructed: LastFinalizedState = {
    timestamp,
    messageNumber,
    messageRollingHash,
    forcedTransactionNumber,
    forcedTransactionRollingHash,
  };

  const reconstructedHash = computeFinalizedStateHash(reconstructed);
  if (reconstructedHash !== onChainStateHash) {
    throw new Error(
      `Reconstructed state hash (${reconstructedHash}) does not match on-chain hash (${onChainStateHash}). ` +
        `State reconstruction failed.`,
    );
  }

  console.log("Reconstructed state validated successfully.");
  return reconstructed;
}

/**
 * Builds an Eip1559TransactionStruct from a JSON file.
 * Expects the file to contain a signed EIP-1559 transaction object.
 */
function buildTransactionFromFile(filePath: string): Eip1559TransactionStruct {
  const absolutePath = path.resolve(filePath);
  console.log(`Reading transaction data from: ${absolutePath}`);

  const raw = JSON.parse(fs.readFileSync(absolutePath, "utf-8"));

  return {
    nonce: BigInt(raw.nonce),
    maxPriorityFeePerGas: BigInt(raw.maxPriorityFeePerGas),
    maxFeePerGas: BigInt(raw.maxFeePerGas),
    gasLimit: BigInt(raw.gasLimit ?? raw.gas),
    to: raw.to,
    value: BigInt(raw.value ?? 0),
    input: raw.input ?? raw.data ?? "0x",
    accessList: (raw.accessList ?? []).map(
      (entry: { address?: string; contractAddress?: string; storageKeys: string[] }) => ({
        contractAddress: entry.contractAddress ?? entry.address,
        storageKeys: entry.storageKeys ?? [],
      }),
    ),
    yParity: Number(raw.yParity ?? raw.v),
    r: BigInt(raw.r),
    s: BigInt(raw.s),
  };
}

/**
 * Builds an Eip1559TransactionStruct by signing a transaction with the provided wallet.
 */
async function buildTransactionFromParams(wallet: ethers.Wallet, chainId: bigint): Promise<Eip1559TransactionStruct> {
  const to = getRequiredEnvVar("TX_TO");
  const gasLimit = BigInt(getRequiredEnvVar("TX_GAS_LIMIT"));
  const maxFeePerGas = BigInt(getRequiredEnvVar("TX_MAX_FEE_PER_GAS"));
  const maxPriorityFeePerGas = BigInt(getRequiredEnvVar("TX_MAX_PRIORITY_FEE_PER_GAS"));
  const nonce = BigInt(getRequiredEnvVar("TX_NONCE"));
  const data = process.env.TX_DATA ?? "0x";
  const value = BigInt(process.env.TX_VALUE ?? "0");

  console.log(
    `Signing L2 transaction — to=${to}, nonce=${nonce}, gasLimit=${gasLimit}, maxFeePerGas=${maxFeePerGas}, maxPriorityFeePerGas=${maxPriorityFeePerGas}, value=${value}, data=${data}, chainId=${chainId}`,
  );

  const tx: ethers.TransactionLike = {
    type: 2,
    chainId,
    nonce: Number(nonce),
    maxPriorityFeePerGas,
    maxFeePerGas,
    gasLimit,
    to,
    value,
    data,
    accessList: [],
  };

  const signedTxHex = await wallet.signTransaction(tx);
  const parsed = ethers.Transaction.from(signedTxHex);

  if (!parsed.signature) {
    throw new Error("Failed to extract signature from signed transaction.");
  }

  return {
    nonce: BigInt(parsed.nonce),
    maxPriorityFeePerGas: parsed.maxPriorityFeePerGas!,
    maxFeePerGas: parsed.maxFeePerGas!,
    gasLimit: BigInt(parsed.gasLimit),
    to: parsed.to!,
    value: parsed.value,
    input: parsed.data,
    accessList: [],
    yParity: parsed.signature.yParity,
    r: BigInt(parsed.signature.r),
    s: BigInt(parsed.signature.s),
  };
}

async function main() {
  // --- Configuration ---
  const l1RpcUrl = getRequiredEnvVar("L1_RPC_URL");
  const privateKey = getRequiredEnvVar("PRIVATE_KEY");
  const gatewayAddress = getRequiredEnvVar("FORCED_TRANSACTION_GATEWAY_ADDRESS");
  const rollupAddress = getRequiredEnvVar("LINEA_ROLLUP_ADDRESS");

  const l2PrivateKey = process.env.L2_PRIVATE_KEY ?? privateKey;
  const txDataFile = process.env.TX_DATA_FILE;

  // --- Provider & Wallets ---
  const provider = new ethers.JsonRpcProvider(l1RpcUrl);
  const { chainId } = await provider.getNetwork();
  const l1Wallet = new ethers.Wallet(privateKey, provider);
  console.log(`Connected — l1RpcUrl=${l1RpcUrl}, chainId=${chainId}, l1Sender=${l1Wallet.address}`);

  // --- Contract Instances ---
  const rollup = new ethers.Contract(rollupAddress, LineaRollupV8Abi, l1Wallet);
  const gateway = new ethers.Contract(gatewayAddress, ForcedTransactionGatewayAbi, l1Wallet);

  // --- Resolve Last Finalized State ---
  console.log("\n--- Resolving last finalized state ---");
  const lastFinalizedState = await resolveLastFinalizedState(rollup, provider);
  console.log(
    `Last finalized state — timestamp=${lastFinalizedState.timestamp}, messageNumber=${lastFinalizedState.messageNumber}, messageRollingHash=${lastFinalizedState.messageRollingHash}, forcedTransactionNumber=${lastFinalizedState.forcedTransactionNumber}, forcedTransactionRollingHash=${lastFinalizedState.forcedTransactionRollingHash}`,
  );

  // --- Build EIP-1559 Transaction ---
  console.log("\n--- Building EIP-1559 transaction ---");
  let forcedTransaction: Eip1559TransactionStruct;

  if (txDataFile) {
    forcedTransaction = buildTransactionFromFile(txDataFile);
  } else {
    const destinationChainId: bigint = await gateway.DESTINATION_CHAIN_ID();
    const l2Wallet = new ethers.Wallet(l2PrivateKey);
    console.log(`Building from params — destinationChainId=${destinationChainId}, l2Signer=${l2Wallet.address}`);

    forcedTransaction = await buildTransactionFromParams(l2Wallet, destinationChainId);
  }

  console.log(
    `Transaction struct — nonce=${forcedTransaction.nonce}, to=${forcedTransaction.to}, value=${forcedTransaction.value}, gasLimit=${forcedTransaction.gasLimit}, maxFeePerGas=${forcedTransaction.maxFeePerGas}, maxPriorityFeePerGas=${forcedTransaction.maxPriorityFeePerGas}, inputLength=${forcedTransaction.input.length}, yParity=${forcedTransaction.yParity}`,
  );

  // --- Read Fee & Submit ---
  console.log("\n--- Submitting forced transaction ---");
  const [, , , , feeAmount] = await rollup.getRequiredForcedTransactionFields();

  // get1559Fees uses truthiness checks, so 0n values are omitted.
  // Query fee data directly to handle zero priority fees on local chains.
  const {
    maxFeePerGas: l1MaxFee,
    maxPriorityFeePerGas: l1PriorityFee,
    gasPrice: l1GasPrice,
  } = await provider.getFeeData();

  // Prefer EIP-1559 fields; only fall back to gasPrice for legacy chains
  const l1GasOverrides =
    l1MaxFee != null && l1PriorityFee != null
      ? { maxFeePerGas: l1MaxFee * 2n, maxPriorityFeePerGas: l1PriorityFee == 0n ? l1MaxFee * 2n : l1PriorityFee * 2n }
      : { gasPrice: l1GasPrice };

  console.log(
    `forcedTransactionFee=${feeAmount}, l1MaxFeePerGas=${l1GasOverrides.maxFeePerGas}, l1MaxPriorityFeePerGas=${l1GasOverrides.maxPriorityFeePerGas}, l1GasPrice=${l1GasPrice}, gasPrice=${l1GasOverrides.gasPrice}`,
  );

  const tx = await gateway.submitForcedTransaction(forcedTransaction, lastFinalizedState, {
    value: feeAmount,
    ...l1GasOverrides,
  });

  console.log(`Transaction submitted — txHash=${tx.hash}`);
  console.log("Waiting for confirmation...");

  const receipt = await tx.wait();
  console.log(`Transaction confirmed — block=${receipt!.blockNumber}, gasUsed=${receipt!.gasUsed}`);

  // Decode ForcedTransactionAdded events from the rollup
  const rollupInterface = new ethers.Interface(LineaRollupV8Abi);
  for (const log of receipt!.logs) {
    try {
      const parsed = rollupInterface.parseLog({ topics: log.topics as string[], data: log.data });
      if (parsed && parsed.name === "ForcedTransactionAdded") {
        console.log(
          `ForcedTransactionAdded — forcedTransactionNumber=${parsed.args.forcedTransactionNumber}, from=${parsed.args.from}, blockNumberDeadline=${parsed.args.blockNumberDeadline}, forcedTransactionRollingHash=${parsed.args.forcedTransactionRollingHash}, rlpEncodedSignedTransaction=${parsed.args.rlpEncodedSignedTransaction}`,
        );
      }
    } catch {
      // Skip logs from other contracts
    }
  }

  console.log("\nDone.");
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
