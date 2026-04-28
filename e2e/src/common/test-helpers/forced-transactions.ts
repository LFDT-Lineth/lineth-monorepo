import { existsSync, readFileSync } from "fs";
import {
  type Address,
  type Client,
  type Hash,
  type Hex,
  encodeAbiParameters,
  keccak256,
  parseTransaction,
  zeroHash,
} from "viem";
import { type PrivateKeyAccount } from "viem/accounts";

import { getLineaRollupContract } from "../../config/contracts/contracts";
import { createTestLogger } from "../../config/logger";
import { TestContext } from "../../config/setup/index";
import { LineaRollupV8Abi } from "../../generated";
import { GENESIS_TIMESTAMP_FILE_PATH } from "../constants";
import { getRawTransactionHex, waitForEvents } from "../utils";

const logger = createTestLogger();

export type LastFinalizedState = {
  timestamp: bigint;
  messageNumber: bigint;
  messageRollingHash: Hex;
  forcedTransactionNumber: bigint;
  forcedTransactionRollingHash: Hex;
};

export type BuildForcedTransactionParams = {
  l2Account: PrivateKeyAccount;
  to: Address;
  nonce: bigint;
  gasLimit: bigint;
  maxFeePerGas: bigint;
  maxPriorityFeePerGas: bigint;
  value?: bigint;
  data?: Hex;
};

export type ForcedTransactionStruct = {
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

export function getDefaultLastFinalizedTimestamp() {
  const filePath = GENESIS_TIMESTAMP_FILE_PATH;
  if (!existsSync(filePath)) {
    throw new Error(`File not found: ${filePath}`);
  }
  const timestamp = readFileSync(filePath, "utf-8").trim();

  if (!Number(timestamp)) {
    throw new Error(`Invalid timestamp value in file: ${filePath}`);
  }

  return BigInt(timestamp);
}

export function computeFinalizedStateHash(state: LastFinalizedState): Hex {
  return keccak256(
    encodeAbiParameters(
      [{ type: "uint256" }, { type: "bytes32" }, { type: "uint256" }, { type: "bytes32" }, { type: "uint256" }],
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

export async function resolveLastFinalizedState(
  lineaRollup: ReturnType<typeof getLineaRollupContract>,
  l1PublicClient: Client,
  genesisTimestamp: bigint,
): Promise<LastFinalizedState> {
  const onChainStateHash: Hex = await lineaRollup.read.currentFinalizedState();

  const defaultState: LastFinalizedState = {
    timestamp: genesisTimestamp,
    messageNumber: 0n,
    messageRollingHash: zeroHash,
    forcedTransactionNumber: 0n,
    forcedTransactionRollingHash: zeroHash,
  };

  const defaultHash = computeFinalizedStateHash(defaultState);

  if (onChainStateHash === defaultHash) {
    logger.debug("Finalized state matches default genesis state.");
    return defaultState;
  }

  logger.debug("Finalized state differs from default — reconstructing from on-chain events...");

  const currentL2BlockNumber: bigint = await lineaRollup.read.currentL2BlockNumber();

  const events = await waitForEvents(l1PublicClient, {
    abi: LineaRollupV8Abi,
    address: lineaRollup.address as Address,
    eventName: "FinalizedStateUpdated",
    fromBlock: 0n,
    toBlock: "latest",
    args: { blockNumber: currentL2BlockNumber },
    pollingIntervalMs: 500,
    timeoutMs: 100_000,
    strict: true,
  });

  const latestEvent = events[events.length - 1];
  const timestamp = latestEvent.args.timestamp;
  const messageNumber = latestEvent.args.messageNumber;
  const forcedTransactionNumber = latestEvent.args.forcedTransactionNumber;

  const [messageRollingHash, forcedTransactionRollingHash] = await Promise.all([
    lineaRollup.read.rollingHashes([messageNumber]),
    lineaRollup.read.forcedTransactionRollingHashes([forcedTransactionNumber]),
  ]);

  const reconstructed: LastFinalizedState = {
    timestamp,
    messageNumber,
    messageRollingHash: messageRollingHash as Hex,
    forcedTransactionNumber,
    forcedTransactionRollingHash: forcedTransactionRollingHash as Hex,
  };

  const reconstructedHash = computeFinalizedStateHash(reconstructed);
  if (reconstructedHash !== onChainStateHash) {
    throw new Error(
      `Reconstructed state hash (${reconstructedHash}) does not match on-chain hash (${onChainStateHash}).`,
    );
  }

  logger.debug("Reconstructed finalized state validated successfully.");
  return reconstructed;
}

export type BuildForcedTransactionResult = {
  forcedTransaction: ForcedTransactionStruct;
  l2TxHash: Hash;
};

export async function buildSignedForcedTransaction(
  context: TestContext,
  params: BuildForcedTransactionParams,
): Promise<BuildForcedTransactionResult> {
  const { l2Account, to, nonce, gasLimit, maxFeePerGas, maxPriorityFeePerGas, value = 0n, data = "0x" } = params;

  const client = context.l2PublicClient();

  const txRequest = await client.prepareTransactionRequest({
    type: "eip1559",
    account: l2Account,
    to,
    value,
    maxPriorityFeePerGas,
    maxFeePerGas,
    gas: gasLimit,
    data,
    nonce: Number(nonce),
    accessList: [],
  });

  const signedTxHex = await getRawTransactionHex(client, txRequest);
  const l2TxHash = keccak256(signedTxHex);

  const parsed = parseTransaction(signedTxHex);

  if (parsed.yParity === undefined || parsed.r === undefined || parsed.s === undefined) {
    throw new Error("Parsed transaction is missing signature fields (yParity, r, s).");
  }

  return {
    forcedTransaction: {
      nonce,
      maxPriorityFeePerGas,
      maxFeePerGas,
      gasLimit,
      to,
      value,
      input: data as Hex,
      accessList: [] as { contractAddress: Address; storageKeys: Hex[] }[],
      yParity: parsed.yParity,
      r: BigInt(parsed.r),
      s: BigInt(parsed.s),
    },
    l2TxHash,
  };
}
