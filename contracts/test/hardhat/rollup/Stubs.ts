/**
 * Tests for the integration placeholder contracts (LinethRollupV9Stub / ValidiumV3Stub).
 *
 * The stubs override data submission and finalization as no-ops while keeping every other behavior inherited.
 * These tests pin that down: the overrides keep their role and pause gating, change no state and emit nothing,
 * and the inherited version-gated initialize / reinitialize behavior is unchanged.
 */
import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { FINALIZATION_PAUSE_TYPE, STATE_DATA_SUBMISSION_PAUSE_TYPE } from "contracts/common/constants";
import { LinethRollupV9Stub, ValidiumV3Stub } from "contracts/typechain-types";

import { deployLinethRollupV9StubFixture, deployValidiumV3StubFixture, getAccountsFixture } from "./helpers";
import { HASH_ZERO, INITIALIZED_ALREADY_MESSAGE, OPERATOR_ROLE } from "../common/constants";
import {
  buildAccessErrorMessage,
  expectNoEvent,
  expectRevertWithReason,
  expectRevertWhenPaused,
  generateRandomBytes,
} from "../common/helpers";

const emptyFinalizationData = () => ({
  parentBlockHash: HASH_ZERO,
  endBlockNumber: 10n,
  lastFinalizedTimestamp: 0n,
  finalTimestamp: 0n,
  lastFinalizedL1RollingHash: HASH_ZERO,
  l1RollingHash: HASH_ZERO,
  lastFinalizedL1RollingHashMessageNumber: 0n,
  l1RollingHashMessageNumber: 0n,
  l2MerkleTreesDepth: 0n,
  lastFinalizedForcedTransactionNumber: 0n,
  finalForcedTransactionNumber: 0n,
  lastFinalizedForcedTransactionRollingHash: HASH_ZERO,
  finalBlockHash: generateRandomBytes(32),
  parentDataRollingHash: generateRandomBytes(32),
  endDataRollingHash: generateRandomBytes(32),
  parentDataTailTakeBytes: 0n,
  finalDataTailDiscardBytes: 0n,
  l2MerkleRoots: [],
  filteredAddresses: [],
  verifierKeys: [],
  l2MessagingBlocksOffsets: "0x",
});

