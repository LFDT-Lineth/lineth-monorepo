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
  BlobSubmission,
  AggregatedProofData,
  ParentAndExpectedDataRollingHash,
  ShnarfDataGenerator,
} from "../types";
import { generateRandomBytes, range } from "./general";

export const generateL2MessagingBlocksOffsets = (start: number, end: number) =>
  `0x${range(start, end)
    .map((num) => ethers.solidityPacked(["uint16"], [num]).slice(2))
    .join("")}`;

/**
 * Context for generating finalization parameters from proof data.
 */
export type ProofFinalizationContext = {
  proofData: AggregatedProofData;
  shnarfDataGenerator: ShnarfDataGenerator;
  blobParentShnarfIndex: number;
  isMultiple: boolean;
  /**
   * Stream position of the previously-finalized range, when this finalization is the second of two
   * chained ranges. When omitted the range is treated as a fresh start opening the genesis position.
   */
  base?: StreamPosition;
};

/**
 * Shape of the pre-generated JSON compressed-data/blob fixtures.
 * @dev Fixtures predate the dataRollingHash ABI cutover and were generated with real
 *   `parentStateRootHash` / `finalStateRootHash` values (no block hashes). Since submission/finalization
 *   tests only need distinct, consistently round-tripped bytes32 values (not real block-hash semantics),
 *   these hash fields are reused as `initialBlockHash` / `finalBlockHash` inputs below rather than
 *   regenerating fixtures. The legacy 5-field/Horner-Y shnarf fields (`snarkHash`, `expectedX`,
 *   `expectedY`, `expectedShnarf`) are gone with the 3-input shnarf scheme.
 */
type FixtureSubmission = {
  parentStateRootHash: string;
  finalStateRootHash: string;
  compressedData: string;
  commitment?: string;
  prevShnarf?: string;
};

/** A stream position: the dataRollingHash accumulator plus the byte offset within its last chunk. */
export type StreamPosition = {
  dataRollingHash: string;
  offset: bigint;
};

export type ComputedCalldataSubmission = CalldataSubmissionData & {
  parentDataRollingHash: string;
  expectedDataRollingHash: string;
};

export type ComputedBlobSubmission = BlobSubmission & {
  parentDataRollingHash: string;
  expectedDataRollingHash: string;
};

/**
 * Mirrors the Solidity 2-input `_computeDataRollingHash`:
 * keccak256(abi.encodePacked(parentDataRollingHash, chunkHash)).
 */
export function computeDataRollingHash(parentDataRollingHash: string, chunkHash: string): string {
  return ethers.keccak256(ethers.concat([parentDataRollingHash, chunkHash]));
}

/**
 * Mirrors the Solidity `_computePositionCommitment`:
 * keccak256(abi.encodePacked(dataRollingHash, offset-as-uint256)).
 */
export function computePositionCommitment(dataRollingHash: string, offset: bigint | number): string {
  return ethers.solidityPackedKeccak256(["bytes32", "uint256"], [dataRollingHash, offset]);
}

/**
 * EIP-4844 versioned blob hash from a KZG commitment: 0x01 || sha256(commitment)[1:].
 */
export function computeBlobVersionedHash(commitment: string): string {
  return `0x01${ethers.sha256(commitment).slice(4)}`;
}

function buildCalldataChain(
  dataSet: FixtureSubmission[],
  startDataIndex: number,
  finalDataIndex: number,
): ComputedCalldataSubmission[] {
  // Genesis accumulator is the empty hash; each chunk folds keccak256(compressedData).
  let parentDataRollingHash = HASH_ZERO;

  const chain: ComputedCalldataSubmission[] = [];
  for (let i = 0; i < finalDataIndex; i++) {
    const data = dataSet[i];
    const compressedData = ethers.hexlify(ethers.decodeBase64(data.compressedData));
    const chunkHash = ethers.keccak256(compressedData);
    const expectedDataRollingHash = computeDataRollingHash(parentDataRollingHash, chunkHash);
    if (i >= startDataIndex) {
      chain.push({ compressedData, parentDataRollingHash, expectedDataRollingHash });
    }
    parentDataRollingHash = expectedDataRollingHash;
  }
  return chain;
}

