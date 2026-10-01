import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { AddressFilter, TestLinethRollup } from "contracts/typechain-types";
import { BaseContract } from "ethers";
import { ethers } from "hardhat";

import { GENERAL_PAUSE_TYPE, OPERATOR_ROLE, STATE_DATA_SUBMISSION_PAUSE_TYPE } from "../../common/constants";
import {
  generateRandomBytes,
  buildAccessErrorMessage,
  expectRevertWithCustomError,
  expectRevertWithReason,
  generateBlobDataSubmission,
  expectEventDirectFromReceiptData,
  expectRevertWhenPaused,
  computeDataRollingHash,
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
    const { blobDataSubmission, compressedBlobs, parentDataRollingHash, storedDataRollingHash } =
      generateBlobDataSubmission(0, 1);

    const receipt = await submitBlobsAndGetReceipt({
      linethRollup,
      blobSubmission: blobDataSubmission,
      compressedBlobs,
      parentDataRollingHash,
      storedDataRollingHash,
    });

    expect(receipt).is.not.null;

    const expectedEventArgs = [parentDataRollingHash, storedDataRollingHash];

    expectEventDirectFromReceiptData(linethRollup as BaseContract, receipt!, "DataSubmittedV4", expectedEventArgs);

    const dataRollingHashExists = await linethRollup.dataRollingHashExists(storedDataRollingHash);
    expect(dataRollingHashExists).to.equal(1n);
  });

  it("Fails the blob submission when the parent dataRollingHash is not anchored", async () => {
    const linethRollupAddress = await linethRollup.getAddress();
    const { blobDataSubmission, compressedBlobs } = generateBlobDataSubmission(0, 1);
    const nonExistingParent = generateRandomBytes(32);
    const wrongStored = computeDataRollingHash(nonExistingParent, blobDataSubmission[0].dataHash);

    const encodedCall = linethRollup.interface.encodeFunctionData("submitBlobs", [nonExistingParent, wrongStored]);

    const transaction = await buildBlobTransaction({
      linethRollupAddress,
      encodedCall,
      compressedBlobs,
    });

    const signedTx = await getWalletForIndex(2).signTransaction(transaction);

    await expectRevertWithCustomError(
      linethRollup,
      ethers.provider.broadcastTransaction(signedTx),
      "ParentDataRollingHashNotAnchored",
      [nonExistingParent],
    );
  });

  it("Fails when the blob submission data is missing", async () => {
    // A plain (non-blob) call to submitBlobs finds blobhash(0) == EMPTY_HASH on-chain and reverts.
    const { parentDataRollingHash, storedDataRollingHash } = generateBlobDataSubmission(0, 1);

    await expectRevertWithCustomError(
      linethRollup,
      linethRollup.connect(operator).submitBlobs(parentDataRollingHash, storedDataRollingHash),
      "BlobSubmissionDataIsMissing",
    );
  });

  it("Should revert if the caller does not have the OPERATOR_ROLE", async () => {
    const { parentDataRollingHash, storedDataRollingHash } = generateBlobDataSubmission(0, 1);

    await expectRevertWithReason(
      linethRollup.connect(nonAuthorizedAccount).submitBlobs(parentDataRollingHash, storedDataRollingHash),
      buildAccessErrorMessage(nonAuthorizedAccount, OPERATOR_ROLE),
    );
  });

  const blobSubmissionPauseTypes = [
    { pauseType: GENERAL_PAUSE_TYPE, name: "GENERAL_PAUSE_TYPE" },
    { pauseType: STATE_DATA_SUBMISSION_PAUSE_TYPE, name: "STATE_DATA_SUBMISSION_PAUSE_TYPE" },
  ];

  blobSubmissionPauseTypes.forEach(({ pauseType, name }) => {
    it(`Should revert if ${name} is enabled`, async () => {
      const { parentDataRollingHash, storedDataRollingHash } = generateBlobDataSubmission(0, 1);

      await linethRollup.connect(securityCouncil).pauseByType(pauseType);

      await expectRevertWhenPaused(
        linethRollup,
        linethRollup.connect(operator).submitBlobs(parentDataRollingHash, storedDataRollingHash),
        pauseType,
      );
    });
  });

  it("Should revert if the folded dataRollingHash does not match the declared stored value", async () => {
    const linethRollupAddress = await linethRollup.getAddress();
    const { compressedBlobs, parentDataRollingHash, storedDataRollingHash } = generateBlobDataSubmission(0, 2);
    // Declare a stored hash that does not match the on-chain fold of the 2 attached blobs.
    const badStored = generateRandomBytes(32);

    const encodedCall = linethRollup.interface.encodeFunctionData("submitBlobs", [parentDataRollingHash, badStored]);

    const transaction = await buildBlobTransaction({
      linethRollupAddress,
      encodedCall,
      compressedBlobs,
    });

    const signedTx = await getWalletForIndex(2).signTransaction(transaction);

    await expectRevertWithCustomError(
      linethRollup,
      ethers.provider.broadcastTransaction(signedTx),
      "DataRollingHashMismatch",
      [badStored, storedDataRollingHash],
    );
  });

  it("Should revert if the data has already been submitted", async () => {
    await sendBlobTransaction(linethRollup, 0, 1);

    const linethRollupAddress = await linethRollup.getAddress();
    const { compressedBlobs, parentDataRollingHash, storedDataRollingHash } = generateBlobDataSubmission(0, 1);

    const encodedCall = linethRollup.interface.encodeFunctionData("submitBlobs", [
      parentDataRollingHash,
      storedDataRollingHash,
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
      "DataRollingHashAlreadyAnchored",
      [storedDataRollingHash],
    );
  });

  it("Should revert when fewer blobs are attached than the declared stored value requires", async () => {
    const linethRollupAddress = await linethRollup.getAddress();

    // Build the 2-blob expectation but attach only 1 blob: the fold stops early and mismatches.
    const { compressedBlobs, parentDataRollingHash, storedDataRollingHash } = generateBlobDataSubmission(0, 2, true);

    const encodedCall = linethRollup.interface.encodeFunctionData("submitBlobs", [
      parentDataRollingHash,
      storedDataRollingHash,
    ]);

    const transaction = await buildBlobTransaction({
      linethRollupAddress,
      encodedCall,
      compressedBlobs: [compressedBlobs[0]],
    });

    const signedTx = await getWalletForIndex(2).signTransaction(transaction);

    // The single-blob fold produces a different accumulator than the declared 2-blob stored value.
    await expectRevertWithCustomError(
      linethRollup,
      ethers.provider.broadcastTransaction(signedTx),
      "DataRollingHashMismatch",
    );
  });
});
