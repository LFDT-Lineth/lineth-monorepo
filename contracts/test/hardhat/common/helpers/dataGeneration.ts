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
 * Shape of the pre-generated JSON compressed-data/blob fixtures.
 * @dev Fixtures predate the blockhash-centric ABI cutover and were generated with real
 *   `parentStateRootHash` / `finalStateRootHash` values (no block hashes). Since submission/finalization
 *   tests only need distinct, consistently round-tripped bytes32 values (not real block-hash semantics),
 *   these hash fields are reused as `initialBlockHash` / `finalBlockHash` inputs below rather than
 *   regenerating fixtures. Fields specific to the legacy 5-field/Horner-Y shnarf scheme (`snarkHash`,
 *   `expectedX`, `expectedY`, `expectedShnarf`) are no longer read and are intentionally omitted here.
 */
type FixtureSubmission = {
  parentStateRootHash: string;
  finalStateRootHash: string;
  compressedData: string;
  commitment?: string;
  prevShnarf?: string;
};

export type ComputedCalldataSubmission = CalldataSubmissionData & {
  parentShnarf: string;
  expectedShnarf: string;
  dataHash: string;
};

export type ComputedBlobSubmission = BlobSubmission & {
  parentShnarf: string;
  expectedShnarf: string;
};

/**
 * Converts AggregatedProofData to finalization data parameters.
 * This consolidates the repeated mapping pattern used across finalization tests.
 */
export function proofDataToFinalizationParams(context: ProofFinalizationContext): Partial<FinalizationData> {
  const { proofData, shnarfDataGenerator, blobParentShnarfIndex, isMultiple } = context;
  const shnarfData = shnarfDataGenerator(blobParentShnarfIndex, isMultiple);
  const isBlob = shnarfDataGenerator === generateBlobParentShnarfData;
  const anchors = getFinalizationAnchors(blobParentShnarfIndex, { isMultiple, isBlob });

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
    shnarfData,
    lastFinalizedL1RollingHash: proofData.lastFinalizedL1RollingHash,
    lastFinalizedL1RollingHashMessageNumber: BigInt(proofData.lastFinalizedL1RollingHashMessageNumber),
    lastFinalizedForcedTransactionRollingHash: proofData.parentAggregationFtxRollingHash,
    lastFinalizedForcedTransactionNumber: BigInt(proofData.parentAggregationFtxNumber),
    finalForcedTransactionNumber: BigInt(proofData.finalFtxNumber),
    filteredAddresses: proofData.filteredAddresses,
    finalBlockHash: anchors?.finalBlockHash ?? proofData.finalStateRootHash,
    finalBlobHash: anchors?.finalBlobHash ?? HASH_ZERO,
    verifierKeys: [],
  };
}

export async function generateFinalizationData(overrides?: Partial<FinalizationData>): Promise<FinalizationData> {
  return {
    aggregatedProof: generateRandomBytes(928),
    endBlockNumber: 99n,
    shnarfData: generateParentShnarfData(1),
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
    finalBlobHash: generateRandomBytes(32),
    verifierKeys: [],
    ...overrides,
  };
}

/**
 * Mirrors the Solidity 3-field _computeShnarf:
 * keccak256(abi.encodePacked(parentShnarf, finalBlockHash, dataHash)).
 */
export function computeShnarfV2(parentShnarf: string, finalBlockHash: string, dataHash: string): string {
  return ethers.keccak256(ethers.concat([parentShnarf, finalBlockHash, dataHash]));
}

/**
 * Legacy 5-field shnarf (kept for migration negative tests).
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
export function computeGenesisShnarf(initialBlockHash: string): string {
  return computeShnarfV2(HASH_ZERO, initialBlockHash, HASH_ZERO);
}

/**
 * EIP-4844 versioned blob hash from a KZG commitment: 0x01 || sha256(commitment)[1:].
 */
export function computeBlobVersionedHash(commitment: string): string {
  return `0x01${ethers.sha256(commitment).slice(4)}`;
}

// See `FixtureSubmission` doc comment: reuses the fixture's state root hash as a stand-in block hash.
function getInitialBlockHash(dataSet: FixtureSubmission[]): string {
  return dataSet[0].parentStateRootHash;
}

