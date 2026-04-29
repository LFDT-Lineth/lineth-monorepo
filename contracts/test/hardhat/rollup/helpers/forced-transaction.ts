import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { time as networkTime } from "@nomicfoundation/hardhat-network-helpers";
import { encodeData } from "contracts/common/helpers";
import { LineaRollup, Mimc } from "contracts/typechain-types";
import { AccessListish, ethers, Transaction } from "ethers";

import { THREE_DAYS_IN_SECONDS } from "../../common/constants";
import { Eip1559Transaction } from "../../common/types";

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
  lineaRollup: LineaRollup,
  nextNetworkTimestamp: bigint,
  lastFinalizedBlockTimestamp: bigint,
  l2BlockDurationSeconds: bigint = 1n,
) => {
  await networkTime.setNextBlockTimestamp(nextNetworkTimestamp);
  const lastFinalizedBlock = await lineaRollup.currentL2BlockNumber();
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
const splitBytes32 = (hashedPayload: string): { msb: string; lsb: string } => {
  // Parse as 256-bit integer
  const value = BigInt(hashedPayload);

  // Masks
  const MASK_128 = (1n << 128n) - 1n;

  // Split
  const msb = value >> 128n;
  const lsb = value & MASK_128;

  // Return as bytes32 (zero-left-padded)
  return {
    msb: "0x" + msb.toString(16).padStart(64, "0"),
    lsb: "0x" + lsb.toString(16).padStart(64, "0"),
  };
};

export const decodeForcedTransactionAdded = async (tx: ethers.ContractTransactionResponse, contract: LineaRollup) => {
  const receipt = await tx.wait();
  if (!receipt) return [];

  return receipt.logs.flatMap((log) => {
    try {
      const parsed = contract.interface.parseLog(log);
      return parsed!.name === "ForcedTransactionAdded" ? [parsed!] : [];
    } catch {
      return [];
    }
  });
};

const _computeForcedTransactionRollingHash = async (
  mimcLibrary: Mimc,
  previousRollingHash: string,
  hashedPayload: string,
  expectedBlockNumber: bigint,
  from: string,
): Promise<string> => {
  const { msb: hashedPayloadMsb, lsb: hashedPayloadLsb } = splitBytes32(hashedPayload);

  const mimcPayload = encodeData(
    ["bytes32", "bytes32", "bytes32", "uint256", "address"],
    [previousRollingHash, hashedPayloadMsb, hashedPayloadLsb, expectedBlockNumber, from],
  );
  return await mimcLibrary.hash(mimcPayload);
};

const hashEip1559LikeSolidity = (tx: Eip1559Transaction, chainId: bigint): string => {
  const unsignedTx = Transaction.from({
    type: 2,
    chainId,
    nonce: Number(tx.nonce),
    maxPriorityFeePerGas: tx.maxPriorityFeePerGas,
    maxFeePerGas: tx.maxFeePerGas,
    gasLimit: tx.gasLimit,
    to: tx.to === ethers.ZeroAddress ? null : tx.to,
    value: tx.value,
    data: tx.input,
    accessList: tx.accessList.map((a) => [a.contractAddress, a.storageKeys]) as AccessListish,
  });

  return unsignedTx.unsignedHash;
};

export const getForcedTransactionRollingHash = async (
  mimcLibrary: Mimc,
  lineaRollup: LineaRollup,
  eip1559Tx: Eip1559Transaction,
  expectedBlockNumber: bigint,
  from: string,
  chainId: bigint,
): Promise<string> => {
  const { previousForcedTransactionRollingHash } = await lineaRollup.getRequiredForcedTransactionFields();

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
  lineaRollup: LineaRollup,
  forcedTransactionFee: bigint,
  signer: SignerWithAddress,
) => {
  await lineaRollup.connect(signer).setForcedTransactionFee(forcedTransactionFee);
};
