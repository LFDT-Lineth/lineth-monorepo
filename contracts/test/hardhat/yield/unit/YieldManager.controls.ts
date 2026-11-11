import { expect } from "chai";
import { network } from "hardhat";
import { describe, it, before, beforeEach } from "node:test";

import {
  GENERAL_PAUSE_TYPE,
  NATIVE_YIELD_STAKING_PAUSE_TYPE,
  NATIVE_YIELD_UNSTAKING_PAUSE_TYPE,
  NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE,
  NATIVE_YIELD_REPORTING_PAUSE_TYPE,
  ProgressOssificationResult,
} from "../../common/constants/index.js";
import {
  expectAccessControlRevert,
  expectPaused,
  expectNotPaused,
  getAccountsFixture,
  type TestAccount,
  expectEvent,
  expectRevertWithCustomError,
} from "../../common/helpers/index.js";
import { deployYieldManagerForUnitTest } from "../helpers/deploy.js";
import { addMockYieldProvider } from "../helpers/mocks.js";
import { randomAddress, setWithdrawalReserveBalance, setWithdrawalReserveToMinimum } from "../helpers/setup.js";

import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

describe("YieldManager contract - control operations", () => {
  let yieldManager: ContractReturnType<"TestYieldManager">;

  let securityCouncil: TestAccount;
  let nonAuthorizedAccount: TestAccount;
  let nativeYieldOperator: TestAccount;

  before(async () => {
    const { networkHelpers } = await network.getOrCreate();
    ({
      securityCouncil,
      operator: nonAuthorizedAccount,
      nativeYieldOperator,
    } = await networkHelpers.loadFixture(getAccountsFixture));
  });

  describe("pausing", () => {
    it("Security council should be able to activate GENERAL_PAUSE_TYPE", async () => {
      await expectEvent(
        yieldManager,
        yieldManager.write.pauseByType([GENERAL_PAUSE_TYPE], { account: securityCouncil.account }),
        "PausedIndefinitely",
        [securityCouncil.address, GENERAL_PAUSE_TYPE],
      );
      await expectPaused(yieldManager, GENERAL_PAUSE_TYPE);
    });

    it("Security council should be able to activate NATIVE_YIELD_STAKING_PAUSE_TYPE", async () => {
      await expectEvent(
        yieldManager,
        yieldManager.write.pauseByType([NATIVE_YIELD_STAKING_PAUSE_TYPE], { account: securityCouncil.account }),
        "PausedIndefinitely",
        [securityCouncil.address, NATIVE_YIELD_STAKING_PAUSE_TYPE],
      );
      await expectPaused(yieldManager, NATIVE_YIELD_STAKING_PAUSE_TYPE);
    });

    it("Security council should be able to activate NATIVE_YIELD_UNSTAKING_PAUSE_TYPE", async () => {
      await expectEvent(
        yieldManager,
        yieldManager.write.pauseByType([NATIVE_YIELD_UNSTAKING_PAUSE_TYPE], { account: securityCouncil.account }),
        "PausedIndefinitely",
        [securityCouncil.address, NATIVE_YIELD_UNSTAKING_PAUSE_TYPE],
      );
      await expectPaused(yieldManager, NATIVE_YIELD_UNSTAKING_PAUSE_TYPE);
    });

    it("Security council should be able to activate NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE", async () => {
      await expectEvent(
        yieldManager,
        yieldManager.write.pauseByType([NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE], {
          account: securityCouncil.account,
        }),
        "PausedIndefinitely",
        [securityCouncil.address, NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE],
      );
      await expectPaused(yieldManager, NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE);
    });

    it("Security council should be able to activate NATIVE_YIELD_REPORTING_PAUSE_TYPE", async () => {
      await expectEvent(
        yieldManager,
        yieldManager.write.pauseByType([NATIVE_YIELD_REPORTING_PAUSE_TYPE], { account: securityCouncil.account }),
        "PausedIndefinitely",
        [securityCouncil.address, NATIVE_YIELD_REPORTING_PAUSE_TYPE],
      );
      await expectPaused(yieldManager, NATIVE_YIELD_REPORTING_PAUSE_TYPE);
    });

    // LST withdrawals are now governed by NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE.

    const pauseRoleExpectations = [
      {
        label: "GENERAL_PAUSE_TYPE",
        pauseType: GENERAL_PAUSE_TYPE,
        roleGetter: () => yieldManager.read.PAUSE_ALL_ROLE(),
      },
      {
        label: "NATIVE_YIELD_STAKING_PAUSE_TYPE",
        pauseType: NATIVE_YIELD_STAKING_PAUSE_TYPE,
        roleGetter: () => yieldManager.read.PAUSE_NATIVE_YIELD_STAKING_ROLE(),
      },
      {
        label: "NATIVE_YIELD_UNSTAKING_PAUSE_TYPE",
        pauseType: NATIVE_YIELD_UNSTAKING_PAUSE_TYPE,
        roleGetter: () => yieldManager.read.PAUSE_NATIVE_YIELD_UNSTAKING_ROLE(),
      },
      {
        label: "NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE",
        pauseType: NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE,
        roleGetter: () => yieldManager.read.PAUSE_NATIVE_YIELD_PERMISSIONLESS_ACTIONS_ROLE(),
      },
      {
        label: "NATIVE_YIELD_REPORTING_PAUSE_TYPE",
        pauseType: NATIVE_YIELD_REPORTING_PAUSE_TYPE,
        roleGetter: () => yieldManager.read.PAUSE_NATIVE_YIELD_REPORTING_ROLE(),
      },
    ] as const;

    for (const { label, pauseType, roleGetter } of pauseRoleExpectations) {
      it(`Non authorized account should not be able to activate ${label}`, async () => {
        const requiredRole = await roleGetter();
        await expectAccessControlRevert(
          yieldManager.write.pauseByType([pauseType], { account: nonAuthorizedAccount.account }),
          nonAuthorizedAccount,
          requiredRole,
        );
      });
    }
  });

  describe("unpausing", () => {
    it("Security council should be able to unpause GENERAL_PAUSE_TYPE", async () => {
      await yieldManager.write.pauseByType([GENERAL_PAUSE_TYPE], { account: securityCouncil.account });
      await expectEvent(
        yieldManager,
        yieldManager.write.unPauseByType([GENERAL_PAUSE_TYPE], { account: securityCouncil.account }),
        "UnPaused",
        [securityCouncil.address, GENERAL_PAUSE_TYPE],
      );
      await expectNotPaused(yieldManager, GENERAL_PAUSE_TYPE);
    });

    async function pauseThenUnpause(pauseType: number) {
      await yieldManager.write.pauseByType([pauseType], { account: securityCouncil.account });
      await expectEvent(
        yieldManager,
        yieldManager.write.unPauseByType([pauseType], { account: securityCouncil.account }),
        "UnPaused",
        [securityCouncil.address, pauseType],
      );
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
        roleGetter: () => yieldManager.read.UNPAUSE_ALL_ROLE(),
      },
      {
        label: "NATIVE_YIELD_STAKING_PAUSE_TYPE",
        pauseType: NATIVE_YIELD_STAKING_PAUSE_TYPE,
        roleGetter: () => yieldManager.read.UNPAUSE_NATIVE_YIELD_STAKING_ROLE(),
      },
      {
        label: "NATIVE_YIELD_UNSTAKING_PAUSE_TYPE",
        pauseType: NATIVE_YIELD_UNSTAKING_PAUSE_TYPE,
        roleGetter: () => yieldManager.read.UNPAUSE_NATIVE_YIELD_UNSTAKING_ROLE(),
      },
      {
        label: "NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE",
        pauseType: NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE,
        roleGetter: () => yieldManager.read.UNPAUSE_NATIVE_YIELD_PERMISSIONLESS_ACTIONS_ROLE(),
      },
      {
        label: "NATIVE_YIELD_REPORTING_PAUSE_TYPE",
        pauseType: NATIVE_YIELD_REPORTING_PAUSE_TYPE,
        roleGetter: () => yieldManager.read.UNPAUSE_NATIVE_YIELD_REPORTING_ROLE(),
      },
    ] as const;

    for (const { label, pauseType, roleGetter } of unpauseRoleExpectations) {
      it(`Non authorized account should not be able to unpause ${label}`, async () => {
        const requiredRole = await roleGetter();
        await expectAccessControlRevert(
          yieldManager.write.unPauseByType([pauseType], { account: nonAuthorizedAccount.account }),
          nonAuthorizedAccount,
          requiredRole,
        );
      });
    }
  });

  beforeEach(async () => {
    const { networkHelpers } = await network.getOrCreate();
    ({ yieldManager } = await networkHelpers.loadFixture(deployYieldManagerForUnitTest));
  });

  // pauseStaking() unit tests
  describe("Pausing staking", () => {
    it("Should revert when adding if the caller does not have the STAKING_PAUSE_CONTROLLER_ROLE", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      const requiredRole = await yieldManager.read.STAKING_PAUSE_CONTROLLER_ROLE();

      await expectAccessControlRevert(
        yieldManager.write.pauseStaking([mockYieldProviderAddress], { account: nonAuthorizedAccount.account }),
        nonAuthorizedAccount,
        requiredRole,
      );
    });

    it("Should revert when pausing an unknown yield provider", async () => {
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.pauseStaking([randomAddress()], { account: nativeYieldOperator.account }),
        "UnknownYieldProvider",
      );
    });

    it("Should revert if staking already paused", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.write.pauseStaking([mockYieldProviderAddress], { account: nativeYieldOperator.account });

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.pauseStaking([mockYieldProviderAddress], { account: nativeYieldOperator.account }),
        "StakingAlreadyPaused",
      );
    });

    it("Should successfully pause and emit the correct event", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await expectEvent(
        yieldManager,
        yieldManager.write.pauseStaking([mockYieldProviderAddress], { account: nativeYieldOperator.account }),
        "YieldProviderStakingPaused",
        [mockYieldProviderAddress],
      );

      expect(await yieldManager.read.isStakingPaused([mockYieldProviderAddress])).to.be.true;
    });
  });

  // pauseStakingIfNotAlready() unit tests
  describe("Pausing staking if not already", () => {
    it("If not currently paused, should pause", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      expect(await yieldManager.read.isStakingPaused([mockYieldProviderAddress])).to.be.false;

      await yieldManager.write.pauseStakingIfNotAlready([mockYieldProviderAddress], {
        account: nativeYieldOperator.account,
      });

      expect(await yieldManager.read.isStakingPaused([mockYieldProviderAddress])).to.be.true;
    });

    it("If already paused, no-op", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.write.setYieldProviderIsStakingPaused([mockYieldProviderAddress, true]);

      await yieldManager.write.pauseStakingIfNotAlready([mockYieldProviderAddress], {
        account: nativeYieldOperator.account,
      });

      expect(await yieldManager.read.isStakingPaused([mockYieldProviderAddress])).to.be.true;
    });
  });

  // unpauseStaking() unit tests
  describe("Unpausing staking", () => {
    it("Should revert when adding if the caller does not have the STAKING_PAUSE_CONTROLLER_ROLE", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      const requiredRole = await yieldManager.read.STAKING_PAUSE_CONTROLLER_ROLE();

      await expectAccessControlRevert(
        yieldManager.write.unpauseStaking([mockYieldProviderAddress], { account: nonAuthorizedAccount.account }),
        nonAuthorizedAccount,
        requiredRole,
      );
    });

    it("Should revert when pausing an unknown yield provider", async () => {
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.unpauseStaking([randomAddress()], { account: nativeYieldOperator.account }),
        "UnknownYieldProvider",
      );
    });

    it("Should revert if staking already unpaused", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      await setWithdrawalReserveToMinimum(yieldManager);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.unpauseStaking([mockYieldProviderAddress], { account: nativeYieldOperator.account }),
        "StakingAlreadyUnpaused",
      );
    });

    it("Should revert if the withdrawal reserve is in deficit", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.write.pauseStaking([mockYieldProviderAddress], { account: nativeYieldOperator.account });
      await setWithdrawalReserveBalance(yieldManager, 0n);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.unpauseStaking([mockYieldProviderAddress], { account: nativeYieldOperator.account }),
        "InsufficientWithdrawalReserve",
      );
    });

    it("Should revert if ossification has been initiated but not completed", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.write.pauseStaking([mockYieldProviderAddress], { account: nativeYieldOperator.account });
      await setWithdrawalReserveToMinimum(yieldManager);
      await yieldManager.write.setYieldProviderIsOssificationInitiated([mockYieldProviderAddress, true]);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.unpauseStaking([mockYieldProviderAddress], { account: nativeYieldOperator.account }),
        "UnpauseStakingForbiddenDuringPendingOssification",
      );
    });

    it("Should succeed if ossification has been completed", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.write.pauseStaking([mockYieldProviderAddress], { account: nativeYieldOperator.account });
      await setWithdrawalReserveToMinimum(yieldManager);
      await yieldManager.write.setYieldProviderIsOssificationInitiated([mockYieldProviderAddress, true]);
      await yieldManager.write.setYieldProviderIsOssified([mockYieldProviderAddress, true]);

      const tx = yieldManager.write.unpauseStaking([mockYieldProviderAddress], {
        account: nativeYieldOperator.account,
      });
      await tx;
    });

    it("Should revert if there is a current lst liability", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.write.pauseStaking([mockYieldProviderAddress], { account: nativeYieldOperator.account });
      await setWithdrawalReserveToMinimum(yieldManager);
      await yieldManager.write.setYieldProviderLstLiabilityPrincipal([mockYieldProviderAddress, 1n]);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.unpauseStaking([mockYieldProviderAddress], { account: nativeYieldOperator.account }),
        "UnpauseStakingForbiddenWithCurrentLSTLiability",
      );
    });

    it("Should successfully unpause and emit the correct event", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.write.pauseStaking([mockYieldProviderAddress], { account: nativeYieldOperator.account });
      await setWithdrawalReserveToMinimum(yieldManager);

      await expectEvent(
        yieldManager,
        yieldManager.write.unpauseStaking([mockYieldProviderAddress], { account: nativeYieldOperator.account }),
        "YieldProviderStakingUnpaused",
        [mockYieldProviderAddress],
      );

      expect(await yieldManager.read.isStakingPaused([mockYieldProviderAddress])).to.be.false;
    });
  });

  // initiateOssification() unit tests
  describe("Initiate ossification", () => {
    it("Should revert when adding if the caller does not have the OSSIFICATION_INITIATOR_ROLE", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      const requiredRole = await yieldManager.read.OSSIFICATION_INITIATOR_ROLE();

      await expectAccessControlRevert(
        yieldManager.write.initiateOssification([mockYieldProviderAddress], { account: nativeYieldOperator.account }),
        nativeYieldOperator,
        requiredRole,
      );
    });

    it("Should revert when requesting for an unknown yield provider", async () => {
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.initiateOssification([randomAddress()], { account: securityCouncil.account }),
        "UnknownYieldProvider",
      );
    });

    it("Should revert if ossification has been initiated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.write.initiateOssification([mockYieldProviderAddress], { account: securityCouncil.account });

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.initiateOssification([mockYieldProviderAddress], { account: securityCouncil.account }),
        "OssificationAlreadyInitiated",
      );
    });

    it("Should revert if ossification has completed", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.write.setYieldProviderIsOssified([mockYieldProviderAddress, true]);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.initiateOssification([mockYieldProviderAddress], { account: securityCouncil.account }),
        "AlreadyOssified",
      );
    });

    it("Should successfully initiate ossification, pause staking and emit the correct event", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      const initiateCall = yieldManager.write.initiateOssification([mockYieldProviderAddress], {
        account: securityCouncil.account,
      });
      await expectEvent(yieldManager, initiateCall, "YieldProviderStakingPaused", [mockYieldProviderAddress]);
      await expectEvent(yieldManager, initiateCall, "YieldProviderOssificationInitiated", [mockYieldProviderAddress]);

      expect(await yieldManager.read.isOssificationInitiated([mockYieldProviderAddress])).to.be.true;
      expect(await yieldManager.read.isStakingPaused([mockYieldProviderAddress])).to.be.true;
    });
  });

  // progressPendingOssification() unit tests
  describe("Process pending ossification", () => {
    it("Should revert when adding if the caller does not have the OSSIFICATION_PROCESSOR_ROLE", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      const requiredRole = await yieldManager.read.OSSIFICATION_PROCESSOR_ROLE();

      await expectAccessControlRevert(
        yieldManager.write.progressPendingOssification([mockYieldProviderAddress], {
          account: nonAuthorizedAccount.account,
        }),
        nonAuthorizedAccount,
        requiredRole,
      );
    });

    it("Should revert when requesting for an unknown yield provider", async () => {
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.progressPendingOssification([randomAddress()], { account: securityCouncil.account }),
        "UnknownYieldProvider",
      );
    });

    it("Should revert if ossification not initiated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.progressPendingOssification([mockYieldProviderAddress], {
          account: securityCouncil.account,
        }),
        "OssificationNotInitiated",
      );
    });

    it("Should revert if ossification completed", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.write.setYieldProviderIsOssificationInitiated([mockYieldProviderAddress, true]);
      await yieldManager.write.setYieldProviderIsOssified([mockYieldProviderAddress, true]);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.progressPendingOssification([mockYieldProviderAddress], {
          account: securityCouncil.account,
        }),
        "AlreadyOssified",
      );
    });

    it("If YieldProvider does no-op for progress pending, should not complete ossification and emit the correct event", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.write.initiateOssification([mockYieldProviderAddress], { account: securityCouncil.account });
      await yieldManager.write.setProgressPendingOssificationReturnVal(
        [mockYieldProviderAddress, ProgressOssificationResult.NOOP],
        { account: securityCouncil.account },
      );

      const progressOssificationResult = (
        await yieldManager.simulate.progressPendingOssification([mockYieldProviderAddress], {
          account: securityCouncil.address,
        })
      ).result;
      expect(progressOssificationResult).to.eq(ProgressOssificationResult.NOOP);

      await expectEvent(
        yieldManager,
        yieldManager.write.progressPendingOssification([mockYieldProviderAddress], {
          account: securityCouncil.account,
        }),
        "YieldProviderOssificationProcessed",
        [mockYieldProviderAddress, ProgressOssificationResult.NOOP],
      );

      expect(await yieldManager.read.isOssificationInitiated([mockYieldProviderAddress])).to.be.true;
      expect(await yieldManager.read.isOssified([mockYieldProviderAddress])).to.be.false;
    });

    it("If YieldProvider does reinitiate ossification for progress, should not complete ossification and emit the correct event", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.write.initiateOssification([mockYieldProviderAddress], { account: securityCouncil.account });
      await yieldManager.write.setProgressPendingOssificationReturnVal(
        [mockYieldProviderAddress, ProgressOssificationResult.REINITIATED],
        { account: securityCouncil.account },
      );

      const progressOssificationResult = (
        await yieldManager.simulate.progressPendingOssification([mockYieldProviderAddress], {
          account: securityCouncil.address,
        })
      ).result;
      expect(progressOssificationResult).to.eq(ProgressOssificationResult.REINITIATED);

      await expectEvent(
        yieldManager,
        yieldManager.write.progressPendingOssification([mockYieldProviderAddress], {
          account: securityCouncil.account,
        }),
        "YieldProviderOssificationProcessed",
        [mockYieldProviderAddress, ProgressOssificationResult.REINITIATED],
      );

      expect(await yieldManager.read.isOssificationInitiated([mockYieldProviderAddress])).to.be.true;
      expect(await yieldManager.read.isOssified([mockYieldProviderAddress])).to.be.false;
    });

    it("If YieldProvider returns completed ossification, should complete ossification and emit the correct event", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.write.initiateOssification([mockYieldProviderAddress], { account: securityCouncil.account });
      await yieldManager.write.setProgressPendingOssificationReturnVal(
        [mockYieldProviderAddress, ProgressOssificationResult.COMPLETE],
        { account: securityCouncil.account },
      );

      const progressOssificationResult = (
        await yieldManager.simulate.progressPendingOssification([mockYieldProviderAddress], {
          account: securityCouncil.address,
        })
      ).result;
      expect(progressOssificationResult).to.eq(ProgressOssificationResult.COMPLETE);

      await expectEvent(
        yieldManager,
        yieldManager.write.progressPendingOssification([mockYieldProviderAddress], {
          account: securityCouncil.account,
        }),
        "YieldProviderOssificationProcessed",
        [mockYieldProviderAddress, ProgressOssificationResult.COMPLETE],
      );

      expect(await yieldManager.read.isOssified([mockYieldProviderAddress])).to.be.true;
    });
  });
});
