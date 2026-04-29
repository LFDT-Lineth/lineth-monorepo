import * as kzg from "c-kzg";
import { TestLineaRollup } from "contracts/typechain-types";
import { BaseContract, Contract, HDNodeWallet, Transaction, TransactionReceipt } from "ethers";
import * as fs from "fs";
import { ethers } from "hardhat";
import path from "path";

import { getWalletForIndex } from "./";
import {
  expectEventDirectFromReceiptData,
  generateBlobDataSubmission,
  generateBlobDataSubmissionFromFile,
} from "../../common/helpers";
import { BlobSubmission } from "../../common/types";

/**
 * Context for building and sending blob transactions
 */
export type BlobTransactionContext = {
  lineaRollupAddress: string;
  encodedCall: string;
  compressedBlobs: string[];
  operatorHDSigner?: HDNodeWallet | undefined;
  gasLimit?: number | undefined;
  targetAddress?: string | undefined; // Override for callforwarder scenarios
};

/**
 * Builds a type-3 blob transaction (EIP-4844)
 */
export async function buildBlobTransaction(context: BlobTransactionContext): Promise<Transaction> {
  const {
    lineaRollupAddress,
    encodedCall,
    compressedBlobs,
    operatorHDSigner,
    gasLimit = 5_000_000,
    targetAddress,
  } = context;

  const signer = operatorHDSigner ?? getWalletForIndex(2);
  const { maxFeePerGas, maxPriorityFeePerGas } = await ethers.provider.getFeeData();
  const nonce = await signer.getNonce();

  return Transaction.from({
    data: encodedCall,
    maxPriorityFeePerGas: maxPriorityFeePerGas!,
    maxFeePerGas: maxFeePerGas!,
    to: targetAddress ?? lineaRollupAddress,
    chainId: (await ethers.provider.getNetwork()).chainId,
    type: 3,
    nonce,
    value: 0,
    gasLimit,
    kzg,
    maxFeePerBlobGas: 1n,
    blobs: compressedBlobs,
  });
}

/**
 * Signs and broadcasts a blob transaction, returning the receipt
 */
export async function signAndBroadcastBlobTransaction(
  transaction: Transaction,
  operatorHDSigner?: HDNodeWallet,
): Promise<TransactionReceipt | null> {
  const signer = operatorHDSigner ?? getWalletForIndex(2);
  const signedTx = await signer.signTransaction(transaction);
  const txResponse = await ethers.provider.broadcastTransaction(signedTx);
  return await ethers.provider.getTransactionReceipt(txResponse.hash);
}

/**
 * Context for submitting blobs with validation
 */
export type SubmitBlobsContext = {
  lineaRollup: TestLineaRollup;
  blobSubmission: BlobSubmission[];
  compressedBlobs: string[];
  parentShnarf: string;
  finalShnarf: string;
  operatorHDSigner?: HDNodeWallet;
  gasLimit?: number;
  targetAddress?: string;
};

/**
 * Builds and submits blobs, returning the receipt
 */
export async function submitBlobsAndGetReceipt(context: SubmitBlobsContext): Promise<TransactionReceipt | null> {
  const {
    lineaRollup,
    blobSubmission,
    compressedBlobs,
    parentShnarf,
    finalShnarf,
    operatorHDSigner,
    gasLimit,
    targetAddress,
  } = context;

  const lineaRollupAddress = await lineaRollup.getAddress();
  const encodedCall = lineaRollup.interface.encodeFunctionData("submitBlobs", [
    blobSubmission,
    parentShnarf,
    finalShnarf,
  ]);

  const transaction = await buildBlobTransaction({
    lineaRollupAddress,
    encodedCall,
    compressedBlobs,
    operatorHDSigner,
    gasLimit,
    targetAddress,
  });

  return signAndBroadcastBlobTransaction(transaction, operatorHDSigner);
}

let kzgLoaded = false;

export function ensureKzgIsLoaded() {
  if (!kzgLoaded) {
    kzg.loadTrustedSetup(0, path.resolve(__dirname, "../../_testData/trusted_setup.txt"));
    kzgLoaded = true;
  }
}

export async function sendBlobTransaction(
  lineaRollup: TestLineaRollup,
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
    lineaRollup,
    blobSubmission,
    compressedBlobs,
    parentShnarf,
    finalShnarf,
  });

  const expectedEventArgs = [parentShnarf, finalShnarf, blobSubmission[blobSubmission.length - 1].finalStateRootHash];
  expectEventDirectFromReceiptData(lineaRollup as BaseContract, receipt!, "DataSubmittedV3", expectedEventArgs);
}

export async function sendVersionedBlobTransactionFromFile(
  lineaRollup: TestLineaRollup,
  filePath: string,
  versionedLineaRollup: TestLineaRollup,
  versionFolderName: string,
) {
  const versionedLineaRollupAddress = await versionedLineaRollup.getAddress();

  const {
    blobDataSubmission: blobSubmission,
    compressedBlobs,
    parentShnarf,
    finalShnarf,
  } = generateBlobDataSubmissionFromFile(path.resolve(__dirname, `../../_testData/${versionFolderName}`, filePath));

  const encodedCall = lineaRollup.interface.encodeFunctionData("submitBlobs", [
    blobSubmission,
    parentShnarf,
    finalShnarf,
  ]);

  const transaction = await buildBlobTransaction({
    lineaRollupAddress: versionedLineaRollupAddress,
    encodedCall,
    compressedBlobs,
  });

  const receipt = await signAndBroadcastBlobTransaction(transaction);
  const expectedEventArgs = [parentShnarf, finalShnarf, blobSubmission[blobSubmission.length - 1].finalStateRootHash];

  expectEventDirectFromReceiptData(lineaRollup as BaseContract, receipt!, "DataSubmittedV3", expectedEventArgs);
}

export async function sendBlobTransactionViaCallForwarder(
  lineaRollupUpgraded: Contract,
  startIndex: number,
  finalIndex: number,
  callforwarderAddress: string,
) {
  const {
    blobDataSubmission: blobSubmission,
    compressedBlobs,
    parentShnarf,
    finalShnarf,
  } = generateBlobDataSubmission(startIndex, finalIndex, false);

  const encodedCall = lineaRollupUpgraded.interface.encodeFunctionData("submitBlobs", [
    blobSubmission,
    parentShnarf,
    finalShnarf,
  ]);

  const transaction = await buildBlobTransaction({
    lineaRollupAddress: callforwarderAddress,
    encodedCall,
    compressedBlobs,
    targetAddress: callforwarderAddress,
  });

  const receipt = await signAndBroadcastBlobTransaction(transaction);
  const expectedEventArgs = [parentShnarf, finalShnarf, blobSubmission[blobSubmission.length - 1].finalStateRootHash];

  expectEventDirectFromReceiptData(lineaRollupUpgraded as BaseContract, receipt!, "DataSubmittedV3", expectedEventArgs);
}

// "betaV1" getBetaV1BlobFiles
export function getVersionedBlobFiles(versionFolderName: string): string[] {
  // Read all files in the folder
  const files = fs.readdirSync(path.resolve(__dirname, `../../_testData/${versionFolderName}`));

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
