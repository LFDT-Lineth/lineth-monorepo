import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import * as fs from "fs";
import * as kzg from "c-kzg";
import { expect } from "chai";
import { BaseContract, Transaction } from "ethers";
import { ethers } from "hardhat";

import betaV4_1_BlobData_Sepolia from "../../_testData/betaV4_1_sepolia/20106872-20107472-bcv0.0-ccv0.0-d27541dec7352a8bb7067b2e061e9d2de9af04502a915f5f69a456aebe2ae8f0-getZkBlobCompressionProof.json";
import betaV4_1_FinalizationData_Sepolia from "../../_testData/betaV4_1_sepolia/proof/20106872-20107472-da29c954a5734a97a851964dc88ec7034f5f3ebd569d3f88b43acc00c9263124-getZkAggregatedProof.json";

import betaV4_1_BlobData_Mainnet from "../../_testData/betaV4_1_mainnet/25472845-25473115-bcv0.0-ccv0.0-81da22585f32b849b1d3ec3dd1aa0c027b517112f3b3bcc4aad086d59967af30-getZkBlobCompressionProof.json";
import betaV4_1_FinalizationData_Mainnet from "../../_testData/betaV4_1_mainnet/proof/25472845-25473115-e76c58a6fad553315848a4e8f1793bfef5667a5434de1fdbc45d4a324c3301d6-getZkAggregatedProof.json";

import blobAggregatedProof1To155 from "../../_testData/compressedDataEip4844/aggregatedProof-1-155.json";
import blobMultipleAggregatedProof1To81 from "../../_testData/compressedDataEip4844/multipleProofs/aggregatedProof-1-81.json";
import firstCompressedDataContent from "../../_testData/compressedData/blocks-1-46.json";
import secondCompressedDataContent from "../../_testData/compressedData/blocks-47-81.json";
import fourthCompressedDataContent from "../../_testData/compressedData/blocks-115-155.json";

import { LINEA_ROLLUP_PAUSE_TYPES_ROLES, LINEA_ROLLUP_UNPAUSE_TYPES_ROLES } from "contracts/common/constants";
import { TestLineaRollup } from "contracts/typechain-types";
import {
  deployLineaRollupFixture,
  deployRevertingVerifier,
  expectSuccessfulFinalize,
  getAccountsFixture,
  getVersionedBlobFiles,
  getRoleAddressesFixture,
  getWalletForIndex,
  sendBlobTransaction,
  sendVersionedBlobTransactionFromFile,
} from "../helpers";
import {
  FALLBACK_OPERATOR_ADDRESS,
  GENERAL_PAUSE_TYPE,
  HASH_ZERO,
  INITIAL_WITHDRAW_LIMIT,
  ONE_DAY_IN_SECONDS,
  OPERATOR_ROLE,
  TEST_PUBLIC_VERIFIER_INDEX,
  LINEA_ROLLUP_INITIALIZE_SIGNATURE,
  BLOB_SUBMISSION_PAUSE_TYPE,
} from "../../common/constants";
import { deployUpgradableFromFactory } from "../../common/deployment";
import {
  generateFinalizationData,
  generateRandomBytes,
  generateKeccak256,
  expectEvent,
  buildAccessErrorMessage,
  expectRevertWithCustomError,
  expectRevertWithReason,
  generateBlobDataSubmission,
  generateBlobParentShnarfData,
  expectEventDirectFromReceiptData,
} from "../../common/helpers";

