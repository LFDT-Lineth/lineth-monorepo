import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture, time as networkTime } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { AddressFilter, LineaRollup__factory, TestLineaRollup } from "contracts/typechain-types";
import { ethers } from "hardhat";

import {
  expectSuccessfulFinalize,
  expectFailedCustomErrorFinalize,
  ensureKzgSetup,
  getAccountsFixture,
  deployLineaRollupFixture,
  sendBlobTransaction,
  deployRevertingVerifier,
} from "./../helpers";
import calldataAggregatedProof1To155 from "../../_testData/compressedData/aggregatedProof-1-155.json";
import aggregatedProof1To81 from "../../_testData/compressedData/multipleProofs/aggregatedProof-1-81.json";
import aggregatedProof82To153 from "../../_testData/compressedData/multipleProofs/aggregatedProof-82-153.json";
import blobAggregatedProof1To155 from "../../_testData/compressedDataEip4844/aggregatedProof-1-155.json";
import blobMultipleAggregatedProof1To81 from "../../_testData/compressedDataEip4844/multipleProofs/aggregatedProof-1-81.json";
import blobMultipleAggregatedProof82To153 from "../../_testData/compressedDataEip4844/multipleProofs/aggregatedProof-82-139.json";
ensureKzgSetup();
import {
  GENERAL_PAUSE_TYPE,
  HASH_ZERO,
  OPERATOR_ROLE,
  TEST_PUBLIC_VERIFIER_INDEX,
  EMPTY_CALLDATA,
  FINALIZATION_PAUSE_TYPE,
  DEFAULT_LAST_FINALIZED_TIMESTAMP,
  HARDHAT_CHAIN_ID,
  MAX_GAS_LIMIT,
  FORCED_TRANSACTION_FEE,
} from "../../common/constants";
import {
  calculateRollingHash,
  generateFinalizationData,
  generateRandomBytes,
  generateKeccak256,
  buildAccessErrorMessage,
  expectRevertWithCustomError,
  expectRevertWithReason,
  generateParentShnarfData,
  generateBlobParentShnarfData,
  calculateLastFinalizedState,
  calculateLastFinalizedStateV6,
  submitCalldataBeforeFinalization,
  proofDataToFinalizationParams,
  expectRevertWhenPaused,
} from "../../common/helpers";
import { AggregatedProofData } from "../../common/types";

