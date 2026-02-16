import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { ethers } from "hardhat";
import { TestPauseManager } from "../../../typechain-types";
import {
  DEFAULT_ADMIN_ROLE,
  GENERAL_PAUSE_TYPE,
  INITIALIZED_ALREADY_MESSAGE,
  L1_L2_PAUSE_TYPE,
  L2_L1_PAUSE_TYPE,
  PAUSE_ALL_ROLE,
  UNPAUSE_ALL_ROLE,
  PAUSE_L1_L2_ROLE,
  UNPAUSE_L1_L2_ROLE,
  PAUSE_L2_L1_ROLE,
  UNPAUSE_L2_L1_ROLE,
  PAUSE_FINALIZATION_ROLE,
  PAUSE_STATE_DATA_SUBMISSION_ROLE,
  UNPAUSE_FINALIZATION_ROLE,
  UNPAUSE_STATE_DATA_SUBMISSION_ROLE,
  SECURITY_COUNCIL_ROLE,
  FINALIZATION_PAUSE_TYPE,
  pauseTypeRoles,
  unpauseTypeRoles,
  UNUSED_PAUSE_TYPE,
  STATE_DATA_SUBMISSION_PAUSE_TYPE,
} from "../common/constants";
import { deployUpgradableFromFactory } from "../common/deployment";
import {
  buildAccessErrorMessage,
  expectEvent,
  expectRevertWithCustomError,
  expectRevertWithReason,
  getLastBlockTimestamp,
  setFutureTimestampForNextBlock,
} from "../common/helpers";

async function deployTestPauseManagerFixture(): Promise<TestPauseManager> {
  return deployUpgradableFromFactory("TestPauseManager", [
    pauseTypeRoles,
    unpauseTypeRoles,
  ]) as unknown as Promise<TestPauseManager>;
}

