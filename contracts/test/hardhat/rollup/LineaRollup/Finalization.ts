import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture, time as networkTime } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { LineaRollup__factory, TestLineaRollup } from "contracts/typechain-types";
import { ethers } from "hardhat";

import { expectSuccessfulFinalize, getAccountsFixture, deployLineaRollupFixture } from "./../helpers";
import calldataAggregatedProof1To155 from "../../_testData/compressedData/aggregatedProof-1-155.json";
import fourthCompressedDataContent from "../../_testData/compressedData/blocks-115-155.json";
import secondCompressedDataContent from "../../_testData/compressedData/blocks-47-81.json";
import aggregatedProof1To81 from "../../_testData/compressedData/multipleProofs/aggregatedProof-1-81.json";
import aggregatedProof82To153 from "../../_testData/compressedData/multipleProofs/aggregatedProof-82-153.json";
import fourthMultipleCompressedDataContent from "../../_testData/compressedData/multipleProofs/blocks-120-153.json";
import {
  GENERAL_PAUSE_TYPE,
  HASH_ZERO,
  OPERATOR_ROLE,
  TEST_PUBLIC_VERIFIER_INDEX,
  EMPTY_CALLDATA,
  FINALIZATION_PAUSE_TYPE,
  DEFAULT_LAST_FINALIZED_TIMESTAMP,
  MAX_GAS_LIMIT,
} from "../../common/constants";
import {
  calculateRollingHash,
  generateFinalizationData,
  generateRandomBytes,
  generateCallDataSubmission,
  generateCallDataSubmissionMultipleProofs,
  generateKeccak256,
  buildAccessErrorMessage,
  expectRevertWithCustomError,
  expectRevertWithReason,
  generateParentAndExpectedShnarfForIndex,
  generateParentAndExpectedShnarfForMulitpleIndex,
  generateParentShnarfData,
} from "../../common/helpers";

