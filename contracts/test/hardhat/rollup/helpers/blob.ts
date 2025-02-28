import * as kzg from "c-kzg";
import { BaseContract, Contract, Transaction } from "ethers";
import * as fs from "fs";
import { ethers } from "hardhat";
import path from "path";

import { TestLineaRollup } from "contracts/typechain-types";
import { getWalletForIndex } from "./";
import {
  expectEventDirectFromReceiptData,
  generateBlobDataSubmission,
  generateBlobDataSubmissionFromFile,
} from "../../common/helpers";

export async function sendBlobTransaction(
  lineaRollup: TestLineaRollup,
  startIndex: number,
  finalIndex: number,
  isMultiple: boolean = false,
) {
  const operatorHDSigner = getWalletForIndex(2);
  const lineaRollupAddress = await lineaRollup.getAddress();

  const {
    blobDataSubmission: blobSubmission,
    compressedBlobs: compressedBlobs,
    parentShnarf: parentShnarf,
    finalShnarf: finalShnarf,
  } = generateBlobDataSubmission(startIndex, finalIndex, isMultiple);

  const encodedCall = lineaRollup.interface.encodeFunctionData("submitBlobs", [
    blobSubmission,
    parentShnarf,
    finalShnarf,
  ]);

  const { maxFeePerGas, maxPriorityFeePerGas } = await ethers.provider.getFeeData();
  const nonce = await operatorHDSigner.getNonce();

  const transaction = Transaction.from({
    data: encodedCall,
    maxPriorityFeePerGas: maxPriorityFeePerGas!,
    maxFeePerGas: maxFeePerGas!,
    to: lineaRollupAddress,
    chainId: (await ethers.provider.getNetwork()).chainId,
    type: 3,
    nonce: nonce,
    value: 0,
    gasLimit: 5_000_000,
    kzg,
    maxFeePerBlobGas: 1n,
    blobs: compressedBlobs,
  });

  const signedTx = await operatorHDSigner.signTransaction(transaction);
  const txResponse = await ethers.provider.broadcastTransaction(signedTx);

  const receipt = await ethers.provider.getTransactionReceipt(txResponse.hash);

  const expectedEventArgs = [parentShnarf, finalShnarf, blobSubmission[blobSubmission.length - 1].finalStateRootHash];

  expectEventDirectFromReceiptData(lineaRollup as BaseContract, receipt!, "DataSubmittedV3", expectedEventArgs);
}

export async function sendBlobTransactionFromFile(
  lineaRollup: TestLineaRollup,
  filePath: string,
  betaV1LineaRollup: TestLineaRollup,
) {
  const operatorHDSigner = getWalletForIndex(2);
  const lineaRollupAddress = await betaV1LineaRollup.getAddress();

  const {
    blobDataSubmission: blobSubmission,
    compressedBlobs: compressedBlobs,
    parentShnarf: parentShnarf,
    finalShnarf: finalShnarf,
  } = generateBlobDataSubmissionFromFile(path.resolve(__dirname, "../../_testData/betaV1", filePath));

  const encodedCall = lineaRollup.interface.encodeFunctionData("submitBlobs", [
    blobSubmission,
    parentShnarf,
    finalShnarf,
  ]);

  const { maxFeePerGas, maxPriorityFeePerGas } = await ethers.provider.getFeeData();
  const nonce = await operatorHDSigner.getNonce();

  const transaction = Transaction.from({
    data: encodedCall,
    maxPriorityFeePerGas: maxPriorityFeePerGas!,
    maxFeePerGas: maxFeePerGas!,
    to: lineaRollupAddress,
    chainId: (await ethers.provider.getNetwork()).chainId,
    type: 3,
    nonce: nonce,
    value: 0,
    gasLimit: 5_000_000,
    kzg,
    maxFeePerBlobGas: 1n,
    blobs: compressedBlobs,
  });

  const signedTx = await operatorHDSigner.signTransaction(transaction);
  const txResponse = await ethers.provider.broadcastTransaction(signedTx);
  const receipt = await ethers.provider.getTransactionReceipt(txResponse.hash);
  const expectedEventArgs = [parentShnarf, finalShnarf, blobSubmission[blobSubmission.length - 1].finalStateRootHash];

  expectEventDirectFromReceiptData(lineaRollup as BaseContract, receipt!, "DataSubmittedV3", expectedEventArgs);
}

export async function sendBlobTransactionViaCallForwarder(
  lineaRollupUpgraded: Contract,
  startIndex: number,
  finalIndex: number,
  callforwarderAddress: string,
) {
  const operatorHDSigner = getWalletForIndex(2);

  const {
    blobDataSubmission: blobSubmission,
    compressedBlobs: compressedBlobs,
    parentShnarf: parentShnarf,
    finalShnarf: finalShnarf,
  } = generateBlobDataSubmission(startIndex, finalIndex, false);

  const encodedCall = lineaRollupUpgraded.interface.encodeFunctionData("submitBlobs", [
    blobSubmission,
    parentShnarf,
    finalShnarf,
  ]);

  const { maxFeePerGas, maxPriorityFeePerGas } = await ethers.provider.getFeeData();
  const nonce = await operatorHDSigner.getNonce();

  const transaction = Transaction.from({
    data: encodedCall,
    maxPriorityFeePerGas: maxPriorityFeePerGas!,
    maxFeePerGas: maxFeePerGas!,
    to: callforwarderAddress,
    chainId: (await ethers.provider.getNetwork()).chainId,
    type: 3,
    nonce: nonce,
    value: 0,
    gasLimit: 5_000_000,
    kzg,
    maxFeePerBlobGas: 1n,
    blobs: compressedBlobs,
  });

  const signedTx = await operatorHDSigner.signTransaction(transaction);
  const txResponse = await ethers.provider.broadcastTransaction(signedTx);
  const receipt = await ethers.provider.getTransactionReceipt(txResponse.hash);

  const expectedEventArgs = [parentShnarf, finalShnarf, blobSubmission[blobSubmission.length - 1].finalStateRootHash];

  expectEventDirectFromReceiptData(lineaRollupUpgraded as BaseContract, receipt!, "DataSubmittedV3", expectedEventArgs);
}

export function getBetaV1BlobFiles(): string[] {
  // Read all files in the folder
  const files = fs.readdirSync(path.resolve(__dirname, "../../_testData/betaV1"));

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