function buildCalldataShnarfChain(
  dataSet: FixtureSubmission[],
  startDataIndex: number,
  finalDataIndex: number,
): ComputedCalldataSubmission[] {
  const initialBlockHash = getInitialBlockHash(dataSet);
  let parentShnarf = computeGenesisShnarf(initialBlockHash);

  const chain: ComputedCalldataSubmission[] = [];
  for (let i = 0; i < finalDataIndex; i++) {
    const data = dataSet[i];
    const compressedData = ethers.hexlify(ethers.decodeBase64(data.compressedData));
    // Fixture state root hash reused as the stand-in block hash (see `FixtureSubmission`).
    const blockHash = data.finalStateRootHash;
    const dataHash = ethers.keccak256(compressedData);
    const expectedShnarf = computeShnarfV2(parentShnarf, blockHash, dataHash);
    if (i >= startDataIndex) {
      chain.push({
        blockHash,
        compressedData,
        parentShnarf,
        expectedShnarf,
        dataHash,
      });
    }
    parentShnarf = expectedShnarf;
  }
  return chain;
}

function buildBlobShnarfChain(
  dataSet: FixtureSubmission[],
  startDataIndex: number,
  finalDataIndex: number,
): ComputedBlobSubmission[] {
  const initialBlockHash = getInitialBlockHash(dataSet);
  let parentShnarf = computeGenesisShnarf(initialBlockHash);

  const chain: ComputedBlobSubmission[] = [];
  for (let i = 0; i < finalDataIndex; i++) {
    const data = dataSet[i];
    const compressedData = ethers.hexlify(ethers.decodeBase64(data.compressedData));
    // Fixture state root hash reused as the stand-in block hash (see `FixtureSubmission`).
    const finalBlockHash = data.finalStateRootHash;
    const dataHash = computeBlobVersionedHash(data.commitment!);
    const expectedShnarf = computeShnarfV2(parentShnarf, finalBlockHash, dataHash);
    if (i >= startDataIndex) {
      chain.push({
        finalBlockHash,
        dataHash,
        compressedData,
        kzgCommitment: data.commitment!,
        parentShnarf,
        expectedShnarf,
      });
    }
    parentShnarf = expectedShnarf;
  }
  return chain;
}

export function generateCallDataSubmission(startDataIndex: number, finalDataIndex: number): CalldataSubmissionData[] {
  return buildCalldataShnarfChain(COMPRESSED_SUBMISSION_DATA, startDataIndex, finalDataIndex).map(
    ({ blockHash, compressedData }) => ({ blockHash, compressedData }),
  );
}

export function generateCallDataSubmissionWithShnarfs(
  startDataIndex: number,
  finalDataIndex: number,
): ComputedCalldataSubmission[] {
  return buildCalldataShnarfChain(COMPRESSED_SUBMISSION_DATA, startDataIndex, finalDataIndex);
}

export function generateBlobDataSubmission(
  startDataIndex: number,
  finalDataIndex: number,
  isMultiple: boolean = false,
): {
  blobDataSubmission: BlobSubmission[];
  blobFinalBlockHashes: string[];
  compressedBlobs: string[];
  parentShnarf: string;
  finalShnarf: string;
} {
  const dataSet = isMultiple ? BLOB_SUBMISSION_DATA_MULTIPLE_PROOF : BLOB_SUBMISSION_DATA;
  const chain = buildBlobShnarfChain(dataSet, startDataIndex, finalDataIndex);
  return {
    blobDataSubmission: chain.map(({ finalBlockHash, dataHash, compressedData, kzgCommitment }) => ({
      finalBlockHash,
      dataHash,
      compressedData,
      kzgCommitment: kzgCommitment!,
    })),
    blobFinalBlockHashes: chain.map((item) => item.finalBlockHash),
    compressedBlobs: chain.map((item) => item.compressedData),
    parentShnarf: chain[0].parentShnarf,
    finalShnarf: chain[chain.length - 1].expectedShnarf,
  };
}

export function generateBlobDataSubmissionFromFile(filePath: string): {
  blobDataSubmission: BlobSubmission[];
  blobFinalBlockHashes: string[];
  compressedBlobs: string[];
  parentShnarf: string;
  finalShnarf: string;
} {
  const fileContents = JSON.parse(fs.readFileSync(filePath, "utf-8")) as FixtureSubmission;
  const compressedData = ethers.hexlify(ethers.decodeBase64(fileContents.compressedData));
  // Fixture state root hash reused as the stand-in block hash (see `FixtureSubmission`).
  const finalBlockHash = fileContents.finalStateRootHash;
  const dataHash = computeBlobVersionedHash(fileContents.commitment!);
  // Single-file helpers are used after a known parent; fall back to fixture prevShnarf when present,
  // otherwise treat parent as genesis from parentStateRootHash.
  const parentShnarf = fileContents.prevShnarf ?? computeGenesisShnarf(fileContents.parentStateRootHash);
  const finalShnarf = computeShnarfV2(parentShnarf, finalBlockHash, dataHash);

  return {
    compressedBlobs: [compressedData],
    blobDataSubmission: [
      {
        finalBlockHash,
        dataHash,
        compressedData,
        kzgCommitment: fileContents.commitment!,
      },
    ],
    blobFinalBlockHashes: [finalBlockHash],
    parentShnarf,
    finalShnarf,
  };
}

