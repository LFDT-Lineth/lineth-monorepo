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
  PAUSE_BLOB_SUBMISSION_ROLE,
  UNPAUSE_FINALIZATION_ROLE,
  UNPAUSE_BLOB_SUBMISSION_ROLE,
  SECURITY_COUNCIL_ROLE,
  BLOB_SUBMISSION_PAUSE_TYPE,
  CALLDATA_SUBMISSION_PAUSE_TYPE,
  FINALIZATION_PAUSE_TYPE,
  pauseTypeRoles,
  unpauseTypeRoles,
  UNUSED_PAUSE_TYPE,
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
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
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
      pauseManager.grantRole(PAUSE_BLOB_SUBMISSION_ROLE, pauseManagerAccount.address),
      pauseManager.grantRole(UNPAUSE_BLOB_SUBMISSION_ROLE, pauseManagerAccount.address),
      pauseManager.grantRole(PAUSE_FINALIZATION_ROLE, pauseManagerAccount.address),
      pauseManager.grantRole(UNPAUSE_FINALIZATION_ROLE, pauseManagerAccount.address),
      // Roles for securityCouncil
      pauseManager.grantRole(PAUSE_ALL_ROLE, securityCouncil.address),
      pauseManager.grantRole(UNPAUSE_ALL_ROLE, securityCouncil.address),
      pauseManager.grantRole(UNPAUSE_L1_L2_ROLE, securityCouncil.address),
      pauseManager.grantRole(UNPAUSE_L2_L1_ROLE, securityCouncil.address),
      pauseManager.grantRole(UNPAUSE_BLOB_SUBMISSION_ROLE, securityCouncil.address),
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

      // pause with non-modified pausing account
      await pauseManager.connect(securityCouncil).pauseByType(GENERAL_PAUSE_TYPE);
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

      it("should pause the BLOB_SUBMISSION_PAUSE_TYPE", async () => {
        await pauseByType(BLOB_SUBMISSION_PAUSE_TYPE);
        expect(await pauseManager.isPaused(BLOB_SUBMISSION_PAUSE_TYPE)).to.be.true;
      });

      it("should unpause the BLOB_SUBMISSION_PAUSE_TYPE", async () => {
        await pauseByType(BLOB_SUBMISSION_PAUSE_TYPE);

        await unPauseByType(BLOB_SUBMISSION_PAUSE_TYPE);
        expect(await pauseManager.isPaused(BLOB_SUBMISSION_PAUSE_TYPE)).to.be.false;
      });

      it("should pause the CALLDATA_SUBMISSION_PAUSE_TYPE", async () => {
        await pauseByType(CALLDATA_SUBMISSION_PAUSE_TYPE);
        expect(await pauseManager.isPaused(CALLDATA_SUBMISSION_PAUSE_TYPE)).to.be.true;
      });

      it("should unpause the CALLDATA_SUBMISSION_PAUSE_TYPE", async () => {
        await pauseByType(CALLDATA_SUBMISSION_PAUSE_TYPE);

        await unPauseByType(CALLDATA_SUBMISSION_PAUSE_TYPE);
        expect(await pauseManager.isPaused(CALLDATA_SUBMISSION_PAUSE_TYPE)).to.be.false;
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

      it("cannot pause the BLOB_SUBMISSION_PAUSE_TYPE as non-manager", async () => {
        await expect(pauseByType(BLOB_SUBMISSION_PAUSE_TYPE, nonManager)).to.be.revertedWith(
          buildAccessErrorMessage(nonManager, PAUSE_BLOB_SUBMISSION_ROLE),
        );
      });

      it("cannot unpause the BLOB_SUBMISSION_PAUSE_TYPE", async () => {
        await pauseByType(BLOB_SUBMISSION_PAUSE_TYPE);

        await expect(unPauseByType(BLOB_SUBMISSION_PAUSE_TYPE, nonManager)).to.be.revertedWith(
          buildAccessErrorMessage(nonManager, UNPAUSE_BLOB_SUBMISSION_ROLE),
        );
      });

      it("cannot pause the CALLDATA_SUBMISSION_PAUSE_TYPE as non-manager", async () => {
        await expect(pauseByType(CALLDATA_SUBMISSION_PAUSE_TYPE, nonManager)).to.be.revertedWith(
          buildAccessErrorMessage(nonManager, PAUSE_BLOB_SUBMISSION_ROLE),
        );
      });

      it("cannot unpause the CALLDATA_SUBMISSION_PAUSE_TYPE", async () => {
        await pauseByType(CALLDATA_SUBMISSION_PAUSE_TYPE);

        await expect(unPauseByType(CALLDATA_SUBMISSION_PAUSE_TYPE, nonManager)).to.be.revertedWith(
          buildAccessErrorMessage(nonManager, UNPAUSE_BLOB_SUBMISSION_ROLE),
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

      it("Should not allow other types to pause if one is already paused", async () => {
        await pauseByType(L1_L2_PAUSE_TYPE);
        const expectedCooldown = (await pauseManager.pauseExpiryTimestamp()) + (await pauseManager.COOLDOWN_DURATION());
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
    it("Pause should set pauseExpiryTimestamp to a time in the future", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE);
      const lastBlockTimestamp = await getLastBlockTimestamp();
      expect(await pauseManager.pauseExpiryTimestamp()).to.equal(
        lastBlockTimestamp + (await pauseManager.PAUSE_DURATION()),
      );
    });

    it("unPauseByExpiredType should fail after pause, if pause has not expired", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE);
      await expectRevertWithCustomError(
        pauseManager,
        pauseManager.connect(pauseManagerAccount).unPauseByExpiredType(GENERAL_PAUSE_TYPE),
        "PauseNotExpired",
        [await pauseManager.pauseExpiryTimestamp()],
      );
    });

    it("unPauseByExpiredType should succeed after pause has expired", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE);
      await setFutureTimestampForNextBlock(await pauseManager.PAUSE_DURATION());
      await unPauseByExpiredType(GENERAL_PAUSE_TYPE, nonManager);
      expect(await pauseManager.isPaused(GENERAL_PAUSE_TYPE)).to.be.false;
    });

    it("unPauseByExpiredType should not change the pauseExpiryTimestamp", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE);
      const beforePauseExpiry = await pauseManager.pauseExpiryTimestamp();
      await setFutureTimestampForNextBlock(await pauseManager.PAUSE_DURATION());
      await unPauseByExpiredType(GENERAL_PAUSE_TYPE, nonManager);
      const afterPauseExpiry = await pauseManager.pauseExpiryTimestamp();
      expect(beforePauseExpiry).to.equal(afterPauseExpiry);
    });

    it("Should not be able to pause while cooldown is active", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE);
      await setFutureTimestampForNextBlock(await pauseManager.PAUSE_DURATION());
      await unPauseByExpiredType(GENERAL_PAUSE_TYPE, nonManager);
      const expectedCooldown = (await pauseManager.pauseExpiryTimestamp()) + (await pauseManager.COOLDOWN_DURATION());
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
    it("should pause the contract with SECURITY_COUNCIL_ROLE, when another pause type is already active", async () => {
      await pauseByType(L1_L2_PAUSE_TYPE);
      await pauseByType(GENERAL_PAUSE_TYPE, securityCouncil);
      expect(await pauseManager.isPaused(GENERAL_PAUSE_TYPE)).to.be.true;
    });

    // Should not revert due to overflow from `pauseExpiryTimestamp + COOLDOWN_DURATION`, but due to custom error.
    it("after pause with SECURITY_COUNCIL_ROLE, should not be able to pause with a non-SECURITY_COUNCIL_ROLE", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE, securityCouncil);
      await expectRevertWithCustomError(
        pauseManager,
        pauseManager.connect(pauseManagerAccount).pauseByType(L1_L2_PAUSE_TYPE),
        "PauseUnavailableDueToCooldown",
        [ethers.MaxUint256],
      );
    });

    it("should set pauseExpiryTimestamp to an unreachable timestamp if pause enacted by SECURITY_COUNCIL_ROLE", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE, securityCouncil);
      expect(await pauseManager.pauseExpiryTimestamp()).to.equal(
        ethers.MaxUint256 - (await pauseManager.COOLDOWN_DURATION()),
      );
    });

    it("Should be unable to unPauseByExpiredType after pause with SECURITY_COUNCIL_ROLE", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE, securityCouncil);
      const pauseTimestamp = await getLastBlockTimestamp();
      const expectedPauseExpiry = pauseTimestamp + (await pauseManager.PAUSE_DURATION());
      await setFutureTimestampForNextBlock((await pauseManager.PAUSE_DURATION()) + BigInt(1));
      await expectRevertWithCustomError(
        pauseManager,
        pauseManager.connect(pauseManagerAccount).unPauseByExpiredType(GENERAL_PAUSE_TYPE),
        "PauseNotExpired",
        [await pauseManager.pauseExpiryTimestamp()],
      );
      // Assert that we have passed expected pause expiry
      expect(await getLastBlockTimestamp()).to.be.above(expectedPauseExpiry);
    });

    it("should reset the pause cooldown when unpause contract with SECURITY_COUNCIL_ROLE", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE, securityCouncil);
      const unPauseBlockTimestamp = await setFutureTimestampForNextBlock();
      await unPauseByType(GENERAL_PAUSE_TYPE, securityCouncil);
      expect(await pauseManager.pauseExpiryTimestamp()).to.equal(
        unPauseBlockTimestamp - (await pauseManager.COOLDOWN_DURATION()),
      );
    });

    it("should not reset the pause cooldown when unpause contract with non-SECURITY_COUNCIL_ROLE", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE, securityCouncil);
      await unPauseByType(GENERAL_PAUSE_TYPE, pauseManagerAccount);
      expect(await pauseManager.pauseExpiryTimestamp()).to.equal(
        ethers.MaxUint256 - (await pauseManager.COOLDOWN_DURATION()),
      );
    });

    it("after unpause contract with SECURITY_COUNCIL_ROLE, any pause should be possible", async () => {
      await pauseByType(GENERAL_PAUSE_TYPE, securityCouncil);
      await unPauseByType(GENERAL_PAUSE_TYPE, securityCouncil);
      await pauseByType(L1_L2_PAUSE_TYPE);
    });

    it("during pause cooldown, SECURITY_COUNCIL_ROLE should be able to pause", async () => {
      await pauseByType(L1_L2_PAUSE_TYPE);
      await setFutureTimestampForNextBlock(await pauseManager.PAUSE_DURATION());
      await unPauseByExpiredType(L1_L2_PAUSE_TYPE, nonManager);
      await pauseByType(GENERAL_PAUSE_TYPE, securityCouncil);
    });

    it("Non-SECURITY_COUNCIL_ROLE pause -> SECURITY_COUNCIL_ROLE pause -> SECURITY_COUNCIL_ROLE unpause -> unPauseByExpiredType should work", async () => {
      await pauseByType(L1_L2_PAUSE_TYPE);
      await pauseByType(GENERAL_PAUSE_TYPE, securityCouncil);
      await unPauseByType(GENERAL_PAUSE_TYPE, securityCouncil);
      await unPauseByExpiredType(L1_L2_PAUSE_TYPE, nonManager);
      expect(await pauseManager.isPaused(GENERAL_PAUSE_TYPE)).to.be.false;
      expect(await pauseManager.isPaused(L1_L2_PAUSE_TYPE)).to.be.false;
    });
  });
});