function buildBlobChain(
  dataSet: FixtureSubmission[],
  startDataIndex: number,
  finalDataIndex: number,
): ComputedBlobSubmission[] {
  // Genesis accumulator is the empty hash; each blob folds its versioned blobhash.
  let parentDataRollingHash = HASH_ZERO;

  const chain: ComputedBlobSubmission[] = [];
  for (let i = 0; i < finalDataIndex; i++) {
    const data = dataSet[i];
    const compressedData = ethers.hexlify(ethers.decodeBase64(data.compressedData));
    const dataHash = computeBlobVersionedHash(data.commitment!);
    const expectedDataRollingHash = computeDataRollingHash(parentDataRollingHash, dataHash);
    if (i >= startDataIndex) {
      chain.push({
        dataHash,
        compressedData,
        kzgCommitment: data.commitment!,
        parentDataRollingHash,
        expectedDataRollingHash,
      });
    }
    parentDataRollingHash = expectedDataRollingHash;
  }
  return chain;
}

export function generateCallDataSubmission(startDataIndex: number, finalDataIndex: number): CalldataSubmissionData[] {
  return buildCalldataChain(COMPRESSED_SUBMISSION_DATA, startDataIndex, finalDataIndex).map(({ compressedData }) => ({
    compressedData,
  }));
}

export function generateCallDataSubmissionWithHashes(
  startDataIndex: number,
  finalDataIndex: number,
): ComputedCalldataSubmission[] {
  return buildCalldataChain(COMPRESSED_SUBMISSION_DATA, startDataIndex, finalDataIndex);
}

export function generateBlobDataSubmission(
  startDataIndex: number,
  finalDataIndex: number,
  isMultiple: boolean = false,
): {
  blobDataSubmission: BlobSubmission[];
  compressedBlobs: string[];
  parentDataRollingHash: string;
  finalDataRollingHash: string;
} {
  const dataSet = isMultiple ? BLOB_SUBMISSION_DATA_MULTIPLE_PROOF : BLOB_SUBMISSION_DATA;
  const chain = buildBlobChain(dataSet, startDataIndex, finalDataIndex);
  return {
    blobDataSubmission: chain.map(({ dataHash, compressedData, kzgCommitment }) => ({
      dataHash,
      compressedData,
      kzgCommitment: kzgCommitment!,
    })),
    compressedBlobs: chain.map((item) => item.compressedData),
    parentDataRollingHash: chain[0].parentDataRollingHash,
    finalDataRollingHash: chain[chain.length - 1].expectedDataRollingHash,
  };
}

export function generateBlobDataSubmissionFromFile(filePath: string): {
  blobDataSubmission: BlobSubmission[];
  compressedBlobs: string[];
  parentDataRollingHash: string;
  finalDataRollingHash: string;
} {
  const fileContents = JSON.parse(fs.readFileSync(filePath, "utf-8")) as FixtureSubmission;
  const compressedData = ethers.hexlify(ethers.decodeBase64(fileContents.compressedData));
  const dataHash = computeBlobVersionedHash(fileContents.commitment!);
  // Single-file helpers are used after a known parent; fall back to fixture prevShnarf when present,
  // otherwise treat parent as the genesis accumulator (empty hash).
  const parentDataRollingHash = fileContents.prevShnarf ?? HASH_ZERO;
  const finalDataRollingHash = computeDataRollingHash(parentDataRollingHash, dataHash);

  return {
    compressedBlobs: [compressedData],
    blobDataSubmission: [
      {
        dataHash,
        compressedData,
        kzgCommitment: fileContents.commitment!,
      },
    ],
    parentDataRollingHash,
    finalDataRollingHash,
  };
}

function emptyStreamPosition(parentDataRollingHash: string): ParentAndExpectedDataRollingHash {
  return { parentDataRollingHash, expectedDataRollingHash: parentDataRollingHash };
}

/**
 * Returns the parent dataRollingHash that precedes chunk `index` (i.e. the accumulator after folding
 * chunks `[0, index)`), used to seed a finalization's stream position.
 */