describe("Linea Rollup contract: Finalization", () => {
  let lineaRollup: TestLineaRollup;
  let addressFilter: AddressFilter;

  let securityCouncil: SignerWithAddress;
  let operator: SignerWithAddress;
  let nonAuthorizedAccount: SignerWithAddress;

  before(async () => {
    ({ securityCouncil, operator, nonAuthorizedAccount } = await loadFixture(getAccountsFixture));
  });

  beforeEach(async () => {
    ({ lineaRollup, addressFilter } = await loadFixture(deployLineaRollupFixture));
  });

  describe("Blocks finalization with proof", () => {
    const messageHash = generateRandomBytes(32);

    beforeEach(async () => {
      await lineaRollup.addRollingHash(10, messageHash);
      await lineaRollup.setLastFinalizedBlock(0);
    });

    describe("With and without submission data", () => {
      it("Should revert if l1 message number == 0 and l1 rolling hash is not empty", async () => {
        const finalizationData = await generateFinalizationData({
          l1RollingHashMessageNumber: 0n,
          l1RollingHash: generateRandomBytes(32),
        });

        const lastFinalizedBlockNumber = await lineaRollup.currentL2BlockNumber();
        const parentStateRootHash = await lineaRollup.stateRootHashes(lastFinalizedBlockNumber);
        finalizationData.parentStateRootHash = parentStateRootHash;

        const proof = calldataAggregatedProof1To155.aggregatedProof;

        const finalizeCall = lineaRollup
          .connect(operator)
          .finalizeBlocks(proof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData);

        await expectRevertWithCustomError(lineaRollup, finalizeCall, "MissingMessageNumberForRollingHash", [
          finalizationData.l1RollingHash,
        ]);
      });

      it("Should revert if l1 message number != 0 and l1 rolling hash is empty", async () => {
        const finalizationData = await generateFinalizationData({
          l1RollingHashMessageNumber: 1n,
          l1RollingHash: HASH_ZERO,
        });

        const lastFinalizedBlockNumber = await lineaRollup.currentL2BlockNumber();
        const parentStateRootHash = await lineaRollup.stateRootHashes(lastFinalizedBlockNumber);
        finalizationData.parentStateRootHash = parentStateRootHash;

        const proof = calldataAggregatedProof1To155.aggregatedProof;

        const finalizeCall = lineaRollup
          .connect(operator)
          .finalizeBlocks(proof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData);

        await expectRevertWithCustomError(lineaRollup, finalizeCall, "MissingRollingHashForMessageNumber", [
          finalizationData.l1RollingHashMessageNumber,
        ]);
      });

      it("Should revert if l1RollingHash does not exist on L1", async () => {
        const finalizationData = await generateFinalizationData({
          l1RollingHashMessageNumber: 1n,
          l1RollingHash: generateRandomBytes(32),
        });

        const lastFinalizedBlockNumber = await lineaRollup.currentL2BlockNumber();
        const parentStateRootHash = await lineaRollup.stateRootHashes(lastFinalizedBlockNumber);
        finalizationData.parentStateRootHash = parentStateRootHash;

        const proof = calldataAggregatedProof1To155.aggregatedProof;

        const finalizeCall = lineaRollup
          .connect(operator)
          .finalizeBlocks(proof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData);

        await expectRevertWithCustomError(lineaRollup, finalizeCall, "L1RollingHashDoesNotExistOnL1", [
          finalizationData.l1RollingHashMessageNumber,
          finalizationData.l1RollingHash,
        ]);
      });

      it("Should revert if timestamps are not in sequence", async () => {
        const { finalIndex } = await submitCalldataBeforeFinalization(lineaRollup.connect(operator), {
          startIndex: 0,
          finalIndex: 4,
          maxGasLimit: MAX_GAS_LIMIT,
        });

        const proofData = calldataAggregatedProof1To155 as AggregatedProofData;
        const finalizationData = await generateFinalizationData({
          ...proofDataToFinalizationParams({
            proofData,
            shnarfDataGenerator: generateParentShnarfData,
            blobParentShnarfIndex: finalIndex,
            isMultiple: false,
          }),
          l1RollingHash: calculateRollingHash(HASH_ZERO, messageHash),
          l1RollingHashMessageNumber: 10n,
          lastFinalizedTimestamp: DEFAULT_LAST_FINALIZED_TIMESTAMP,
        });

        await lineaRollup.setRollingHash(
          calldataAggregatedProof1To155.l1RollingHashMessageNumber,
          calldataAggregatedProof1To155.l1RollingHash,
        );

        const expectedHashValue = calculateLastFinalizedState(
          finalizationData.lastFinalizedL1RollingHashMessageNumber,
          finalizationData.lastFinalizedL1RollingHash,
          BigInt(calldataAggregatedProof1To155.parentAggregationFtxNumber),
          calldataAggregatedProof1To155.parentAggregationFtxRollingHash,
          finalizationData.lastFinalizedTimestamp,
        );

        finalizationData.lastFinalizedTimestamp = finalizationData.finalTimestamp + 1n;

        const actualHashValue = calculateLastFinalizedStateV6(
          finalizationData.lastFinalizedL1RollingHashMessageNumber,
          finalizationData.lastFinalizedL1RollingHash,
          finalizationData.lastFinalizedTimestamp,
        );

        const finalizeCompressedCall = lineaRollup
          .connect(operator)
          .finalizeBlocks(calldataAggregatedProof1To155.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData);

        await expectRevertWithCustomError(lineaRollup, finalizeCompressedCall, "FinalizationStateIncorrect", [
          expectedHashValue,
          actualHashValue,
        ]);
      });

      it("Should revert if the final shnarf does not exist", async () => {
        const { finalIndex } = await submitCalldataBeforeFinalization(lineaRollup.connect(operator), {
          startIndex: 0,
          finalIndex: 4,
          maxGasLimit: MAX_GAS_LIMIT,
        });

        const proofData = calldataAggregatedProof1To155 as AggregatedProofData;
        const finalizationData = await generateFinalizationData({
          ...proofDataToFinalizationParams({
            proofData,
            shnarfDataGenerator: generateParentShnarfData,
            blobParentShnarfIndex: finalIndex,
            isMultiple: false,
          }),
          l1RollingHash: calculateRollingHash(HASH_ZERO, messageHash),
          l1RollingHashMessageNumber: 10n,
          lastFinalizedTimestamp: DEFAULT_LAST_FINALIZED_TIMESTAMP,
        });

        await lineaRollup.setRollingHash(
          calldataAggregatedProof1To155.l1RollingHashMessageNumber,
          calldataAggregatedProof1To155.l1RollingHash,
        );

        finalizationData.shnarfData.snarkHash = generateRandomBytes(32);

        const { dataEvaluationClaim, dataEvaluationPoint, finalStateRootHash, parentShnarf, snarkHash } =
          finalizationData.shnarfData;
        const expectedMissingBlobShnarf = generateKeccak256(
          ["bytes32", "bytes32", "bytes32", "bytes32", "bytes32"],
          [parentShnarf, snarkHash, finalStateRootHash, dataEvaluationPoint, dataEvaluationClaim],
        );

        const finalizeCompressedCall = lineaRollup
          .connect(operator)
          .finalizeBlocks(calldataAggregatedProof1To155.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData);

        await expectRevertWithCustomError(lineaRollup, finalizeCompressedCall, "FinalShnarfNotSubmitted", [
          expectedMissingBlobShnarf,
        ]);
      });

      it("Should revert if finalizationData.finalTimestamp is greater than the block.timestamp", async () => {
        const { finalIndex } = await submitCalldataBeforeFinalization(lineaRollup.connect(operator), {
          startIndex: 0,
          finalIndex: 4,
          maxGasLimit: MAX_GAS_LIMIT,
        });

        const proofData = calldataAggregatedProof1To155 as AggregatedProofData;
        const finalizationData = await generateFinalizationData({
          ...proofDataToFinalizationParams({
            proofData,
            shnarfDataGenerator: generateParentShnarfData,
            blobParentShnarfIndex: finalIndex,
            isMultiple: false,
          }),
          l1RollingHash: calculateRollingHash(HASH_ZERO, messageHash),
          l1RollingHashMessageNumber: 10n,
          lastFinalizedTimestamp: DEFAULT_LAST_FINALIZED_TIMESTAMP,
          finalTimestamp: BigInt(new Date(new Date().setHours(new Date().getHours() + 2)).getTime()), // Set to 2 hours in the future
        });

        await lineaRollup.setRollingHash(
          calldataAggregatedProof1To155.l1RollingHashMessageNumber,
          calldataAggregatedProof1To155.l1RollingHash,
        );

        const finalizeCompressedCall = lineaRollup
          .connect(operator)
          .finalizeBlocks(calldataAggregatedProof1To155.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData);

        await expectRevertWithCustomError(lineaRollup, finalizeCompressedCall, "FinalizationInTheFuture", [
          finalizationData.finalTimestamp,
          (await networkTime.latest()) + 1,
        ]);
      });
    });

    describe("Without submission data", () => {
      it("Should revert if the final block state equals the zero hash", async () => {
        const { finalIndex } = await submitCalldataBeforeFinalization(lineaRollup.connect(operator), {
          startIndex: 0,
          finalIndex: 4,
          maxGasLimit: MAX_GAS_LIMIT,
        });

        const proofData = calldataAggregatedProof1To155 as AggregatedProofData;
        const finalizationData = await generateFinalizationData({
          ...proofDataToFinalizationParams({
            proofData,
            shnarfDataGenerator: generateParentShnarfData,
            blobParentShnarfIndex: finalIndex,
            isMultiple: false,
          }),
          l1RollingHash: calculateRollingHash(HASH_ZERO, messageHash),
          l1RollingHashMessageNumber: 10n,
          lastFinalizedTimestamp: DEFAULT_LAST_FINALIZED_TIMESTAMP,
        });

        await lineaRollup.setRollingHash(
          calldataAggregatedProof1To155.l1RollingHashMessageNumber,
          calldataAggregatedProof1To155.l1RollingHash,
        );

        // Set the final state root hash to zero
        finalizationData.shnarfData.finalStateRootHash = HASH_ZERO;

        const finalizeCall = lineaRollup
          .connect(operator)
          .finalizeBlocks(calldataAggregatedProof1To155.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData);

        await expectRevertWithCustomError(lineaRollup, finalizeCall, "FinalBlockStateEqualsZeroHash");
      });
    });
  });

  describe("Compressed data finalization with proof", () => {
    beforeEach(async () => {
      await lineaRollup.setLastFinalizedBlock(0);
    });

    it("Should revert if the caller does not have the OPERATOR_ROLE", async () => {
      const finalizationData = await generateFinalizationData();

      const finalizeCall = lineaRollup
        .connect(nonAuthorizedAccount)
        .finalizeBlocks(calldataAggregatedProof1To155.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData);
      await expectRevertWithReason(finalizeCall, buildAccessErrorMessage(nonAuthorizedAccount, OPERATOR_ROLE));
    });

    // Parameterized pause type tests for finalization
    const finalizationPauseTypes = [
      { pauseType: GENERAL_PAUSE_TYPE, name: "GENERAL_PAUSE_TYPE" },
      { pauseType: FINALIZATION_PAUSE_TYPE, name: "FINALIZATION_PAUSE_TYPE" },
    ];

    finalizationPauseTypes.forEach(({ pauseType, name }) => {
      it(`Should revert if ${name} is enabled`, async () => {
        const finalizationData = await generateFinalizationData();

        await lineaRollup.connect(securityCouncil).pauseByType(pauseType);

        const finalizeCall = lineaRollup
          .connect(operator)
          .finalizeBlocks(EMPTY_CALLDATA, TEST_PUBLIC_VERIFIER_INDEX, finalizationData);
        await expectRevertWhenPaused(lineaRollup, finalizeCall, pauseType);
      });
    });

    it("Should revert if the proof is empty", async () => {
      const finalizationData = await generateFinalizationData();

      const finalizeCall = lineaRollup
        .connect(operator)
        .finalizeBlocks(EMPTY_CALLDATA, TEST_PUBLIC_VERIFIER_INDEX, finalizationData);
      await expectRevertWithCustomError(lineaRollup, finalizeCall, "ProofIsEmpty");
    });

    it("Should revert when finalization parentStateRootHash is different than last finalized state root hash", async () => {
      // Submit 4 sets of compressed data setting the correct shnarf in storage
      await submitCalldataBeforeFinalization(lineaRollup.connect(operator), {
        startIndex: 0,
        finalIndex: 4,
        maxGasLimit: MAX_GAS_LIMIT,
      });

      const finalizationData = await generateFinalizationData({
        lastFinalizedTimestamp: DEFAULT_LAST_FINALIZED_TIMESTAMP,
        parentStateRootHash: generateRandomBytes(32),
        aggregatedProof: calldataAggregatedProof1To155.aggregatedProof,
      });

      const finalizeCall = lineaRollup
        .connect(operator)
        .finalizeBlocks(calldataAggregatedProof1To155.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData, {
          gasLimit: MAX_GAS_LIMIT,
        });
      await expectRevertWithCustomError(lineaRollup, finalizeCall, "StartingRootHashDoesNotMatch");
    });

    it("Should successfully finalize with only previously submitted data", async () => {
      // Submit 4 sets of compressed data setting the correct shnarf in storage
      const { finalIndex } = await submitCalldataBeforeFinalization(lineaRollup.connect(operator), {
        startIndex: 0,
        finalIndex: 4,
        maxGasLimit: MAX_GAS_LIMIT,
      });

      await expectSuccessfulFinalize({
        context: {
          lineaRollup,
          operator,
        },
        proofConfig: {
          proofData: calldataAggregatedProof1To155,
          blobParentShnarfIndex: finalIndex,
          shnarfDataGenerator: generateParentShnarfData,
          isMultiple: false,
        },
      });
    });

    it("Should revert when proofType is invalid", async () => {
      const { finalIndex } = await submitCalldataBeforeFinalization(lineaRollup.connect(operator), {
        startIndex: 0,
        finalIndex: 4,
        maxGasLimit: MAX_GAS_LIMIT,
      });

      const proofData = calldataAggregatedProof1To155 as AggregatedProofData;
      const finalizationData = await generateFinalizationData({
        ...proofDataToFinalizationParams({
          proofData,
          shnarfDataGenerator: generateParentShnarfData,
          blobParentShnarfIndex: finalIndex,
          isMultiple: false,
        }),
      });

      await lineaRollup.setRollingHash(
        calldataAggregatedProof1To155.l1RollingHashMessageNumber,
        calldataAggregatedProof1To155.l1RollingHash,
      );

      const finalizeCall = lineaRollup
        .connect(operator)
        .finalizeBlocks(calldataAggregatedProof1To155.aggregatedProof, 99, finalizationData);
      await expectRevertWithCustomError(lineaRollup, finalizeCall, "InvalidProofType");
    });

    it("Should revert when using a proofType index that was removed", async () => {
      const { finalIndex } = await submitCalldataBeforeFinalization(lineaRollup.connect(operator), {
        startIndex: 0,
        finalIndex: 4,
        maxGasLimit: MAX_GAS_LIMIT,
      });

      const proofData = calldataAggregatedProof1To155 as AggregatedProofData;
      const finalizationData = await generateFinalizationData({
        ...proofDataToFinalizationParams({
          proofData,
          shnarfDataGenerator: generateParentShnarfData,
          blobParentShnarfIndex: finalIndex,
          isMultiple: false,
        }),
      });

      await lineaRollup.setRollingHash(
        calldataAggregatedProof1To155.l1RollingHashMessageNumber,
        calldataAggregatedProof1To155.l1RollingHash,
      );

      // removing the verifier index
      await lineaRollup.connect(securityCouncil).unsetVerifierAddress(TEST_PUBLIC_VERIFIER_INDEX);

      const finalizeCall = lineaRollup
        .connect(operator)
        .finalizeBlocks(calldataAggregatedProof1To155.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData);
      await expectRevertWithCustomError(lineaRollup, finalizeCall, "InvalidProofType");
    });

    it("Should fail when proof does not match", async () => {
      const { finalIndex } = await submitCalldataBeforeFinalization(lineaRollup.connect(operator), {
        startIndex: 0,
        finalIndex: 4,
        maxGasLimit: MAX_GAS_LIMIT,
      });

      const proofData = calldataAggregatedProof1To155 as AggregatedProofData;
      const finalizationData = await generateFinalizationData({
        ...proofDataToFinalizationParams({
          proofData,
          shnarfDataGenerator: generateParentShnarfData,
          blobParentShnarfIndex: finalIndex,
          isMultiple: false,
        }),
      });

      await lineaRollup.setRollingHash(
        calldataAggregatedProof1To155.l1RollingHashMessageNumber,
        calldataAggregatedProof1To155.l1RollingHash,
      );

      // aggregatedProof1To81.aggregatedProof, wrong proof on purpose
      const finalizeCall = lineaRollup
        .connect(operator)
        .finalizeBlocks(aggregatedProof1To81.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData);
      await expectRevertWithCustomError(lineaRollup, finalizeCall, "InvalidProof");
    });

    it("Should fail if shnarf does not exist when finalizing", async () => {
      await submitCalldataBeforeFinalization(lineaRollup.connect(operator), {
        startIndex: 0,
        finalIndex: 4,
        maxGasLimit: MAX_GAS_LIMIT,
      });

      const proofData = calldataAggregatedProof1To155 as AggregatedProofData;
      const finalizationData = await generateFinalizationData({
        ...proofDataToFinalizationParams({
          proofData,
          shnarfDataGenerator: generateParentShnarfData,
          blobParentShnarfIndex: 1, // Wrong index to simulate missing shnarf
          isMultiple: false,
        }),
      });

      await lineaRollup.setRollingHash(
        calldataAggregatedProof1To155.l1RollingHashMessageNumber,
        calldataAggregatedProof1To155.l1RollingHash,
      );

      const finalizeCall = lineaRollup
        .connect(operator)
        .finalizeBlocks(calldataAggregatedProof1To155.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData);
      await expectRevertWithCustomError(lineaRollup, finalizeCall, "InvalidProof");
    });

    it("Should successfully finalize 1-81 and then 82-153 in two separate finalizations", async () => {
      await submitCalldataBeforeFinalization(lineaRollup.connect(operator), {
        startIndex: 0,
        finalIndex: 4,
        useMultipleProofs: true,
        maxGasLimit: MAX_GAS_LIMIT,
      });

      await expectSuccessfulFinalize({
        context: {
          lineaRollup,
          operator,
        },
        proofConfig: {
          proofData: aggregatedProof1To81,
          blobParentShnarfIndex: 2,
          shnarfDataGenerator: generateParentShnarfData,
          isMultiple: true,
        },
      });

      for (const filteredAddress of aggregatedProof82To153.filteredAddresses) {
        await addressFilter.connect(securityCouncil).setFilteredStatus([filteredAddress], true);
      }

      await expectSuccessfulFinalize({
        context: {
          lineaRollup,
          operator,
        },
        proofConfig: {
          proofData: aggregatedProof82To153,
          blobParentShnarfIndex: 4,
          shnarfDataGenerator: generateParentShnarfData,
          isMultiple: true,
        },
      });
    });

    it("Should succeed when sending with pure calldata", async () => {
      await submitCalldataBeforeFinalization(lineaRollup.connect(operator), {
        startIndex: 0,
        finalIndex: 4,
        useMultipleProofs: true,
        maxGasLimit: MAX_GAS_LIMIT,
      });
      await lineaRollup.setRollingHash(
        aggregatedProof1To81.l1RollingHashMessageNumber,
        aggregatedProof1To81.l1RollingHash,
      );

      const lineaRollupAddress = await lineaRollup.getAddress();

      const { maxFeePerGas, maxPriorityFeePerGas } = await ethers.provider.getFeeData();

      const proofData = aggregatedProof1To81 as AggregatedProofData;
      const finalizationData = await generateFinalizationData({
        ...proofDataToFinalizationParams({
          proofData,
          shnarfDataGenerator: generateParentShnarfData,
          blobParentShnarfIndex: 2,
          isMultiple: true,
        }),
      });

      const encodedCall = LineaRollup__factory.createInterface().encodeFunctionData("finalizeBlocks", [
        aggregatedProof1To81.aggregatedProof,
        TEST_PUBLIC_VERIFIER_INDEX,
        finalizationData,
      ]);

      const transaction = {
        data: encodedCall,
        maxPriorityFeePerGas: maxPriorityFeePerGas!,
        maxFeePerGas: maxFeePerGas!,
        to: lineaRollupAddress,
        chainId: 31337,
        value: 0,
        gasLimit: 5_000_000,
      };

      await expect(operator.sendTransaction(transaction)).to.not.be.reverted;
    });

    // TODO: decide on removing - not really applicable anymore
    it.skip("Should fail when sending with wrong merkle root location", async () => {
      await submitCalldataBeforeFinalization(lineaRollup.connect(operator), {
        startIndex: 0,
        finalIndex: 4,
        useMultipleProofs: true,
        maxGasLimit: MAX_GAS_LIMIT,
      });

      await lineaRollup.setRollingHash(
        aggregatedProof1To81.l1RollingHashMessageNumber,
        aggregatedProof1To81.l1RollingHash,
      );

      const lineaRollupAddress = await lineaRollup.getAddress();

      const { maxFeePerGas, maxPriorityFeePerGas } = await ethers.provider.getFeeData();

      // This contains the Merkle roots length for public input at a specific location and a contrived pointer to an alternate location.
      const encodedCall =
        "0x4abc041c0000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003e00000000000000000000000000000000000000000000000000000000000000360008e483358dc0ac1d3f6a27b43f6332e88c4196151b2a1ce6c8575013cec2cbe2f666e5649165dfc5a88ede985203341f5910ec2f4e3d4e9fa2a0a2621e4b0a70a91e0f995a8d754a3ddec1698800be83da8815650488540d2ffb699eaf3518d14bff479974f1f5f8bef7687f89c6301247cd92a4ccfa76a868bd24090a670f20eb06513431e74411648ab8c02a270353146bc759451d00cf2a59477d6106b9d11df3e18bca0d3a36951e31699b21a73e22a77fb7fe43a33b717a23dd06a889a102e4143d1e024e053e4318f903c3dbd0ec0f6e49672dd0ccb10749ac00adaf311149a24bf2c2ee4cfe7ed1db713526c6f7c0893f2ec153e5f55a4b390ce135a0d3ebe372c1beea8142afebdc96fe2764b8b5a33efc0457b9ad65f678c8b9ede196eb1fe34a82e0f370fe1d53e5cfc34bc77f38a64eddbb74f6530a82feb0e0a148de4d309b179697ff8a3c97a5516a1bbffc36d8cf153e9f5ee8fdc98002fd1136a2a12603aa1c7a985b34c5dc6774a5efd6270eac180406b4f03c7ef08eee428bb45d04a3c6635a0225aae76c0522049df5080045629cd6a686c2d50d8b7fe10e6facded652c8d75d4a67d5f8f4c3c6b491f44cc23c307064ec94f7c3a44eb1d1dcd369464960e54a88ad3f9edbdd818f2ac711d6c8f49efe224fb2616f2ac0005ca0c753c801d9e2df1133cdd4de5e54554197ac3e2f42d0b140261a12796050d64ec14bb22355173d157a7a2a9a5c0a20d47430819b4c106fda54923f9bf273cbc4145962e07b853279e64e198d3da5919814935c4cf12ffc03128d0e4fc24e8f35180d257253c0ce33725cb665a81141f16418fb20cf2abb96fa65a03bc0fa8e1e842123e2a2590f423e34c1db2c51c9e872ea41942d44d94d735cf8b4b0ccc265c6bdc41dd6f1167da133450a5ea0c07ea4704c9ca377cd201d8a63f932f7d4c9edb88269008bfe08d91d576cbf1a46c2da88b13d2a38e68f66f330552114f37a226c38ed3a9acd35b75f5852e6879eb3c8746ed2676f9e3957d32bbee1493ac055f03a56e964c4ea278d323abde55b36439b5460897771bf5ef3be1992448a5479f3515d5605d403a51391c6c6db94a90b3353aa956a5ba6f7b951fee2b7657ab431f90220c84820c2db8e4b57eaa5c3ea13e8d1f4ed31b9434c5df5c083dbb24fdf9dc5a64ba7416e6d154e6b17727a3acaa9f59820e926196efd232072ead6777750dc20232d1cee8dc9a395c2d350df4bbaa5096c6f59b214dcecd00000000000000000000000000000000000000000000000000000000000000519562fa89830a0ba0063a636ca96e52ce2f032b855336c95dd788321f4e1934190eacb8ed649249b3ec6efd9fbd6ffebe1b325b53380a91f7d689bfc1aff3b6dcf0f26782f7afb93f926cacb145f55530714f20b1356725e3971dc99e0ef8b59101216d3e1700c3a0d5115686fa51caa982cb4e002a5bb9f9488c9c44e4d9a3042d2f290de42ce8ea03cf8ac09288166932f174cb069ff8147c95ed6374e4e6cb00000000000000000000000000000000000000000000000000000000645580d1000000000000000000000000000000000000000000000000000000006455a7e10000000000000000000000000000000000000000000000000000000000000000dc8e70637c1048e1e0406c4ed6fab51a7489ccb52f37ddd2c135cb1aa18ec69700000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000500000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002e000000000000000000000000000000000000000000000000000000000000002a000000000000000000000000000000000000000000000000000000000000000016de197db892fd7d3fe0a389452dae0c5d0520e23d18ad20327546e2189a7e3f1000000000000000000000000000000000000000000000000000000000000000e00000000000000000000000fffff000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000001cafecafe";
      const transaction = {
        data: encodedCall,
        maxPriorityFeePerGas: maxPriorityFeePerGas!,
        maxFeePerGas: maxFeePerGas!,
        to: lineaRollupAddress,
        chainId: HARDHAT_CHAIN_ID,
        value: 0,
        gasLimit: 5_000_000,
      };

      await expectRevertWithCustomError(lineaRollup, operator.sendTransaction(transaction), "InvalidProof");
    });

    it("Should fail to finalize with extra merkle roots", async () => {
      await submitCalldataBeforeFinalization(lineaRollup.connect(operator), {
        startIndex: 0,
        finalIndex: 4,
        useMultipleProofs: true,
        maxGasLimit: MAX_GAS_LIMIT,
      });

      const merkleRoots = [...aggregatedProof1To81.l2MerkleRoots, generateRandomBytes(32)];

      const proofData = aggregatedProof1To81 as AggregatedProofData;
      const finalizationData = await generateFinalizationData({
        ...proofDataToFinalizationParams({
          proofData,
          shnarfDataGenerator: generateParentShnarfData,
          blobParentShnarfIndex: 2,
          isMultiple: true,
        }),
        l2MerkleRoots: merkleRoots,
        lastFinalizedL1RollingHash: HASH_ZERO,
        lastFinalizedL1RollingHashMessageNumber: 0n,
      });

      await lineaRollup.setRollingHash(
        aggregatedProof1To81.l1RollingHashMessageNumber,
        aggregatedProof1To81.l1RollingHash,
      );

      const finalizeCall = lineaRollup
        .connect(operator)
        .finalizeBlocks(aggregatedProof1To81.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData);

      await expectRevertWithCustomError(lineaRollup, finalizeCall, "InvalidProof");
    });
  });

  describe("Blob-based finalization tests", () => {
    let revertingVerifier: string;

    beforeEach(async () => {
      ({ lineaRollup, addressFilter } = await loadFixture(deployLineaRollupFixture));
      await lineaRollup.setLastFinalizedBlock(0);
    });

    it("Should submit 2 blobs, then submit another 2 blobs and finalize", async () => {
      // Address filter is already set during initialize; only the forced transaction fee needs explicit setup
      await lineaRollup.connect(securityCouncil).setForcedTransactionFee(FORCED_TRANSACTION_FEE);

      // validating address filtering checking by marking the security council as filtered
      await addressFilter.connect(securityCouncil).setFilteredStatus([securityCouncil.getAddress()], true);

      // Submit 2 blobs
      await sendBlobTransaction(lineaRollup, 0, 2);
      // Submit another 2 blobs
      await sendBlobTransaction(lineaRollup, 2, 4);
      // Finalize 4 blobs
      await expectSuccessfulFinalize({
        context: {
          lineaRollup,
          operator,
        },
        proofConfig: {
          proofData: blobAggregatedProof1To155,
          blobParentShnarfIndex: 4,
          shnarfDataGenerator: generateBlobParentShnarfData,
          isMultiple: false,
        },
      });
    });

    it("Should revert if the address filter is set and the address is not marked as filtered", async () => {
      const filteredAddress = await securityCouncil.getAddress();

      // Address filter is already set during initialize; only the forced transaction fee needs explicit setup
      await lineaRollup.connect(securityCouncil).setForcedTransactionFee(FORCED_TRANSACTION_FEE);

      // Submit 2 blobs
      await sendBlobTransaction(lineaRollup, 0, 2);
      // Submit another 2 blobs
      await sendBlobTransaction(lineaRollup, 2, 4);
      // Finalize 4 blobs
      await expectFailedCustomErrorFinalize({
        context: {
          lineaRollup,
          operator,
        },
        proofConfig: {
          proofData: blobAggregatedProof1To155,
          blobParentShnarfIndex: 4,
          shnarfDataGenerator: generateBlobParentShnarfData,
          isMultiple: false,
        },
        overrides: {
          filteredAddresses: [filteredAddress],
        },
        expectedError: {
          name: "AddressIsNotFiltered",
          args: [filteredAddress],
        },
      });
    });

    it("Should fail to finalize with not enough gas for the rollup (pre-verifier)", async () => {
      // Submit 2 blobs
      await sendBlobTransaction(lineaRollup, 0, 2);
      // Submit another 2 blobs
      await sendBlobTransaction(lineaRollup, 2, 4);

      const proofData = blobAggregatedProof1To155 as AggregatedProofData;
      const finalizationData = await generateFinalizationData({
        ...proofDataToFinalizationParams({
          proofData,
          shnarfDataGenerator: generateBlobParentShnarfData,
          blobParentShnarfIndex: 4,
          isMultiple: false,
        }),
        lastFinalizedL1RollingHash: HASH_ZERO,
        lastFinalizedL1RollingHashMessageNumber: 0n,
      });

      await lineaRollup.setRollingHash(proofData.l1RollingHashMessageNumber, proofData.l1RollingHash);

      const finalizeCompressedCall = lineaRollup
        .connect(operator)
        .finalizeBlocks(proofData.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData, {
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

      const proofData = blobAggregatedProof1To155 as AggregatedProofData;
      const finalizationData = await generateFinalizationData({
        ...proofDataToFinalizationParams({
          proofData,
          shnarfDataGenerator: generateBlobParentShnarfData,
          blobParentShnarfIndex: 4,
          isMultiple: false,
        }),
        lastFinalizedL1RollingHash: HASH_ZERO,
        lastFinalizedL1RollingHashMessageNumber: 0n,
      });

      await lineaRollup.setRollingHash(proofData.l1RollingHashMessageNumber, proofData.l1RollingHash);

      const finalizeCompressedCall = lineaRollup
        .connect(operator)
        .finalizeBlocks(proofData.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData, {
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

        const proofData = blobAggregatedProof1To155 as AggregatedProofData;
        const finalizationData = await generateFinalizationData({
          ...proofDataToFinalizationParams({
            proofData,
            shnarfDataGenerator: generateBlobParentShnarfData,
            blobParentShnarfIndex: 4,
            isMultiple: false,
          }),
          lastFinalizedL1RollingHash: HASH_ZERO,
          lastFinalizedL1RollingHashMessageNumber: 0n,
        });

        await lineaRollup.setRollingHash(proofData.l1RollingHashMessageNumber, proofData.l1RollingHash);

        const finalizeCompressedCall = lineaRollup
          .connect(operator)
          .finalizeBlocks(proofData.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData, {
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

    it("Should fail to finalize if finalForcedTransactionNumber is non-zero but rolling hash is empty", async () => {
      // Submit 2 blobs
      await sendBlobTransaction(lineaRollup, 0, 2, true);
      // Submit another 2 blobs
      await sendBlobTransaction(lineaRollup, 2, 4, true);

      const proofData = blobAggregatedProof1To155 as AggregatedProofData;
      const finalizationData = await generateFinalizationData({
        ...proofDataToFinalizationParams({
          proofData,
          shnarfDataGenerator: generateBlobParentShnarfData,
          blobParentShnarfIndex: 4,
          isMultiple: false,
        }),
        parentStateRootHash: HASH_ZERO,
        lastFinalizedL1RollingHash: HASH_ZERO,
        lastFinalizedL1RollingHashMessageNumber: 0n,
        finalForcedTransactionNumber: 1n,
      });

      await lineaRollup.setRollingHash(proofData.l1RollingHashMessageNumber, proofData.l1RollingHash);

      await lineaRollup.setLastFinalizedBlock(10_000_000);

      await expectRevertWithCustomError(
        lineaRollup,
        lineaRollup
          .connect(operator)
          .finalizeBlocks(proofData.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData),
        "MissingRollingHashForForcedTransactionNumber",
        [1],
      );
    });

    it("Should fail to finalize if there are missing forced transactions", async () => {
      // Submit 2 blobs
      await sendBlobTransaction(lineaRollup, 0, 2, true);
      // Submit another 2 blobs
      await sendBlobTransaction(lineaRollup, 2, 4, true);

      const proofData = blobAggregatedProof1To155 as AggregatedProofData;
      await lineaRollup.setForcedTransactionBlockNumber(BigInt(proofData.finalBlockNumber));

      const expectedErrorTransactionNumber = 1; // first transaction

      const finalizationData = await generateFinalizationData({
        ...proofDataToFinalizationParams({
          proofData,
          shnarfDataGenerator: generateBlobParentShnarfData,
          blobParentShnarfIndex: 4,
          isMultiple: false,
        }),
        parentStateRootHash: HASH_ZERO, // Manipulate for bypass
        lastFinalizedL1RollingHash: HASH_ZERO,
        lastFinalizedL1RollingHashMessageNumber: 0n,
      });

      await lineaRollup.setRollingHash(proofData.l1RollingHashMessageNumber, proofData.l1RollingHash);

      await lineaRollup.setLastFinalizedBlock(10_000_000);

      await expectRevertWithCustomError(
        lineaRollup,
        lineaRollup
          .connect(operator)
          .finalizeBlocks(proofData.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData),
        "FinalizationDataMissingForcedTransaction",
        [expectedErrorTransactionNumber],
      );
    });

    it("Should successfully submit 2 blobs twice then finalize in two separate finalizations", async () => {
      // Submit 2 blobs
      await sendBlobTransaction(lineaRollup, 0, 2, true);
      // Submit another 2 blobs
      await sendBlobTransaction(lineaRollup, 2, 4, true);

      await lineaRollup.setForcedTransactionRollingHash(
        blobMultipleAggregatedProof1To81.finalFtxNumber,
        blobMultipleAggregatedProof1To81.finalFtxRollingHash,
      );
      await lineaRollup.setForcedTransactionRollingHash(
        blobMultipleAggregatedProof82To153.finalFtxNumber,
        blobMultipleAggregatedProof82To153.finalFtxRollingHash,
      );

      for (const filteredAddress of blobMultipleAggregatedProof1To81.filteredAddresses) {
        await addressFilter.connect(securityCouncil).setFilteredStatus([filteredAddress], true);
      }

      await expectSuccessfulFinalize({
        context: {
          lineaRollup,
          operator,
        },
        proofConfig: {
          proofData: blobMultipleAggregatedProof1To81,
          blobParentShnarfIndex: 2,
          shnarfDataGenerator: generateBlobParentShnarfData,
          isMultiple: true,
        },
      });

      for (const filteredAddress of blobMultipleAggregatedProof82To153.filteredAddresses) {
        await addressFilter.connect(securityCouncil).setFilteredStatus([filteredAddress], true);
      }

      // Finalize second 2 blobs
      await expectSuccessfulFinalize({
        context: {
          lineaRollup,
          operator,
        },
        proofConfig: {
          proofData: blobMultipleAggregatedProof82To153,
          blobParentShnarfIndex: 4,
          shnarfDataGenerator: generateBlobParentShnarfData,
          isMultiple: true,
        },
      });
    });

    it("Should successfully submit 2 blobs twice then finalize in two separate finalizations using 3 and then 6 finalizationState fields", async () => {
      // Explicitly use the 3 fields to simulate an existing finalization
      await lineaRollup.setLastFinalizedStateV6(0, HASH_ZERO, DEFAULT_LAST_FINALIZED_TIMESTAMP);

      // Submit 2 blobs
      await sendBlobTransaction(lineaRollup, 0, 2, true);
      // Submit another 2 blobs
      await sendBlobTransaction(lineaRollup, 2, 4, true);

      await lineaRollup.setForcedTransactionRollingHash(
        blobMultipleAggregatedProof1To81.finalFtxNumber,
        blobMultipleAggregatedProof1To81.finalFtxRollingHash,
      );

      for (const filteredAddress of blobMultipleAggregatedProof1To81.filteredAddresses) {
        await addressFilter.connect(securityCouncil).setFilteredStatus([filteredAddress], true);
      }

      // Finalize first 2 blobs
      await expectSuccessfulFinalize({
        context: {
          lineaRollup,
          operator,
        },
        proofConfig: {
          proofData: blobMultipleAggregatedProof1To81,
          blobParentShnarfIndex: 2,
          shnarfDataGenerator: generateBlobParentShnarfData,
          isMultiple: true,
        },
      });

      await lineaRollup.setForcedTransactionRollingHash(
        blobMultipleAggregatedProof82To153.finalFtxNumber,
        blobMultipleAggregatedProof82To153.finalFtxRollingHash,
      );

      for (const filteredAddress of blobMultipleAggregatedProof82To153.filteredAddresses) {
        await addressFilter.connect(securityCouncil).setFilteredStatus([filteredAddress], true);
      }

      // Finalize second 2 blobs
      await expectSuccessfulFinalize({
        context: {
          lineaRollup,
          operator,
        },
        proofConfig: {
          proofData: blobMultipleAggregatedProof82To153,
          blobParentShnarfIndex: 4,
          shnarfDataGenerator: generateBlobParentShnarfData,
          isMultiple: true,
        },
      });
    });

    it("Should fail to prove if last finalized is higher than proving range", async () => {
      // Submit 2 blobs
      await sendBlobTransaction(lineaRollup, 0, 2, false);
      // Submit another 2 blobs
      await sendBlobTransaction(lineaRollup, 2, 4, false);

      await lineaRollup.setLastFinalizedBlock(10_000_000);

      await expectFailedCustomErrorFinalize({
        context: {
          lineaRollup,
          operator,
        },
        proofConfig: {
          proofData: blobAggregatedProof1To155,
          blobParentShnarfIndex: 4,
          shnarfDataGenerator: generateBlobParentShnarfData,
          isMultiple: false,
        },
        overrides: {
          parentStateRootHash: HASH_ZERO,
        },
        expectedError: {
          name: "InvalidProof",
        },
      });
    });
  });
});
