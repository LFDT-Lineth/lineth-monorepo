import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { TestYieldManager } from "contracts/typechain-types";
import { ethers } from "hardhat";

import {
  GENERAL_PAUSE_TYPE,
  NATIVE_YIELD_STAKING_PAUSE_TYPE,
  NATIVE_YIELD_UNSTAKING_PAUSE_TYPE,
  NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE,
  NATIVE_YIELD_REPORTING_PAUSE_TYPE,
  ProgressOssificationResult,
} from "../../common/constants";
import {
  expectAccessControlRevert,
  expectEvent,
  expectRevertWithCustomError,
  expectPaused,
  expectNotPaused,
  getAccountsFixture,
} from "../../common/helpers";
import { deployYieldManagerForUnitTest } from "../helpers/deploy";
import { addMockYieldProvider } from "../helpers/mocks";
import { setWithdrawalReserveBalance, setWithdrawalReserveToMinimum } from "../helpers/setup";

describe("YieldManager contract - control operations", () => {
  let yieldManager: TestYieldManager;

  let securityCouncil: SignerWithAddress;
  let nonAuthorizedAccount: SignerWithAddress;
  let nativeYieldOperator: SignerWithAddress;

  before(async () => {
    ({ securityCouncil, operator: nonAuthorizedAccount, nativeYieldOperator } = await loadFixture(getAccountsFixture));
  });

  describe("pausing", () => {
    it("Security council should be able to activate GENERAL_PAUSE_TYPE", async () => {
      await expectEvent(
        yieldManager,
        yieldManager.connect(securityCouncil).pauseByType(GENERAL_PAUSE_TYPE),
        "PausedIndefinitely",
        [securityCouncil.address, GENERAL_PAUSE_TYPE],
      );
      await expectPaused(yieldManager, GENERAL_PAUSE_TYPE);
    });

    it("Security council should be able to activate NATIVE_YIELD_STAKING_PAUSE_TYPE", async () => {
      await expectEvent(
        yieldManager,
        yieldManager.connect(securityCouncil).pauseByType(NATIVE_YIELD_STAKING_PAUSE_TYPE),
        "PausedIndefinitely",
        [securityCouncil.address, NATIVE_YIELD_STAKING_PAUSE_TYPE],
      );
      await expectPaused(yieldManager, NATIVE_YIELD_STAKING_PAUSE_TYPE);
    });

    it("Security council should be able to activate NATIVE_YIELD_UNSTAKING_PAUSE_TYPE", async () => {
      await expectEvent(
        yieldManager,
        yieldManager.connect(securityCouncil).pauseByType(NATIVE_YIELD_UNSTAKING_PAUSE_TYPE),
        "PausedIndefinitely",
        [securityCouncil.address, NATIVE_YIELD_UNSTAKING_PAUSE_TYPE],
      );
      await expectPaused(yieldManager, NATIVE_YIELD_UNSTAKING_PAUSE_TYPE);
    });

    it("Security council should be able to activate NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE", async () => {
      await expectEvent(
        yieldManager,
        yieldManager.connect(securityCouncil).pauseByType(NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE),
        "PausedIndefinitely",
        [securityCouncil.address, NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE],
      );
      await expectPaused(yieldManager, NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE);
    });

    it("Security council should be able to activate NATIVE_YIELD_REPORTING_PAUSE_TYPE", async () => {
      await expectEvent(
        yieldManager,
        yieldManager.connect(securityCouncil).pauseByType(NATIVE_YIELD_REPORTING_PAUSE_TYPE),
        "PausedIndefinitely",
        [securityCouncil.address, NATIVE_YIELD_REPORTING_PAUSE_TYPE],
      );
      await expectPaused(yieldManager, NATIVE_YIELD_REPORTING_PAUSE_TYPE);
    });

    // LST withdrawals are now governed by NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE.

    const pauseRoleExpectations = [
      { label: "GENERAL_PAUSE_TYPE", pauseType: GENERAL_PAUSE_TYPE, roleGetter: () => yieldManager.PAUSE_ALL_ROLE() },
      {
        label: "NATIVE_YIELD_STAKING_PAUSE_TYPE",
        pauseType: NATIVE_YIELD_STAKING_PAUSE_TYPE,
        roleGetter: () => yieldManager.PAUSE_NATIVE_YIELD_STAKING_ROLE(),
      },
      {
        label: "NATIVE_YIELD_UNSTAKING_PAUSE_TYPE",
        pauseType: NATIVE_YIELD_UNSTAKING_PAUSE_TYPE,
        roleGetter: () => yieldManager.PAUSE_NATIVE_YIELD_UNSTAKING_ROLE(),
      },
      {
        label: "NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE",
        pauseType: NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE,
        roleGetter: () => yieldManager.PAUSE_NATIVE_YIELD_PERMISSIONLESS_ACTIONS_ROLE(),
      },
      {
        label: "NATIVE_YIELD_REPORTING_PAUSE_TYPE",
        pauseType: NATIVE_YIELD_REPORTING_PAUSE_TYPE,
        roleGetter: () => yieldManager.PAUSE_NATIVE_YIELD_REPORTING_ROLE(),
      },
    ] as const;

    for (const { label, pauseType, roleGetter } of pauseRoleExpectations) {
      it(`Non authorized account should not be able to activate ${label}`, async () => {
        const requiredRole = await roleGetter();
        await expectAccessControlRevert(
          yieldManager.connect(nonAuthorizedAccount).pauseByType(pauseType),
          nonAuthorizedAccount,
          requiredRole,
        );
      });
    }
  });

  describe("unpausing", () => {
    it("Security council should be able to unpause GENERAL_PAUSE_TYPE", async () => {
      await yieldManager.connect(securityCouncil).pauseByType(GENERAL_PAUSE_TYPE);
      await expectEvent(
        yieldManager,
        yieldManager.connect(securityCouncil).unPauseByType(GENERAL_PAUSE_TYPE),
        "UnPaused",
        [securityCouncil.address, GENERAL_PAUSE_TYPE],
      );
      await expectNotPaused(yieldManager, GENERAL_PAUSE_TYPE);
    });

    async function pauseThenUnpause(pauseType: number) {
      await yieldManager.connect(securityCouncil).pauseByType(pauseType);
      await expectEvent(yieldManager, yieldManager.connect(securityCouncil).unPauseByType(pauseType), "UnPaused", [
        securityCouncil.address,
        pauseType,
      ]);
      await expectNotPaused(yieldManager, pauseType);
    }

    it("Security council should be able to unpause NATIVE_YIELD_STAKING_PAUSE_TYPE", async () => {
      await pauseThenUnpause(NATIVE_YIELD_STAKING_PAUSE_TYPE);
    });

    it("Security council should be able to unpause NATIVE_YIELD_UNSTAKING_PAUSE_TYPE", async () => {
      await pauseThenUnpause(NATIVE_YIELD_UNSTAKING_PAUSE_TYPE);
    });

    it("Security council should be able to unpause NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE", async () => {
      await pauseThenUnpause(NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE);
    });

    it("Security council should be able to unpause NATIVE_YIELD_REPORTING_PAUSE_TYPE", async () => {
      await pauseThenUnpause(NATIVE_YIELD_REPORTING_PAUSE_TYPE);
    });

    // LST withdrawal flow is covered by NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE.

    const unpauseRoleExpectations = [
      {
        label: "GENERAL_PAUSE_TYPE",
        pauseType: GENERAL_PAUSE_TYPE,
        roleGetter: () => yieldManager.UNPAUSE_ALL_ROLE(),
      },
      {
        label: "NATIVE_YIELD_STAKING_PAUSE_TYPE",
        pauseType: NATIVE_YIELD_STAKING_PAUSE_TYPE,
        roleGetter: () => yieldManager.UNPAUSE_NATIVE_YIELD_STAKING_ROLE(),
      },
      {
        label: "NATIVE_YIELD_UNSTAKING_PAUSE_TYPE",
        pauseType: NATIVE_YIELD_UNSTAKING_PAUSE_TYPE,
        roleGetter: () => yieldManager.UNPAUSE_NATIVE_YIELD_UNSTAKING_ROLE(),
      },
      {
        label: "NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE",
        pauseType: NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE,
        roleGetter: () => yieldManager.UNPAUSE_NATIVE_YIELD_PERMISSIONLESS_ACTIONS_ROLE(),
      },
      {
        label: "NATIVE_YIELD_REPORTING_PAUSE_TYPE",
        pauseType: NATIVE_YIELD_REPORTING_PAUSE_TYPE,
        roleGetter: () => yieldManager.UNPAUSE_NATIVE_YIELD_REPORTING_ROLE(),
      },
    ] as const;

    for (const { label, pauseType, roleGetter } of unpauseRoleExpectations) {
      it(`Non authorized account should not be able to unpause ${label}`, async () => {
        const requiredRole = await roleGetter();
        await expectAccessControlRevert(
          yieldManager.connect(nonAuthorizedAccount).unPauseByType(pauseType),
          nonAuthorizedAccount,
          requiredRole,
        );
      });
    }
  });

  beforeEach(async () => {
    ({ yieldManager } = await loadFixture(deployYieldManagerForUnitTest));
  });

  // pauseStaking() unit tests
  describe("Pausing staking", () => {
    it("Should revert when adding if the caller does not have the STAKING_PAUSE_CONTROLLER_ROLE", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      const requiredRole = await yieldManager.STAKING_PAUSE_CONTROLLER_ROLE();

      await expectAccessControlRevert(
        yieldManager.connect(nonAuthorizedAccount).pauseStaking(mockYieldProviderAddress),
        nonAuthorizedAccount,
        requiredRole,
      );
    });

    it("Should revert when pausing an unknown yield provider", async () => {
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).pauseStaking(ethers.Wallet.createRandom().address),
        "UnknownYieldProvider",
      );
    });

    it("Should revert if staking already paused", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.connect(nativeYieldOperator).pauseStaking(mockYieldProviderAddress);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).pauseStaking(mockYieldProviderAddress),
        "StakingAlreadyPaused",
      );
    });

    it("Should successfully pause and emit the correct event", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await expectEvent(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).pauseStaking(mockYieldProviderAddress),
        "YieldProviderStakingPaused",
        [mockYieldProviderAddress],
      );

      expect(await yieldManager.isStakingPaused(mockYieldProviderAddress)).to.be.true;
    });
  });

  // pauseStakingIfNotAlready() unit tests
  describe("Pausing staking if not already", () => {
    it("If not currently paused, should pause", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      expect(await yieldManager.isStakingPaused(mockYieldProviderAddress)).to.be.false;

      await yieldManager.connect(nativeYieldOperator).pauseStakingIfNotAlready(mockYieldProviderAddress);

      expect(await yieldManager.isStakingPaused(mockYieldProviderAddress)).to.be.true;
    });

    it("If already paused, no-op", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.setYieldProviderIsStakingPaused(mockYieldProviderAddress, true);

      await yieldManager.connect(nativeYieldOperator).pauseStakingIfNotAlready(mockYieldProviderAddress);

      expect(await yieldManager.isStakingPaused(mockYieldProviderAddress)).to.be.true;
    });
  });

  // unpauseStaking() unit tests
  describe("Unpausing staking", () => {
    it("Should revert when adding if the caller does not have the STAKING_PAUSE_CONTROLLER_ROLE", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      const requiredRole = await yieldManager.STAKING_PAUSE_CONTROLLER_ROLE();

      await expectAccessControlRevert(
        yieldManager.connect(nonAuthorizedAccount).unpauseStaking(mockYieldProviderAddress),
        nonAuthorizedAccount,
        requiredRole,
      );
    });

    it("Should revert when pausing an unknown yield provider", async () => {
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).unpauseStaking(ethers.Wallet.createRandom().address),
        "UnknownYieldProvider",
      );
    });

    it("Should revert if staking already unpaused", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      await setWithdrawalReserveToMinimum(yieldManager);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).unpauseStaking(mockYieldProviderAddress),
        "StakingAlreadyUnpaused",
      );
    });

    it("Should revert if the withdrawal reserve is in deficit", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.connect(nativeYieldOperator).pauseStaking(mockYieldProviderAddress);
      await setWithdrawalReserveBalance(yieldManager, 0n);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).unpauseStaking(mockYieldProviderAddress),
        "InsufficientWithdrawalReserve",
      );
    });

    it("Should revert if ossification has been initiated but not completed", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.connect(nativeYieldOperator).pauseStaking(mockYieldProviderAddress);
      await setWithdrawalReserveToMinimum(yieldManager);
      await yieldManager.setYieldProviderIsOssificationInitiated(mockYieldProviderAddress, true);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).unpauseStaking(mockYieldProviderAddress),
        "UnpauseStakingForbiddenDuringPendingOssification",
      );
    });

    it("Should succeed if ossification has been completed", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.connect(nativeYieldOperator).pauseStaking(mockYieldProviderAddress);
      await setWithdrawalReserveToMinimum(yieldManager);
      await yieldManager.setYieldProviderIsOssificationInitiated(mockYieldProviderAddress, true);
      await yieldManager.setYieldProviderIsOssified(mockYieldProviderAddress, true);

      const tx = yieldManager.connect(nativeYieldOperator).unpauseStaking(mockYieldProviderAddress);
      await expect(tx).to.not.be.reverted;
    });

    it("Should revert if there is a current lst liability", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.connect(nativeYieldOperator).pauseStaking(mockYieldProviderAddress);
      await setWithdrawalReserveToMinimum(yieldManager);
      await yieldManager.setYieldProviderLstLiabilityPrincipal(mockYieldProviderAddress, 1n);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).unpauseStaking(mockYieldProviderAddress),
        "UnpauseStakingForbiddenWithCurrentLSTLiability",
      );
    });

    it("Should successfully unpause and emit the correct event", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.connect(nativeYieldOperator).pauseStaking(mockYieldProviderAddress);
      await setWithdrawalReserveToMinimum(yieldManager);

      await expectEvent(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).unpauseStaking(mockYieldProviderAddress),
        "YieldProviderStakingUnpaused",
        [mockYieldProviderAddress],
      );

      expect(await yieldManager.isStakingPaused(mockYieldProviderAddress)).to.be.false;
    });
  });

  // initiateOssification() unit tests
  describe("Initiate ossification", () => {
    it("Should revert when adding if the caller does not have the OSSIFICATION_INITIATOR_ROLE", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      const requiredRole = await yieldManager.OSSIFICATION_INITIATOR_ROLE();

      await expectAccessControlRevert(
        yieldManager.connect(nativeYieldOperator).initiateOssification(mockYieldProviderAddress),
        nativeYieldOperator,
        requiredRole,
      );
    });

    it("Should revert when requesting for an unknown yield provider", async () => {
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(securityCouncil).initiateOssification(ethers.Wallet.createRandom().address),
        "UnknownYieldProvider",
      );
    });

    it("Should revert if ossification has been initiated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.connect(securityCouncil).initiateOssification(mockYieldProviderAddress);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(securityCouncil).initiateOssification(mockYieldProviderAddress),
        "OssificationAlreadyInitiated",
      );
    });

    it("Should revert if ossification has completed", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.setYieldProviderIsOssified(mockYieldProviderAddress, true);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(securityCouncil).initiateOssification(mockYieldProviderAddress),
        "AlreadyOssified",
      );
    });

    it("Should successfully initiate ossification, pause staking and emit the correct event", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      const initiateCall = yieldManager.connect(securityCouncil).initiateOssification(mockYieldProviderAddress);
      await expectEvent(yieldManager, initiateCall, "YieldProviderStakingPaused", [mockYieldProviderAddress]);
      await expectEvent(yieldManager, initiateCall, "YieldProviderOssificationInitiated", [mockYieldProviderAddress]);

      expect(await yieldManager.isOssificationInitiated(mockYieldProviderAddress)).to.be.true;
      expect(await yieldManager.isStakingPaused(mockYieldProviderAddress)).to.be.true;
    });
  });

  // progressPendingOssification() unit tests
  describe("Process pending ossification", () => {
    it("Should revert when adding if the caller does not have the OSSIFICATION_PROCESSOR_ROLE", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      const requiredRole = await yieldManager.OSSIFICATION_PROCESSOR_ROLE();

      await expectAccessControlRevert(
        yieldManager.connect(nonAuthorizedAccount).progressPendingOssification(mockYieldProviderAddress),
        nonAuthorizedAccount,
        requiredRole,
      );
    });

    it("Should revert when requesting for an unknown yield provider", async () => {
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(securityCouncil).progressPendingOssification(ethers.Wallet.createRandom().address),
        "UnknownYieldProvider",
      );
    });

    it("Should revert if ossification not initiated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(securityCouncil).progressPendingOssification(mockYieldProviderAddress),
        "OssificationNotInitiated",
      );
    });

    it("Should revert if ossification completed", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.setYieldProviderIsOssificationInitiated(mockYieldProviderAddress, true);
      await yieldManager.setYieldProviderIsOssified(mockYieldProviderAddress, true);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(securityCouncil).progressPendingOssification(mockYieldProviderAddress),
        "AlreadyOssified",
      );
    });

    it("If YieldProvider does no-op for progress pending, should not complete ossification and emit the correct event", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.connect(securityCouncil).initiateOssification(mockYieldProviderAddress);
      await yieldManager
        .connect(securityCouncil)
        .setProgressPendingOssificationReturnVal(mockYieldProviderAddress, ProgressOssificationResult.NOOP);

      const progressOssificationResult = await yieldManager
        .connect(securityCouncil)
        .progressPendingOssification.staticCall(mockYieldProviderAddress);
      expect(progressOssificationResult).to.eq(ProgressOssificationResult.NOOP);

      await expectEvent(
        yieldManager,
        yieldManager.connect(securityCouncil).progressPendingOssification(mockYieldProviderAddress),
        "YieldProviderOssificationProcessed",
        [mockYieldProviderAddress, ProgressOssificationResult.NOOP],
      );

      expect(await yieldManager.isOssificationInitiated(mockYieldProviderAddress)).to.be.true;
      expect(await yieldManager.isOssified(mockYieldProviderAddress)).to.be.false;
    });

    it("If YieldProvider does reinitiate ossification for progress, should not complete ossification and emit the correct event", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.connect(securityCouncil).initiateOssification(mockYieldProviderAddress);
      await yieldManager
        .connect(securityCouncil)
        .setProgressPendingOssificationReturnVal(mockYieldProviderAddress, ProgressOssificationResult.REINITIATED);

      const progressOssificationResult = await yieldManager
        .connect(securityCouncil)
        .progressPendingOssification.staticCall(mockYieldProviderAddress);
      expect(progressOssificationResult).to.eq(ProgressOssificationResult.REINITIATED);

      await expectEvent(
        yieldManager,
        yieldManager.connect(securityCouncil).progressPendingOssification(mockYieldProviderAddress),
        "YieldProviderOssificationProcessed",
        [mockYieldProviderAddress, ProgressOssificationResult.REINITIATED],
      );

      expect(await yieldManager.isOssificationInitiated(mockYieldProviderAddress)).to.be.true;
      expect(await yieldManager.isOssified(mockYieldProviderAddress)).to.be.false;
    });

    it("If YieldProvider returns completed ossification, should complete ossification and emit the correct event", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.connect(securityCouncil).initiateOssification(mockYieldProviderAddress);
      await yieldManager
        .connect(securityCouncil)
        .setProgressPendingOssificationReturnVal(mockYieldProviderAddress, ProgressOssificationResult.COMPLETE);

      const progressOssificationResult = await yieldManager
        .connect(securityCouncil)
        .progressPendingOssification.staticCall(mockYieldProviderAddress);
      expect(progressOssificationResult).to.eq(ProgressOssificationResult.COMPLETE);

      await expectEvent(
        yieldManager,
        yieldManager.connect(securityCouncil).progressPendingOssification(mockYieldProviderAddress),
        "YieldProviderOssificationProcessed",
        [mockYieldProviderAddress, ProgressOssificationResult.COMPLETE],
      );

      expect(await yieldManager.isOssified(mockYieldProviderAddress)).to.be.true;
    });
  });
});