export function generateParentDataRollingHash(index: number, multiple?: boolean): ParentAndExpectedDataRollingHash {
  if (index === 0) {
    return emptyStreamPosition(HASH_ZERO);
  }
  const dataSet = multiple ? COMPRESSED_SUBMISSION_DATA_MULTIPLE_PROOF : COMPRESSED_SUBMISSION_DATA;
  const chain = buildCalldataChain(dataSet, index - 1, index);
  return {
    parentDataRollingHash: chain[0].parentDataRollingHash,
    expectedDataRollingHash: chain[0].expectedDataRollingHash,
  };
}

export function generateBlobParentDataRollingHash(index: number, multiple?: boolean): ParentAndExpectedDataRollingHash {
  if (index === 0) {
    return emptyStreamPosition(HASH_ZERO);
  }
  const dataSet = multiple ? BLOB_SUBMISSION_DATA_MULTIPLE_PROOF : BLOB_SUBMISSION_DATA;
  const chain = buildBlobChain(dataSet, index - 1, index);
  return {
    parentDataRollingHash: chain[0].parentDataRollingHash,
    expectedDataRollingHash: chain[0].expectedDataRollingHash,
  };
}

/**
 * Stream position anchors for a finalization that ends after the submission range `[0, blobParentShnarfIndex)`.
 * `finalDataRollingHash` is the accumulator after folding every chunk in the range (the value that must have
 * been anchored by the last submission); offsets are chunk-boundary positions (0 == fresh-start sentinel).
 */
export function getFinalizationStreamPosition(
  blobParentShnarfIndex: number,
  options: { isMultiple?: boolean; isBlob?: boolean; base?: StreamPosition } = {},
): { prevDataRollingHash: string; parentDataRollingHash: string; endDataRollingHash: string } | undefined {
  const { isMultiple = false, isBlob = false, base } = options;
  if (blobParentShnarfIndex < 0) {
    return undefined;
  }

  // Fresh start (first finalization): open the genesis position commitment with the empty accumulator.
  if (!base) {
    const gen = isBlob ? generateBlobParentDataRollingHash : generateParentDataRollingHash;
    const { expectedDataRollingHash } = gen(blobParentShnarfIndex, isMultiple);
    return {
      prevDataRollingHash: HASH_ZERO,
      parentDataRollingHash: HASH_ZERO,
      endDataRollingHash: expectedDataRollingHash,
    };
  }

  // Second finalization: open the position stored by the first finalize. The fixtures submit a single
  // continuous chunk chain `[0, N)` and the second range's tail index points past the last fixture chunk,
  // so no new chunks are folded — the end accumulator equals the first range's end (the chain tail).
  return {
    prevDataRollingHash: base.dataRollingHash,
    parentDataRollingHash: base.dataRollingHash,
    endDataRollingHash: base.dataRollingHash,
  };
}

export function generateParentAndExpectedDataRollingHashForIndex(index: number): ParentAndExpectedDataRollingHash {
  const chain = buildCalldataChain(COMPRESSED_SUBMISSION_DATA, index, index + 1);
  return {
    parentDataRollingHash: chain[0].parentDataRollingHash,
    expectedDataRollingHash: chain[0].expectedDataRollingHash,
  };
}

export function generateParentAndExpectedDataRollingHashForMultipleIndex(
  index: number,
): ParentAndExpectedDataRollingHash {
  const chain = buildCalldataChain(COMPRESSED_SUBMISSION_DATA_MULTIPLE_PROOF, index, index + 1);
  return {
    parentDataRollingHash: chain[0].parentDataRollingHash,
    expectedDataRollingHash: chain[0].expectedDataRollingHash,
  };
}

export function generateCallDataSubmissionMultipleProofs(
  startDataIndex: number,
  finalDataIndex: number,
): CalldataSubmissionData[] {
  return buildCalldataChain(COMPRESSED_SUBMISSION_DATA_MULTIPLE_PROOF, startDataIndex, finalDataIndex).map(
    ({ compressedData }) => ({ compressedData }),
  );
}

/**
 * Converts AggregatedProofData to finalization data parameters for a fresh-start (offset 0) range that
 * spans the entire submission chain `[0, blobParentShnarfIndex)`.
 */
