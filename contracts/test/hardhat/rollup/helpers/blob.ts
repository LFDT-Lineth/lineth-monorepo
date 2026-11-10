import * as fs from "fs";
import path from "path";
import {
  type Abi,
  type Address,
  type Hex,
  type TransactionReceipt,
  type TransactionSerializableEIP4844,
  encodeFunctionData,
  pad,
} from "viem";

import { getWalletForIndex } from "./index.js";
import { ensureKzgSetup } from "./kzg-setup.js";
import { getConnection } from "../../common/helpers/connection.js";
import {
  expectEventDirectFromReceiptData,
  generateBlobDataSubmission,
  generateBlobDataSubmissionFromFile,
} from "../../common/helpers/index.js";

import type { BlobSubmission } from "../../common/types.js";
import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";
import type { HDAccount } from "viem/accounts";

/** EIP-4844 blob size in bytes (4096 field elements of 32 bytes). */
const BLOB_SIZE = 4096 * 32;

/**
 * Converts blob submissions to the `submitBlobs` struct shape: `dataEvaluationClaim` is a `uint256` in the ABI.
 */
export function toBlobSubmissionStructs(blobSubmission: BlobSubmission[]) {
  return blobSubmission.map((submission) => ({
    ...submission,
    dataEvaluationClaim: BigInt(submission.dataEvaluationClaim),
  }));
}

/**
 * Context for building and sending blob transactions
 */
export type BlobTransactionContext = {
  linethRollupAddress: Address;
  encodedCall: Hex;
  compressedBlobs: Hex[];
  operatorHDSigner?: HDAccount | undefined;
  gasLimit?: number | undefined;
  targetAddress?: Address | undefined; // Override for callforwarder scenarios
};

/**
 * Builds a type-3 blob transaction (EIP-4844). Each blob is zero-padded on the right to the blob size; the KZG
 * commitments and proofs are computed when the transaction is signed.
 */
export async function buildBlobTransaction(context: BlobTransactionContext): Promise<TransactionSerializableEIP4844> {
  const {
    linethRollupAddress,
    encodedCall,
    compressedBlobs,
    operatorHDSigner,
    gasLimit = 5_000_000,
    targetAddress,
  } = context;

  const signer = operatorHDSigner ?? (await getWalletForIndex(2));
  const { viem } = await getConnection();
  const publicClient = await viem.getPublicClient();
  const { maxFeePerGas, maxPriorityFeePerGas } = await publicClient.estimateFeesPerGas();
  const nonce = await publicClient.getTransactionCount({ address: signer.address, blockTag: "pending" });

  return {
    type: "eip4844",
    data: encodedCall,
    maxPriorityFeePerGas,
    maxFeePerGas,
    to: targetAddress ?? linethRollupAddress,
    chainId: await publicClient.getChainId(),
    nonce,
    value: 0n,
    gas: BigInt(gasLimit),
    kzg: ensureKzgSetup(),
    maxFeePerBlobGas: 1n,
    blobs: compressedBlobs.map((blob) => pad(blob, { dir: "right", size: BLOB_SIZE })),
  };
}

/**
 * Signs and broadcasts a blob transaction, returning the receipt
 */
export async function signAndBroadcastBlobTransaction(
  transaction: TransactionSerializableEIP4844,
  operatorHDSigner?: HDAccount,
): Promise<TransactionReceipt> {
  const signer = operatorHDSigner ?? (await getWalletForIndex(2));
  const signedTx = await signer.signTransaction(transaction);
  const { viem } = await getConnection();
  const publicClient = await viem.getPublicClient();
  const hash = await publicClient.sendRawTransaction({ serializedTransaction: signedTx });
  return await publicClient.waitForTransactionReceipt({ hash });
}

/**
 * Context for submitting blobs with validation
 */
export type SubmitBlobsContext = {
  linethRollup: ContractReturnType<"TestLinethRollup">;
  blobSubmission: BlobSubmission[];
  compressedBlobs: Hex[];
  parentShnarf: Hex;
  finalShnarf: Hex;
  operatorHDSigner?: HDAccount;
  gasLimit?: number;
  targetAddress?: Address;
};

/**
 * Builds and submits blobs, returning the receipt
 */
export async function submitBlobsAndGetReceipt(context: SubmitBlobsContext): Promise<TransactionReceipt> {
  const {
    linethRollup,
    blobSubmission,
    compressedBlobs,
    parentShnarf,
    finalShnarf,
    operatorHDSigner,
    gasLimit,
    targetAddress,
  } = context;

  const encodedCall = encodeFunctionData({
    abi: linethRollup.abi,
    functionName: "submitBlobs",
    args: [toBlobSubmissionStructs(blobSubmission), parentShnarf, finalShnarf],
  });

  const transaction = await buildBlobTransaction({
    linethRollupAddress: linethRollup.address,
    encodedCall,
    compressedBlobs,
    operatorHDSigner,
    gasLimit,
    targetAddress,
  });

  return signAndBroadcastBlobTransaction(transaction, operatorHDSigner);
}

