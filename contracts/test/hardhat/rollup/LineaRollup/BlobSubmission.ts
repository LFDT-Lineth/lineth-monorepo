import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { AddressFilter, TestLineaRollup } from "contracts/typechain-types";
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

describe("Linea Rollup contract: EIP-4844 Blob submission tests", () => {
  let lineaRollup: TestLineaRollup;

  let securityCouncil: SignerWithAddress;
  let operator: SignerWithAddress;
  let nonAuthorizedAccount: SignerWithAddress;
  let addressFilterAddress: string;
  let addressFilter: AddressFilter;

  before(async () => {
    ({ securityCouncil, operator, nonAuthorizedAccount } = await loadFixture(getAccountsFixture));
  });

  beforeEach(async () => {
    ({ lineaRollup, addressFilter } = await loadFixture(deployForcedTransactionGatewayFixture));

    addressFilterAddress = await addressFilter.getAddress();

    await lineaRollup.setLastFinalizedBlock(0);
    await lineaRollup.connect(securityCouncil).setAddressFilter(addressFilterAddress);
  });

  it("Should successfully submit blobs", async () => {
    const { blobDataSubmission, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

    const receipt = await submitBlobsAndGetReceipt({
      lineaRollup,
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

    expectEventDirectFromReceiptData(lineaRollup as BaseContract, receipt!, "DataSubmittedV4", expectedEventArgs);

    const blobShnarfExists = await lineaRollup.blobShnarfExists(finalShnarf);
    expect(blobShnarfExists).to.equal(1n);
  });

  it("Fails the blob submission when the parent shnarf is missing", async () => {
    const lineaRollupAddress = await lineaRollup.getAddress();
    const { blobDataSubmission, compressedBlobs } = generateBlobDataSubmission(0, 1);
    const nonExistingParentShnarf = generateRandomBytes(32);
    const wrongExpectedShnarf = computeShnarfV2(
      nonExistingParentShnarf,
      blobDataSubmission[0].finalBlockHash,
      blobDataSubmission[0].dataHash,
    );

    const encodedCall = lineaRollup.interface.encodeFunctionData("submitBlobs", [
      blobDataSubmission.map((b) => b.finalBlockHash),
      nonExistingParentShnarf,
      wrongExpectedShnarf,
    ]);

    const transaction = await buildBlobTransaction({
      lineaRollupAddress,
      encodedCall,
      compressedBlobs,
    });

    const signedTx = await getWalletForIndex(2).signTransaction(transaction);

    await expectRevertWithCustomError(
      lineaRollup,
      ethers.provider.broadcastTransaction(signedTx),
      "ParentShnarfNotSubmitted",
      [nonExistingParentShnarf],
    );
  });

  it("Fails when the blob submission data is missing", async () => {
    const lineaRollupAddress = await lineaRollup.getAddress();
    const { compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

    const encodedCall = lineaRollup.interface.encodeFunctionData("submitBlobs", [[], parentShnarf, finalShnarf]);

    const transaction = await buildBlobTransaction({
      lineaRollupAddress,
      encodedCall,
      compressedBlobs,
    });

    const signedTx = await getWalletForIndex(2).signTransaction(transaction);

    await expectRevertWithCustomError(
      lineaRollup,
      ethers.provider.broadcastTransaction(signedTx),
      "BlobSubmissionDataIsMissing",
    );
  });

  it("Should revert if the caller does not have the OPERATOR_ROLE", async () => {
    const { blobFinalBlockHashes, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

    await expectRevertWithReason(
      lineaRollup.connect(nonAuthorizedAccount).submitBlobs(blobFinalBlockHashes, parentShnarf, finalShnarf),
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

      await lineaRollup.connect(securityCouncil).pauseByType(pauseType);

      await expectRevertWhenPaused(
        lineaRollup,
        lineaRollup.connect(operator).submitBlobs(blobFinalBlockHashes, parentShnarf, finalShnarf),
        pauseType,
      );
    });
  });

  it("Should revert if the blob data is empty at any index", async () => {
    const lineaRollupAddress = await lineaRollup.getAddress();
    const { blobFinalBlockHashes, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 2);

    const encodedCall = lineaRollup.interface.encodeFunctionData("submitBlobs", [
      blobFinalBlockHashes,
      parentShnarf,
      finalShnarf,
    ]);

    const transaction = await buildBlobTransaction({
      lineaRollupAddress,
      encodedCall,
      compressedBlobs: [compressedBlobs[0]],
    });

    const signedTx = await getWalletForIndex(2).signTransaction(transaction);

    await expectRevertWithCustomError(
      lineaRollup,
      ethers.provider.broadcastTransaction(signedTx),
      "EmptyBlobDataAtIndex",
      [1n],
    );
  });

  it("Should fail if the final block hash yields a wrong expected shnarf", async () => {
    const lineaRollupAddress = await lineaRollup.getAddress();
    const { blobDataSubmission, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

    const wrongHashes = [HASH_ZERO];
    const actualShnarf = computeShnarfV2(parentShnarf, HASH_ZERO, blobDataSubmission[0].dataHash);

    const encodedCall = lineaRollup.interface.encodeFunctionData("submitBlobs", [
      wrongHashes,
      parentShnarf,
      finalShnarf,
    ]);

    const transaction = await buildBlobTransaction({
      lineaRollupAddress,
      encodedCall,
      compressedBlobs,
    });

    const signedTx = await getWalletForIndex(2).signTransaction(transaction);

    await expectRevertWithCustomError(lineaRollup, ethers.provider.broadcastTransaction(signedTx), "FinalShnarfWrong", [
      finalShnarf,
      actualShnarf,
    ]);
  });

  it("Should revert if the final shnarf is wrong", async () => {
    const lineaRollupAddress = await lineaRollup.getAddress();
    const { blobFinalBlockHashes, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 2);
    const badFinalShnarf = generateRandomBytes(32);

    const encodedCall = lineaRollup.interface.encodeFunctionData("submitBlobs", [
      blobFinalBlockHashes,
      parentShnarf,
      badFinalShnarf,
    ]);

    const transaction = await buildBlobTransaction({
      lineaRollupAddress,
      encodedCall,
      compressedBlobs,
    });

    const signedTx = await getWalletForIndex(2).signTransaction(transaction);

    await expectRevertWithCustomError(lineaRollup, ethers.provider.broadcastTransaction(signedTx), "FinalShnarfWrong", [
      badFinalShnarf,
      finalShnarf,
    ]);
  });

  it("Should revert if the data has already been submitted", async () => {
    await sendBlobTransaction(lineaRollup, 0, 1);

    const lineaRollupAddress = await lineaRollup.getAddress();
    const { blobFinalBlockHashes, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

    const encodedCall = lineaRollup.interface.encodeFunctionData("submitBlobs", [
      blobFinalBlockHashes,
      parentShnarf,
      finalShnarf,
    ]);

    const transaction = await buildBlobTransaction({
      lineaRollupAddress,
      encodedCall,
      compressedBlobs,
    });

    const signedTx = await getWalletForIndex(2).signTransaction(transaction);

    await expectRevertWithCustomError(
      lineaRollup,
      ethers.provider.broadcastTransaction(signedTx),
      "ShnarfAlreadySubmitted",
      [finalShnarf],
    );
  });

  it("Should revert if there is less data than blobs", async () => {
    const lineaRollupAddress = await lineaRollup.getAddress();

    const { blobFinalBlockHashes, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 2, true);

    const encodedCall = lineaRollup.interface.encodeFunctionData("submitBlobs", [
      [blobFinalBlockHashes[0]],
      parentShnarf,
      finalShnarf,
    ]);

    const transaction = await buildBlobTransaction({
      lineaRollupAddress,
      encodedCall,
      compressedBlobs,
    });

    const signedTx = await getWalletForIndex(2).signTransaction(transaction);

    await expectRevertWithCustomError(
      lineaRollup,
      ethers.provider.broadcastTransaction(signedTx),
      "BlobSubmissionDataEmpty",
      [1],
    );
  });
});