describe("Integration stubs", () => {
  let securityCouncil: SignerWithAddress;
  let operator: SignerWithAddress;
  let nonAuthorizedAccount: SignerWithAddress;

  before(async () => {
    ({ securityCouncil, operator, nonAuthorizedAccount } = await loadFixture(getAccountsFixture));
  });

  describe("LinethRollupV9Stub", () => {
    let stub: LinethRollupV9Stub;

    beforeEach(async () => {
      ({ linethRollup: stub } = await loadFixture(deployLinethRollupV9StubFixture));
    });

    it("Initializes at the latest version and rejects the reinitializer", async () => {
      expect(await stub.CONTRACT_VERSION()).to.equal("9.0");
      await expectRevertWithReason(stub.reinitializeLineaRollupV10(), INITIALIZED_ALREADY_MESSAGE);
    });

    it("submitBlobs is a no-op for the operator", async () => {
      const stored = generateRandomBytes(32);
      const currentDataRollingHash = await stub.currentDataRollingHash();

      await expectNoEvent(stub, stub.connect(operator).submitBlobs(currentDataRollingHash, stored), "DataSubmittedV4");

      expect(await stub.dataRollingHashExists(stored)).to.equal(0n);
      expect(await stub.currentDataRollingHash()).to.equal(currentDataRollingHash);
    });

    it("finalizeBlocks is a no-op for the operator", async () => {
      const before = {
        currentL2BlockNumber: await stub.currentL2BlockNumber(),
        currentFinalizedState: await stub.currentFinalizedState(),
        currentDataRollingHash: await stub.currentDataRollingHash(),
      };

      await expectNoEvent(
        stub,
        stub.connect(operator).finalizeBlocks("0x", 0, emptyFinalizationData()),
        "FinalizedStateUpdated",
      );

      expect(await stub.currentL2BlockNumber()).to.equal(before.currentL2BlockNumber);
      expect(await stub.currentFinalizedState()).to.equal(before.currentFinalizedState);
      expect(await stub.currentDataRollingHash()).to.equal(before.currentDataRollingHash);
    });

    it("Keeps OPERATOR_ROLE gating on the no-op overrides", async () => {
      await expectRevertWithReason(
        stub.connect(nonAuthorizedAccount).submitBlobs(HASH_ZERO, generateRandomBytes(32)),
        buildAccessErrorMessage(nonAuthorizedAccount, OPERATOR_ROLE),
      );
      await expectRevertWithReason(
        stub.connect(nonAuthorizedAccount).finalizeBlocks("0x", 0, emptyFinalizationData()),
        buildAccessErrorMessage(nonAuthorizedAccount, OPERATOR_ROLE),
      );
    });

    it("Keeps pause gating on the no-op overrides", async () => {
      await stub.connect(securityCouncil).pauseByType(STATE_DATA_SUBMISSION_PAUSE_TYPE);
      await expectRevertWhenPaused(
        stub,
        stub.connect(operator).submitBlobs(HASH_ZERO, generateRandomBytes(32)),
        STATE_DATA_SUBMISSION_PAUSE_TYPE,
      );

      await stub.connect(securityCouncil).pauseByType(FINALIZATION_PAUSE_TYPE);
      await expectRevertWhenPaused(
        stub,
        stub.connect(operator).finalizeBlocks("0x", 0, emptyFinalizationData()),
        FINALIZATION_PAUSE_TYPE,
      );
    });
  });

  describe("ValidiumV3Stub", () => {
    let stub: ValidiumV3Stub;

    beforeEach(async () => {
      ({ validium: stub } = await loadFixture(deployValidiumV3StubFixture));
    });

    it("Initializes at the latest version and rejects the reinitializer", async () => {
      expect(await stub.CONTRACT_VERSION()).to.equal("3.0");
      await expectRevertWithReason(stub.reinitializeValidiumV3(), INITIALIZED_ALREADY_MESSAGE);
    });

    it("acceptDataRollingHash is a no-op for the operator", async () => {
      const stored = generateRandomBytes(32);
      const currentDataRollingHash = await stub.currentDataRollingHash();

      await expectNoEvent(
        stub,
        stub.connect(operator).acceptDataRollingHash(currentDataRollingHash, stored),
        "DataSubmittedV4",
      );

      expect(await stub.dataRollingHashExists(stored)).to.equal(0n);
      expect(await stub.currentDataRollingHash()).to.equal(currentDataRollingHash);
    });

    it("finalizeBlocks is a no-op for the operator", async () => {
      const before = {
        currentL2BlockNumber: await stub.currentL2BlockNumber(),
        currentFinalizedState: await stub.currentFinalizedState(),
        currentDataRollingHash: await stub.currentDataRollingHash(),
      };

      await expectNoEvent(
        stub,
        stub.connect(operator).finalizeBlocks("0x", 0, emptyFinalizationData()),
        "FinalizedStateUpdated",
      );

      expect(await stub.currentL2BlockNumber()).to.equal(before.currentL2BlockNumber);
      expect(await stub.currentFinalizedState()).to.equal(before.currentFinalizedState);
      expect(await stub.currentDataRollingHash()).to.equal(before.currentDataRollingHash);
    });

    it("Keeps OPERATOR_ROLE gating on the no-op overrides", async () => {
      await expectRevertWithReason(
        stub.connect(nonAuthorizedAccount).acceptDataRollingHash(HASH_ZERO, generateRandomBytes(32)),
        buildAccessErrorMessage(nonAuthorizedAccount, OPERATOR_ROLE),
      );
      await expectRevertWithReason(
        stub.connect(nonAuthorizedAccount).finalizeBlocks("0x", 0, emptyFinalizationData()),
        buildAccessErrorMessage(nonAuthorizedAccount, OPERATOR_ROLE),
      );
    });

    it("Keeps pause gating on the no-op overrides", async () => {
      await stub.connect(securityCouncil).pauseByType(STATE_DATA_SUBMISSION_PAUSE_TYPE);
      await expectRevertWhenPaused(
        stub,
        stub.connect(operator).acceptDataRollingHash(HASH_ZERO, generateRandomBytes(32)),
        STATE_DATA_SUBMISSION_PAUSE_TYPE,
      );

      await stub.connect(securityCouncil).pauseByType(FINALIZATION_PAUSE_TYPE);
      await expectRevertWhenPaused(
        stub,
        stub.connect(operator).finalizeBlocks("0x", 0, emptyFinalizationData()),
        FINALIZATION_PAUSE_TYPE,
      );
    });
  });
});
