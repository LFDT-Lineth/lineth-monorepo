import { type Hash, type Hex, keccak256, parseEventLogs, serializeTransaction, toHex, zeroAddress } from "viem";

import { encodeData } from "../../../../common/helpers/index.js";
import { THREE_DAYS_IN_SECONDS } from "../../common/constants/index.js";
import { getConnection } from "../../common/helpers/connection.js";
import { type TestAccount } from "../../common/helpers/index.js";

import type { Eip1559Transaction } from "../../common/types.js";
import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

const _getExpectedL2BlockNumberForForcedTx = (params: {
  blockTimestamp: bigint;
  lastFinalizedBlockTimestamp: bigint;
  currentFinalizedL2BlockNumber: bigint;
  l2BlockBuffer: bigint;
  l2BlockDurationSeconds?: bigint;
}) => {
  const {
    blockTimestamp,
    lastFinalizedBlockTimestamp,
    currentFinalizedL2BlockNumber,
    l2BlockBuffer,
    l2BlockDurationSeconds = 1n,
  } = params;
  const elapsedTime = blockTimestamp - lastFinalizedBlockTimestamp;
  const elapsedBlocks = elapsedTime / l2BlockDurationSeconds;
  return currentFinalizedL2BlockNumber + elapsedBlocks + l2BlockBuffer;
};

export const setNextExpectedL2BlockNumberForForcedTx = async (
  linethRollup: ContractReturnType<"TestLinethRollup">,
  nextNetworkTimestamp: bigint,
  lastFinalizedBlockTimestamp: bigint,
  l2BlockDurationSeconds: bigint = 1n,
) => {
  const { networkHelpers } = await getConnection();
  await networkHelpers.time.setNextBlockTimestamp(nextNetworkTimestamp);
  const lastFinalizedBlock = await linethRollup.read.currentL2BlockNumber();
  const expectedBlockNumber = _getExpectedL2BlockNumberForForcedTx({
    blockTimestamp: nextNetworkTimestamp,
    l2BlockBuffer: BigInt(THREE_DAYS_IN_SECONDS),
    currentFinalizedL2BlockNumber: lastFinalizedBlock,
    lastFinalizedBlockTimestamp: lastFinalizedBlockTimestamp,
    l2BlockDurationSeconds,
  });
  return expectedBlockNumber;
};

/**
 * Splits a 32-byte value into MSB and LSB (128 bits each),
 * matching Solidity assembly behavior.
 */
const splitBytes32 = (hashedPayload: Hex): { msb: Hex; lsb: Hex } => {
  // Parse as 256-bit integer
  const value = BigInt(hashedPayload);

  // Masks
  const MASK_128 = (1n << 128n) - 1n;

  // Split
  const msb = value >> 128n;
  const lsb = value & MASK_128;

  // Return as bytes32 (zero-left-padded)
  return {
    msb: toHex(msb, { size: 32 }),
    lsb: toHex(lsb, { size: 32 }),
  };
};

/**
 * Waits for the transaction and returns its decoded `ForcedTransactionAdded` events (parsed with `contract`'s ABI).
 */
export const decodeForcedTransactionAdded = async (txHash: Hash, contract: ContractReturnType<"TestLinethRollup">) => {
  const { viem } = await getConnection();
  const publicClient = await viem.getPublicClient();
  const receipt = await publicClient.waitForTransactionReceipt({ hash: txHash });

  return parseEventLogs({ abi: contract.abi, eventName: "ForcedTransactionAdded", logs: receipt.logs });
};

const _computeForcedTransactionRollingHash = async (
  mimcLibrary: ContractReturnType<"Mimc">,
  previousRollingHash: Hex,
  hashedPayload: Hex,
  expectedBlockNumber: bigint,
  from: string,
): Promise<Hex> => {
  const { msb: hashedPayloadMsb, lsb: hashedPayloadLsb } = splitBytes32(hashedPayload);

  const mimcPayload = encodeData(
    ["bytes32", "bytes32", "bytes32", "uint256", "address"],
    [previousRollingHash, hashedPayloadMsb, hashedPayloadLsb, expectedBlockNumber, from],
  );
  return await mimcLibrary.read.hash([mimcPayload]);
};

const hashEip1559LikeSolidity = (tx: Eip1559Transaction, chainId: bigint): Hex => {
  // The unsigned EIP-1559 hash: keccak256 of the type-2 envelope without signature.
  const unsignedTx = serializeTransaction({
    type: "eip1559",
    chainId: Number(chainId),
    nonce: Number(tx.nonce),
    maxPriorityFeePerGas: BigInt(tx.maxPriorityFeePerGas),
    maxFeePerGas: BigInt(tx.maxFeePerGas),
    gas: BigInt(tx.gasLimit),
    to: tx.to === zeroAddress ? undefined : tx.to,
    value: BigInt(tx.value),
    data: tx.input,
    accessList: tx.accessList.map((a) => ({ address: a.contractAddress, storageKeys: a.storageKeys })),
  });

  return keccak256(unsignedTx);
};

export const getForcedTransactionRollingHash = async (
  mimcLibrary: ContractReturnType<"Mimc">,
  linethRollup: ContractReturnType<"TestLinethRollup">,
  eip1559Tx: Eip1559Transaction,
  expectedBlockNumber: bigint,
  from: string,
  chainId: bigint,
): Promise<Hex> => {
  const [, previousForcedTransactionRollingHash] = await linethRollup.read.getRequiredForcedTransactionFields();

  const hashedPayload = hashEip1559LikeSolidity(eip1559Tx, chainId);

  return await _computeForcedTransactionRollingHash(
    mimcLibrary,
    previousForcedTransactionRollingHash,
    hashedPayload,
    expectedBlockNumber,
    from,
  );
};

export const setForcedTransactionFee = async (
  linethRollup: ContractReturnType<"TestLinethRollup">,
  forcedTransactionFee: bigint,
  signer: TestAccount,
) => {
  await linethRollup.write.setForcedTransactionFee([forcedTransactionFee], { account: signer.account });
};
