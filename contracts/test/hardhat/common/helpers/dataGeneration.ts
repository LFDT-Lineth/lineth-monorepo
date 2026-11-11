import * as fs from "fs";
import { type Account, type Hash, type Hex, bytesToHex, concat, encodePacked, keccak256 } from "viem";

import { getConnection } from "./connection.js";
import { address, generateRandomBytes, hex, range } from "./general.js";
import {
  HASH_ZERO,
  COMPRESSED_SUBMISSION_DATA,
  COMPRESSED_SUBMISSION_DATA_MULTIPLE_PROOF,
  BLOB_SUBMISSION_DATA,
  BLOB_SUBMISSION_DATA_MULTIPLE_PROOF,
} from "../constants/index.js";

import type {
  FinalizationData,
  CalldataSubmissionData,
  ShnarfData,
  ParentAndExpectedShnarf,
  BlobSubmission,
  AggregatedProofData,
  ShnarfDataGenerator,
} from "../types.js";

export const generateL2MessagingBlocksOffsets = (start: number, end: number) =>
  `0x${range(start, end)
    .map((num) => encodePacked(["uint16"], [num]).slice(2))
    .join("")}` as const;

/**
 * Context for generating finalization parameters from proof data
 */
export type ProofFinalizationContext = {
  proofData: AggregatedProofData;
  shnarfDataGenerator: ShnarfDataGenerator;
  blobParentShnarfIndex: number;
  isMultiple: boolean;
};

/**
 * Converts AggregatedProofData to finalization data parameters.
 * This consolidates the repeated mapping pattern used across finalization tests.
 */
export function proofDataToFinalizationParams(context: ProofFinalizationContext): Partial<FinalizationData> {
  const { proofData, shnarfDataGenerator, blobParentShnarfIndex, isMultiple } = context;

  return {
    l1RollingHash: hex(proofData.l1RollingHash),
    l1RollingHashMessageNumber: BigInt(proofData.l1RollingHashMessageNumber),
    lastFinalizedTimestamp: BigInt(proofData.parentAggregationLastBlockTimestamp),
    endBlockNumber: BigInt(proofData.finalBlockNumber),
    parentStateRootHash: hex(proofData.parentStateRootHash),
    finalTimestamp: BigInt(proofData.finalTimestamp),
    l2MerkleRoots: proofData.l2MerkleRoots.map(hex),
    l2MerkleTreesDepth: BigInt(proofData.l2MerkleTreesDepth),
    l2MessagingBlocksOffsets: hex(proofData.l2MessagingBlocksOffsets),
    aggregatedProof: hex(proofData.aggregatedProof),
    shnarfData: shnarfDataGenerator(blobParentShnarfIndex, isMultiple),
    lastFinalizedL1RollingHash: hex(proofData.lastFinalizedL1RollingHash),
    lastFinalizedL1RollingHashMessageNumber: BigInt(proofData.lastFinalizedL1RollingHashMessageNumber),
    lastFinalizedForcedTransactionRollingHash: hex(proofData.parentAggregationFtxRollingHash),
    lastFinalizedForcedTransactionNumber: BigInt(proofData.parentAggregationFtxNumber),
    finalForcedTransactionNumber: BigInt(proofData.finalFtxNumber),
    filteredAddresses: proofData.filteredAddresses.map(address),
  };
}

export async function generateFinalizationData(overrides?: Partial<FinalizationData>): Promise<FinalizationData> {
  const { networkHelpers } = await getConnection();
  const latest = await networkHelpers.time.latest();
  return {
    aggregatedProof: generateRandomBytes(928),
    endBlockNumber: 99n,
    shnarfData: generateParentShnarfData(1),
    parentStateRootHash: generateRandomBytes(32),
    lastFinalizedTimestamp: BigInt(latest - 2),
    finalTimestamp: BigInt(latest),
    l1RollingHash: generateRandomBytes(32),
    l1RollingHashMessageNumber: 10n,
    l2MerkleRoots: [generateRandomBytes(32)],
    filteredAddresses: [],
    l2MerkleTreesDepth: 5n,
    l2MessagingBlocksOffsets: generateL2MessagingBlocksOffsets(1, 1),
    lastFinalizedL1RollingHash: HASH_ZERO,
    lastFinalizedL1RollingHashMessageNumber: 0n,
    lastFinalizedForcedTransactionNumber: 0n,
    finalForcedTransactionNumber: 0n,
    lastFinalizedForcedTransactionRollingHash: HASH_ZERO,
    ...overrides,
  };
}

export function generateCallDataSubmission(startDataIndex: number, finalDataIndex: number): CalldataSubmissionData[] {
  return COMPRESSED_SUBMISSION_DATA.slice(startDataIndex, finalDataIndex).map((data) => {
    const returnData = {
      finalStateRootHash: hex(data.finalStateRootHash),
      snarkHash: hex(data.snarkHash),
      compressedData: base64ToHex(data.compressedData),
    };
    return returnData;
  });
}