describe("Linea Rollup contract: Finalization", () => {
  let lineaRollup: TestLineaRollup;

  let securityCouncil: SignerWithAddress;
  let operator: SignerWithAddress;
  let nonAuthorizedAccount: SignerWithAddress;

  before(async () => {
    ({ securityCouncil, operator, nonAuthorizedAccount } = await loadFixture(getAccountsFixture));
  });

  beforeEach(async () => {
    ({ lineaRollup } = await loadFixture(deployLineaRollupFixture));
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
        const submissionDataBeforeFinalization = generateCallDataSubmission(0, 4);
        let index = 0;
        for (const data of submissionDataBeforeFinalization) {
          const parentAndExpectedShnarf = generateParentAndExpectedShnarfForIndex(index);
          await lineaRollup
            .connect(operator)
            .submitDataAsCalldata(data, parentAndExpectedShnarf.parentShnarf, parentAndExpectedShnarf.expectedShnarf, {
              gasLimit: MAX_GAS_LIMIT,
            });
          index++;
        }

        const finalizationData = await generateFinalizationData({
          l1RollingHash: calculateRollingHash(HASH_ZERO, messageHash),
          l1RollingHashMessageNumber: 10n,
          lastFinalizedTimestamp: DEFAULT_LAST_FINALIZED_TIMESTAMP,
          endBlockNumber: BigInt(calldataAggregatedProof1To155.finalBlockNumber),
          parentStateRootHash: calldataAggregatedProof1To155.parentStateRootHash,
          finalTimestamp: BigInt(calldataAggregatedProof1To155.finalTimestamp),
          l2MerkleRoots: calldataAggregatedProof1To155.l2MerkleRoots,
          l2MerkleTreesDepth: BigInt(calldataAggregatedProof1To155.l2MerkleTreesDepth),
          l2MessagingBlocksOffsets: calldataAggregatedProof1To155.l2MessagingBlocksOffsets,
          aggregatedProof: calldataAggregatedProof1To155.aggregatedProof,
          shnarfData: generateParentShnarfData(index),
        });

        await lineaRollup.setRollingHash(
          calldataAggregatedProof1To155.l1RollingHashMessageNumber,
          calldataAggregatedProof1To155.l1RollingHash,
        );

        finalizationData.lastFinalizedTimestamp = finalizationData.finalTimestamp + 1n;

        const expectedHashValue = generateKeccak256(
          ["uint256", "bytes32", "uint256"],
          [
            finalizationData.lastFinalizedL1RollingHashMessageNumber,
            finalizationData.lastFinalizedL1RollingHash,
            finalizationData.lastFinalizedTimestamp,
          ],
        );
        const actualHashValue = generateKeccak256(
          ["uint256", "bytes32", "uint256"],
          [
            finalizationData.lastFinalizedL1RollingHashMessageNumber,
            finalizationData.lastFinalizedL1RollingHash,
            DEFAULT_LAST_FINALIZED_TIMESTAMP,
          ],
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
        const submissionDataBeforeFinalization = generateCallDataSubmission(0, 4);
        let index = 0;
        for (const data of submissionDataBeforeFinalization) {
          const parentAndExpectedShnarf = generateParentAndExpectedShnarfForIndex(index);
          await lineaRollup
            .connect(operator)
            .submitDataAsCalldata(data, parentAndExpectedShnarf.parentShnarf, parentAndExpectedShnarf.expectedShnarf, {
              gasLimit: MAX_GAS_LIMIT,
            });
          index++;
        }

        const finalizationData = await generateFinalizationData({
          l1RollingHash: calculateRollingHash(HASH_ZERO, messageHash),
          l1RollingHashMessageNumber: 10n,
          lastFinalizedTimestamp: DEFAULT_LAST_FINALIZED_TIMESTAMP,
          endBlockNumber: BigInt(calldataAggregatedProof1To155.finalBlockNumber),
          parentStateRootHash: calldataAggregatedProof1To155.parentStateRootHash,
          finalTimestamp: BigInt(calldataAggregatedProof1To155.finalTimestamp),
          l2MerkleRoots: calldataAggregatedProof1To155.l2MerkleRoots,
          l2MerkleTreesDepth: BigInt(calldataAggregatedProof1To155.l2MerkleTreesDepth),
          l2MessagingBlocksOffsets: calldataAggregatedProof1To155.l2MessagingBlocksOffsets,
          aggregatedProof: calldataAggregatedProof1To155.aggregatedProof,
          shnarfData: generateParentShnarfData(index),
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
        const submissionDataBeforeFinalization = generateCallDataSubmission(0, 4);
        let index = 0;
        for (const data of submissionDataBeforeFinalization) {
          const parentAndExpectedShnarf = generateParentAndExpectedShnarfForIndex(index);
          await lineaRollup
            .connect(operator)
            .submitDataAsCalldata(data, parentAndExpectedShnarf.parentShnarf, parentAndExpectedShnarf.expectedShnarf, {
              gasLimit: MAX_GAS_LIMIT,
            });
          index++;
        }

        const finalizationData = await generateFinalizationData({
          l1RollingHash: calculateRollingHash(HASH_ZERO, messageHash),
          l1RollingHashMessageNumber: 10n,
          lastFinalizedTimestamp: DEFAULT_LAST_FINALIZED_TIMESTAMP,
          endBlockNumber: BigInt(calldataAggregatedProof1To155.finalBlockNumber),
          parentStateRootHash: calldataAggregatedProof1To155.parentStateRootHash,
          finalTimestamp: BigInt(new Date(new Date().setHours(new Date().getHours() + 2)).getTime()), // Set to 2 hours in the future
          l2MerkleRoots: calldataAggregatedProof1To155.l2MerkleRoots,
          l2MerkleTreesDepth: BigInt(calldataAggregatedProof1To155.l2MerkleTreesDepth),
          l2MessagingBlocksOffsets: calldataAggregatedProof1To155.l2MessagingBlocksOffsets,
          aggregatedProof: calldataAggregatedProof1To155.aggregatedProof,
          shnarfData: generateParentShnarfData(index),
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
        const submissionDataBeforeFinalization = generateCallDataSubmission(0, 4);
        let index = 0;
        for (const data of submissionDataBeforeFinalization) {
          const parentAndExpectedShnarf = generateParentAndExpectedShnarfForIndex(index);
          await lineaRollup
            .connect(operator)
            .submitDataAsCalldata(data, parentAndExpectedShnarf.parentShnarf, parentAndExpectedShnarf.expectedShnarf, {
              gasLimit: MAX_GAS_LIMIT,
            });
          index++;
        }

        const finalizationData = await generateFinalizationData({
          l1RollingHash: calculateRollingHash(HASH_ZERO, messageHash),
          l1RollingHashMessageNumber: 10n,
          lastFinalizedTimestamp: DEFAULT_LAST_FINALIZED_TIMESTAMP,
          endBlockNumber: BigInt(calldataAggregatedProof1To155.finalBlockNumber),
          parentStateRootHash: calldataAggregatedProof1To155.parentStateRootHash,
          finalTimestamp: BigInt(calldataAggregatedProof1To155.finalTimestamp),
          l2MerkleRoots: calldataAggregatedProof1To155.l2MerkleRoots,
          l2MerkleTreesDepth: BigInt(calldataAggregatedProof1To155.l2MerkleTreesDepth),
          l2MessagingBlocksOffsets: calldataAggregatedProof1To155.l2MessagingBlocksOffsets,
          aggregatedProof: calldataAggregatedProof1To155.aggregatedProof,
          shnarfData: generateParentShnarfData(index),
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

    it("Should revert if GENERAL_PAUSE_TYPE is enabled", async () => {
      const finalizationData = await generateFinalizationData();

      await lineaRollup.connect(securityCouncil).pauseByType(GENERAL_PAUSE_TYPE);

      const finalizeCall = lineaRollup
        .connect(operator)
        .finalizeBlocks(EMPTY_CALLDATA, TEST_PUBLIC_VERIFIER_INDEX, finalizationData);
      await expectRevertWithCustomError(lineaRollup, finalizeCall, "IsPaused", [GENERAL_PAUSE_TYPE]);
    });

    it("Should revert if FINALIZATION_PAUSE_TYPE is enabled", async () => {
      const finalizationData = await generateFinalizationData();

      await lineaRollup.connect(securityCouncil).pauseByType(FINALIZATION_PAUSE_TYPE);

      const finalizeCall = lineaRollup
        .connect(operator)
        .finalizeBlocks(EMPTY_CALLDATA, TEST_PUBLIC_VERIFIER_INDEX, finalizationData);
      await expectRevertWithCustomError(lineaRollup, finalizeCall, "IsPaused", [FINALIZATION_PAUSE_TYPE]);
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
      const submissionDataBeforeFinalization = generateCallDataSubmission(0, 4);

      let index = 0;
      for (const data of submissionDataBeforeFinalization) {
        const parentAndExpectedShnarf = generateParentAndExpectedShnarfForIndex(index);
        await lineaRollup
          .connect(operator)
          .submitDataAsCalldata(data, parentAndExpectedShnarf.parentShnarf, parentAndExpectedShnarf.expectedShnarf, {
            gasLimit: MAX_GAS_LIMIT,
          });
        index++;
      }

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
      const submissionDataBeforeFinalization = generateCallDataSubmission(0, 4);
      let index = 0;
      for (const data of submissionDataBeforeFinalization) {
        const parentAndExpectedShnarf = generateParentAndExpectedShnarfForIndex(index);
        await lineaRollup
          .connect(operator)
          .submitDataAsCalldata(data, parentAndExpectedShnarf.parentShnarf, parentAndExpectedShnarf.expectedShnarf, {
            gasLimit: MAX_GAS_LIMIT,
          });
        index++;
      }

      await expectSuccessfulFinalize(
        lineaRollup,
        operator,
        calldataAggregatedProof1To155,
        index,
        fourthCompressedDataContent.finalStateRootHash,
        generateParentShnarfData,
      );
    });

    it("Should revert when proofType is invalid", async () => {
      const submissionDataBeforeFinalization = generateCallDataSubmission(0, 4);
      let index = 0;
      for (const data of submissionDataBeforeFinalization) {
        const parentAndExpectedShnarf = generateParentAndExpectedShnarfForIndex(index);
        await lineaRollup
          .connect(operator)
          .submitDataAsCalldata(data, parentAndExpectedShnarf.parentShnarf, parentAndExpectedShnarf.expectedShnarf, {
            gasLimit: MAX_GAS_LIMIT,
          });
        index++;
      }

      const finalizationData = await generateFinalizationData({
        l1RollingHash: calldataAggregatedProof1To155.l1RollingHash,
        l1RollingHashMessageNumber: BigInt(calldataAggregatedProof1To155.l1RollingHashMessageNumber),
        lastFinalizedTimestamp: BigInt(calldataAggregatedProof1To155.parentAggregationLastBlockTimestamp),
        endBlockNumber: BigInt(calldataAggregatedProof1To155.finalBlockNumber),
        parentStateRootHash: calldataAggregatedProof1To155.parentStateRootHash,
        finalTimestamp: BigInt(calldataAggregatedProof1To155.finalTimestamp),
        l2MerkleRoots: calldataAggregatedProof1To155.l2MerkleRoots,
        l2MerkleTreesDepth: BigInt(calldataAggregatedProof1To155.l2MerkleTreesDepth),
        l2MessagingBlocksOffsets: calldataAggregatedProof1To155.l2MessagingBlocksOffsets,
        aggregatedProof: calldataAggregatedProof1To155.aggregatedProof,
        shnarfData: generateParentShnarfData(index),
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
      const submissionDataBeforeFinalization = generateCallDataSubmission(0, 4);
      let index = 0;
      for (const data of submissionDataBeforeFinalization) {
        const parentAndExpectedShnarf = generateParentAndExpectedShnarfForIndex(index);
        await lineaRollup
          .connect(operator)
          .submitDataAsCalldata(data, parentAndExpectedShnarf.parentShnarf, parentAndExpectedShnarf.expectedShnarf, {
            gasLimit: MAX_GAS_LIMIT,
          });
        index++;
      }

      const finalizationData = await generateFinalizationData({
        l1RollingHash: calldataAggregatedProof1To155.l1RollingHash,
        l1RollingHashMessageNumber: BigInt(calldataAggregatedProof1To155.l1RollingHashMessageNumber),
        lastFinalizedTimestamp: BigInt(calldataAggregatedProof1To155.parentAggregationLastBlockTimestamp),
        endBlockNumber: BigInt(calldataAggregatedProof1To155.finalBlockNumber),
        parentStateRootHash: calldataAggregatedProof1To155.parentStateRootHash,
        finalTimestamp: BigInt(calldataAggregatedProof1To155.finalTimestamp),
        l2MerkleRoots: calldataAggregatedProof1To155.l2MerkleRoots,
        l2MerkleTreesDepth: BigInt(calldataAggregatedProof1To155.l2MerkleTreesDepth),
        l2MessagingBlocksOffsets: calldataAggregatedProof1To155.l2MessagingBlocksOffsets,
        aggregatedProof: calldataAggregatedProof1To155.aggregatedProof,
        shnarfData: generateParentShnarfData(index),
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
      const submissionDataBeforeFinalization = generateCallDataSubmission(0, 4);
      let index = 0;
      for (const data of submissionDataBeforeFinalization) {
        const parentAndExpectedShnarf = generateParentAndExpectedShnarfForIndex(index);
        await lineaRollup
          .connect(operator)
          .submitDataAsCalldata(data, parentAndExpectedShnarf.parentShnarf, parentAndExpectedShnarf.expectedShnarf, {
            gasLimit: MAX_GAS_LIMIT,
          });
        index++;
      }

      const finalizationData = await generateFinalizationData({
        l1RollingHash: calldataAggregatedProof1To155.l1RollingHash,
        l1RollingHashMessageNumber: BigInt(calldataAggregatedProof1To155.l1RollingHashMessageNumber),
        lastFinalizedTimestamp: BigInt(calldataAggregatedProof1To155.parentAggregationLastBlockTimestamp),
        endBlockNumber: BigInt(calldataAggregatedProof1To155.finalBlockNumber),
        parentStateRootHash: calldataAggregatedProof1To155.parentStateRootHash,
        finalTimestamp: BigInt(calldataAggregatedProof1To155.finalTimestamp),
        l2MerkleRoots: calldataAggregatedProof1To155.l2MerkleRoots,
        l2MerkleTreesDepth: BigInt(calldataAggregatedProof1To155.l2MerkleTreesDepth),
        l2MessagingBlocksOffsets: calldataAggregatedProof1To155.l2MessagingBlocksOffsets,
        aggregatedProof: calldataAggregatedProof1To155.aggregatedProof,
        shnarfData: generateParentShnarfData(index),
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
      const submissionDataBeforeFinalization = generateCallDataSubmission(0, 4);
      let index = 0;
      for (const data of submissionDataBeforeFinalization) {
        const parentAndExpectedShnarf = generateParentAndExpectedShnarfForIndex(index);
        await lineaRollup
          .connect(operator)
          .submitDataAsCalldata(data, parentAndExpectedShnarf.parentShnarf, parentAndExpectedShnarf.expectedShnarf, {
            gasLimit: MAX_GAS_LIMIT,
          });
        index++;
      }

      const finalizationData = await generateFinalizationData({
        l1RollingHash: calldataAggregatedProof1To155.l1RollingHash,
        l1RollingHashMessageNumber: BigInt(calldataAggregatedProof1To155.l1RollingHashMessageNumber),
        lastFinalizedTimestamp: BigInt(calldataAggregatedProof1To155.parentAggregationLastBlockTimestamp),
        endBlockNumber: BigInt(calldataAggregatedProof1To155.finalBlockNumber),
        parentStateRootHash: calldataAggregatedProof1To155.parentStateRootHash,
        finalTimestamp: BigInt(calldataAggregatedProof1To155.finalTimestamp),
        l2MerkleRoots: calldataAggregatedProof1To155.l2MerkleRoots,
        l2MerkleTreesDepth: BigInt(calldataAggregatedProof1To155.l2MerkleTreesDepth),
        l2MessagingBlocksOffsets: calldataAggregatedProof1To155.l2MessagingBlocksOffsets,
        aggregatedProof: calldataAggregatedProof1To155.aggregatedProof,
        shnarfData: generateParentShnarfData(1),
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
      const submissionDataBeforeFinalization = generateCallDataSubmissionMultipleProofs(0, 4);
      let index = 0;
      for (const data of submissionDataBeforeFinalization) {
        const parentAndExpectedShnarf = generateParentAndExpectedShnarfForMulitpleIndex(index);
        await lineaRollup
          .connect(operator)
          .submitDataAsCalldata(data, parentAndExpectedShnarf.parentShnarf, parentAndExpectedShnarf.expectedShnarf, {
            gasLimit: MAX_GAS_LIMIT,
          });
        index++;
      }

      await expectSuccessfulFinalize(
        lineaRollup,
        operator,
        aggregatedProof1To81,
        2,
        secondCompressedDataContent.finalStateRootHash,
        generateParentShnarfData,
        true,
      );

      await expectSuccessfulFinalize(
        lineaRollup,
        operator,
        aggregatedProof82To153,
        4,
        fourthMultipleCompressedDataContent.finalStateRootHash,
        generateParentShnarfData,
        true,
        aggregatedProof1To81.l1RollingHash,
        BigInt(aggregatedProof1To81.l1RollingHashMessageNumber),
      );
    });

    it("Should succeed when sending with pure calldata", async () => {
      const submissionDataBeforeFinalization = generateCallDataSubmissionMultipleProofs(0, 4);
      let index = 0;
      for (const data of submissionDataBeforeFinalization) {
        const parentAndExpectedShnarf = generateParentAndExpectedShnarfForMulitpleIndex(index);
        await lineaRollup
          .connect(operator)
          .submitDataAsCalldata(data, parentAndExpectedShnarf.parentShnarf, parentAndExpectedShnarf.expectedShnarf, {
            gasLimit: MAX_GAS_LIMIT,
          });
        index++;
      }
      await lineaRollup.setRollingHash(
        aggregatedProof1To81.l1RollingHashMessageNumber,
        aggregatedProof1To81.l1RollingHash,
      );

      const lineaRollupAddress = await lineaRollup.getAddress();

      const { maxFeePerGas, maxPriorityFeePerGas } = await ethers.provider.getFeeData();

      const finalizationData = await generateFinalizationData({
        l1RollingHash: aggregatedProof1To81.l1RollingHash,
        l1RollingHashMessageNumber: BigInt(aggregatedProof1To81.l1RollingHashMessageNumber),
        lastFinalizedTimestamp: BigInt(aggregatedProof1To81.parentAggregationLastBlockTimestamp),
        parentStateRootHash: aggregatedProof1To81.parentStateRootHash,
        finalTimestamp: BigInt(aggregatedProof1To81.finalTimestamp),
        l2MerkleRoots: aggregatedProof1To81.l2MerkleRoots,
        l2MerkleTreesDepth: BigInt(aggregatedProof1To81.l2MerkleTreesDepth),
        l2MessagingBlocksOffsets: aggregatedProof1To81.l2MessagingBlocksOffsets,
        aggregatedProof: aggregatedProof1To81.aggregatedProof,
        shnarfData: generateParentShnarfData(2, true),
        endBlockNumber: BigInt(aggregatedProof1To81.finalBlockNumber),
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

    it("Should fail when sending with wrong merkle root location", async () => {
      const submissionDataBeforeFinalization = generateCallDataSubmissionMultipleProofs(0, 4);
      let index = 0;
      for (const data of submissionDataBeforeFinalization) {
        const parentAndExpectedShnarf = generateParentAndExpectedShnarfForMulitpleIndex(index);
        await lineaRollup
          .connect(operator)
          .submitDataAsCalldata(data, parentAndExpectedShnarf.parentShnarf, parentAndExpectedShnarf.expectedShnarf, {
            gasLimit: MAX_GAS_LIMIT,
          });
        index++;
      }

      await lineaRollup.setRollingHash(
        aggregatedProof1To81.l1RollingHashMessageNumber,
        aggregatedProof1To81.l1RollingHash,
      );

      const lineaRollupAddress = await lineaRollup.getAddress();

      const { maxFeePerGas, maxPriorityFeePerGas } = await ethers.provider.getFeeData();

      // This contains the Merkle roots length for public input at 0x220 and a contrived pointer to an alternate location.
      const encodedCall =
        "0x5603c65f0000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003e0000000000000000000000000000000000000000000000000000000000000036029093956ab5035511b506befae52761827f5c0b29e5f3fa143f7be36903f86ef0a63534b32dea73409e29e219adc9e09b00fb4558db1a1739e79cb3a4703281f0b9d0a323788faa38d9458e3a3183bd287992feddde4cd6cb87496592ccb3bca1dd7295bdb77b48ef8988fe6526edd09a0ede4cce8c9c6f7e515c0efe30277292ba2fadf583b5733e40160dc694e051c5b337e9ad1fadde2ab9d6f4e1002f5de21c59d7f748ce46d9b4acbe44d94aed8bcc0156f7049ade3682008d3fe2bdaa72d68d3df8990602e3d269eb340912b6ba727d9860c6bcdde6cbe7228c6585b0e0f35f6639f670594e7e9e88223d9347ab41335acf70042522e4293bfc107f9f90b0a914931f52ac9381d77dc2c76f8c725c27bf11c90f0ed24ffc0ca47267c032c9a8f301d25494b7c452a07569a5bf36c7348d6c24f7a01c2915f2f61fb67940b15c61094a97233781358069193096ea7ae59e647fb8db081b0686af975465326fdabf16c3c84287ce479e6f7193791ff4e48f0cbe936f95a1997c00fd9b56400051a583c351752e45eda356fe2718f5f6acdef13c164f0e8034721bd2e2ed7182085ec23369082a63e76bb855556c7ce61cf8a3a5b64777385a3a4f16ca3aa1269c9e17b9f7d5f972b595987d41ff9ba367ead153d577bcc42f8cc9e3514552deac55abe689986825032bde231b654d603926d26edf53dda0770dabaddbca9000000000000000000000000000000000000000000000000000000000000028009084f28cf6e68e4ac57cd82dcb41b7cb56582c5aaf46b73600b8827428362b627f5327d19461a3c38284b0f7f82bad8e2387622f6e6fcd079bbe870ea28fa4a21c410e80f368850186b8b90e2ac2ca4ae7f1e5c45952a53350c379411e97f9815bf75539c93a0ac5d92127e2783cadc95f7b4175a4e6540f20c8e4cb11e6a561eabc321ed04cd7a16654b224f9be0dd2720e23a9ddc64bca5e09207c196876627a9a2327c896d8f22f653a4b825817bdb0af45a90d8b102f18e6124066a70260da7bd8c45e30c2112aedf651e9eb28baaa67ba94f41b4a86a268e8e7fde04f6069c4955704012f7aac86268e86ac97cb6dd16d2b377b506ceae8befe75f97af24436b34ab6038c4df3c0a2fb824606759243502d54448265ada6e735349545f063059274a39d495cd84ae7cee59a4314de7a97e0c2d77b08fc27aecb2867faf072ead6777750dc20232d1cee8dc9a395c2d350df4bbaa5096c6f59b214dcecd00000000000000000000000000000000000000000000000000000000000000516f19a5c3028ba90d326cd4e590c586b254386522b9628ed6f7939d214b1c8cea045431943d21416d41524505917a59698c3dab060d85e56f9c372c1dafc52344f0f26782f7afb93f926cacb145f55530714f20b1356725e3971dc99e0ef8b5911ed2f0038be4e78f2a8e767d889df61a483777a0aeb5453ada13ed1d5b4f012a2199bae65182f02c716c8e54f5714561a45694b982da8c8143333d0206078d3f00000000000000000000000000000000000000000000000000000000645580d1000000000000000000000000000000000000000000000000000000006455a7e10000000000000000000000000000000000000000000000000000000000000000dc8e70637c1048e1e0406c4ed6fab51a7489ccb52f37ddd2c135cb1aa18ec6970000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000050000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000024000000000000000000000000000000000000000000000000000000000000000016de197db892fd7d3fe0a389452dae0c5d0520e23d18ad20327546e2189a7e3f1000000000000000000000000000000000000000000000000000000000000000e00000000000000000000000fffff000000000000000000000000000000000000";
      const transaction = {
        data: encodedCall,
        maxPriorityFeePerGas: maxPriorityFeePerGas!,
        maxFeePerGas: maxFeePerGas!,
        to: lineaRollupAddress,
        chainId: 31337,
        value: 0,
        gasLimit: 5_000_000,
      };

      await expectRevertWithCustomError(lineaRollup, operator.sendTransaction(transaction), "InvalidProof");
    });

    it("Should fail to finalize with extra merkle roots", async () => {
      const submissionDataBeforeFinalization = generateCallDataSubmissionMultipleProofs(0, 4);
      let index = 0;
      for (const data of submissionDataBeforeFinalization) {
        const parentAndExpectedShnarf = generateParentAndExpectedShnarfForMulitpleIndex(index);
        await lineaRollup
          .connect(operator)
          .submitDataAsCalldata(data, parentAndExpectedShnarf.parentShnarf, parentAndExpectedShnarf.expectedShnarf, {
            gasLimit: MAX_GAS_LIMIT,
          });
        index++;
      }

      const merkleRoots = aggregatedProof1To81.l2MerkleRoots;
      merkleRoots.push(generateRandomBytes(32));

      const finalizationData = await generateFinalizationData({
        l1RollingHash: aggregatedProof1To81.l1RollingHash,
        l1RollingHashMessageNumber: BigInt(aggregatedProof1To81.l1RollingHashMessageNumber),
        lastFinalizedTimestamp: BigInt(aggregatedProof1To81.parentAggregationLastBlockTimestamp),
        parentStateRootHash: aggregatedProof1To81.parentStateRootHash,
        finalTimestamp: BigInt(aggregatedProof1To81.finalTimestamp),
        l2MerkleRoots: merkleRoots,
        l2MerkleTreesDepth: BigInt(aggregatedProof1To81.l2MerkleTreesDepth),
        l2MessagingBlocksOffsets: aggregatedProof1To81.l2MessagingBlocksOffsets,
        aggregatedProof: aggregatedProof1To81.aggregatedProof,
        shnarfData: generateParentShnarfData(2, true),
      });

      finalizationData.lastFinalizedL1RollingHash = HASH_ZERO;
      finalizationData.lastFinalizedL1RollingHashMessageNumber = 0n;

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
});
