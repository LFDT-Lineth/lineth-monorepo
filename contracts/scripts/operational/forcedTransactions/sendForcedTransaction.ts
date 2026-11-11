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
 *   LINETH_ROLLUP_ADDRESS               Deployed LinethRollup proxy address
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
 *   LINETH_ROLLUP_ADDRESS=0xDc64a140Aa3E981100a9becA4E685f962f0cF6C9 \
 *   TX_TO=0x70997970C51812dc3A010C7d01b50e0d17dc79C8 \
 *   TX_GAS_LIMIT=21000 \
 *   TX_MAX_FEE_PER_GAS=1000000000 \
 *   TX_MAX_PRIORITY_FEE_PER_GAS=100000000 \
 *   TX_NONCE=0 \
 *   pnpm exec tsx scripts/operational/forcedTransactions/sendForcedTransaction.ts
 *
 *   # Mode A — submit a pre-signed transaction from a JSON file
 *   L1_RPC_URL=http://localhost:8445 \
 *   PRIVATE_KEY=0x7c852118294e51e653712a81e05800f419141751be58f605c371e15141b007a6 \
 *   FORCED_TRANSACTION_GATEWAY_ADDRESS=0x0165878A594ca255338adfa4d48449f69242Eb8F \
 *   LINETH_ROLLUP_ADDRESS=0xDc64a140Aa3E981100a9becA4E685f962f0cF6C9 \
 *   TX_DATA_FILE=./my-signed-tx.json \
 *   pnpm exec tsx scripts/operational/forcedTransactions/sendForcedTransaction.ts
 */

import * as dotenv from "dotenv";
import fs from "fs";
import path from "path";
import {
  type Address,
  type Hex,
  type PrivateKeyAccount,
  type PublicClient,
  createPublicClient,
  createWalletClient,
  encodeAbiParameters,
  getAddress,
  hexToBigInt,
  http,
  isHex,
  keccak256,
  parseAbi,
  parseAbiParameters,
  parseEventLogs,
  parseTransaction,
  zeroHash,
} from "viem";
import { privateKeyToAccount } from "viem/accounts";

import { getRequiredEnvVar } from "../../../common/helpers/environment.js";

// Fragments of the LinethRollupV8 / ForcedTransactionGateway ABIs
// (local-deployments-artifacts/{dynamic-artifacts/LinethRollupV8,static-artifacts/ForcedTransactionGateway}.json).
const LinethRollupV8Abi = parseAbi([
  "function currentFinalizedState() view returns (bytes32)",
  "function currentL2BlockNumber() view returns (uint256)",
  "function rollingHashes(uint256 messageNumber) view returns (bytes32 rollingHash)",
  "function forcedTransactionRollingHashes(uint256 forcedTransactionNumber) view returns (bytes32 rollingHash)",
  "function getRequiredForcedTransactionFields() view returns (bytes32 finalizedState, bytes32 previousForcedTransactionRollingHash, uint256 previousForcedTransactionBlockDeadline, uint256 currentFinalizedL2BlockNumber, uint256 forcedTransactionFeeAmount)",
  "event FinalizedStateUpdated(uint256 indexed blockNumber, uint256 timestamp, uint256 messageNumber, uint256 forcedTransactionNumber)",
  "event ForcedTransactionAdded(uint256 indexed forcedTransactionNumber, address indexed from, uint256 blockNumberDeadline, bytes32 forcedTransactionRollingHash, bytes rlpEncodedSignedTransaction)",
]);
const ForcedTransactionGatewayAbi = parseAbi([
  "struct AccessList { address contractAddress; bytes32[] storageKeys; }",
  "struct Eip1559Transaction { uint256 nonce; uint256 maxPriorityFeePerGas; uint256 maxFeePerGas; uint256 gasLimit; address to; uint256 value; bytes input; AccessList[] accessList; uint8 yParity; uint256 r; uint256 s; }",
  "struct LastFinalizedState { uint256 timestamp; uint256 messageNumber; bytes32 messageRollingHash; uint256 forcedTransactionNumber; bytes32 forcedTransactionRollingHash; }",
  "function DESTINATION_CHAIN_ID() view returns (uint256)",
  "function submitForcedTransaction(Eip1559Transaction _forcedTransaction, LastFinalizedState _lastFinalizedState) payable",
]);

dotenv.config();

// Default finalized state used on fresh deployments (genesis).
// This timestamp is specific to the local dev stack. Real-world deployments
// will have a different genesis timestamp — always verify against the chain's
// genesis configuration file before using this value.
const DEFAULT_LAST_FINALIZED_TIMESTAMP = 1770394477n;
const DEFAULT_FINALIZED_STATE = {
  timestamp: DEFAULT_LAST_FINALIZED_TIMESTAMP,
  messageNumber: 0n,
  messageRollingHash: zeroHash,
  forcedTransactionNumber: 0n,
  forcedTransactionRollingHash: zeroHash,
};