export function generateBlobDataSubmission(
  startDataIndex: number,
  finalDataIndex: number,
  isMultiple: boolean = false,
): {
  blobDataSubmission: BlobSubmission[];
  compressedBlobs: Hex[];
  parentShnarf: Hex;
  finalShnarf: Hex;
} {
  const dataSet = isMultiple ? BLOB_SUBMISSION_DATA_MULTIPLE_PROOF : BLOB_SUBMISSION_DATA;
  const compressedBlobs: Hex[] = [];
  const parentShnarf = hex(dataSet[startDataIndex].prevShnarf);
  const finalShnarf = hex(dataSet[finalDataIndex - 1].expectedShnarf);

  const blobDataSubmission = dataSet.slice(startDataIndex, finalDataIndex).map((data) => {
    compressedBlobs.push(base64ToHex(data.compressedData));
    const returnData: BlobSubmission = {
      dataEvaluationClaim: hex(data.expectedY),
      kzgCommitment: hex(data.commitment),
      kzgProof: hex(data.kzgProofContract),
      finalStateRootHash: hex(data.finalStateRootHash),
      snarkHash: hex(data.snarkHash),
    };
    return returnData;
  });
  return {
    compressedBlobs,
    blobDataSubmission,
    parentShnarf,
    finalShnarf,
  };
}

export function generateBlobDataSubmissionFromFile(filePath: string): {
  blobDataSubmission: BlobSubmission[];
  compressedBlobs: Hex[];
  parentShnarf: Hex;
  finalShnarf: Hex;
} {
  const fileContents = JSON.parse(fs.readFileSync(filePath, "utf-8"));

  const compressedBlobs: Hex[] = [];
  const parentShnarf = hex(fileContents.prevShnarf);
  const finalShnarf = hex(fileContents.expectedShnarf);

  compressedBlobs.push(base64ToHex(fileContents.compressedData));

  const blobDataSubmission = [
    {
      dataEvaluationClaim: hex(fileContents.expectedY),
      kzgCommitment: hex(fileContents.commitment),
      kzgProof: hex(fileContents.kzgProofContract),
      finalStateRootHash: hex(fileContents.finalStateRootHash),
      snarkHash: hex(fileContents.snarkHash),
    },
  ];

  return {
    compressedBlobs,
    blobDataSubmission,
    parentShnarf,
    finalShnarf,
  };
}

/**
 * Mirrors the Solidity _computeShnarf: keccak256(abi.encodePacked(parentShnarf, snarkHash, finalStateRootHash, dataEvaluationPoint, dataEvaluationClaim)).
 */
export function computeShnarf(shnarfData: ShnarfData): Hex {
  return keccak256(
    concat([
      shnarfData.parentShnarf,
      shnarfData.snarkHash,
      shnarfData.finalStateRootHash,
      shnarfData.dataEvaluationPoint,
      shnarfData.dataEvaluationClaim,
    ]),
  );
}

/**
 * Computes the genesis shnarf the same way the contract does during initialization.
 */
export function computeGenesisShnarf(initialStateRootHash: Hex): Hex {
  return computeShnarf({
    parentShnarf: HASH_ZERO,
    snarkHash: HASH_ZERO,
    finalStateRootHash: initialStateRootHash,
    dataEvaluationPoint: HASH_ZERO,
    dataEvaluationClaim: HASH_ZERO,
  });
}

export function generateParentShnarfData(index: number, multiple?: boolean): ShnarfData {
  if (index === 0) {
    return {
      parentShnarf: HASH_ZERO,
      snarkHash: HASH_ZERO,
      finalStateRootHash: multiple
        ? hex(COMPRESSED_SUBMISSION_DATA_MULTIPLE_PROOF[0].parentStateRootHash)
        : hex(COMPRESSED_SUBMISSION_DATA[0].parentStateRootHash),
      dataEvaluationPoint: HASH_ZERO,
      dataEvaluationClaim: HASH_ZERO,
    };
  }
  const parentSubmissionData = multiple
    ? COMPRESSED_SUBMISSION_DATA_MULTIPLE_PROOF[index - 1]
    : COMPRESSED_SUBMISSION_DATA[index - 1];

  return {
    parentShnarf: hex(parentSubmissionData.prevShnarf),
    snarkHash: hex(parentSubmissionData.snarkHash),
    finalStateRootHash: hex(parentSubmissionData.finalStateRootHash),
    dataEvaluationPoint: hex(parentSubmissionData.expectedX),
    dataEvaluationClaim: hex(parentSubmissionData.expectedY),
  };
}

export function generateBlobParentShnarfData(index: number, multiple?: boolean): ShnarfData {
  if (index === 0) {
    return {
      parentShnarf: HASH_ZERO,
      snarkHash: HASH_ZERO,
      finalStateRootHash: multiple
        ? hex(BLOB_SUBMISSION_DATA_MULTIPLE_PROOF[0].parentStateRootHash)
        : hex(BLOB_SUBMISSION_DATA[0].parentStateRootHash),
      dataEvaluationPoint: HASH_ZERO,
      dataEvaluationClaim: HASH_ZERO,
    };
  }
  const parentSubmissionData = multiple
    ? BLOB_SUBMISSION_DATA_MULTIPLE_PROOF[index - 1]
    : BLOB_SUBMISSION_DATA[index - 1];

  return {
    parentShnarf: hex(parentSubmissionData.prevShnarf),
    snarkHash: hex(parentSubmissionData.snarkHash),
    finalStateRootHash: hex(parentSubmissionData.finalStateRootHash),
    dataEvaluationPoint: hex(parentSubmissionData.expectedX),
    dataEvaluationClaim: hex(parentSubmissionData.expectedY),
  };
}