export function proofDataToFinalizationParams(context: ProofFinalizationContext): Partial<FinalizationData> {
  const { proofData, shnarfDataGenerator, blobParentShnarfIndex, isMultiple, base } = context;
  const isBlob = shnarfDataGenerator === generateBlobParentDataRollingHash;
  const stream = getFinalizationStreamPosition(blobParentShnarfIndex, { isMultiple, isBlob, base });

  return {
    l1RollingHash: proofData.l1RollingHash,
    l1RollingHashMessageNumber: BigInt(proofData.l1RollingHashMessageNumber),
    lastFinalizedTimestamp: BigInt(proofData.parentAggregationLastBlockTimestamp),
    endBlockNumber: BigInt(proofData.finalBlockNumber),
    parentStateRootHash: proofData.parentStateRootHash,
    // Fresh deploy seeds blockHashes[initial]; soft continuity expects that parent on the new path.
    parentBlockHash: proofData.parentStateRootHash,
    finalTimestamp: BigInt(proofData.finalTimestamp),
    l2MerkleRoots: proofData.l2MerkleRoots,
    l2MerkleTreesDepth: BigInt(proofData.l2MerkleTreesDepth),
    l2MessagingBlocksOffsets: proofData.l2MessagingBlocksOffsets,
    aggregatedProof: proofData.aggregatedProof,
    lastFinalizedL1RollingHash: proofData.lastFinalizedL1RollingHash,
    lastFinalizedL1RollingHashMessageNumber: BigInt(proofData.lastFinalizedL1RollingHashMessageNumber),
    lastFinalizedForcedTransactionRollingHash: proofData.parentAggregationFtxRollingHash,
    lastFinalizedForcedTransactionNumber: BigInt(proofData.parentAggregationFtxNumber),
    finalForcedTransactionNumber: BigInt(proofData.finalFtxNumber),
    filteredAddresses: proofData.filteredAddresses,
    finalBlockHash: proofData.finalStateRootHash,
    // Fresh-start stream position: open the genesis position commitment (0 || 0) and span to the end.
    prevDataRollingHash: stream?.prevDataRollingHash ?? HASH_ZERO,
    prevOffset: 0n,
    parentDataRollingHash: stream?.parentDataRollingHash ?? HASH_ZERO,
    endDataRollingHash: stream?.endDataRollingHash ?? HASH_ZERO,
    startOffset: 0n,
    endOffset: 0n,
    verifierKeys: [],
  };
}

export async function generateFinalizationData(overrides?: Partial<FinalizationData>): Promise<FinalizationData> {
  return {
    aggregatedProof: generateRandomBytes(928),
    endBlockNumber: 99n,
    parentStateRootHash: generateRandomBytes(32),
    parentBlockHash: HASH_ZERO,
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
    finalBlockHash: generateRandomBytes(32),
    prevDataRollingHash: HASH_ZERO,
    prevOffset: 0n,
    parentDataRollingHash: HASH_ZERO,
    endDataRollingHash: HASH_ZERO,
    startOffset: 0n,
    endOffset: 0n,
    verifierKeys: [],
    ...overrides,
  };
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
 * @param linethRollup - The LinethRollup contract instance (connected to operator)
 * @param config - Configuration for the submission
 * @returns Final index for use in subsequent operations
 */
export async function submitCalldataBeforeFinalization(
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  linethRollup: any,
  config: SubmissionSetupConfig,
): Promise<SubmissionSetupResult> {
  const { startIndex, finalIndex, useMultipleProofs = false, maxGasLimit } = config;

  const submissionData = useMultipleProofs
    ? generateCallDataSubmissionMultipleProofs(startIndex, finalIndex)
    : generateCallDataSubmission(startIndex, finalIndex);

  const getHashesFn = useMultipleProofs
    ? generateParentAndExpectedDataRollingHashForMultipleIndex
    : generateParentAndExpectedDataRollingHashForIndex;

  let index = startIndex;
  for (const data of submissionData) {
    const parentAndExpected = getHashesFn(index);
    await linethRollup.submitDataAsCalldata(
      data.compressedData,
      parentAndExpected.parentDataRollingHash,
      parentAndExpected.expectedDataRollingHash,
      { gasLimit: maxGasLimit },
    );
    index++;
  }

  return {
    finalIndex: index,
    submissionCount: submissionData.length,
  };
}
