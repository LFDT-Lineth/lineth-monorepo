import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { time as networkTime } from "@nomicfoundation/hardhat-network-helpers";
import { LinethRollup } from "contracts/typechain-types";
import { AccessListish, ethers, Signature, Transaction } from "ethers";

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
  linethRollup: LinethRollup,
  nextNetworkTimestamp: bigint,
  lastFinalizedBlockTimestamp: bigint,
  l2BlockDurationSeconds: bigint = 1n,
) => {
  await networkTime.setNextBlockTimestamp(nextNetworkTimestamp);
  const lastFinalizedBlock = await linethRollup.currentL2BlockNumber();
  const expectedBlockNumber = _getExpectedL2BlockNumberForForcedTx({
    blockTimestamp: nextNetworkTimestamp,
    l2BlockBuffer: BigInt(THREE_DAYS_IN_SECONDS),
    currentFinalizedL2BlockNumber: lastFinalizedBlock,
    lastFinalizedBlockTimestamp: lastFinalizedBlockTimestamp,
    l2BlockDurationSeconds,
  });
  return expectedBlockNumber;
};

export const decodeForcedTransactionAdded = async (tx: ethers.ContractTransactionResponse, contract: LinethRollup) => {
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

/**
 * Rolling hash formula per the rollup spec (§6.3):
 * keccak256(prev ‖ txHash ‖ deadlineBlockNumber ‖ fromAddress), packed (32 + 32 + 32 + 20 bytes),
 * where txHash = keccak256(signedTxRlp) is the standard EIP-2718 transaction hash.
 */
const _computeForcedTransactionRollingHash = (
  previousRollingHash: string,
  signedTransactionHash: string,
  expectedBlockNumber: bigint,
  from: string,
): string => {
  return ethers.keccak256(
    ethers.solidityPacked(
      ["bytes32", "bytes32", "uint256", "address"],
      [previousRollingHash, signedTransactionHash, expectedBlockNumber, from],
    ),
  );
};

/** Computes keccak256 of the signed type-2 transaction envelope, matching the contract's stored RLP payload. */
const hashSignedEip1559LikeSolidity = (tx: Eip1559Transaction, chainId: bigint): string => {
  const signedTx = Transaction.from({
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
    signature: Signature.from({
      r: ethers.toBeHex(tx.r, 32),
      s: ethers.toBeHex(tx.s, 32),
      yParity: Number(tx.yParity) as 0 | 1,
    }),
  });

  return ethers.keccak256(signedTx.serialized);
};

export const getForcedTransactionRollingHash = async (
  linethRollup: LinethRollup,
  eip1559Tx: Eip1559Transaction,
  expectedBlockNumber: bigint,
  from: string,
  chainId: bigint,
): Promise<string> => {
  const { previousForcedTransactionRollingHash } = await linethRollup.getRequiredForcedTransactionFields();

  const signedTransactionHash = hashSignedEip1559LikeSolidity(eip1559Tx, chainId);

  return _computeForcedTransactionRollingHash(
    previousForcedTransactionRollingHash,
    signedTransactionHash,
    expectedBlockNumber,
    from,
  );
};

export const setForcedTransactionFee = async (
  linethRollup: LinethRollup,
  forcedTransactionFee: bigint,
  signer: SignerWithAddress,
) => {
  await linethRollup.connect(signer).setForcedTransactionFee(forcedTransactionFee);
};