export function generateParentAndExpectedShnarfForIndex(index: number): ParentAndExpectedShnarf {
  return {
    parentShnarf: hex(COMPRESSED_SUBMISSION_DATA[index].prevShnarf),
    expectedShnarf: hex(COMPRESSED_SUBMISSION_DATA[index].expectedShnarf),
  };
}

export function generateParentAndExpectedShnarfForMulitpleIndex(index: number): ParentAndExpectedShnarf {
  return {
    parentShnarf: hex(COMPRESSED_SUBMISSION_DATA_MULTIPLE_PROOF[index].prevShnarf),
    expectedShnarf: hex(COMPRESSED_SUBMISSION_DATA_MULTIPLE_PROOF[index].expectedShnarf),
  };
}

export function generateCallDataSubmissionMultipleProofs(
  startDataIndex: number,
  finalDataIndex: number,
): CalldataSubmissionData[] {
  return COMPRESSED_SUBMISSION_DATA_MULTIPLE_PROOF.slice(startDataIndex, finalDataIndex).map((data) => {
    const returnData = {
      parentStateRootHash: hex(data.parentStateRootHash),
      dataParentHash: hex(data.parentDataHash),
      finalStateRootHash: hex(data.finalStateRootHash),
      firstBlockNumber: BigInt(data.conflationOrder.startingBlockNumber),
      endBlockNumber: BigInt(data.conflationOrder.upperBoundaries.slice(-1)[0]),
      snarkHash: hex(data.snarkHash),
      compressedData: base64ToHex(data.compressedData),
      parentShnarf: hex(data.prevShnarf),
    };
    return returnData;
  });
}

/**
 * Configuration for submission data setup helper.
 */
export interface SubmissionSetupConfig {
  /** Starting index for submission data generation */
  startIndex: number;
  /** Final index for submission data generation */
  finalIndex: number;
  /** Whether to use multiple proof data */
  useMultipleProofs?: boolean;
  /** Maximum gas limit for transactions */
  maxGasLimit: number | bigint;
}

/**
 * Result from submission setup helper.
 */
export interface SubmissionSetupResult {
  /** Final index after submission */
  finalIndex: number;
  /** Number of submissions made */
  submissionCount: number;
}

/**
 * A LinethRollup-like contract (viem instance) that accepts calldata submissions.
 */
export interface CalldataSubmitter {
  write: {
    submitDataAsCalldata(
      args: readonly [CalldataSubmissionData, Hex, Hex],
      options: { account: Account; gas: bigint },
    ): Promise<Hash>;
  };
}

/**
 * Helper to submit calldata before finalization tests.
 * Encapsulates the repeated pattern of generating and submitting calldata in a loop.
 *
 * @param linethRollup - The LinethRollup contract instance
 * @param operator - The account submitting the data
 * @param config - Configuration for the submission
 * @returns Final index for use in subsequent operations
 *
 * @example
 * ```typescript
 * const { finalIndex } = await submitCalldataBeforeFinalization(
 *   linethRollup,
 *   operator,
 *   { startIndex: 0, finalIndex: 4, maxGasLimit: MAX_GAS_LIMIT }
 * );
 * // Use finalIndex for generateParentShnarfData(finalIndex)
 * ```
 */
export async function submitCalldataBeforeFinalization(
  linethRollup: CalldataSubmitter,
  operator: { account: Account },
  config: SubmissionSetupConfig,
): Promise<SubmissionSetupResult> {
  const { startIndex, finalIndex, useMultipleProofs = false, maxGasLimit } = config;

  const submissionData = useMultipleProofs
    ? generateCallDataSubmissionMultipleProofs(startIndex, finalIndex)
    : generateCallDataSubmission(startIndex, finalIndex);

  const getShnarfFn = useMultipleProofs
    ? generateParentAndExpectedShnarfForMulitpleIndex
    : generateParentAndExpectedShnarfForIndex;

  let index = startIndex;
  for (const data of submissionData) {
    const parentAndExpectedShnarf = getShnarfFn(index);
    await linethRollup.write.submitDataAsCalldata(
      [data, parentAndExpectedShnarf.parentShnarf, parentAndExpectedShnarf.expectedShnarf],
      { account: operator.account, gas: BigInt(maxGasLimit) },
    );
    index++;
  }

  return {
    finalIndex: index,
    submissionCount: submissionData.length,
  };
}

function base64ToHex(base64: string): Hex {
  return bytesToHex(Buffer.from(base64, "base64"));
}
