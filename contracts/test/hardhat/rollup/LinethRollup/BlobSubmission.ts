import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { AddressFilter, TestLinethRollup } from "contracts/typechain-types";
import { BaseContract } from "ethers";
import { ethers } from "hardhat";

import firstCompressedDataContent from "../../_testData/compressedData/blocks-1-46.json";
import { GENERAL_PAUSE_TYPE, HASH_ZERO, OPERATOR_ROLE, STATE_DATA_SUBMISSION_PAUSE_TYPE } from "../../common/constants";
import {
  generateRandomBytes,
  generateKeccak256,
  buildAccessErrorMessage,
  expectRevertWithCustomError,
  expectRevertWithReason,
  generateBlobDataSubmission,
  expectEventDirectFromReceiptData,
  expectRevertWhenPaused,
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
  let linethRollup: TestLinethRollup;

  let securityCouncil: SignerWithAddress;
  let operator: SignerWithAddress;
  let nonAuthorizedAccount: SignerWithAddress;
  let addressFilterAddress: string;
  let addressFilter: AddressFilter;

  const { prevShnarf } = firstCompressedDataContent;

  before(async () => {
    ({ securityCouncil, operator, nonAuthorizedAccount } = await loadFixture(getAccountsFixture));
  });

  beforeEach(async () => {
    ({ linethRollup, addressFilter } = await loadFixture(deployForcedTransactionGatewayFixture));

    addressFilterAddress = await addressFilter.getAddress();

    await linethRollup.setLastFinalizedBlock(0);
    await linethRollup.setupParentShnarf(prevShnarf);
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
      blobDataSubmission[blobDataSubmission.length - 1].finalStateRootHash,
    ];

    expectEventDirectFromReceiptData(linethRollup as BaseContract, receipt!, "DataSubmittedV3", expectedEventArgs);

    const blobShnarfExists = await linethRollup.blobShnarfExists(finalShnarf);
    expect(blobShnarfExists).to.equal(1n);
  });

  it("Fails the blob submission when the parent shnarf is missing", async () => {
    const linethRollupAddress = await linethRollup.getAddress();
    const { blobDataSubmission, compressedBlobs } = generateBlobDataSubmission(0, 1);
    const nonExistingParentShnarf = generateRandomBytes(32);

    const wrongExpectedShnarf = generateKeccak256(
      ["bytes32", "bytes32", "bytes32", "bytes32", "bytes32"],
      [HASH_ZERO, HASH_ZERO, blobDataSubmission[0].finalStateRootHash, HASH_ZERO, HASH_ZERO],
    );

    const encodedCall = linethRollup.interface.encodeFunctionData("submitBlobs", [
      blobDataSubmission,
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
    const { blobDataSubmission, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

    await expectRevertWithReason(
      linethRollup.connect(nonAuthorizedAccount).submitBlobs(blobDataSubmission, parentShnarf, finalShnarf),
      buildAccessErrorMessage(nonAuthorizedAccount, OPERATOR_ROLE),
    );
  });

  // Parameterized pause type tests for blob submission
  const blobSubmissionPauseTypes = [
    { pauseType: GENERAL_PAUSE_TYPE, name: "GENERAL_PAUSE_TYPE" },
    { pauseType: STATE_DATA_SUBMISSION_PAUSE_TYPE, name: "STATE_DATA_SUBMISSION_PAUSE_TYPE" },
  ];

  blobSubmissionPauseTypes.forEach(({ pauseType, name }) => {
    it(`Should revert if ${name} is enabled`, async () => {
      const { blobDataSubmission, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

      await linethRollup.connect(securityCouncil).pauseByType(pauseType);

      await expectRevertWhenPaused(
        linethRollup,
        linethRollup.connect(operator).submitBlobs(blobDataSubmission, parentShnarf, finalShnarf),
        pauseType,
      );
    });
  });

  it("Should revert if the blob data is empty at any index", async () => {
    const linethRollupAddress = await linethRollup.getAddress();
    const { blobDataSubmission, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 2);

    const encodedCall = linethRollup.interface.encodeFunctionData("submitBlobs", [
      blobDataSubmission,
      parentShnarf,
      finalShnarf,
    ]);

    // Pass only the first blob to simulate empty blob at index 1
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

  it("Should fail if the final state root hash is empty", async () => {
    const linethRollupAddress = await linethRollup.getAddress();
    const { blobDataSubmission, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

    blobDataSubmission[0].finalStateRootHash = HASH_ZERO;

    const encodedCall = linethRollup.interface.encodeFunctionData("submitBlobs", [
      blobDataSubmission,
      parentShnarf,
      finalShnarf,
    ]);

    const transaction = await buildBlobTransaction({
      linethRollupAddress,
      encodedCall,
      compressedBlobs,
    });

    const signedTx = await getWalletForIndex(2).signTransaction(transaction);

    // TODO: Make the failure shnarf dynamic and computed
    await expectRevertWithCustomError(
      linethRollup,
      ethers.provider.broadcastTransaction(signedTx),
      "FinalShnarfWrong",
      [finalShnarf, "0x4337f604948e810af24274096031ee2bbf4d6bc87919c72dd87ede2c091803e1"],
    );
  });

  it("Should revert when snarkHash is zero hash", async () => {
    const linethRollupAddress = await linethRollup.getAddress();
    const { blobDataSubmission, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

    // Set the snarkHash to a random value for a specific index
    blobDataSubmission[0].snarkHash = generateRandomBytes(32);

    const encodedCall = linethRollup.interface.encodeFunctionData("submitBlobs", [
      blobDataSubmission,
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
      "PointEvaluationFailed",
    );
  });

  it("Should revert if the final shnarf is wrong", async () => {
    const linethRollupAddress = await linethRollup.getAddress();
    const { blobDataSubmission, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 2);
    const badFinalShnarf = generateRandomBytes(32);

    const encodedCall = linethRollup.interface.encodeFunctionData("submitBlobs", [
      blobDataSubmission,
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
    const { blobDataSubmission, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

    // Try to submit the same blob data again
    const encodedCall = linethRollup.interface.encodeFunctionData("submitBlobs", [
      blobDataSubmission,
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

  it("Should revert with PointEvaluationFailed when point evaluation fails", async () => {
    const linethRollupAddress = await linethRollup.getAddress();
    const { blobDataSubmission, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

    // Modify the kzgProof to an invalid value to trigger the PointEvaluationFailed revert
    blobDataSubmission[0].kzgProof = HASH_ZERO;

    const encodedCall = linethRollup.interface.encodeFunctionData("submitBlobs", [
      blobDataSubmission,
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
      "PointEvaluationFailed",
    );
  });

  it("Should revert if there is less data than blobs", async () => {
    const linethRollupAddress = await linethRollup.getAddress();

    const {
      blobDataSubmission: blobSubmission,
      compressedBlobs,
      parentShnarf,
      finalShnarf,
    } = generateBlobDataSubmission(0, 2, true);

    const encodedCall = linethRollup.interface.encodeFunctionData("submitBlobs", [
      [blobSubmission[0]],
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
