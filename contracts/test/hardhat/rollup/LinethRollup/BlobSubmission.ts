import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { AddressFilter, TestLinethRollup } from "contracts/typechain-types";
import { BaseContract } from "ethers";
import { ethers } from "hardhat";

import { GENERAL_PAUSE_TYPE, HASH_ZERO, OPERATOR_ROLE, STATE_DATA_SUBMISSION_PAUSE_TYPE } from "../../common/constants";
import {
  generateRandomBytes,
  buildAccessErrorMessage,
  expectRevertWithCustomError,
  expectRevertWithReason,
  generateBlobDataSubmission,
  expectEventDirectFromReceiptData,
  expectRevertWhenPaused,
  computeShnarfV2,
} from "../../common/helpers";
import {
  deployForcedTransactionGatewayFixture,
  ensureKzgSetup,
  getAccountsFixture,
  getWalletForIndex,
  buildBlobTransaction,
  sendBlobTransaction,
  submitBlobsAndGetReceipt,
} from "../helpers";

ensureKzgSetup();

describe("Lineth Rollup contract: EIP-4844 Blob submission tests", () => {
  let linethRollup: TestLinethRollup;

  let securityCouncil: SignerWithAddress;
  let operator: SignerWithAddress;
  let nonAuthorizedAccount: SignerWithAddress;
  let addressFilterAddress: string;
  let addressFilter: AddressFilter;

  before(async () => {
    ({ securityCouncil, operator, nonAuthorizedAccount } = await loadFixture(getAccountsFixture));
  });

  beforeEach(async () => {
    ({ linethRollup, addressFilter } = await loadFixture(deployForcedTransactionGatewayFixture));

    addressFilterAddress = await addressFilter.getAddress();

    await linethRollup.setLastFinalizedBlock(0);
    await linethRollup.connect(securityCouncil).setAddressFilter(addressFilterAddress);
  });

  it("Should successfully submit blobs", async () => {
    const { blobDataSubmission, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

    const receipt = await submitBlobsAndGetReceipt({
      linethRollup,
      blobSubmission: blobDataSubmission,
      compressedBlobs,
      parentShnarf,
      finalShnarf,
    });

    expect(receipt).is.not.null;

    const expectedEventArgs = [
      parentShnarf,
      finalShnarf,
      blobDataSubmission[blobDataSubmission.length - 1].finalBlockHash,
    ];

    expectEventDirectFromReceiptData(linethRollup as BaseContract, receipt!, "DataSubmittedV4", expectedEventArgs);

    const blobShnarfExists = await linethRollup.blobShnarfExists(finalShnarf);
    expect(blobShnarfExists).to.equal(1n);
  });

  it("Fails the blob submission when the parent shnarf is missing", async () => {
    const linethRollupAddress = await linethRollup.getAddress();
    const { blobDataSubmission, compressedBlobs } = generateBlobDataSubmission(0, 1);
    const nonExistingParentShnarf = generateRandomBytes(32);
    const wrongExpectedShnarf = computeShnarfV2(
      nonExistingParentShnarf,
      blobDataSubmission[0].finalBlockHash,
      blobDataSubmission[0].dataHash,
    );

    const encodedCall = linethRollup.interface.encodeFunctionData("submitBlobs", [
      blobDataSubmission.map((b) => b.finalBlockHash),
      nonExistingParentShnarf,
      wrongExpectedShnarf,
    ]);

    const transaction = await buildBlobTransaction({
      linethRollupAddress,
      encodedCall,
      compressedBlobs,
    });

    const signedTx = await getWalletForIndex(2).signTransaction(transaction);

    await expectRevertWithCustomError(
      linethRollup,
      ethers.provider.broadcastTransaction(signedTx),
      "ParentShnarfNotSubmitted",
      [nonExistingParentShnarf],
    );
  });

  it("Fails when the blob submission data is missing", async () => {
    const linethRollupAddress = await linethRollup.getAddress();
    const { compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

    const encodedCall = linethRollup.interface.encodeFunctionData("submitBlobs", [[], parentShnarf, finalShnarf]);

    const transaction = await buildBlobTransaction({
      linethRollupAddress,
      encodedCall,
      compressedBlobs,
    });

    const signedTx = await getWalletForIndex(2).signTransaction(transaction);

    await expectRevertWithCustomError(
      linethRollup,
      ethers.provider.broadcastTransaction(signedTx),
      "BlobSubmissionDataIsMissing",
    );
  });

  it("Should revert if the caller does not have the OPERATOR_ROLE", async () => {
    const { blobFinalBlockHashes, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

    await expectRevertWithReason(
      linethRollup.connect(nonAuthorizedAccount).submitBlobs(blobFinalBlockHashes, parentShnarf, finalShnarf),
      buildAccessErrorMessage(nonAuthorizedAccount, OPERATOR_ROLE),
    );
  });

  const blobSubmissionPauseTypes = [
    { pauseType: GENERAL_PAUSE_TYPE, name: "GENERAL_PAUSE_TYPE" },
    { pauseType: STATE_DATA_SUBMISSION_PAUSE_TYPE, name: "STATE_DATA_SUBMISSION_PAUSE_TYPE" },
  ];

  blobSubmissionPauseTypes.forEach(({ pauseType, name }) => {
    it(`Should revert if ${name} is enabled`, async () => {
      const { blobFinalBlockHashes, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

      await linethRollup.connect(securityCouncil).pauseByType(pauseType);

      await expectRevertWhenPaused(
        linethRollup,
        linethRollup.connect(operator).submitBlobs(blobFinalBlockHashes, parentShnarf, finalShnarf),
        pauseType,
      );
    });
  });

  it("Should revert if the blob data is empty at any index", async () => {
    const linethRollupAddress = await linethRollup.getAddress();
    const { blobFinalBlockHashes, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 2);

    const encodedCall = linethRollup.interface.encodeFunctionData("submitBlobs", [
      blobFinalBlockHashes,
      parentShnarf,
      finalShnarf,
    ]);

    const transaction = await buildBlobTransaction({
      linethRollupAddress,
      encodedCall,
      compressedBlobs: [compressedBlobs[0]],
    });

    const signedTx = await getWalletForIndex(2).signTransaction(transaction);

    await expectRevertWithCustomError(
      linethRollup,
      ethers.provider.broadcastTransaction(signedTx),
      "EmptyBlobDataAtIndex",
      [1n],
    );
  });

  it("Should fail if the final block hash yields a wrong expected shnarf", async () => {
    const linethRollupAddress = await linethRollup.getAddress();
    const { blobDataSubmission, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

    const wrongHashes = [HASH_ZERO];
    const actualShnarf = computeShnarfV2(parentShnarf, HASH_ZERO, blobDataSubmission[0].dataHash);

    const encodedCall = linethRollup.interface.encodeFunctionData("submitBlobs", [
      wrongHashes,
      parentShnarf,
      finalShnarf,
    ]);

    const transaction = await buildBlobTransaction({
      linethRollupAddress,
      encodedCall,
      compressedBlobs,
    });

    const signedTx = await getWalletForIndex(2).signTransaction(transaction);

    await expectRevertWithCustomError(
      linethRollup,
      ethers.provider.broadcastTransaction(signedTx),
      "FinalShnarfWrong",
      [finalShnarf, actualShnarf],
    );
  });

  it("Should revert if the final shnarf is wrong", async () => {
    const linethRollupAddress = await linethRollup.getAddress();
    const { blobFinalBlockHashes, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 2);
    const badFinalShnarf = generateRandomBytes(32);

    const encodedCall = linethRollup.interface.encodeFunctionData("submitBlobs", [
      blobFinalBlockHashes,
      parentShnarf,
      badFinalShnarf,
    ]);

    const transaction = await buildBlobTransaction({
      linethRollupAddress,
      encodedCall,
      compressedBlobs,
    });

    const signedTx = await getWalletForIndex(2).signTransaction(transaction);

    await expectRevertWithCustomError(
      linethRollup,
      ethers.provider.broadcastTransaction(signedTx),
      "FinalShnarfWrong",
      [badFinalShnarf, finalShnarf],
    );
  });

  it("Should revert if the data has already been submitted", async () => {
    await sendBlobTransaction(linethRollup, 0, 1);

    const linethRollupAddress = await linethRollup.getAddress();
    const { blobFinalBlockHashes, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

    const encodedCall = linethRollup.interface.encodeFunctionData("submitBlobs", [
      blobFinalBlockHashes,
      parentShnarf,
      finalShnarf,
    ]);

    const transaction = await buildBlobTransaction({
      linethRollupAddress,
      encodedCall,
      compressedBlobs,
    });

    const signedTx = await getWalletForIndex(2).signTransaction(transaction);

    await expectRevertWithCustomError(
      linethRollup,
      ethers.provider.broadcastTransaction(signedTx),
      "ShnarfAlreadySubmitted",
      [finalShnarf],
    );
  });

  it("Should revert if there is less data than blobs", async () => {
    const linethRollupAddress = await linethRollup.getAddress();

    const { blobFinalBlockHashes, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 2, true);

    const encodedCall = linethRollup.interface.encodeFunctionData("submitBlobs", [
      [blobFinalBlockHashes[0]],
      parentShnarf,
      finalShnarf,
    ]);

    const transaction = await buildBlobTransaction({
      linethRollupAddress,
      encodedCall,
      compressedBlobs,
    });

    const signedTx = await getWalletForIndex(2).signTransaction(transaction);

    await expectRevertWithCustomError(
      linethRollup,
      ethers.provider.broadcastTransaction(signedTx),
      "BlobSubmissionDataEmpty",
      [1],
    );
  });
});