/** Kept for compatibility: delegates to the idempotent `ensureKzgSetup`. */
export function ensureKzgIsLoaded() {
  ensureKzgSetup();
}

export async function sendBlobTransaction(
  linethRollup: ContractReturnType<"TestLinethRollup">,
  startIndex: number,
  finalIndex: number,
  isMultiple: boolean = false,
) {
  const {
    blobDataSubmission: blobSubmission,
    compressedBlobs,
    parentShnarf,
    finalShnarf,
  } = generateBlobDataSubmission(startIndex, finalIndex, isMultiple);

  const receipt = await submitBlobsAndGetReceipt({
    linethRollup,
    blobSubmission,
    compressedBlobs,
    parentShnarf,
    finalShnarf,
  });

  const expectedEventArgs = [parentShnarf, finalShnarf, blobSubmission[blobSubmission.length - 1].finalStateRootHash];
  expectEventDirectFromReceiptData(linethRollup, receipt, "DataSubmittedV3", expectedEventArgs);
}

export async function sendVersionedBlobTransactionFromFile(
  linethRollup: ContractReturnType<"TestLinethRollup">,
  filePath: string,
  versionedLinethRollup: { address: Address },
  versionFolderName: string,
) {
  const {
    blobDataSubmission: blobSubmission,
    compressedBlobs,
    parentShnarf,
    finalShnarf,
  } = generateBlobDataSubmissionFromFile(
    path.resolve(import.meta.dirname, `../../_testData/${versionFolderName}`, filePath),
  );

  const encodedCall = encodeFunctionData({
    abi: linethRollup.abi,
    functionName: "submitBlobs",
    args: [toBlobSubmissionStructs(blobSubmission), parentShnarf, finalShnarf],
  });

  const transaction = await buildBlobTransaction({
    linethRollupAddress: versionedLinethRollup.address,
    encodedCall,
    compressedBlobs,
  });

  const receipt = await signAndBroadcastBlobTransaction(transaction);
  const expectedEventArgs = [parentShnarf, finalShnarf, blobSubmission[blobSubmission.length - 1].finalStateRootHash];

  expectEventDirectFromReceiptData(linethRollup, receipt, "DataSubmittedV3", expectedEventArgs);
}

export async function sendBlobTransactionViaCallForwarder(
  linethRollupUpgraded: { abi: Abi },
  startIndex: number,
  finalIndex: number,
  callforwarderAddress: Address,
) {
  const {
    blobDataSubmission: blobSubmission,
    compressedBlobs,
    parentShnarf,
    finalShnarf,
  } = generateBlobDataSubmission(startIndex, finalIndex, false);

  const encodedCall = encodeFunctionData({
    abi: linethRollupUpgraded.abi,
    functionName: "submitBlobs",
    args: [toBlobSubmissionStructs(blobSubmission), parentShnarf, finalShnarf],
  });

  const transaction = await buildBlobTransaction({
    linethRollupAddress: callforwarderAddress,
    encodedCall,
    compressedBlobs,
    targetAddress: callforwarderAddress,
  });

  const receipt = await signAndBroadcastBlobTransaction(transaction);
  const expectedEventArgs = [parentShnarf, finalShnarf, blobSubmission[blobSubmission.length - 1].finalStateRootHash];

  expectEventDirectFromReceiptData(linethRollupUpgraded, receipt, "DataSubmittedV3", expectedEventArgs);
}

// "betaV1" getBetaV1BlobFiles
export function getVersionedBlobFiles(versionFolderName: string): string[] {
  // Read all files in the folder
  const files = fs.readdirSync(path.resolve(import.meta.dirname, `../../_testData/${versionFolderName}`));

  // Map files to their ranges and filter invalid ones
  const filesWithRanges = files
    .map((fileName) => {
      const range = extractBlockRangeFromFileName(fileName);
      return range ? { fileName, range } : null;
    })
    .filter(Boolean) as { fileName: string; range: [number, number] }[];

  return filesWithRanges.sort((a, b) => a.range[0] - b.range[0]).map((f) => f.fileName);
}

// Function to extract range from the file name
function extractBlockRangeFromFileName(fileName: string): [number, number] | null {
  const rangeRegex = /(\d+)-(\d+)-/;
  const match = fileName.match(rangeRegex);
  if (match && match.length >= 3) {
    return [parseInt(match[1], 10), parseInt(match[2], 10)];
  }
  return null;
}