type LastFinalizedState = {
  timestamp: bigint;
  messageNumber: bigint;
  messageRollingHash: Hex;
  forcedTransactionNumber: bigint;
  forcedTransactionRollingHash: Hex;
};

type Eip1559TransactionStruct = {
  nonce: bigint;
  maxPriorityFeePerGas: bigint;
  maxFeePerGas: bigint;
  gasLimit: bigint;
  to: Address;
  value: bigint;
  input: Hex;
  accessList: { contractAddress: Address; storageKeys: Hex[] }[];
  yParity: number;
  r: bigint;
  s: bigint;
};

function toHexValue(value: unknown, label: string): Hex {
  if (typeof value !== "string" || !isHex(value)) {
    throw new Error(`Invalid ${label}: expected a 0x-prefixed hex string.`);
  }
  return value;
}

/**
 * Computes the finalized state hash identical to
 * FinalizedStateHashing._computeLastFinalizedState in Solidity.
 */
function computeFinalizedStateHash(state: LastFinalizedState): Hex {
  return keccak256(
    encodeAbiParameters(parseAbiParameters("uint256, bytes32, uint256, bytes32, uint256"), [
      state.messageNumber,
      state.messageRollingHash,
      state.forcedTransactionNumber,
      state.forcedTransactionRollingHash,
      state.timestamp,
    ]),
  );
}

/**
 * Resolves the current LastFinalizedState from on-chain data.
 *
 * 1. Checks if the on-chain hash matches the default (genesis) state.
 * 2. If not, reconstructs the state from the FinalizedStateUpdated event
 *    and the rollingHashes / forcedTransactionRollingHashes mappings.
 */