describe("Linea Rollup contract: EIP-4844 Blob submission tests", () => {
  let lineaRollup: TestLineaRollup;
  let revertingVerifier: string;

  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  let securityCouncil: SignerWithAddress;
  let operator: SignerWithAddress;
  let nonAuthorizedAccount: SignerWithAddress;
  let roleAddresses: { addressWithRole: string; role: string }[];
  const { prevShnarf } = firstCompressedDataContent;

  before(async () => {
    ({ securityCouncil, operator, nonAuthorizedAccount } = await loadFixture(getAccountsFixture));
    roleAddresses = await loadFixture(getRoleAddressesFixture);
  });

  beforeEach(async () => {
    ({ lineaRollup } = await loadFixture(deployLineaRollupFixture));
    await lineaRollup.setLastFinalizedBlock(0);
    await lineaRollup.setupParentShnarf(prevShnarf);
  });

  it("Should successfully submit blobs", async () => {
    const operatorHDSigner = getWalletForIndex(2);
    const lineaRollupAddress = await lineaRollup.getAddress();
    const { blobDataSubmission, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

    const encodedCall = lineaRollup.interface.encodeFunctionData("submitBlobs", [
      blobDataSubmission,
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
      nonce,
      value: 0,
      gasLimit: 5_000_000,
      kzg,
      maxFeePerBlobGas: 1n,
      blobs: compressedBlobs,
    });

    const signedTx = await operatorHDSigner.signTransaction(transaction);

    const txResponse = await ethers.provider.broadcastTransaction(signedTx);
    const receipt = await ethers.provider.getTransactionReceipt(txResponse.hash);
    expect(receipt).is.not.null;

    const expectedEventArgs = [
      parentShnarf,
      finalShnarf,
      blobDataSubmission[blobDataSubmission.length - 1].finalStateRootHash,
    ];

    expectEventDirectFromReceiptData(lineaRollup as BaseContract, receipt!, "DataSubmittedV3", expectedEventArgs);

    const blobShnarfExists = await lineaRollup.blobShnarfExists(finalShnarf);
    expect(blobShnarfExists).to.equal(1n);
  });

  it("Fails the blob submission when the parent shnarf is missing", async () => {
    const operatorHDSigner = getWalletForIndex(2);

    const lineaRollupAddress = await lineaRollup.getAddress();
    const { blobDataSubmission, compressedBlobs, finalShnarf } = generateBlobDataSubmission(0, 1);
    const nonExistingParentShnarf = generateRandomBytes(32);

    const encodedCall = lineaRollup.interface.encodeFunctionData("submitBlobs", [
      blobDataSubmission,
      nonExistingParentShnarf,
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
      nonce,
      value: 0,
      gasLimit: 5_000_000,
      kzg,
      maxFeePerBlobGas: 1n,
      blobs: compressedBlobs,
    });

    const signedTx = await operatorHDSigner.signTransaction(transaction);

    await expectRevertWithCustomError(
      lineaRollup,
      ethers.provider.broadcastTransaction(signedTx),
      "ParentBlobNotSubmitted",
      [nonExistingParentShnarf],
    );
  });

  it("Fails when the blob submission data is missing", async () => {
    const operatorHDSigner = getWalletForIndex(2);

    const lineaRollupAddress = await lineaRollup.getAddress();
    const { compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

    const encodedCall = lineaRollup.interface.encodeFunctionData("submitBlobs", [[], parentShnarf, finalShnarf]);

    const { maxFeePerGas, maxPriorityFeePerGas } = await ethers.provider.getFeeData();
    const nonce = await operatorHDSigner.getNonce();

    const transaction = Transaction.from({
      data: encodedCall,
      maxPriorityFeePerGas: maxPriorityFeePerGas!,
      maxFeePerGas: maxFeePerGas!,
      to: lineaRollupAddress,
      chainId: (await ethers.provider.getNetwork()).chainId,
      type: 3,
      nonce,
      value: 0,
      gasLimit: 5_000_000,
      kzg,
      maxFeePerBlobGas: 1n,
      blobs: compressedBlobs,
    });

    const signedTx = await operatorHDSigner.signTransaction(transaction);

    await expectRevertWithCustomError(
      lineaRollup,
      ethers.provider.broadcastTransaction(signedTx),
      "BlobSubmissionDataIsMissing",
    );
  });

  it("Should revert if the caller does not have the OPERATOR_ROLE", async () => {
    const { blobDataSubmission, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

    await expectRevertWithReason(
      lineaRollup.connect(nonAuthorizedAccount).submitBlobs(blobDataSubmission, parentShnarf, finalShnarf),
      buildAccessErrorMessage(nonAuthorizedAccount, OPERATOR_ROLE),
    );
  });

  it("Should revert if GENERAL_PAUSE_TYPE is enabled", async () => {
    const { blobDataSubmission, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

    await lineaRollup.connect(securityCouncil).pauseByType(GENERAL_PAUSE_TYPE);

    await expectRevertWithCustomError(
      lineaRollup,
      lineaRollup.connect(operator).submitBlobs(blobDataSubmission, parentShnarf, finalShnarf),
      "IsPaused",
      [GENERAL_PAUSE_TYPE],
    );
  });

  it("Should revert if BLOB_SUBMISSION_PAUSE_TYPE is enabled", async () => {
    const { blobDataSubmission, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

    await lineaRollup.connect(securityCouncil).pauseByType(BLOB_SUBMISSION_PAUSE_TYPE);

    await expectRevertWithCustomError(
      lineaRollup,
      lineaRollup.connect(operator).submitBlobs(blobDataSubmission, parentShnarf, finalShnarf),
      "IsPaused",
      [BLOB_SUBMISSION_PAUSE_TYPE],
    );
  });

  it("Should revert if the blob data is empty at any index", async () => {
    const operatorHDSigner = getWalletForIndex(2);
    const lineaRollupAddress = await lineaRollup.getAddress();
    const { blobDataSubmission, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 2);

    const encodedCall = lineaRollup.interface.encodeFunctionData("submitBlobs", [
      blobDataSubmission,
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
      nonce,
      value: 0,
      gasLimit: 5_000_000,
      kzg,
      maxFeePerBlobGas: 1n,
      blobs: [compressedBlobs[0]],
    });

    const signedTx = await operatorHDSigner.signTransaction(transaction);

    await expectRevertWithCustomError(
      lineaRollup,
      ethers.provider.broadcastTransaction(signedTx),
      "EmptyBlobDataAtIndex",
      [1n],
    );
  });

  it("Should fail if the final state root hash is empty", async () => {
    const operatorHDSigner = getWalletForIndex(2);

    const lineaRollupAddress = await lineaRollup.getAddress();
    const { blobDataSubmission, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

    blobDataSubmission[0].finalStateRootHash = HASH_ZERO;

    const encodedCall = lineaRollup.interface.encodeFunctionData("submitBlobs", [
      blobDataSubmission,
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
      nonce,
      value: 0,
      gasLimit: 5_000_000,
      kzg,
      maxFeePerBlobGas: 1n,
      blobs: compressedBlobs,
    });

    const signedTx = await operatorHDSigner.signTransaction(transaction);

    // TODO: Make the failure shnarf dynamic and computed
    await expectRevertWithCustomError(lineaRollup, ethers.provider.broadcastTransaction(signedTx), "FinalShnarfWrong", [
      finalShnarf,
      "0x22f8fb954df8328627fe9c48b60192f4d970a92891417aaadea39300ca244d36",
    ]);
  });

  it("Should revert when snarkHash is zero hash", async () => {
    const operatorHDSigner = getWalletForIndex(2);

    const lineaRollupAddress = await lineaRollup.getAddress();
    const { blobDataSubmission, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

    // Set the snarkHash to HASH_ZERO for a specific index
    const emptyDataIndex = 0;
    blobDataSubmission[emptyDataIndex].snarkHash = generateRandomBytes(32);

    const encodedCall = lineaRollup.interface.encodeFunctionData("submitBlobs", [
      blobDataSubmission,
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
      nonce,
      value: 0,
      gasLimit: 5_000_000,
      kzg,
      maxFeePerBlobGas: 1n,
      blobs: compressedBlobs,
    });

    const signedTx = await operatorHDSigner.signTransaction(transaction);

    await expectRevertWithCustomError(
      lineaRollup,
      ethers.provider.broadcastTransaction(signedTx),
      "PointEvaluationFailed",
    );
  });

  it("Should revert if the final shnarf is wrong", async () => {
    const operatorHDSigner = getWalletForIndex(2);
    const lineaRollupAddress = await lineaRollup.getAddress();
    const { blobDataSubmission, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 2);
    const badFinalShnarf = generateRandomBytes(32);

    const encodedCall = lineaRollup.interface.encodeFunctionData("submitBlobs", [
      blobDataSubmission,
      parentShnarf,
      badFinalShnarf,
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
      nonce,
      value: 0,
      gasLimit: 5_000_000,
      kzg,
      maxFeePerBlobGas: 1n,
      blobs: compressedBlobs,
    });

    const signedTx = await operatorHDSigner.signTransaction(transaction);

    await expectRevertWithCustomError(lineaRollup, ethers.provider.broadcastTransaction(signedTx), "FinalShnarfWrong", [
      badFinalShnarf,
      finalShnarf,
    ]);
  });

  it("Should revert if the data has already been submitted", async () => {
    await sendBlobTransaction(lineaRollup, 0, 1);

    const operatorHDSigner = getWalletForIndex(2);

    const lineaRollupAddress = await lineaRollup.getAddress();
    const { blobDataSubmission, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

    // Try to submit the same blob data again
    const encodedCall2 = lineaRollup.interface.encodeFunctionData("submitBlobs", [
      blobDataSubmission,
      parentShnarf,
      finalShnarf,
    ]);

    const { maxFeePerGas: maxFeePerGas2, maxPriorityFeePerGas: maxPriorityFeePerGas2 } =
      await ethers.provider.getFeeData();
    const nonce2 = await operatorHDSigner.getNonce();

    const transaction2 = Transaction.from({
      data: encodedCall2,
      maxPriorityFeePerGas: maxPriorityFeePerGas2!,
      maxFeePerGas: maxFeePerGas2!,
      to: lineaRollupAddress,
      chainId: (await ethers.provider.getNetwork()).chainId,
      type: 3,
      nonce: nonce2,
      value: 0,
      gasLimit: 5_000_000,
      kzg,
      maxFeePerBlobGas: 1n,
      blobs: compressedBlobs,
    });

    const signedTx2 = await operatorHDSigner.signTransaction(transaction2);

    await expectRevertWithCustomError(
      lineaRollup,
      ethers.provider.broadcastTransaction(signedTx2),
      "DataAlreadySubmitted",
      [finalShnarf],
    );
  });

  it("Should revert with PointEvaluationFailed when point evaluation fails", async () => {
    const operatorHDSigner = getWalletForIndex(2);

    const lineaRollupAddress = await lineaRollup.getAddress();
    const { blobDataSubmission, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

    // Modify the kzgProof to an invalid value to trigger the PointEvaluationFailed revert
    blobDataSubmission[0].kzgProof = HASH_ZERO;

    const encodedCall = lineaRollup.interface.encodeFunctionData("submitBlobs", [
      blobDataSubmission,
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
      nonce,
      value: 0,
      gasLimit: 5_000_000,
      kzg,
      maxFeePerBlobGas: 1n,
      blobs: compressedBlobs,
    });

    const signedTx = await operatorHDSigner.signTransaction(transaction);

    await expectRevertWithCustomError(
      lineaRollup,
      ethers.provider.broadcastTransaction(signedTx),
      "PointEvaluationFailed",
    );
  });

  it("Should submit 2 blobs, then submit another 2 blobs and finalize", async () => {
    // Submit 2 blobs
    await sendBlobTransaction(lineaRollup, 0, 2);
    // Submit another 2 blobs
    await sendBlobTransaction(lineaRollup, 2, 4);
    // Finalize 4 blobs
    await expectSuccessfulFinalize(
      lineaRollup,
      operator,
      blobAggregatedProof1To155,
      4,
      fourthCompressedDataContent.finalStateRootHash,
      generateBlobParentShnarfData,
    );
  });

  it("Should revert if there is less data than blobs", async () => {
    const operatorHDSigner = getWalletForIndex(2);
    const lineaRollupAddress = await lineaRollup.getAddress();

    const {
      blobDataSubmission: blobSubmission,
      compressedBlobs: compressedBlobs,
      parentShnarf: parentShnarf,
      finalShnarf: finalShnarf,
    } = generateBlobDataSubmission(0, 2, true);

    const encodedCall = lineaRollup.interface.encodeFunctionData("submitBlobs", [
      [blobSubmission[0]],
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
    await expectRevertWithCustomError(
      lineaRollup,
      ethers.provider.broadcastTransaction(signedTx),
      "BlobSubmissionDataEmpty",
      [1],
    );
  });

  it("Should fail to finalize with not enough gas for the rollup (pre-verifier)", async () => {
    // Submit 2 blobs
    await sendBlobTransaction(lineaRollup, 0, 2);
    // Submit another 2 blobs
    await sendBlobTransaction(lineaRollup, 2, 4);

    // Finalize 4 blobs
    const finalizationData = await generateFinalizationData({
      l1RollingHash: blobAggregatedProof1To155.l1RollingHash,
      l1RollingHashMessageNumber: BigInt(blobAggregatedProof1To155.l1RollingHashMessageNumber),
      lastFinalizedTimestamp: BigInt(blobAggregatedProof1To155.parentAggregationLastBlockTimestamp),
      endBlockNumber: BigInt(blobAggregatedProof1To155.finalBlockNumber),
      parentStateRootHash: blobAggregatedProof1To155.parentStateRootHash,
      finalTimestamp: BigInt(blobAggregatedProof1To155.finalTimestamp),
      l2MerkleRoots: blobAggregatedProof1To155.l2MerkleRoots,
      l2MerkleTreesDepth: BigInt(blobAggregatedProof1To155.l2MerkleTreesDepth),
      l2MessagingBlocksOffsets: blobAggregatedProof1To155.l2MessagingBlocksOffsets,
      aggregatedProof: blobAggregatedProof1To155.aggregatedProof,
      shnarfData: generateBlobParentShnarfData(4, false),
    });
    finalizationData.lastFinalizedL1RollingHash = HASH_ZERO;
    finalizationData.lastFinalizedL1RollingHashMessageNumber = 0n;

    await lineaRollup.setRollingHash(
      blobAggregatedProof1To155.l1RollingHashMessageNumber,
      blobAggregatedProof1To155.l1RollingHash,
    );

    const finalizeCompressedCall = lineaRollup
      .connect(operator)
      .finalizeBlocks(blobAggregatedProof1To155.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData, {
        gasLimit: 80_000,
      });

    // there is no reason
    await expect(finalizeCompressedCall).to.be.reverted;
  });

  it("Should fail to finalize with not enough gas to verify", async () => {
    // Submit 2 blobs
    await sendBlobTransaction(lineaRollup, 0, 2);
    // Submit another 2 blobs
    await sendBlobTransaction(lineaRollup, 2, 4);

    // Finalize 4 blobs
    const finalizationData = await generateFinalizationData({
      l1RollingHash: blobAggregatedProof1To155.l1RollingHash,
      l1RollingHashMessageNumber: BigInt(blobAggregatedProof1To155.l1RollingHashMessageNumber),
      lastFinalizedTimestamp: BigInt(blobAggregatedProof1To155.parentAggregationLastBlockTimestamp),
      endBlockNumber: BigInt(blobAggregatedProof1To155.finalBlockNumber),
      parentStateRootHash: blobAggregatedProof1To155.parentStateRootHash,
      finalTimestamp: BigInt(blobAggregatedProof1To155.finalTimestamp),
      l2MerkleRoots: blobAggregatedProof1To155.l2MerkleRoots,
      l2MerkleTreesDepth: BigInt(blobAggregatedProof1To155.l2MerkleTreesDepth),
      l2MessagingBlocksOffsets: blobAggregatedProof1To155.l2MessagingBlocksOffsets,
      aggregatedProof: blobAggregatedProof1To155.aggregatedProof,
      shnarfData: generateBlobParentShnarfData(4, false),
    });
    finalizationData.lastFinalizedL1RollingHash = HASH_ZERO;
    finalizationData.lastFinalizedL1RollingHashMessageNumber = 0n;

    await lineaRollup.setRollingHash(
      blobAggregatedProof1To155.l1RollingHashMessageNumber,
      blobAggregatedProof1To155.l1RollingHash,
    );

    const finalizeCompressedCall = lineaRollup
      .connect(operator)
      .finalizeBlocks(blobAggregatedProof1To155.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData, {
        gasLimit: 400000,
      });

    await expectRevertWithCustomError(
      lineaRollup,
      finalizeCompressedCall,
      "InvalidProofOrProofVerificationRanOutOfGas",
      ["error pairing"],
    );
  });

  const testCases = [
    { revertScenario: 0n, title: "Should fail to finalize via EMPTY_REVERT scenario with 'Unknown'" },
    { revertScenario: 1n, title: "Should fail to finalize via GAS_GUZZLE scenario with 'Unknown'" },
  ];

  testCases.forEach(({ revertScenario, title }) => {
    it(title, async () => {
      revertingVerifier = await deployRevertingVerifier(revertScenario);
      await lineaRollup.connect(securityCouncil).setVerifierAddress(revertingVerifier, 0);

      // Submit 2 blobs
      await sendBlobTransaction(lineaRollup, 0, 2);
      // Submit another 2 blobs
      await sendBlobTransaction(lineaRollup, 2, 4);

      // Finalize 4 blobs
      const finalizationData = await generateFinalizationData({
        l1RollingHash: blobAggregatedProof1To155.l1RollingHash,
        l1RollingHashMessageNumber: BigInt(blobAggregatedProof1To155.l1RollingHashMessageNumber),
        lastFinalizedTimestamp: BigInt(blobAggregatedProof1To155.parentAggregationLastBlockTimestamp),
        endBlockNumber: BigInt(blobAggregatedProof1To155.finalBlockNumber),
        parentStateRootHash: blobAggregatedProof1To155.parentStateRootHash,
        finalTimestamp: BigInt(blobAggregatedProof1To155.finalTimestamp),
        l2MerkleRoots: blobAggregatedProof1To155.l2MerkleRoots,
        l2MerkleTreesDepth: BigInt(blobAggregatedProof1To155.l2MerkleTreesDepth),
        l2MessagingBlocksOffsets: blobAggregatedProof1To155.l2MessagingBlocksOffsets,
        aggregatedProof: blobAggregatedProof1To155.aggregatedProof,
        shnarfData: generateBlobParentShnarfData(4, false),
      });
      finalizationData.lastFinalizedL1RollingHash = HASH_ZERO;
      finalizationData.lastFinalizedL1RollingHashMessageNumber = 0n;

      await lineaRollup.setRollingHash(
        blobAggregatedProof1To155.l1RollingHashMessageNumber,
        blobAggregatedProof1To155.l1RollingHash,
      );

      const finalizeCompressedCall = lineaRollup
        .connect(operator)
        .finalizeBlocks(blobAggregatedProof1To155.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData, {
          gasLimit: 400000,
        });

      await expectRevertWithCustomError(
        lineaRollup,
        finalizeCompressedCall,
        "InvalidProofOrProofVerificationRanOutOfGas",
        ["Unknown"],
      );
    });
  });

  it("Should successfully submit 2 blobs twice then finalize in two separate finalizations", async () => {
    // Submit 2 blobs
    await sendBlobTransaction(lineaRollup, 0, 2, true);
    // Submit another 2 blobs
    await sendBlobTransaction(lineaRollup, 2, 4, true);
    // Finalize first 2 blobs
    await expectSuccessfulFinalize(
      lineaRollup,
      operator,
      blobMultipleAggregatedProof1To81,
      2,
      secondCompressedDataContent.finalStateRootHash,
      generateBlobParentShnarfData,
      true,
    );
  });

  it("Should fail to prove if last finalized is higher than proving range", async () => {
    // Submit 2 blobs
    await sendBlobTransaction(lineaRollup, 0, 2, true);
    // Submit another 2 blobs
    await sendBlobTransaction(lineaRollup, 2, 4, true);

    await lineaRollup.setLastFinalizedBlock(10_000_000);

    const finalizationData = await generateFinalizationData({
      l1RollingHash: blobAggregatedProof1To155.l1RollingHash,
      l1RollingHashMessageNumber: BigInt(blobAggregatedProof1To155.l1RollingHashMessageNumber),
      lastFinalizedTimestamp: BigInt(blobAggregatedProof1To155.parentAggregationLastBlockTimestamp),
      endBlockNumber: BigInt(blobAggregatedProof1To155.finalBlockNumber),
      parentStateRootHash: HASH_ZERO, // Manipulate for bypass
      finalTimestamp: BigInt(blobAggregatedProof1To155.finalTimestamp),
      l2MerkleRoots: blobAggregatedProof1To155.l2MerkleRoots,
      l2MerkleTreesDepth: BigInt(blobAggregatedProof1To155.l2MerkleTreesDepth),
      l2MessagingBlocksOffsets: blobAggregatedProof1To155.l2MessagingBlocksOffsets,
      aggregatedProof: blobAggregatedProof1To155.aggregatedProof,
      shnarfData: generateBlobParentShnarfData(4, false),
      lastFinalizedL1RollingHash: HASH_ZERO,
      lastFinalizedL1RollingHashMessageNumber: 0n,
    });

    await lineaRollup.setRollingHash(
      blobAggregatedProof1To155.l1RollingHashMessageNumber,
      blobAggregatedProof1To155.l1RollingHash,
    );

    await lineaRollup.setLastFinalizedBlock(10_000_000);

    expectRevertWithCustomError(
      lineaRollup,
      lineaRollup
        .connect(operator)
        .finalizeBlocks(blobAggregatedProof1To155.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData),
      "InvalidProof",
    );
  });

  describe("Prover Beta V4_1", () => {
    it("Can submit blobs and finalize with Prover Beta V4_1 on Sepolia", async () => {
      // *** ARRANGE ***
      // custom verifier deploy.

      const sepoliaFullVerifierFactory = await ethers.getContractFactory(
        "src/verifiers/beta-v4.1/PlonkVerifierSepolia.sol:PlonkVerifierSepoliaFull",
      );
      const verifier = await sepoliaFullVerifierFactory.deploy();
      await verifier.waitForDeployment();
      const mainnetFullVerifier = await verifier.getAddress();

      // Deploy and initialize LineaRollup
      const initializationData = {
        initialStateRootHash: betaV4_1_FinalizationData_Sepolia.parentStateRootHash,
        initialL2BlockNumber: betaV4_1_FinalizationData_Sepolia.lastFinalizedBlockNumber,
        genesisTimestamp: betaV4_1_FinalizationData_Sepolia.parentAggregationLastBlockTimestamp,
        defaultVerifier: mainnetFullVerifier,
        rateLimitPeriodInSeconds: ONE_DAY_IN_SECONDS,
        rateLimitAmountInWei: INITIAL_WITHDRAW_LIMIT,
        roleAddresses,
        pauseTypeRoles: LINEA_ROLLUP_PAUSE_TYPES_ROLES,
        unpauseTypeRoles: LINEA_ROLLUP_UNPAUSE_TYPES_ROLES,
        fallbackOperator: FALLBACK_OPERATOR_ADDRESS,
        defaultAdmin: securityCouncil.address,
      };

      const betaV4_1_LineaRollup = (await deployUpgradableFromFactory("TestLineaRollup", [initializationData], {
        initializer: LINEA_ROLLUP_INITIALIZE_SIGNATURE,
        unsafeAllow: ["constructor", "incorrect-initializer-order"],
      })) as unknown as TestLineaRollup;

      await betaV4_1_LineaRollup.setupParentShnarf(betaV4_1_FinalizationData_Sepolia.parentAggregationFinalShnarf);

      // Send blobs
      const blobFiles = getVersionedBlobFiles("betaV4_1_sepolia");

      await betaV4_1_LineaRollup.setupParentShnarf(betaV4_1_BlobData_Sepolia.prevShnarf);

      for (let i = 0; i < blobFiles.length; i++) {
        await sendVersionedBlobTransactionFromFile(lineaRollup, blobFiles[i], betaV4_1_LineaRollup, "betaV4_1_sepolia");
      }

      const finalBlobFile = JSON.parse(
        fs.readFileSync(`${__dirname}/../../_testData/betaV4_1_sepolia/${blobFiles.slice(-1)[0]}`, "utf-8"),
      );

      // Setup finalize call
      const finalizationData = await generateFinalizationData({
        l1RollingHash: betaV4_1_FinalizationData_Sepolia.l1RollingHash,
        l1RollingHashMessageNumber: BigInt(betaV4_1_FinalizationData_Sepolia.l1RollingHashMessageNumber),
        lastFinalizedTimestamp: BigInt(betaV4_1_FinalizationData_Sepolia.parentAggregationLastBlockTimestamp),
        endBlockNumber: BigInt(betaV4_1_FinalizationData_Sepolia.finalBlockNumber),
        parentStateRootHash: betaV4_1_FinalizationData_Sepolia.parentStateRootHash,
        finalTimestamp: BigInt(betaV4_1_FinalizationData_Sepolia.finalTimestamp),
        l2MerkleRoots: betaV4_1_FinalizationData_Sepolia.l2MerkleRoots,
        l2MerkleTreesDepth: BigInt(betaV4_1_FinalizationData_Sepolia.l2MerkleTreesDepth),
        l2MessagingBlocksOffsets: betaV4_1_FinalizationData_Sepolia.l2MessagingBlocksOffsets,
        aggregatedProof: betaV4_1_FinalizationData_Sepolia.aggregatedProof,
        shnarfData: {
          parentShnarf: finalBlobFile.prevShnarf,
          snarkHash: finalBlobFile.snarkHash,
          finalStateRootHash: finalBlobFile.finalStateRootHash,
          dataEvaluationPoint: finalBlobFile.expectedX,
          dataEvaluationClaim: finalBlobFile.expectedY,
        },
      });

      finalizationData.lastFinalizedL1RollingHash =
        betaV4_1_FinalizationData_Sepolia.parentAggregationLastL1RollingHash;
      finalizationData.lastFinalizedL1RollingHashMessageNumber = BigInt(
        betaV4_1_FinalizationData_Sepolia.parentAggregationLastL1RollingHashMessageNumber,
      );

      // Setup LineaRollup state so that its references to past state, mirror the references in the finalization proof
      await betaV4_1_LineaRollup.setLastFinalizedShnarf(betaV4_1_FinalizationData_Sepolia.parentAggregationFinalShnarf);
      await betaV4_1_LineaRollup.setLastFinalizedState(
        betaV4_1_FinalizationData_Sepolia.parentAggregationLastL1RollingHashMessageNumber,
        betaV4_1_FinalizationData_Sepolia.parentAggregationLastL1RollingHash,
        betaV4_1_FinalizationData_Sepolia.parentAggregationLastBlockTimestamp,
      );
      await betaV4_1_LineaRollup.setRollingHash(
        betaV4_1_FinalizationData_Sepolia.l1RollingHashMessageNumber,
        betaV4_1_FinalizationData_Sepolia.l1RollingHash,
      );

      // *** ACT ***
      const finalizeCompressedCall = betaV4_1_LineaRollup
        .connect(operator)
        .finalizeBlocks(
          betaV4_1_FinalizationData_Sepolia.aggregatedProof,
          TEST_PUBLIC_VERIFIER_INDEX,
          finalizationData,
        );

      // *** ASSERT ***
      const eventArgs = [
        BigInt(betaV4_1_FinalizationData_Sepolia.lastFinalizedBlockNumber) + 1n,
        finalizationData.endBlockNumber,
        betaV4_1_FinalizationData_Sepolia.finalShnarf,
        finalizationData.parentStateRootHash,
        finalBlobFile.finalStateRootHash,
      ];

      await expectEvent(betaV4_1_LineaRollup, finalizeCompressedCall, "DataFinalizedV3", eventArgs);

      const [expectedFinalStateRootHash, lastFinalizedBlockNumber, lastFinalizedState] = await Promise.all([
        betaV4_1_LineaRollup.stateRootHashes(finalizationData.endBlockNumber),
        betaV4_1_LineaRollup.currentL2BlockNumber(),
        betaV4_1_LineaRollup.currentFinalizedState(),
      ]);

      expect(expectedFinalStateRootHash).to.equal(finalizationData.shnarfData.finalStateRootHash);
      expect(lastFinalizedBlockNumber).to.equal(finalizationData.endBlockNumber);
      expect(lastFinalizedState).to.equal(
        generateKeccak256(
          ["uint256", "bytes32", "uint256"],
          [
            finalizationData.l1RollingHashMessageNumber,
            finalizationData.l1RollingHash,
            finalizationData.finalTimestamp,
          ],
        ),
      );
    });

    it("Can submit blobs and finalize with Prover Beta V4_1 on Mainnet", async () => {
      // *** ARRANGE ***
      // custom verifier deploy.

      const mainnetFullVerifierFactory = await ethers.getContractFactory(
        "src/verifiers/beta-v4.1/PlonkVerifierMainnet.sol:PlonkVerifierMainnetFull",
      );
      const verifier = await mainnetFullVerifierFactory.deploy();
      await verifier.waitForDeployment();
      const mainnetFullVerifier = await verifier.getAddress();

      // Deploy and initialize LineaRollup
      const initializationData = {
        initialStateRootHash: betaV4_1_FinalizationData_Mainnet.parentStateRootHash,
        initialL2BlockNumber: betaV4_1_FinalizationData_Mainnet.lastFinalizedBlockNumber,
        genesisTimestamp: betaV4_1_FinalizationData_Mainnet.parentAggregationLastBlockTimestamp,
        defaultVerifier: mainnetFullVerifier,
        rateLimitPeriodInSeconds: ONE_DAY_IN_SECONDS,
        rateLimitAmountInWei: INITIAL_WITHDRAW_LIMIT,
        roleAddresses,
        pauseTypeRoles: LINEA_ROLLUP_PAUSE_TYPES_ROLES,
        unpauseTypeRoles: LINEA_ROLLUP_UNPAUSE_TYPES_ROLES,
        fallbackOperator: FALLBACK_OPERATOR_ADDRESS,
        defaultAdmin: securityCouncil.address,
      };

      const betaV4_1_LineaRollup = (await deployUpgradableFromFactory("TestLineaRollup", [initializationData], {
        initializer: LINEA_ROLLUP_INITIALIZE_SIGNATURE,
        unsafeAllow: ["constructor", "incorrect-initializer-order"],
      })) as unknown as TestLineaRollup;

      await betaV4_1_LineaRollup.setupParentShnarf(betaV4_1_FinalizationData_Mainnet.parentAggregationFinalShnarf);

      // Send blobs
      const blobFiles = getVersionedBlobFiles("betaV4_1_mainnet");

      await betaV4_1_LineaRollup.setupParentShnarf(betaV4_1_BlobData_Mainnet.prevShnarf);

      for (let i = 0; i < blobFiles.length; i++) {
        await sendVersionedBlobTransactionFromFile(lineaRollup, blobFiles[i], betaV4_1_LineaRollup, "betaV4_1_mainnet");
      }

      const finalBlobFile = JSON.parse(
        fs.readFileSync(`${__dirname}/../../_testData/betaV4_1_mainnet/${blobFiles.slice(-1)[0]}`, "utf-8"),
      );

      // Setup finalize call
      const finalizationData = await generateFinalizationData({
        l1RollingHash: betaV4_1_FinalizationData_Mainnet.l1RollingHash,
        l1RollingHashMessageNumber: BigInt(betaV4_1_FinalizationData_Mainnet.l1RollingHashMessageNumber),
        lastFinalizedTimestamp: BigInt(betaV4_1_FinalizationData_Mainnet.parentAggregationLastBlockTimestamp),
        endBlockNumber: BigInt(betaV4_1_FinalizationData_Mainnet.finalBlockNumber),
        parentStateRootHash: betaV4_1_FinalizationData_Mainnet.parentStateRootHash,
        finalTimestamp: BigInt(betaV4_1_FinalizationData_Mainnet.finalTimestamp),
        l2MerkleRoots: betaV4_1_FinalizationData_Mainnet.l2MerkleRoots,
        l2MerkleTreesDepth: BigInt(betaV4_1_FinalizationData_Mainnet.l2MerkleTreesDepth),
        l2MessagingBlocksOffsets: betaV4_1_FinalizationData_Mainnet.l2MessagingBlocksOffsets,
        aggregatedProof: betaV4_1_FinalizationData_Mainnet.aggregatedProof,
        shnarfData: {
          parentShnarf: finalBlobFile.prevShnarf,
          snarkHash: finalBlobFile.snarkHash,
          finalStateRootHash: finalBlobFile.finalStateRootHash,
          dataEvaluationPoint: finalBlobFile.expectedX,
          dataEvaluationClaim: finalBlobFile.expectedY,
        },
      });

      finalizationData.lastFinalizedL1RollingHash =
        betaV4_1_FinalizationData_Mainnet.parentAggregationLastL1RollingHash;
      finalizationData.lastFinalizedL1RollingHashMessageNumber = BigInt(
        betaV4_1_FinalizationData_Mainnet.parentAggregationLastL1RollingHashMessageNumber,
      );

      // Setup LineaRollup state so that its references to past state, mirror the references in the finalization proof
      await betaV4_1_LineaRollup.setLastFinalizedShnarf(betaV4_1_FinalizationData_Mainnet.parentAggregationFinalShnarf);
      await betaV4_1_LineaRollup.setLastFinalizedState(
        betaV4_1_FinalizationData_Mainnet.parentAggregationLastL1RollingHashMessageNumber,
        betaV4_1_FinalizationData_Mainnet.parentAggregationLastL1RollingHash,
        betaV4_1_FinalizationData_Mainnet.parentAggregationLastBlockTimestamp,
      );
      await betaV4_1_LineaRollup.setRollingHash(
        betaV4_1_FinalizationData_Mainnet.l1RollingHashMessageNumber,
        betaV4_1_FinalizationData_Mainnet.l1RollingHash,
      );

      // *** ACT ***
      const finalizeCompressedCall = betaV4_1_LineaRollup
        .connect(operator)
        .finalizeBlocks(
          betaV4_1_FinalizationData_Mainnet.aggregatedProof,
          TEST_PUBLIC_VERIFIER_INDEX,
          finalizationData,
        );

      // *** ASSERT ***
      const eventArgs = [
        BigInt(betaV4_1_FinalizationData_Mainnet.lastFinalizedBlockNumber) + 1n,
        finalizationData.endBlockNumber,
        betaV4_1_FinalizationData_Mainnet.finalShnarf,
        finalizationData.parentStateRootHash,
        finalBlobFile.finalStateRootHash,
      ];

      await expectEvent(betaV4_1_LineaRollup, finalizeCompressedCall, "DataFinalizedV3", eventArgs);

      const [expectedFinalStateRootHash, lastFinalizedBlockNumber, lastFinalizedState] = await Promise.all([
        betaV4_1_LineaRollup.stateRootHashes(finalizationData.endBlockNumber),
        betaV4_1_LineaRollup.currentL2BlockNumber(),
        betaV4_1_LineaRollup.currentFinalizedState(),
      ]);

      expect(expectedFinalStateRootHash).to.equal(finalizationData.shnarfData.finalStateRootHash);
      expect(lastFinalizedBlockNumber).to.equal(finalizationData.endBlockNumber);
      expect(lastFinalizedState).to.equal(
        generateKeccak256(
          ["uint256", "bytes32", "uint256"],
          [
            finalizationData.l1RollingHashMessageNumber,
            finalizationData.l1RollingHash,
            finalizationData.finalTimestamp,
          ],
        ),
      );
    });
  });
});
