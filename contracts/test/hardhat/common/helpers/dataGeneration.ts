import { time as networkTime } from "@nomicfoundation/hardhat-network-helpers";
import { ethers } from "ethers";
import * as fs from "fs";

import {
  HASH_ZERO,
  COMPRESSED_SUBMISSION_DATA,
  COMPRESSED_SUBMISSION_DATA_MULTIPLE_PROOF,
  BLOB_SUBMISSION_DATA,
  BLOB_SUBMISSION_DATA_MULTIPLE_PROOF,
} from "../constants";
import {
  FinalizationData,
  CalldataSubmissionData,
  ShnarfData,
  ParentAndExpectedShnarf,
  BlobSubmission,
  AggregatedProofData,
  ShnarfDataGenerator,
} from "../types";
import { generateRandomBytes, range } from "./general";

export const generateL2MessagingBlocksOffsets = (start: number, end: number) =>
  `0x${range(start, end)
    .map((num) => ethers.solidityPacked(["uint16"], [num]).slice(2))
    .join("")}`;

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
    l1RollingHash: proofData.l1RollingHash,
    l1RollingHashMessageNumber: BigInt(proofData.l1RollingHashMessageNumber),
    lastFinalizedTimestamp: BigInt(proofData.parentAggregationLastBlockTimestamp),
    endBlockNumber: BigInt(proofData.finalBlockNumber),
    parentStateRootHash: proofData.parentStateRootHash,
    finalTimestamp: BigInt(proofData.finalTimestamp),
    l2MerkleRoots: proofData.l2MerkleRoots,
    l2MerkleTreesDepth: BigInt(proofData.l2MerkleTreesDepth),
    l2MessagingBlocksOffsets: proofData.l2MessagingBlocksOffsets,
    aggregatedProof: proofData.aggregatedProof,
    shnarfData: shnarfDataGenerator(blobParentShnarfIndex, isMultiple),
    lastFinalizedL1RollingHash: proofData.lastFinalizedL1RollingHash,
    lastFinalizedL1RollingHashMessageNumber: BigInt(proofData.lastFinalizedL1RollingHashMessageNumber),
    lastFinalizedForcedTransactionRollingHash: proofData.parentAggregationFtxRollingHash,
    lastFinalizedForcedTransactionNumber: BigInt(proofData.parentAggregationFtxNumber),
    finalForcedTransactionNumber: BigInt(proofData.finalFtxNumber),
    filteredAddresses: proofData.filteredAddresses,
  };
}

