import { expect } from "chai";
import { network } from "hardhat";
import { describe, it, before, beforeEach } from "node:test";
import { encodeFunctionData } from "viem";

import firstCompressedDataContent from "../../_testData/compressedData/blocks-1-46.json" with { type: "json" };
import {
  GENERAL_PAUSE_TYPE,
  HASH_ZERO,
  OPERATOR_ROLE,
  STATE_DATA_SUBMISSION_PAUSE_TYPE,
} from "../../common/constants/index.js";
import {
  type TestAccount,
  hex,
  generateRandomBytes,
  generateKeccak256,
  buildAccessErrorMessage,
  generateBlobDataSubmission,
  expectEventDirectFromReceiptData,
  expectRevertWhenPaused,
  expectRevertWithCustomError,
  expectRevertWithReason,
} from "../../common/helpers/index.js";
import {
  deployForcedTransactionGatewayFixture,
  ensureKzgSetup,
  getAccountsFixture,
  getWalletForIndex,
  buildBlobTransaction,
  sendBlobTransaction,
  submitBlobsAndGetReceipt,
  toBlobSubmissionStructs,
} from "../helpers/index.js";

import type { ContractReturnType, PublicClient } from "@nomicfoundation/hardhat-viem/types";
import type { Address } from "viem";

ensureKzgSetup();