describe("PauseManager", () => {
  let defaultAdmin: SignerWithAddress;
  let pauseManagerAccount: SignerWithAddress;
  let nonManager: SignerWithAddress;
  let securityCouncil: SignerWithAddress;
  let pauseManager: TestPauseManager;

  beforeEach(async () => {
    [defaultAdmin, pauseManagerAccount, nonManager, securityCouncil] = await ethers.getSigners();
    pauseManager = await loadFixture(deployTestPauseManagerFixture);

    await Promise.all([
      // Roles for pauseManagerAccount
      pauseManager.grantRole(PAUSE_ALL_ROLE, pauseManagerAccount.address),
      pauseManager.grantRole(UNPAUSE_ALL_ROLE, pauseManagerAccount.address),
      pauseManager.grantRole(PAUSE_L1_L2_ROLE, pauseManagerAccount.address),
      pauseManager.grantRole(UNPAUSE_L1_L2_ROLE, pauseManagerAccount.address),
      pauseManager.grantRole(PAUSE_L2_L1_ROLE, pauseManagerAccount.address),
      pauseManager.grantRole(UNPAUSE_L2_L1_ROLE, pauseManagerAccount.address),
      pauseManager.grantRole(PAUSE_STATE_DATA_SUBMISSION_ROLE, pauseManagerAccount.address),
      pauseManager.grantRole(UNPAUSE_STATE_DATA_SUBMISSION_ROLE, pauseManagerAccount.address),
      pauseManager.grantRole(PAUSE_FINALIZATION_ROLE, pauseManagerAccount.address),
      pauseManager.grantRole(UNPAUSE_FINALIZATION_ROLE, pauseManagerAccount.address),
      // Roles for securityCouncil
      pauseManager.grantRole(PAUSE_ALL_ROLE, securityCouncil.address),
      pauseManager.grantRole(UNPAUSE_ALL_ROLE, securityCouncil.address),
      pauseManager.grantRole(UNPAUSE_L1_L2_ROLE, securityCouncil.address),
      pauseManager.grantRole(UNPAUSE_L2_L1_ROLE, securityCouncil.address),
      pauseManager.grantRole(UNPAUSE_STATE_DATA_SUBMISSION_ROLE, securityCouncil.address),
      pauseManager.grantRole(UNPAUSE_FINALIZATION_ROLE, securityCouncil.address),
      pauseManager.grantRole(SECURITY_COUNCIL_ROLE, securityCouncil.address),
    ]);
  });

  async function pauseByType(pauseType: number, account: SignerWithAddress = pauseManagerAccount) {
    return pauseManager.connect(account).pauseByType(pauseType);
  }

  async function unPauseByType(pauseType: number, account: SignerWithAddress = pauseManagerAccount) {
    return pauseManager.connect(account).unPauseByType(pauseType);
  }

  async function unPauseByExpiredType(pauseType: number, account: SignerWithAddress) {
    return pauseManager.connect(account).unPauseByExpiredType(pauseType);
  }

  describe("Initialization checks", () => {
    it("Deployer has default admin role", async () => {
      expect(await pauseManager.hasRole(DEFAULT_ADMIN_ROLE, defaultAdmin.address)).to.be.true;
    });

    it("Second initialisation while initializing fails", async () => {
      await expect(pauseManager.initialize(pauseTypeRoles, unpauseTypeRoles)).to.be.revertedWith(
        INITIALIZED_ALREADY_MESSAGE,
      );
    });
  });

  describe("Updating pause type and unpause type roles", () => {
    it("should fail updatePauseTypeRole if unused pause type is used", async () => {
      const updateCall = pauseManager.updatePauseTypeRole(UNUSED_PAUSE_TYPE, DEFAULT_ADMIN_ROLE);
      await expectRevertWithCustomError(pauseManager, updateCall, "PauseTypeNotUsed");
    });

    it("should fail updateUnpauseTypeRole if unused pause type is used", async () => {
      const updateCall = pauseManager.updateUnpauseTypeRole(UNUSED_PAUSE_TYPE, DEFAULT_ADMIN_ROLE);
      await expectRevertWithCustomError(pauseManager, updateCall, "PauseTypeNotUsed");
    });

    it("should fail updatePauseTypeRole if correct role not used", async () => {
      const updateCall = pauseManager
        .connect(pauseManagerAccount)
        .updatePauseTypeRole(GENERAL_PAUSE_TYPE, DEFAULT_ADMIN_ROLE);
      await expectRevertWithReason(updateCall, buildAccessErrorMessage(pauseManagerAccount, SECURITY_COUNCIL_ROLE));
    });

    it("should fail updateUnpauseTypeRole if correct role not used", async () => {
      const updateCall = pauseManager
        .connect(pauseManagerAccount)
        .updateUnpauseTypeRole(GENERAL_PAUSE_TYPE, DEFAULT_ADMIN_ROLE);
      await expectRevertWithReason(updateCall, buildAccessErrorMessage(pauseManagerAccount, SECURITY_COUNCIL_ROLE));
    });

    it("should fail updateUnpauseTypeRole if roles are not different", async () => {
      const updateCall = pauseManager.connect(securityCouncil).updatePauseTypeRole(GENERAL_PAUSE_TYPE, PAUSE_ALL_ROLE);
      await expectRevertWithCustomError(pauseManager, updateCall, "RolesNotDifferent");
    });

    it("should fail updateUnpauseTypeRole if roles are not different", async () => {
      const updateCall = pauseManager
        .connect(securityCouncil)
        .updateUnpauseTypeRole(GENERAL_PAUSE_TYPE, UNPAUSE_ALL_ROLE);
      await expectRevertWithCustomError(pauseManager, updateCall, "RolesNotDifferent");
    });

    it("should update pause type role with pausing working", async () => {
      const updateCall = pauseManager
        .connect(securityCouncil)
        .updatePauseTypeRole(GENERAL_PAUSE_TYPE, DEFAULT_ADMIN_ROLE);
      await expectEvent(pauseManager, updateCall, "PauseTypeRoleUpdated", [
        GENERAL_PAUSE_TYPE,
        DEFAULT_ADMIN_ROLE,
        PAUSE_ALL_ROLE,
      ]);

      await pauseManager.connect(defaultAdmin).pauseByType(GENERAL_PAUSE_TYPE);
      expect(await pauseManager.isPaused(GENERAL_PAUSE_TYPE)).to.be.true;
    });

    it("should fail to pause with old role", async () => {
      const updateCall = pauseManager
        .connect(securityCouncil)
        .updatePauseTypeRole(GENERAL_PAUSE_TYPE, DEFAULT_ADMIN_ROLE);
      await expectEvent(pauseManager, updateCall, "PauseTypeRoleUpdated", [
        GENERAL_PAUSE_TYPE,
        DEFAULT_ADMIN_ROLE,
        PAUSE_ALL_ROLE,
      ]);

      await expectRevertWithReason(
        pauseManager.connect(securityCouncil).pauseByType(GENERAL_PAUSE_TYPE),
        buildAccessErrorMessage(securityCouncil, DEFAULT_ADMIN_ROLE),
      );
    });

    it("should update unpause type role with unpausing working", async () => {
      const updateCall = pauseManager
        .connect(securityCouncil)
        .updateUnpauseTypeRole(GENERAL_PAUSE_TYPE, DEFAULT_ADMIN_ROLE);
      await expectEvent(pauseManager, updateCall, "UnPauseTypeRoleUpdated", [
        GENERAL_PAUSE_TYPE,
        DEFAULT_ADMIN_ROLE,
        UNPAUSE_ALL_ROLE,
      ]);

      // pause with EP (non-SC) so unpause is not blocked by the SC-only guard
      await pauseManager.connect(pauseManagerAccount).pauseByType(GENERAL_PAUSE_TYPE);
      expect(await pauseManager.isPaused(GENERAL_PAUSE_TYPE)).to.be.true;

      await pauseManager.connect(defaultAdmin).unPauseByType(GENERAL_PAUSE_TYPE);
      expect(await pauseManager.isPaused(GENERAL_PAUSE_TYPE)).to.be.false;
    });

    it("should fail to unpause with old role", async () => {
      const updateCall = pauseManager
        .connect(securityCouncil)
        .updateUnpauseTypeRole(GENERAL_PAUSE_TYPE, DEFAULT_ADMIN_ROLE);
      await expectEvent(pauseManager, updateCall, "UnPauseTypeRoleUpdated", [
        GENERAL_PAUSE_TYPE,
        DEFAULT_ADMIN_ROLE,
        UNPAUSE_ALL_ROLE,
      ]);

      // pause with non-modified pausing account
      await pauseManager.connect(securityCouncil).pauseByType(GENERAL_PAUSE_TYPE);
      expect(await pauseManager.isPaused(GENERAL_PAUSE_TYPE)).to.be.true;

      await expectRevertWithReason(
        pauseManager.connect(securityCouncil).unPauseByType(GENERAL_PAUSE_TYPE),
        buildAccessErrorMessage(securityCouncil, DEFAULT_ADMIN_ROLE),
      );
    });
  });

  describe("Pausing and unpausing with GENERAL_PAUSE_TYPE", () => {
    // can pause as PAUSE_ALL_ROLE
    it("should pause the contract if PAUSE_ALL_ROLE", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE);
      expect(await pauseManager.isPaused(GENERAL_PAUSE_TYPE)).to.be.true;
    });

    // cannot pause as non-PAUSE_ALL_ROLE
    it("should revert pause attempt if not PAUSE_ALL_ROLE", async () => {
      await expect(pauseByType(GENERAL_PAUSE_TYPE, nonManager)).to.be.revertedWith(
        buildAccessErrorMessage(nonManager, PAUSE_ALL_ROLE),
      );
    });

    // can unpause as UNPAUSE_ALL_ROLE
    it("should unpause the contract if UNPAUSE_ALL_ROLE", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE);
      await unPauseByType(GENERAL_PAUSE_TYPE);

      expect(await pauseManager.isPaused(GENERAL_PAUSE_TYPE)).to.be.false;
    });

    // cannot unpause as non-UNPAUSE_ALL_ROLE
    it("should revert unpause attempt if not UNPAUSE_ALL_ROLE", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE);

      await expect(unPauseByType(GENERAL_PAUSE_TYPE, nonManager)).to.be.revertedWith(
        buildAccessErrorMessage(nonManager, UNPAUSE_ALL_ROLE),
      );
    });
  });

  describe("Specific type pausing", () => {
    describe("Unused pause type", () => {
      it("should revert when pausing with the unused pause type", async () => {
        await expectRevertWithCustomError(
          pauseManager,
          pauseManager.pauseByType(UNUSED_PAUSE_TYPE),
          "PauseTypeNotUsed",
        );
      });

      it("should revert when unpausing with the unused pause type", async () => {
        await expectRevertWithCustomError(
          pauseManager,
          pauseManager.unPauseByType(UNUSED_PAUSE_TYPE),
          "PauseTypeNotUsed",
        );
      });

      it("should revert when unPauseByExpiredType with the unused pause type", async () => {
        await expectRevertWithCustomError(
          pauseManager,
          pauseManager.unPauseByExpiredType(UNUSED_PAUSE_TYPE),
          "PauseTypeNotUsed",
        );
      });
    });

    describe("With permissions granted by granular pause role", () => {
      it("should pause the L1_L2_PAUSE_TYPE", async () => {
        await pauseByType(L1_L2_PAUSE_TYPE);
        expect(await pauseManager.isPaused(L1_L2_PAUSE_TYPE)).to.be.true;
      });

      it("should unpause the L1_L2_PAUSE_TYPE", async () => {
        await pauseByType(L1_L2_PAUSE_TYPE);

        await unPauseByType(L1_L2_PAUSE_TYPE);
        expect(await pauseManager.isPaused(L1_L2_PAUSE_TYPE)).to.be.false;
      });

      it("should pause the L2_L1_PAUSE_TYPE", async () => {
        await pauseByType(L2_L1_PAUSE_TYPE);
        expect(await pauseManager.isPaused(L2_L1_PAUSE_TYPE)).to.be.true;
      });

      it("should unpause the L2_L1_PAUSE_TYPE", async () => {
        await pauseByType(L2_L1_PAUSE_TYPE);

        await unPauseByType(L2_L1_PAUSE_TYPE);
        expect(await pauseManager.isPaused(L2_L1_PAUSE_TYPE)).to.be.false;
      });

      it("should pause the STATE_DATA_SUBMISSION_PAUSE_TYPE", async () => {
        await pauseByType(STATE_DATA_SUBMISSION_PAUSE_TYPE);
        expect(await pauseManager.isPaused(STATE_DATA_SUBMISSION_PAUSE_TYPE)).to.be.true;
      });

      it("should unpause the STATE_DATA_SUBMISSION_PAUSE_TYPE", async () => {
        await pauseByType(STATE_DATA_SUBMISSION_PAUSE_TYPE);

        await unPauseByType(STATE_DATA_SUBMISSION_PAUSE_TYPE);
        expect(await pauseManager.isPaused(STATE_DATA_SUBMISSION_PAUSE_TYPE)).to.be.false;
      });

      it("should pause the FINALIZATION_PAUSE_TYPE", async () => {
        await pauseByType(FINALIZATION_PAUSE_TYPE);
        expect(await pauseManager.isPaused(FINALIZATION_PAUSE_TYPE)).to.be.true;
      });

      it("should unpause the FINALIZATION_PAUSE_TYPE", async () => {
        await pauseByType(FINALIZATION_PAUSE_TYPE);

        await unPauseByType(FINALIZATION_PAUSE_TYPE);
        expect(await pauseManager.isPaused(FINALIZATION_PAUSE_TYPE)).to.be.false;
      });
    });

    describe("Without permissions granted by granular pause role", () => {
      it("cannot pause the L1_L2_PAUSE_TYPE as non-manager", async () => {
        await expect(pauseByType(L1_L2_PAUSE_TYPE, nonManager)).to.be.revertedWith(
          buildAccessErrorMessage(nonManager, PAUSE_L1_L2_ROLE),
        );
      });

      it("cannot unpause the L2_L1_PAUSE_TYPE", async () => {
        await pauseByType(L1_L2_PAUSE_TYPE);

        await expect(unPauseByType(L1_L2_PAUSE_TYPE, nonManager)).to.be.revertedWith(
          buildAccessErrorMessage(nonManager, UNPAUSE_L1_L2_ROLE),
        );
      });

      it("cannot pause the L2_L1_PAUSE_TYPE as non-manager", async () => {
        await expect(pauseByType(L2_L1_PAUSE_TYPE, nonManager)).to.be.revertedWith(
          buildAccessErrorMessage(nonManager, PAUSE_L2_L1_ROLE),
        );
      });

      it("cannot unpause the L2_L1_PAUSE_TYPE", async () => {
        await pauseByType(L2_L1_PAUSE_TYPE);

        await expect(unPauseByType(L2_L1_PAUSE_TYPE, nonManager)).to.be.revertedWith(
          buildAccessErrorMessage(nonManager, UNPAUSE_L2_L1_ROLE),
        );
      });

      it("cannot pause the STATE_DATA_SUBMISSION_PAUSE_TYPE as non-manager", async () => {
        await expect(pauseByType(STATE_DATA_SUBMISSION_PAUSE_TYPE, nonManager)).to.be.revertedWith(
          buildAccessErrorMessage(nonManager, PAUSE_STATE_DATA_SUBMISSION_ROLE),
        );
      });

      it("cannot unpause the STATE_DATA_SUBMISSION_PAUSE_TYPE", async () => {
        await pauseByType(STATE_DATA_SUBMISSION_PAUSE_TYPE);

        await expect(unPauseByType(STATE_DATA_SUBMISSION_PAUSE_TYPE, nonManager)).to.be.revertedWith(
          buildAccessErrorMessage(nonManager, UNPAUSE_STATE_DATA_SUBMISSION_ROLE),
        );
      });

      it("cannot pause the FINALIZATION_PAUSE_TYPE as non-manager", async () => {
        await expect(pauseByType(FINALIZATION_PAUSE_TYPE, nonManager)).to.be.revertedWith(
          buildAccessErrorMessage(nonManager, PAUSE_FINALIZATION_ROLE),
        );
      });

      it("cannot unpause the FINALIZATION_PAUSE_TYPE", async () => {
        await pauseByType(FINALIZATION_PAUSE_TYPE);

        await expect(unPauseByType(FINALIZATION_PAUSE_TYPE, nonManager)).to.be.revertedWith(
          buildAccessErrorMessage(nonManager, UNPAUSE_FINALIZATION_ROLE),
        );
      });
    });

    describe("Incorrect pausing and unpausing", () => {
      it("Should pause and fail to pause when paused", async () => {
        await pauseByType(L1_L2_PAUSE_TYPE);
        await expect(pauseByType(L1_L2_PAUSE_TYPE)).to.be.revertedWithCustomError(pauseManager, "IsPaused");
      });

      it("EP can pause additional types within the pause window", async () => {
        await pauseByType(L1_L2_PAUSE_TYPE);
        const cooldownEnd = await pauseManager.nonSecurityCouncilCooldownEnd();
        const cooldownDuration = await pauseManager.COOLDOWN_DURATION();
        const expectedExpiry = cooldownEnd - cooldownDuration;

        await pauseByType(L2_L1_PAUSE_TYPE);

        expect(await pauseManager.isPaused(L1_L2_PAUSE_TYPE)).to.be.true;
        expect(await pauseManager.isPaused(L2_L1_PAUSE_TYPE)).to.be.true;
        expect(await pauseManager.pauseTypeExpiryTimestamps(L2_L1_PAUSE_TYPE)).to.equal(expectedExpiry);
        expect(await pauseManager.nonSecurityCouncilCooldownEnd()).to.equal(cooldownEnd);
      });

      it("EP cannot pause after the pause window closes (in cooldown period)", async () => {
        await pauseByType(L1_L2_PAUSE_TYPE);
        await setFutureTimestampForNextBlock(await pauseManager.PAUSE_DURATION());
        const expectedCooldown = await pauseManager.nonSecurityCouncilCooldownEnd();
        await expectRevertWithCustomError(
          pauseManager,
          pauseManager.connect(pauseManagerAccount).pauseByType(L2_L1_PAUSE_TYPE),
          "PauseUnavailableDueToCooldown",
          [expectedCooldown],
        );
      });

      it("Should fail to unpause if not paused", async () => {
        await expect(unPauseByType(L1_L2_PAUSE_TYPE)).to.be.revertedWithCustomError(pauseManager, "IsNotPaused");
      });

      it("Should fail to unPauseByExpiredType if not paused", async () => {
        await expect(unPauseByExpiredType(L1_L2_PAUSE_TYPE, nonManager)).to.be.revertedWithCustomError(
          pauseManager,
          "IsNotPaused",
        );
      });
    });
  });

  describe("Pause and unpause event emitting", () => {
    it("should pause the L1_L2_PAUSE_TYPE", async () => {
      await expectEvent(pauseManager, pauseByType(L1_L2_PAUSE_TYPE), "Paused", [
        pauseManagerAccount.address,
        L1_L2_PAUSE_TYPE,
      ]);
    });

    it("should unpause the L1_L2_PAUSE_TYPE", async () => {
      await pauseByType(L1_L2_PAUSE_TYPE);

      await expectEvent(pauseManager, unPauseByType(L1_L2_PAUSE_TYPE), "UnPaused", [
        pauseManagerAccount.address,
        L1_L2_PAUSE_TYPE,
      ]);
    });

    it("should unPauseByExpiredType the L1_L2_PAUSE_TYPE", async () => {
      await pauseByType(L1_L2_PAUSE_TYPE);
      await setFutureTimestampForNextBlock(await pauseManager.PAUSE_DURATION());
      await expectEvent(pauseManager, unPauseByExpiredType(L1_L2_PAUSE_TYPE, nonManager), "UnPausedDueToExpiry", [
        L1_L2_PAUSE_TYPE,
      ]);
    });
  });

  describe("Pausing/unpausing with expiry and cooldown:", () => {
    it("EP pause should set per-type expiry and nonSecurityCouncilCooldownEnd", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE);
      const lastBlockTimestamp = await getLastBlockTimestamp();
      const pauseDuration = await pauseManager.PAUSE_DURATION();
      const cooldownDuration = await pauseManager.COOLDOWN_DURATION();

      expect(await pauseManager.pauseTypeExpiryTimestamps(GENERAL_PAUSE_TYPE)).to.equal(
        lastBlockTimestamp + pauseDuration,
      );
      expect(await pauseManager.nonSecurityCouncilCooldownEnd()).to.equal(
        lastBlockTimestamp + pauseDuration + cooldownDuration,
      );
    });

    it("unPauseByExpiredType should fail if per-type expiry has not passed", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE);
      await expectRevertWithCustomError(
        pauseManager,
        pauseManager.connect(pauseManagerAccount).unPauseByExpiredType(GENERAL_PAUSE_TYPE),
        "PauseNotExpired",
        [await pauseManager.pauseTypeExpiryTimestamps(GENERAL_PAUSE_TYPE)],
      );
    });

    it("unPauseByExpiredType should succeed after per-type expiry has passed and reset expiry to zero", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE);
      expect(await pauseManager.pauseTypeExpiryTimestamps(GENERAL_PAUSE_TYPE)).to.not.equal(0);

      await setFutureTimestampForNextBlock(await pauseManager.PAUSE_DURATION());
      await unPauseByExpiredType(GENERAL_PAUSE_TYPE, nonManager);

      expect(await pauseManager.isPaused(GENERAL_PAUSE_TYPE)).to.be.false;
      expect(await pauseManager.pauseTypeExpiryTimestamps(GENERAL_PAUSE_TYPE)).to.equal(0);
    });

    it("unPauseByExpiredType should not change nonSecurityCouncilCooldownEnd", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE);
      const cooldownBefore = await pauseManager.nonSecurityCouncilCooldownEnd();
      await setFutureTimestampForNextBlock(await pauseManager.PAUSE_DURATION());
      await unPauseByExpiredType(GENERAL_PAUSE_TYPE, nonManager);
      expect(await pauseManager.nonSecurityCouncilCooldownEnd()).to.equal(cooldownBefore);
    });

    it("Should not be able to pause while cooldown is active", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE);
      await setFutureTimestampForNextBlock(await pauseManager.PAUSE_DURATION());
      await unPauseByExpiredType(GENERAL_PAUSE_TYPE, nonManager);
      const expectedCooldown = await pauseManager.nonSecurityCouncilCooldownEnd();
      await expectRevertWithCustomError(
        pauseManager,
        pauseManager.connect(pauseManagerAccount).pauseByType(GENERAL_PAUSE_TYPE),
        "PauseUnavailableDueToCooldown",
        [expectedCooldown],
      );
    });

    it("Should be able to pause after cooldown has passed", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE);
      await setFutureTimestampForNextBlock(await pauseManager.PAUSE_DURATION());
      await unPauseByExpiredType(GENERAL_PAUSE_TYPE, nonManager);
      await setFutureTimestampForNextBlock(await pauseManager.COOLDOWN_DURATION());
      await pauseByType(GENERAL_PAUSE_TYPE);
      expect(await pauseManager.isPaused(GENERAL_PAUSE_TYPE)).to.be.true;
    });
  });

  describe("Pausing/unpausing with SECURITY_COUNCIL_ROLE", () => {
    it("SC can pause when another EP pause type is already active", async () => {
      await pauseByType(L1_L2_PAUSE_TYPE);
      await pauseByType(GENERAL_PAUSE_TYPE, securityCouncil);
      expect(await pauseManager.isPaused(GENERAL_PAUSE_TYPE)).to.be.true;
    });

    it("EP can pause another type while SC has paused something indefinitely", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE, securityCouncil);
      await pauseByType(L1_L2_PAUSE_TYPE);
      expect(await pauseManager.isPaused(GENERAL_PAUSE_TYPE)).to.be.true;
      expect(await pauseManager.isPaused(L1_L2_PAUSE_TYPE)).to.be.true;
    });

    it("SC pause should set per-type expiry to max", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE, securityCouncil);
      expect(await pauseManager.pauseTypeExpiryTimestamps(GENERAL_PAUSE_TYPE)).to.equal(ethers.MaxUint256);
    });

    it("SC pause should not affect nonSecurityCouncilCooldownEnd", async () => {
      const cooldownBefore = await pauseManager.nonSecurityCouncilCooldownEnd();
      await pauseByType(GENERAL_PAUSE_TYPE, securityCouncil);
      expect(await pauseManager.nonSecurityCouncilCooldownEnd()).to.equal(cooldownBefore);
    });

    it("Should be unable to unPauseByExpiredType after SC pause", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE, securityCouncil);
      await setFutureTimestampForNextBlock((await pauseManager.PAUSE_DURATION()) + BigInt(1));
      await expectRevertWithCustomError(
        pauseManager,
        pauseManager.connect(pauseManagerAccount).unPauseByExpiredType(GENERAL_PAUSE_TYPE),
        "PauseNotExpired",
        [await pauseManager.pauseTypeExpiryTimestamps(GENERAL_PAUSE_TYPE)],
      );
    });

    it("unPauseByType should reset per-type expiry and clear paused state", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE, securityCouncil);
      expect(await pauseManager.isPaused(GENERAL_PAUSE_TYPE)).to.be.true;
      expect(await pauseManager.pauseTypeExpiryTimestamps(GENERAL_PAUSE_TYPE)).to.equal(ethers.MaxUint256);

      await unPauseByType(GENERAL_PAUSE_TYPE, securityCouncil);

      expect(await pauseManager.isPaused(GENERAL_PAUSE_TYPE)).to.be.false;
      expect(await pauseManager.pauseTypeExpiryTimestamps(GENERAL_PAUSE_TYPE)).to.equal(0);
    });

    it("unPauseByType should not change nonSecurityCouncilCooldownEnd", async () => {
      await pauseByType(L1_L2_PAUSE_TYPE);
      const cooldownAfterEpPause = await pauseManager.nonSecurityCouncilCooldownEnd();
      await pauseByType(GENERAL_PAUSE_TYPE, securityCouncil);
      await unPauseByType(GENERAL_PAUSE_TYPE, securityCouncil);
      expect(await pauseManager.nonSecurityCouncilCooldownEnd()).to.equal(cooldownAfterEpPause);
    });

    it("EP can pause when their own cooldown has not been triggered", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE, securityCouncil);
      await unPauseByType(GENERAL_PAUSE_TYPE, securityCouncil);
      await pauseByType(L1_L2_PAUSE_TYPE);
      expect(await pauseManager.isPaused(L1_L2_PAUSE_TYPE)).to.be.true;
    });

    it("during EP cooldown, SC should be able to pause", async () => {
      await pauseByType(L1_L2_PAUSE_TYPE);
      await setFutureTimestampForNextBlock(await pauseManager.PAUSE_DURATION());
      await unPauseByExpiredType(L1_L2_PAUSE_TYPE, nonManager);
      await pauseByType(GENERAL_PAUSE_TYPE, securityCouncil);
      expect(await pauseManager.isPaused(GENERAL_PAUSE_TYPE)).to.be.true;
    });

    it("EP pause expiry is independent of SC pause", async () => {
      await pauseByType(L1_L2_PAUSE_TYPE);
      await pauseByType(GENERAL_PAUSE_TYPE, securityCouncil);
      await setFutureTimestampForNextBlock(await pauseManager.PAUSE_DURATION());
      await unPauseByExpiredType(L1_L2_PAUSE_TYPE, nonManager);
      expect(await pauseManager.isPaused(L1_L2_PAUSE_TYPE)).to.be.false;
      expect(await pauseManager.isPaused(GENERAL_PAUSE_TYPE)).to.be.true;
      await expectRevertWithCustomError(
        pauseManager,
        pauseManager.unPauseByExpiredType(GENERAL_PAUSE_TYPE),
        "PauseNotExpired",
        [ethers.MaxUint256],
      );
    });

    it("EP pause → SC pause → SC unpause → EP pause expires via unPauseByExpiredType", async () => {
      await pauseByType(L1_L2_PAUSE_TYPE);
      await pauseByType(GENERAL_PAUSE_TYPE, securityCouncil);
      await unPauseByType(GENERAL_PAUSE_TYPE, securityCouncil);
      await setFutureTimestampForNextBlock(await pauseManager.PAUSE_DURATION());
      await unPauseByExpiredType(L1_L2_PAUSE_TYPE, nonManager);
      expect(await pauseManager.isPaused(GENERAL_PAUSE_TYPE)).to.be.false;
      expect(await pauseManager.isPaused(L1_L2_PAUSE_TYPE)).to.be.false;
    });

    it("EP front-running SC does not create indefinite pause on EP type", async () => {
      await pauseByType(L1_L2_PAUSE_TYPE);
      await pauseByType(GENERAL_PAUSE_TYPE, securityCouncil);
      await setFutureTimestampForNextBlock(await pauseManager.PAUSE_DURATION());
      await unPauseByExpiredType(L1_L2_PAUSE_TYPE, nonManager);
      expect(await pauseManager.isPaused(L1_L2_PAUSE_TYPE)).to.be.false;
    });

    it("SC can unpause/repause to upgrade an EP pause to indefinite", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE);
      expect(await pauseManager.pauseTypeExpiryTimestamps(GENERAL_PAUSE_TYPE)).to.not.equal(ethers.MaxUint256);
      await unPauseByType(GENERAL_PAUSE_TYPE, securityCouncil);
      await pauseByType(GENERAL_PAUSE_TYPE, securityCouncil);
      expect(await pauseManager.pauseTypeExpiryTimestamps(GENERAL_PAUSE_TYPE)).to.equal(ethers.MaxUint256);
    });

    it("non-SC cannot unpause an indefinite (SC) pause", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE, securityCouncil);
      await expectRevertWithCustomError(
        pauseManager,
        pauseManager.connect(pauseManagerAccount).unPauseByType(GENERAL_PAUSE_TYPE),
        "OnlySecurityCouncilCanUnpauseIndefinitePause",
        [GENERAL_PAUSE_TYPE],
      );
      expect(await pauseManager.isPaused(GENERAL_PAUSE_TYPE)).to.be.true;
      expect(await pauseManager.pauseTypeExpiryTimestamps(GENERAL_PAUSE_TYPE)).to.equal(ethers.MaxUint256);
    });

    it("non-SC can unpause a finite (EP) pause", async () => {
      await pauseByType(L1_L2_PAUSE_TYPE);
      const cooldownBefore = await pauseManager.nonSecurityCouncilCooldownEnd();
      expect(await pauseManager.pauseTypeExpiryTimestamps(L1_L2_PAUSE_TYPE)).to.not.equal(ethers.MaxUint256);
      expect(await pauseManager.pauseTypeExpiryTimestamps(L1_L2_PAUSE_TYPE)).to.not.equal(0);

      await unPauseByType(L1_L2_PAUSE_TYPE);

      expect(await pauseManager.isPaused(L1_L2_PAUSE_TYPE)).to.be.false;
      expect(await pauseManager.pauseTypeExpiryTimestamps(L1_L2_PAUSE_TYPE)).to.equal(0);
      expect(await pauseManager.nonSecurityCouncilCooldownEnd()).to.equal(cooldownBefore);
    });

    it("SC can unpause their own indefinite pause", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE, securityCouncil);
      expect(await pauseManager.pauseTypeExpiryTimestamps(GENERAL_PAUSE_TYPE)).to.equal(ethers.MaxUint256);

      await unPauseByType(GENERAL_PAUSE_TYPE, securityCouncil);

      expect(await pauseManager.isPaused(GENERAL_PAUSE_TYPE)).to.be.false;
      expect(await pauseManager.pauseTypeExpiryTimestamps(GENERAL_PAUSE_TYPE)).to.equal(0);
    });
  });

  describe("pauseByType re-pause guard", () => {
    it("non-SC cannot re-pause a type with a finite expiry", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE);
      expect(await pauseManager.pauseTypeExpiryTimestamps(GENERAL_PAUSE_TYPE)).to.not.equal(ethers.MaxUint256);

      await expectRevertWithCustomError(
        pauseManager,
        pauseManager.connect(pauseManagerAccount).pauseByType(GENERAL_PAUSE_TYPE),
        "IsPaused",
        [GENERAL_PAUSE_TYPE],
      );
    });

    it("SC cannot re-pause a type that is already indefinitely paused", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE, securityCouncil);
      expect(await pauseManager.pauseTypeExpiryTimestamps(GENERAL_PAUSE_TYPE)).to.equal(ethers.MaxUint256);

      await expectRevertWithCustomError(
        pauseManager,
        pauseManager.connect(securityCouncil).pauseByType(GENERAL_PAUSE_TYPE),
        "IsPaused",
        [GENERAL_PAUSE_TYPE],
      );
    });

    it("non-SC cannot re-pause a type that was indefinitely paused by SC", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE, securityCouncil);
      expect(await pauseManager.pauseTypeExpiryTimestamps(GENERAL_PAUSE_TYPE)).to.equal(ethers.MaxUint256);

      await expectRevertWithCustomError(
        pauseManager,
        pauseManager.connect(pauseManagerAccount).pauseByType(GENERAL_PAUSE_TYPE),
        "IsPaused",
        [GENERAL_PAUSE_TYPE],
      );
    });
  });

  describe("Security council extending pauses", () => {
    it("SC can directly extend an EP-paused type to indefinite without unpausing first", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE);
      const finiteExpiry = await pauseManager.pauseTypeExpiryTimestamps(GENERAL_PAUSE_TYPE);
      expect(finiteExpiry).to.not.equal(ethers.MaxUint256);
      expect(finiteExpiry).to.not.equal(0n);

      await pauseByType(GENERAL_PAUSE_TYPE, securityCouncil);

      expect(await pauseManager.isPaused(GENERAL_PAUSE_TYPE)).to.be.true;
      expect(await pauseManager.pauseTypeExpiryTimestamps(GENERAL_PAUSE_TYPE)).to.equal(ethers.MaxUint256);
    });

    it("SC extending an EP pause emits PausedIndefinitely", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE);

      await expectEvent(pauseManager, pauseByType(GENERAL_PAUSE_TYPE, securityCouncil), "PausedIndefinitely", [
        securityCouncil.address,
        GENERAL_PAUSE_TYPE,
      ]);
    });

    it("SC fresh pause emits PausedIndefinitely", async () => {
      await expectEvent(pauseManager, pauseByType(GENERAL_PAUSE_TYPE, securityCouncil), "PausedIndefinitely", [
        securityCouncil.address,
        GENERAL_PAUSE_TYPE,
      ]);
    });

    it("SC extending an EP pause does not affect nonSecurityCouncilCooldownEnd", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE);
      const cooldownBefore = await pauseManager.nonSecurityCouncilCooldownEnd();

      await pauseByType(GENERAL_PAUSE_TYPE, securityCouncil);

      expect(await pauseManager.nonSecurityCouncilCooldownEnd()).to.equal(cooldownBefore);
    });

    it("after SC extends an EP pause, non-SC cannot unpause it", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE);
      await pauseByType(GENERAL_PAUSE_TYPE, securityCouncil);

      await expectRevertWithCustomError(
        pauseManager,
        pauseManager.connect(pauseManagerAccount).unPauseByType(GENERAL_PAUSE_TYPE),
        "OnlySecurityCouncilCanUnpauseIndefinitePause",
        [GENERAL_PAUSE_TYPE],
      );
    });

    it("after SC extends an EP pause, unPauseByExpiredType is blocked", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE);
      await pauseByType(GENERAL_PAUSE_TYPE, securityCouncil);

      await setFutureTimestampForNextBlock(await pauseManager.PAUSE_DURATION());

      await expectRevertWithCustomError(
        pauseManager,
        pauseManager.connect(nonManager).unPauseByExpiredType(GENERAL_PAUSE_TYPE),
        "PauseNotExpired",
        [ethers.MaxUint256],
      );
    });

    it("after SC extends an EP pause, only SC can unpause it", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE);
      await pauseByType(GENERAL_PAUSE_TYPE, securityCouncil);
      expect(await pauseManager.pauseTypeExpiryTimestamps(GENERAL_PAUSE_TYPE)).to.equal(ethers.MaxUint256);

      await unPauseByType(GENERAL_PAUSE_TYPE, securityCouncil);

      expect(await pauseManager.isPaused(GENERAL_PAUSE_TYPE)).to.be.false;
      expect(await pauseManager.pauseTypeExpiryTimestamps(GENERAL_PAUSE_TYPE)).to.equal(0);
    });
  });

  describe("resetNonSecurityCouncilCooldownEnd", () => {
    it("EP pause sets nonSecurityCouncilCooldownEnd", async () => {
      expect(await pauseManager.nonSecurityCouncilCooldownEnd()).to.equal(0);

      await pauseByType(GENERAL_PAUSE_TYPE);

      const lastBlockTimestamp = await getLastBlockTimestamp();
      const pauseDuration = await pauseManager.PAUSE_DURATION();
      const cooldownDuration = await pauseManager.COOLDOWN_DURATION();
      expect(await pauseManager.nonSecurityCouncilCooldownEnd()).to.equal(
        lastBlockTimestamp + pauseDuration + cooldownDuration,
      );
    });

    it("non-SC cannot call resetNonSecurityCouncilCooldownEnd", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE);
      expect(await pauseManager.nonSecurityCouncilCooldownEnd()).to.not.equal(0);

      await expectRevertWithReason(
        pauseManager.connect(pauseManagerAccount).resetNonSecurityCouncilCooldownEnd(),
        buildAccessErrorMessage(pauseManagerAccount, SECURITY_COUNCIL_ROLE),
      );
    });

    it("anonymous account cannot call resetNonSecurityCouncilCooldownEnd", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE);

      await expectRevertWithReason(
        pauseManager.connect(nonManager).resetNonSecurityCouncilCooldownEnd(),
        buildAccessErrorMessage(nonManager, SECURITY_COUNCIL_ROLE),
      );
    });

    it("SC can reset cooldownEnd and it emits NonSecurityCouncilCooldownEndReset", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE);
      expect(await pauseManager.nonSecurityCouncilCooldownEnd()).to.not.equal(0);

      await expectEvent(
        pauseManager,
        pauseManager.connect(securityCouncil).resetNonSecurityCouncilCooldownEnd(),
        "NonSecurityCouncilCooldownEndReset",
        [],
      );

      const lastBlockTimestamp = await getLastBlockTimestamp();
      expect(await pauseManager.nonSecurityCouncilCooldownEnd()).to.equal(lastBlockTimestamp);
    });

    it("after SC resets cooldown, EP can open a fresh pause window immediately", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE);
      await setFutureTimestampForNextBlock(await pauseManager.PAUSE_DURATION());
      await unPauseByExpiredType(GENERAL_PAUSE_TYPE, nonManager);

      const cooldownEnd = await pauseManager.nonSecurityCouncilCooldownEnd();
      await expectRevertWithCustomError(
        pauseManager,
        pauseManager.connect(pauseManagerAccount).pauseByType(GENERAL_PAUSE_TYPE),
        "PauseUnavailableDueToCooldown",
        [cooldownEnd],
      );

      await pauseManager.connect(securityCouncil).resetNonSecurityCouncilCooldownEnd();

      await pauseByType(GENERAL_PAUSE_TYPE);
      expect(await pauseManager.isPaused(GENERAL_PAUSE_TYPE)).to.be.true;
    });

    it("SC reset during an active EP pause allows EP to pause another type in a fresh window", async () => {
      await pauseByType(L1_L2_PAUSE_TYPE);
      await setFutureTimestampForNextBlock(await pauseManager.PAUSE_DURATION());

      await pauseManager.connect(securityCouncil).resetNonSecurityCouncilCooldownEnd();

      await pauseByType(L2_L1_PAUSE_TYPE);
      expect(await pauseManager.isPaused(L2_L1_PAUSE_TYPE)).to.be.true;
    });
  });
});