function emptyShnarfPadding(parentShnarf: string): ShnarfData {
  return {
    parentShnarf,
    snarkHash: HASH_ZERO,
    finalStateRootHash: HASH_ZERO,
    dataEvaluationPoint: HASH_ZERO,
    dataEvaluationClaim: HASH_ZERO,
  };
}

export function generateParentShnarfData(index: number, multiple?: boolean): ShnarfData {
  const dataSet = multiple ? COMPRESSED_SUBMISSION_DATA_MULTIPLE_PROOF : COMPRESSED_SUBMISSION_DATA;
  if (index === 0) {
    return emptyShnarfPadding(HASH_ZERO);
  }
  const chain = buildCalldataShnarfChain(dataSet, index - 1, index);
  return emptyShnarfPadding(chain[0].parentShnarf);
}

export function generateBlobParentShnarfData(index: number, multiple?: boolean): ShnarfData {
  const dataSet = multiple ? BLOB_SUBMISSION_DATA_MULTIPLE_PROOF : BLOB_SUBMISSION_DATA;
  if (index === 0) {
    return emptyShnarfPadding(HASH_ZERO);
  }
  const chain = buildBlobShnarfChain(dataSet, index - 1, index);
  return emptyShnarfPadding(chain[0].parentShnarf);
}

/**
 * Anchors for finalization after submissions `[0, blobParentShnarfIndex)`.
 */
export function getFinalizationAnchors(
  blobParentShnarfIndex: number,
  options: { isMultiple?: boolean; isBlob?: boolean } = {},
): { finalBlockHash: string; finalBlobHash: string } | undefined {
  const lastSubmissionIndex = blobParentShnarfIndex - 1;
  if (lastSubmissionIndex < 0) {
    return undefined;
  }
  const { isMultiple = false, isBlob = false } = options;
  if (isBlob) {
    const dataSet = isMultiple ? BLOB_SUBMISSION_DATA_MULTIPLE_PROOF : BLOB_SUBMISSION_DATA;
    const chain = buildBlobShnarfChain(dataSet, lastSubmissionIndex, lastSubmissionIndex + 1);
    return { finalBlockHash: chain[0].finalBlockHash, finalBlobHash: chain[0].dataHash };
  }
  const dataSet = isMultiple ? COMPRESSED_SUBMISSION_DATA_MULTIPLE_PROOF : COMPRESSED_SUBMISSION_DATA;
  const chain = buildCalldataShnarfChain(dataSet, lastSubmissionIndex, lastSubmissionIndex + 1);
  return { finalBlockHash: chain[0].blockHash, finalBlobHash: chain[0].dataHash };
}

export function generateParentAndExpectedShnarfForIndex(index: number): ParentAndExpectedShnarf {
  const chain = buildCalldataShnarfChain(COMPRESSED_SUBMISSION_DATA, index, index + 1);
  return {
    parentShnarf: chain[0].parentShnarf,
    expectedShnarf: chain[0].expectedShnarf,
  };
}

export function generateParentAndExpectedShnarfForMulitpleIndex(index: number): ParentAndExpectedShnarf {
  const chain = buildCalldataShnarfChain(COMPRESSED_SUBMISSION_DATA_MULTIPLE_PROOF, index, index + 1);
  return {
    parentShnarf: chain[0].parentShnarf,
    expectedShnarf: chain[0].expectedShnarf,
  };
}

export function generateCallDataSubmissionMultipleProofs(
  startDataIndex: number,
  finalDataIndex: number,
): CalldataSubmissionData[] {
  return buildCalldataShnarfChain(COMPRESSED_SUBMISSION_DATA_MULTIPLE_PROOF, startDataIndex, finalDataIndex).map(
    ({ blockHash, compressedData }) => ({ blockHash, compressedData }),
  );
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