describe("Lineth Rollup contract: EIP-4844 Blob submission tests", () => {
  let linethRollup: ContractReturnType<"TestLinethRollup">;

  let securityCouncil: TestAccount;
  let operator: TestAccount;
  let nonAuthorizedAccount: TestAccount;
  let addressFilterAddress: Address;
  let addressFilter: ContractReturnType<"AddressFilter">;
  let publicClient: PublicClient;

  const prevShnarf = hex(firstCompressedDataContent.prevShnarf);

  before(async () => {
    const { viem, networkHelpers } = await network.getOrCreate();
    ({ securityCouncil, operator, nonAuthorizedAccount } = await networkHelpers.loadFixture(getAccountsFixture));
    publicClient = await viem.getPublicClient();
  });

  beforeEach(async () => {
    const { networkHelpers } = await network.getOrCreate();
    ({ linethRollup, addressFilter } = await networkHelpers.loadFixture(deployForcedTransactionGatewayFixture));

    addressFilterAddress = addressFilter.address;

    await linethRollup.write.setLastFinalizedBlock([0n]);
    await linethRollup.write.setupParentShnarf([prevShnarf]);
    await linethRollup.write.setAddressFilter([addressFilterAddress], { account: securityCouncil.account });
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

    expectEventDirectFromReceiptData(linethRollup, receipt, "DataSubmittedV3", [
      parentShnarf,
      finalShnarf,
      blobDataSubmission[blobDataSubmission.length - 1].finalStateRootHash,
    ]);

    const blobShnarfExists = await linethRollup.read.blobShnarfExists([finalShnarf]);
    expect(blobShnarfExists).to.equal(1n);
  });

  it("Fails the blob submission when the parent shnarf is missing", async () => {
    const linethRollupAddress = linethRollup.address;
    const { blobDataSubmission, compressedBlobs } = generateBlobDataSubmission(0, 1);
    const nonExistingParentShnarf = generateRandomBytes(32);

    const wrongExpectedShnarf = generateKeccak256(
      ["bytes32", "bytes32", "bytes32", "bytes32", "bytes32"],
      [HASH_ZERO, HASH_ZERO, blobDataSubmission[0].finalStateRootHash, HASH_ZERO, HASH_ZERO],
    );

    const encodedCall = encodeFunctionData({
      abi: linethRollup.abi,
      functionName: "submitBlobs",
      args: [toBlobSubmissionStructs(blobDataSubmission), nonExistingParentShnarf, wrongExpectedShnarf],
    });

    const transaction = await buildBlobTransaction({
      linethRollupAddress,
      encodedCall,
      compressedBlobs,
    });

    const operatorHDSigner = await getWalletForIndex(2);
    const signedTx = await operatorHDSigner.signTransaction(transaction);

    await expectRevertWithCustomError(
      linethRollup,
      publicClient.sendRawTransaction({ serializedTransaction: signedTx }),
      "ParentShnarfNotSubmitted",
      [nonExistingParentShnarf],
    );
  });

  it("Fails when the blob submission data is missing", async () => {
    const linethRollupAddress = linethRollup.address;
    const { compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

    const encodedCall = encodeFunctionData({
      abi: linethRollup.abi,
      functionName: "submitBlobs",
      args: [[], parentShnarf, finalShnarf],
    });

    const transaction = await buildBlobTransaction({
      linethRollupAddress,
      encodedCall,
      compressedBlobs,
    });

    const operatorHDSigner = await getWalletForIndex(2);
    const signedTx = await operatorHDSigner.signTransaction(transaction);

    await expectRevertWithCustomError(
      linethRollup,
      publicClient.sendRawTransaction({ serializedTransaction: signedTx }),
      "BlobSubmissionDataIsMissing",
    );
  });

  it("Should revert if the caller does not have the OPERATOR_ROLE", async () => {
    const { blobDataSubmission, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

    await expectRevertWithReason(
      linethRollup.write.submitBlobs([toBlobSubmissionStructs(blobDataSubmission), parentShnarf, finalShnarf], {
        account: nonAuthorizedAccount.account,
      }),
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

      await linethRollup.write.pauseByType([pauseType], { account: securityCouncil.account });

      await expectRevertWhenPaused(
        linethRollup,
        linethRollup.write.submitBlobs([toBlobSubmissionStructs(blobDataSubmission), parentShnarf, finalShnarf], {
          account: operator.account,
        }),
        pauseType,
      );
    });
  });

  it("Should revert if the blob data is empty at any index", async () => {
    const linethRollupAddress = linethRollup.address;
    const { blobDataSubmission, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 2);

    const encodedCall = encodeFunctionData({
      abi: linethRollup.abi,
      functionName: "submitBlobs",
      args: [toBlobSubmissionStructs(blobDataSubmission), parentShnarf, finalShnarf],
    });

    // Pass only the first blob to simulate empty blob at index 1
    const transaction = await buildBlobTransaction({
      linethRollupAddress,
      encodedCall,
      compressedBlobs: [compressedBlobs[0]],
    });

    const operatorHDSigner = await getWalletForIndex(2);
    const signedTx = await operatorHDSigner.signTransaction(transaction);

    await expectRevertWithCustomError(
      linethRollup,
      publicClient.sendRawTransaction({ serializedTransaction: signedTx }),
      "EmptyBlobDataAtIndex",
      [1n],
    );
  });

  it("Should fail if the final state root hash is empty", async () => {
    const linethRollupAddress = linethRollup.address;
    const { blobDataSubmission, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

    blobDataSubmission[0].finalStateRootHash = HASH_ZERO;

    const encodedCall = encodeFunctionData({
      abi: linethRollup.abi,
      functionName: "submitBlobs",
      args: [toBlobSubmissionStructs(blobDataSubmission), parentShnarf, finalShnarf],
    });

    const transaction = await buildBlobTransaction({
      linethRollupAddress,
      encodedCall,
      compressedBlobs,
    });

    const operatorHDSigner = await getWalletForIndex(2);
    const signedTx = await operatorHDSigner.signTransaction(transaction);

    // TODO: Make the failure shnarf dynamic and computed
    await expectRevertWithCustomError(
      linethRollup,
      publicClient.sendRawTransaction({ serializedTransaction: signedTx }),
      "FinalShnarfWrong",
      [finalShnarf, "0x4337f604948e810af24274096031ee2bbf4d6bc87919c72dd87ede2c091803e1"],
    );
  });

  it("Should revert when snarkHash is zero hash", async () => {
    const linethRollupAddress = linethRollup.address;
    const { blobDataSubmission, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

    // Set the snarkHash to a random value for a specific index
    blobDataSubmission[0].snarkHash = generateRandomBytes(32);

    const encodedCall = encodeFunctionData({
      abi: linethRollup.abi,
      functionName: "submitBlobs",
      args: [toBlobSubmissionStructs(blobDataSubmission), parentShnarf, finalShnarf],
    });

    const transaction = await buildBlobTransaction({
      linethRollupAddress,
      encodedCall,
      compressedBlobs,
    });

    const operatorHDSigner = await getWalletForIndex(2);
    const signedTx = await operatorHDSigner.signTransaction(transaction);

    await expectRevertWithCustomError(
      linethRollup,
      publicClient.sendRawTransaction({ serializedTransaction: signedTx }),
      "PointEvaluationFailed",
    );
  });

  it("Should revert if the final shnarf is wrong", async () => {
    const linethRollupAddress = linethRollup.address;
    const { blobDataSubmission, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 2);
    const badFinalShnarf = generateRandomBytes(32);

    const encodedCall = encodeFunctionData({
      abi: linethRollup.abi,
      functionName: "submitBlobs",
      args: [toBlobSubmissionStructs(blobDataSubmission), parentShnarf, badFinalShnarf],
    });

    const transaction = await buildBlobTransaction({
      linethRollupAddress,
      encodedCall,
      compressedBlobs,
    });

    const operatorHDSigner = await getWalletForIndex(2);
    const signedTx = await operatorHDSigner.signTransaction(transaction);

    await expectRevertWithCustomError(
      linethRollup,
      publicClient.sendRawTransaction({ serializedTransaction: signedTx }),
      "FinalShnarfWrong",
      [badFinalShnarf, finalShnarf],
    );
  });

  it("Should revert if the data has already been submitted", async () => {
    await sendBlobTransaction(linethRollup, 0, 1);

    const linethRollupAddress = linethRollup.address;
    const { blobDataSubmission, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

    // Try to submit the same blob data again
    const encodedCall = encodeFunctionData({
      abi: linethRollup.abi,
      functionName: "submitBlobs",
      args: [toBlobSubmissionStructs(blobDataSubmission), parentShnarf, finalShnarf],
    });

    const transaction = await buildBlobTransaction({
      linethRollupAddress,
      encodedCall,
      compressedBlobs,
    });

    const operatorHDSigner = await getWalletForIndex(2);
    const signedTx = await operatorHDSigner.signTransaction(transaction);

    await expectRevertWithCustomError(
      linethRollup,
      publicClient.sendRawTransaction({ serializedTransaction: signedTx }),
      "ShnarfAlreadySubmitted",
      [finalShnarf],
    );
  });

  it("Should revert with PointEvaluationFailed when point evaluation fails", async () => {
    const linethRollupAddress = linethRollup.address;
    const { blobDataSubmission, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

    // Modify the kzgProof to an invalid value to trigger the PointEvaluationFailed revert
    blobDataSubmission[0].kzgProof = HASH_ZERO;

    const encodedCall = encodeFunctionData({
      abi: linethRollup.abi,
      functionName: "submitBlobs",
      args: [toBlobSubmissionStructs(blobDataSubmission), parentShnarf, finalShnarf],
    });

    const transaction = await buildBlobTransaction({
      linethRollupAddress,
      encodedCall,
      compressedBlobs,
    });

    const operatorHDSigner = await getWalletForIndex(2);
    const signedTx = await operatorHDSigner.signTransaction(transaction);

    await expectRevertWithCustomError(
      linethRollup,
      publicClient.sendRawTransaction({ serializedTransaction: signedTx }),
      "PointEvaluationFailed",
    );
  });

  it("Should revert if there is less data than blobs", async () => {
    const linethRollupAddress = linethRollup.address;

    const {
      blobDataSubmission: blobSubmission,
      compressedBlobs,
      parentShnarf,
      finalShnarf,
    } = generateBlobDataSubmission(0, 2, true);

    const encodedCall = encodeFunctionData({
      abi: linethRollup.abi,
      functionName: "submitBlobs",
      args: [toBlobSubmissionStructs([blobSubmission[0]]), parentShnarf, finalShnarf],
    });

    const transaction = await buildBlobTransaction({
      linethRollupAddress,
      encodedCall,
      compressedBlobs,
    });

    const operatorHDSigner = await getWalletForIndex(2);
    const signedTx = await operatorHDSigner.signTransaction(transaction);

    await expectRevertWithCustomError(
      linethRollup,
      publicClient.sendRawTransaction({ serializedTransaction: signedTx }),
      "BlobSubmissionDataEmpty",
      [1n],
    );
  });
});