export async function generateFinalizationData(overrides?: Partial<FinalizationData>): Promise<FinalizationData> {
  return {
    aggregatedProof: generateRandomBytes(928),
    endBlockNumber: 99n,
    shnarfData: generateParentShnarfData(1),
    parentStateRootHash: generateRandomBytes(32),
    lastFinalizedTimestamp: BigInt((await networkTime.latest()) - 2),
    finalTimestamp: BigInt(await networkTime.latest()),
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
      finalStateRootHash: data.finalStateRootHash,
      snarkHash: data.snarkHash,
      compressedData: ethers.hexlify(ethers.decodeBase64(data.compressedData)),
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
  compressedBlobs: string[];
  parentShnarf: string;
  finalShnarf: string;
} {
  const dataSet = isMultiple ? BLOB_SUBMISSION_DATA_MULTIPLE_PROOF : BLOB_SUBMISSION_DATA;
  const compressedBlobs: string[] = [];
  const parentShnarf = dataSet[startDataIndex].prevShnarf;
  const finalShnarf = dataSet[finalDataIndex - 1].expectedShnarf;

  const blobDataSubmission = dataSet.slice(startDataIndex, finalDataIndex).map((data) => {
    compressedBlobs.push(ethers.hexlify(ethers.decodeBase64(data.compressedData)));
    const returnData: BlobSubmission = {
      dataEvaluationClaim: data.expectedY,
      kzgCommitment: data.commitment,
      kzgProof: data.kzgProofContract,
      finalStateRootHash: data.finalStateRootHash,
      snarkHash: data.snarkHash,
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
  compressedBlobs: string[];
  parentShnarf: string;
  finalShnarf: string;
} {
  const fileContents = JSON.parse(fs.readFileSync(filePath, "utf-8"));

  const compressedBlobs: string[] = [];
  const parentShnarf = fileContents.prevShnarf;
  const finalShnarf = fileContents.expectedShnarf;

  compressedBlobs.push(ethers.hexlify(ethers.decodeBase64(fileContents.compressedData)));

  const blobDataSubmission = [
    {
      dataEvaluationClaim: fileContents.expectedY,
      kzgCommitment: fileContents.commitment,
      kzgProof: fileContents.kzgProofContract,
      finalStateRootHash: fileContents.finalStateRootHash,
      snarkHash: fileContents.snarkHash,
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
export function computeShnarf(shnarfData: ShnarfData): string {
  return ethers.keccak256(
    ethers.concat([
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
export function computeGenesisShnarf(initialStateRootHash: string): string {
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
        ? COMPRESSED_SUBMISSION_DATA_MULTIPLE_PROOF[0].parentStateRootHash
        : COMPRESSED_SUBMISSION_DATA[0].parentStateRootHash,
      dataEvaluationPoint: HASH_ZERO,
      dataEvaluationClaim: HASH_ZERO,
    };
  }
  const parentSubmissionData = multiple
    ? COMPRESSED_SUBMISSION_DATA_MULTIPLE_PROOF[index - 1]
    : COMPRESSED_SUBMISSION_DATA[index - 1];

  return {
    parentShnarf: parentSubmissionData.prevShnarf,
    snarkHash: parentSubmissionData.snarkHash,
    finalStateRootHash: parentSubmissionData.finalStateRootHash,
    dataEvaluationPoint: parentSubmissionData.expectedX,
    dataEvaluationClaim: parentSubmissionData.expectedY,
  };
}

export function generateBlobParentShnarfData(index: number, multiple?: boolean): ShnarfData {
  if (index === 0) {
    return {
      parentShnarf: HASH_ZERO,
      snarkHash: HASH_ZERO,
      finalStateRootHash: multiple
        ? BLOB_SUBMISSION_DATA_MULTIPLE_PROOF[0].parentStateRootHash
        : BLOB_SUBMISSION_DATA[0].parentStateRootHash,
      dataEvaluationPoint: HASH_ZERO,
      dataEvaluationClaim: HASH_ZERO,
    };
  }
  const parentSubmissionData = multiple
    ? BLOB_SUBMISSION_DATA_MULTIPLE_PROOF[index - 1]
    : BLOB_SUBMISSION_DATA[index - 1];

  return {
    parentShnarf: parentSubmissionData.prevShnarf,
    snarkHash: parentSubmissionData.snarkHash,
    finalStateRootHash: parentSubmissionData.finalStateRootHash,
    dataEvaluationPoint: parentSubmissionData.expectedX,
    dataEvaluationClaim: parentSubmissionData.expectedY,
  };
}

export function generateParentAndExpectedShnarfForIndex(index: number): ParentAndExpectedShnarf {
  return {
    parentShnarf: COMPRESSED_SUBMISSION_DATA[index].prevShnarf,
    expectedShnarf: COMPRESSED_SUBMISSION_DATA[index].expectedShnarf,
  };
}

export function generateParentAndExpectedShnarfForMulitpleIndex(index: number): ParentAndExpectedShnarf {
  return {
    parentShnarf: COMPRESSED_SUBMISSION_DATA_MULTIPLE_PROOF[index].prevShnarf,
    expectedShnarf: COMPRESSED_SUBMISSION_DATA_MULTIPLE_PROOF[index].expectedShnarf,
  };
}

export function generateCallDataSubmissionMultipleProofs(
  startDataIndex: number,
  finalDataIndex: number,
): CalldataSubmissionData[] {
  return COMPRESSED_SUBMISSION_DATA_MULTIPLE_PROOF.slice(startDataIndex, finalDataIndex).map((data) => {
    const returnData = {
      parentStateRootHash: data.parentStateRootHash,
      dataParentHash: data.parentDataHash,
      finalStateRootHash: data.finalStateRootHash,
      firstBlockNumber: BigInt(data.conflationOrder.startingBlockNumber),
      endBlockNumber: BigInt(data.conflationOrder.upperBoundaries.slice(-1)[0]),
      snarkHash: data.snarkHash,
      compressedData: ethers.hexlify(ethers.decodeBase64(data.compressedData)),
      parentShnarf: data.prevShnarf,
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
 * Helper to submit calldata before finalization tests.
 * Encapsulates the repeated pattern of generating and submitting calldata in a loop.
 *
 * @param lineaRollup - The LineaRollup contract instance (connected to operator)
 * @param config - Configuration for the submission
 * @returns Final index for use in subsequent operations
 *
 * @example
 * ```typescript
 * const { finalIndex } = await submitCalldataBeforeFinalization(
 *   lineaRollup.connect(operator),
 *   { startIndex: 0, finalIndex: 4, maxGasLimit: MAX_GAS_LIMIT }
 * );
 * // Use finalIndex for generateParentShnarfData(finalIndex)
 * ```
 */
export async function submitCalldataBeforeFinalization(
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  lineaRollup: any,
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
    await lineaRollup.submitDataAsCalldata(
      data,
      parentAndExpectedShnarf.parentShnarf,
      parentAndExpectedShnarf.expectedShnarf,
      { gasLimit: maxGasLimit },
    );
    index++;
  }

  return {
    finalIndex: index,
    submissionCount: submissionData.length,
  };
}