async function resolveLastFinalizedState(provider: PublicClient, rollupAddress: Address): Promise<LastFinalizedState> {
  const rollup = { address: rollupAddress, abi: LinethRollupV8Abi } as const;
  const onChainStateHash = await provider.readContract({ ...rollup, functionName: "currentFinalizedState" });
  console.log(`On-chain currentFinalizedState: ${onChainStateHash}`);

  const defaultHash = computeFinalizedStateHash(DEFAULT_FINALIZED_STATE);
  if (onChainStateHash === defaultHash) {
    console.log("Finalized state matches default genesis state.");
    return { ...DEFAULT_FINALIZED_STATE };
  }

  console.log("Finalized state differs from default — reconstructing from on-chain events...");

  const currentL2BlockNumber = await provider.readContract({ ...rollup, functionName: "currentL2BlockNumber" });
  console.log(`currentL2BlockNumber: ${currentL2BlockNumber}`);

  // FinalizedStateUpdated(uint256,uint256,uint256,uint256) filtered on the indexed blockNumber.
  const logs = await provider.getContractEvents({
    ...rollup,
    eventName: "FinalizedStateUpdated",
    args: { blockNumber: currentL2BlockNumber },
    fromBlock: 0n,
    toBlock: "latest",
  });

  if (logs.length === 0) {
    throw new Error(
      `No FinalizedStateUpdated event found for blockNumber=${currentL2BlockNumber}. ` +
        `Cannot reconstruct the last finalized state.`,
    );
  }

  const latestLog = logs[logs.length - 1];
  const { timestamp, messageNumber, forcedTransactionNumber } = latestLog.args;

  if (timestamp === undefined || messageNumber === undefined || forcedTransactionNumber === undefined) {
    throw new Error("Failed to parse FinalizedStateUpdated event.");
  }

  console.log(
    `Event data — timestamp=${timestamp}, messageNumber=${messageNumber}, forcedTransactionNumber=${forcedTransactionNumber}`,
  );

  const [messageRollingHash, forcedTransactionRollingHash] = await Promise.all([
    provider.readContract({ ...rollup, functionName: "rollingHashes", args: [messageNumber] }),
    provider.readContract({
      ...rollup,
      functionName: "forcedTransactionRollingHashes",
      args: [forcedTransactionNumber],
    }),
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
    to: getAddress(raw.to),
    value: BigInt(raw.value ?? 0),
    input: toHexValue(raw.input ?? raw.data ?? "0x", "input"),
    accessList: (raw.accessList ?? []).map(
      (entry: { address?: string; contractAddress?: string; storageKeys: string[] }) => ({
        contractAddress: getAddress(entry.contractAddress ?? entry.address ?? ""),
        storageKeys: (entry.storageKeys ?? []).map((key) => toHexValue(key, "accessList storage key")),
      }),
    ),
    yParity: Number(raw.yParity ?? raw.v),
    r: BigInt(raw.r),
    s: BigInt(raw.s),
  };
}

/**
 * Builds an Eip1559TransactionStruct by signing a transaction with the provided account.
 */
async function buildTransactionFromParams(
  account: PrivateKeyAccount,
  chainId: bigint,
): Promise<Eip1559TransactionStruct> {
  const to = getAddress(getRequiredEnvVar("TX_TO"));
  const gasLimit = BigInt(getRequiredEnvVar("TX_GAS_LIMIT"));
  const maxFeePerGas = BigInt(getRequiredEnvVar("TX_MAX_FEE_PER_GAS"));
  const maxPriorityFeePerGas = BigInt(getRequiredEnvVar("TX_MAX_PRIORITY_FEE_PER_GAS"));
  const nonce = BigInt(getRequiredEnvVar("TX_NONCE"));
  const data = toHexValue(process.env.TX_DATA ?? "0x", "TX_DATA");
  const value = BigInt(process.env.TX_VALUE ?? "0");

  console.log(
    `Signing L2 transaction — to=${to}, nonce=${nonce}, gasLimit=${gasLimit}, maxFeePerGas=${maxFeePerGas}, maxPriorityFeePerGas=${maxPriorityFeePerGas}, value=${value}, data=${data}, chainId=${chainId}`,
  );

  const signedTxHex = await account.signTransaction({
    type: "eip1559",
    chainId: Number(chainId),
    nonce: Number(nonce),
    maxPriorityFeePerGas,
    maxFeePerGas,
    gas: gasLimit,
    to,
    value,
    data,
    accessList: [],
  });
  const parsed = parseTransaction(signedTxHex);

  if (parsed.r === undefined || parsed.s === undefined || parsed.yParity === undefined) {
    throw new Error("Failed to extract signature from signed transaction.");
  }
  if (
    parsed.nonce === undefined ||
    parsed.maxPriorityFeePerGas === undefined ||
    parsed.maxFeePerGas === undefined ||
    parsed.gas === undefined ||
    !parsed.to
  ) {
    throw new Error("Failed to parse signed transaction.");
  }

  return {
    nonce: BigInt(parsed.nonce),
    maxPriorityFeePerGas: parsed.maxPriorityFeePerGas,
    maxFeePerGas: parsed.maxFeePerGas,
    gasLimit: parsed.gas,
    to: parsed.to,
    value: parsed.value ?? 0n,
    input: parsed.data ?? "0x",
    accessList: [],
    yParity: parsed.yParity,
    r: hexToBigInt(parsed.r),
    s: hexToBigInt(parsed.s),
  };
}

async function main() {
  // --- Configuration ---
  const l1RpcUrl = getRequiredEnvVar("L1_RPC_URL");
  const privateKey = toHexValue(getRequiredEnvVar("PRIVATE_KEY"), "PRIVATE_KEY");
  const gatewayAddress = getAddress(getRequiredEnvVar("FORCED_TRANSACTION_GATEWAY_ADDRESS"));
  const rollupAddress = getAddress(getRequiredEnvVar("LINETH_ROLLUP_ADDRESS"));

  const l2PrivateKey = process.env.L2_PRIVATE_KEY
    ? toHexValue(process.env.L2_PRIVATE_KEY, "L2_PRIVATE_KEY")
    : privateKey;
  const txDataFile = process.env.TX_DATA_FILE;

  // --- Clients & Accounts ---
  const provider = createPublicClient({ transport: http(l1RpcUrl) });
  const chainId = await provider.getChainId();
  const l1Account = privateKeyToAccount(privateKey);
  const l1Wallet = createWalletClient({ account: l1Account, transport: http(l1RpcUrl) });
  console.log(`Connected — l1RpcUrl=${l1RpcUrl}, chainId=${chainId}, l1Sender=${l1Account.address}`);

  // --- Contract Instances ---
  const rollup = { address: rollupAddress, abi: LinethRollupV8Abi } as const;
  const gateway = { address: gatewayAddress, abi: ForcedTransactionGatewayAbi } as const;

  // --- Resolve Last Finalized State ---
  console.log("\n--- Resolving last finalized state ---");
  const lastFinalizedState = await resolveLastFinalizedState(provider, rollupAddress);
  console.log(
    `Last finalized state — timestamp=${lastFinalizedState.timestamp}, messageNumber=${lastFinalizedState.messageNumber}, messageRollingHash=${lastFinalizedState.messageRollingHash}, forcedTransactionNumber=${lastFinalizedState.forcedTransactionNumber}, forcedTransactionRollingHash=${lastFinalizedState.forcedTransactionRollingHash}`,
  );

  // --- Build EIP-1559 Transaction ---
  console.log("\n--- Building EIP-1559 transaction ---");
  let forcedTransaction: Eip1559TransactionStruct;

  if (txDataFile) {
    forcedTransaction = buildTransactionFromFile(txDataFile);
  } else {
    const destinationChainId = await provider.readContract({ ...gateway, functionName: "DESTINATION_CHAIN_ID" });
    const l2Account = privateKeyToAccount(l2PrivateKey);
    console.log(`Building from params — destinationChainId=${destinationChainId}, l2Signer=${l2Account.address}`);

    forcedTransaction = await buildTransactionFromParams(l2Account, destinationChainId);
  }

  console.log(
    `Transaction struct — nonce=${forcedTransaction.nonce}, to=${forcedTransaction.to}, value=${forcedTransaction.value}, gasLimit=${forcedTransaction.gasLimit}, maxFeePerGas=${forcedTransaction.maxFeePerGas}, maxPriorityFeePerGas=${forcedTransaction.maxPriorityFeePerGas}, inputLength=${forcedTransaction.input.length}, yParity=${forcedTransaction.yParity}`,
  );

  // --- Read Fee & Submit ---
  console.log("\n--- Submitting forced transaction ---");
  const [, , , , feeAmount] = await provider.readContract({
    ...rollup,
    functionName: "getRequiredForcedTransactionFields",
  });

  // Query fee data directly to handle zero priority fees on local chains.
  const block = await provider.getBlock();
  const l1GasPrice = await provider.getGasPrice();
  const l1FeesPerGas = block.baseFeePerGas == null ? undefined : await provider.estimateFeesPerGas();
  const l1MaxFee = l1FeesPerGas?.maxFeePerGas;
  const l1PriorityFee = l1FeesPerGas?.maxPriorityFeePerGas;

  // Prefer EIP-1559 fields; only fall back to gasPrice for legacy chains
  const l1GasOverrides =
    l1MaxFee != null && l1PriorityFee != null
      ? {
          maxFeePerGas: l1MaxFee * 2n,
          maxPriorityFeePerGas: l1PriorityFee == 0n ? l1MaxFee * 2n : l1PriorityFee * 2n,
          gasPrice: undefined,
        }
      : { gasPrice: l1GasPrice, maxFeePerGas: undefined, maxPriorityFeePerGas: undefined };

  console.log(
    `forcedTransactionFee=${feeAmount}, l1MaxFeePerGas=${l1GasOverrides.maxFeePerGas}, l1MaxPriorityFeePerGas=${l1GasOverrides.maxPriorityFeePerGas}, l1GasPrice=${l1GasPrice}, gasPrice=${l1GasOverrides.gasPrice}`,
  );

  const submitArgs = {
    ...gateway,
    functionName: "submitForcedTransaction",
    args: [forcedTransaction, lastFinalizedState],
    value: feeAmount,
    chain: null,
  } as const;
  const hash =
    l1GasOverrides.gasPrice !== undefined
      ? await l1Wallet.writeContract({ ...submitArgs, gasPrice: l1GasOverrides.gasPrice })
      : await l1Wallet.writeContract({
          ...submitArgs,
          maxFeePerGas: l1GasOverrides.maxFeePerGas,
          maxPriorityFeePerGas: l1GasOverrides.maxPriorityFeePerGas,
        });

  console.log(`Transaction submitted — txHash=${hash}`);
  console.log("Waiting for confirmation...");

  const receipt = await provider.waitForTransactionReceipt({ hash });
  console.log(`Transaction confirmed — block=${receipt.blockNumber}, gasUsed=${receipt.gasUsed}`);

  // Decode ForcedTransactionAdded events from the rollup (logs from other contracts are skipped)
  const forcedTransactionAddedLogs = parseEventLogs({
    abi: LinethRollupV8Abi,
    eventName: "ForcedTransactionAdded",
    logs: receipt.logs,
  });
  for (const parsed of forcedTransactionAddedLogs) {
    console.log(
      `ForcedTransactionAdded — forcedTransactionNumber=${parsed.args.forcedTransactionNumber}, from=${parsed.args.from}, blockNumberDeadline=${parsed.args.blockNumberDeadline}, forcedTransactionRollingHash=${parsed.args.forcedTransactionRollingHash}, rlpEncodedSignedTransaction=${parsed.args.rlpEncodedSignedTransaction}`,
    );
  }

  console.log("\nDone.");
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
