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
  let pauseManager: TestPauseManager;

  beforeEach(async () => {
    [defaultAdmin, pauseManagerAccount, nonManager] = await ethers.getSigners();
    pauseManager = await loadFixture(deployTestPauseManagerFixture);

    await Promise.all([
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
    ]);
  });

  async function pauseByType(pauseType: number, account: SignerWithAddress = pauseManagerAccount) {
    return pauseManager.connect(account).pauseByType(pauseType);
  }

  async function unPauseByType(pauseType: number, account: SignerWithAddress = pauseManagerAccount) {
    return pauseManager.connect(account).unPauseByType(pauseType);
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
      const updateCall = pauseManager.connect(nonManager).updatePauseTypeRole(GENERAL_PAUSE_TYPE, DEFAULT_ADMIN_ROLE);
      await expectRevertWithReason(updateCall, buildAccessErrorMessage(nonManager, PAUSE_ALL_ROLE));
    });

    it("should fail updateUnpauseTypeRole if correct role not used", async () => {
      const updateCall = pauseManager.connect(nonManager).updateUnpauseTypeRole(GENERAL_PAUSE_TYPE, DEFAULT_ADMIN_ROLE);
      await expectRevertWithReason(updateCall, buildAccessErrorMessage(nonManager, UNPAUSE_ALL_ROLE));
    });

    it("should fail updateUnpauseTypeRole if roles are not different", async () => {
      const updateCall = pauseManager
        .connect(pauseManagerAccount)
        .updatePauseTypeRole(GENERAL_PAUSE_TYPE, PAUSE_ALL_ROLE);
      await expectRevertWithCustomError(pauseManager, updateCall, "RolesNotDifferent");
    });

    it("should fail updateUnpauseTypeRole if roles are not different", async () => {
      const updateCall = pauseManager
        .connect(pauseManagerAccount)
        .updateUnpauseTypeRole(GENERAL_PAUSE_TYPE, UNPAUSE_ALL_ROLE);
      await expectRevertWithCustomError(pauseManager, updateCall, "RolesNotDifferent");
    });

    it("should update pause type role with pausing working", async () => {
      const updateCall = pauseManager
        .connect(pauseManagerAccount)
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
        .connect(pauseManagerAccount)
        .updatePauseTypeRole(GENERAL_PAUSE_TYPE, DEFAULT_ADMIN_ROLE);
      await expectEvent(pauseManager, updateCall, "PauseTypeRoleUpdated", [
        GENERAL_PAUSE_TYPE,
        DEFAULT_ADMIN_ROLE,
        PAUSE_ALL_ROLE,
      ]);

      await expectRevertWithReason(
        pauseManager.connect(pauseManagerAccount).pauseByType(GENERAL_PAUSE_TYPE),
        buildAccessErrorMessage(pauseManagerAccount, DEFAULT_ADMIN_ROLE),
      );
    });

    it("should update unpause type role with unpausing working", async () => {
      const updateCall = pauseManager
        .connect(pauseManagerAccount)
        .updateUnpauseTypeRole(GENERAL_PAUSE_TYPE, DEFAULT_ADMIN_ROLE);
      await expectEvent(pauseManager, updateCall, "UnPauseTypeRoleUpdated", [
        GENERAL_PAUSE_TYPE,
        DEFAULT_ADMIN_ROLE,
        UNPAUSE_ALL_ROLE,
      ]);

      // pause with non-modified pausing account
      await pauseManager.connect(pauseManagerAccount).pauseByType(GENERAL_PAUSE_TYPE);
      expect(await pauseManager.isPaused(GENERAL_PAUSE_TYPE)).to.be.true;

      await pauseManager.connect(defaultAdmin).unPauseByType(GENERAL_PAUSE_TYPE);
      expect(await pauseManager.isPaused(GENERAL_PAUSE_TYPE)).to.be.false;
    });

    it("should fail to unpause with old role", async () => {
      const updateCall = pauseManager
        .connect(pauseManagerAccount)
        .updateUnpauseTypeRole(GENERAL_PAUSE_TYPE, DEFAULT_ADMIN_ROLE);
      await expectEvent(pauseManager, updateCall, "UnPauseTypeRoleUpdated", [
        GENERAL_PAUSE_TYPE,
        DEFAULT_ADMIN_ROLE,
        UNPAUSE_ALL_ROLE,
      ]);

      // pause with non-modified pausing account
      await pauseManager.connect(pauseManagerAccount).pauseByType(GENERAL_PAUSE_TYPE);
      expect(await pauseManager.isPaused(GENERAL_PAUSE_TYPE)).to.be.true;

      await expectRevertWithReason(
        pauseManager.connect(pauseManagerAccount).unPauseByType(GENERAL_PAUSE_TYPE),
        buildAccessErrorMessage(pauseManagerAccount, DEFAULT_ADMIN_ROLE),
      );
    });
  });

  describe("General pausing", () => {
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
    });

    describe("With permissions as PAUSE_ALL_ROLE", () => {
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
    describe("Without permissions - non-PAUSE_ALL_ROLE", () => {
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
    describe("Incorrect states for pausing and unpausing", () => {
      it("Should pause and fail to pause when paused", async () => {
        await pauseByType(L1_L2_PAUSE_TYPE);

        await expect(pauseByType(L1_L2_PAUSE_TYPE)).to.be.revertedWithCustomError(pauseManager, "IsPaused");
      });

      it("Should allow other types to pause if one is paused", async () => {
        await pauseByType(L1_L2_PAUSE_TYPE);

        await expect(pauseByType(L1_L2_PAUSE_TYPE)).to.be.revertedWithCustomError(pauseManager, "IsPaused");

        await expectEvent(pauseManager, pauseByType(BLOB_SUBMISSION_PAUSE_TYPE), "Paused", [
          pauseManagerAccount.address,
          BLOB_SUBMISSION_PAUSE_TYPE,
        ]);
      });

      it("Should fail to unpause if not paused", async () => {
        await expect(unPauseByType(L1_L2_PAUSE_TYPE)).to.be.revertedWithCustomError(pauseManager, "IsNotPaused");
      });
    });
  });
});
