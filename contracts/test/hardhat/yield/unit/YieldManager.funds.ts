// Unit tests on functions handling ETH transfer

import { expect } from "chai";
import { network } from "hardhat";
import { describe, it, before, beforeEach } from "node:test";
import { zeroAddress } from "viem";

import {
  ONE_THOUSAND_ETHER,
  ONE_ETHER,
  GENERAL_PAUSE_TYPE,
  NATIVE_YIELD_STAKING_PAUSE_TYPE,
  NATIVE_YIELD_REPORTING_PAUSE_TYPE,
  NATIVE_YIELD_UNSTAKING_PAUSE_TYPE,
  NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE,
} from "../../common/constants/index.js";
import {
  expectAccessControlRevert,
  expectRevertWhenPaused,
  generateRandomBytes,
  getAccountsFixture,
  type TestAccount,
  expectRevertWithCustomError,
  expectEvent,
} from "../../common/helpers/index.js";
import { deployYieldManagerForUnitTest } from "../helpers/deploy.js";
import {
  buildSetWithdrawalReserveParams,
  fundYieldProviderForWithdrawal,
  getBalance,
  getImpersonatedAccount,
  incrementBalance,
  randomAddress,
  setBalance,
  setWithdrawalReserveToMinimum,
  setWithdrawalReserveToTarget,
  type YieldManagerInitializationData,
} from "../helpers/index.js";
import { addMockYieldProvider } from "../helpers/mocks.js";

import type { ContractReturnType, PublicClient } from "@nomicfoundation/hardhat-viem/types";

describe("YieldManager contract - ETH transfer operations", () => {
  let yieldManager: ContractReturnType<"TestYieldManager">;
  let publicClient: PublicClient;

  let securityCouncil: TestAccount;
  let nonAuthorizedAccount: TestAccount;
  let nativeYieldOperator: TestAccount;
  let l2YieldRecipient: TestAccount;
  let mockLinethRollup: ContractReturnType<"MockLinethRollup">;
  let initializationData: YieldManagerInitializationData;

  const mockWithdrawalParams = generateRandomBytes(8);
  const mockWithdrawalParamsProof = generateRandomBytes(8);
  const mockValidatorIndex = 0n;
  const mockSlot = 100000n; // Must be > lastProvenSlot + SLOTS_PER_HISTORICAL_ROOT (8192)

  before(async () => {
    const { viem, networkHelpers } = await network.getOrCreate();
    publicClient = await viem.getPublicClient();
    ({
      securityCouncil,
      operator: nonAuthorizedAccount,
      nativeYieldOperator,
      l2YieldRecipient,
    } = await networkHelpers.loadFixture(getAccountsFixture));
  });

  beforeEach(async () => {
    const { networkHelpers } = await network.getOrCreate();
    ({ yieldManager, mockLinethRollup, initializationData } =
      await networkHelpers.loadFixture(deployYieldManagerForUnitTest));
  });

  describe("receiving ETH from the L1MessageService", () => {
    it("Should revert when the caller is not the L1MessageService", async () => {
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.receiveFundsFromReserve({ account: nativeYieldOperator.account, value: 1n }),
        "SenderNotL1MessageService",
      );
    });

    it("Should revert when the withdrawal reserve is below minimum", async () => {
      const l1MessageService = mockLinethRollup.address;
      const minimumEffectiveBalance = await yieldManager.read.getEffectiveMinimumWithdrawalReserve();
      const yieldManagerAddress = yieldManager.address;

      await setBalance(l1MessageService, minimumEffectiveBalance);

      // Act - Use helper function on MockLinethRollup
      await expectRevertWithCustomError(
        yieldManager,
        mockLinethRollup.write.callReceiveFundsFromReserve([yieldManagerAddress, 1n], {
          account: nativeYieldOperator.account,
        }),
        "InsufficientWithdrawalReserve",
      );
    });

    it("Should successfully receive funds and emit the expected event", async () => {
      const l1MessageService = mockLinethRollup.address;
      const minimumEffectiveBalance = await yieldManager.read.getEffectiveMinimumWithdrawalReserve();
      const yieldManagerAddress = yieldManager.address;

      await setBalance(l1MessageService, minimumEffectiveBalance + 1n);

      await expectEvent(
        yieldManager,
        mockLinethRollup.write.callReceiveFundsFromReserve([yieldManagerAddress, 1n], {
          account: nativeYieldOperator.account,
        }),
        "ReserveFundsReceived",
        [1n],
      );
    });
  });

  describe("sending ETH to the L1MessageService", () => {
    it("Should revert when the caller is not the YIELD_PROVIDER_UNSTAKER_ROLE", async () => {
      await expectAccessControlRevert(
        yieldManager.write.transferFundsToReserve([1n], { account: nonAuthorizedAccount.account }),
        nonAuthorizedAccount,
        await yieldManager.read.YIELD_PROVIDER_UNSTAKER_ROLE(),
      );
    });

    it("Should revert when the NATIVE_YIELD_UNSTAKING pause type is activated", async () => {
      await yieldManager.write.pauseByType([NATIVE_YIELD_UNSTAKING_PAUSE_TYPE], { account: securityCouncil.account });

      await expectRevertWhenPaused(
        yieldManager,
        yieldManager.write.transferFundsToReserve([1n], { account: nativeYieldOperator.account }),
        NATIVE_YIELD_UNSTAKING_PAUSE_TYPE,
      );
    });

    it("Should revert when the GENERAL pause type is activated", async () => {
      await yieldManager.write.pauseByType([GENERAL_PAUSE_TYPE], { account: securityCouncil.account });

      await expectRevertWhenPaused(
        yieldManager,
        yieldManager.write.transferFundsToReserve([1n], { account: nativeYieldOperator.account }),
        GENERAL_PAUSE_TYPE,
      );
    });

    it("Should successfully send ETH to the L1MessageService", async () => {
      const yieldManagerAddress = yieldManager.address;
      await setBalance(yieldManagerAddress, ONE_THOUSAND_ETHER);
      const transferAmount = 1_000_000_000_000_000_000n;
      const l1MessageService = mockLinethRollup.address;

      const reserveBalanceBefore = await publicClient.getBalance({ address: l1MessageService });
      const yieldManagerBalanceBefore = await publicClient.getBalance({ address: yieldManagerAddress });

      await yieldManager.write.transferFundsToReserve([transferAmount], { account: nativeYieldOperator.account });

      const reserveBalanceAfter = await publicClient.getBalance({ address: l1MessageService });
      const yieldManagerBalanceAfter = await publicClient.getBalance({ address: yieldManagerAddress });

      expect(reserveBalanceAfter).to.equal(reserveBalanceBefore + transferAmount);
      expect(yieldManagerBalanceAfter).to.equal(yieldManagerBalanceBefore - transferAmount);
    });
  });

  describe("sending ETH to a YieldProvider", () => {
    it("Should revert when the caller is not the YIELD_PROVIDER_STAKING_ROLE", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await expectAccessControlRevert(
        yieldManager.write.fundYieldProvider([mockYieldProviderAddress, 1n], { account: nonAuthorizedAccount.account }),
        nonAuthorizedAccount,
        await yieldManager.read.YIELD_PROVIDER_STAKING_ROLE(),
      );
    });

    it("Should revert when the NATIVE_YIELD_STAKING pause type is activated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.write.pauseByType([NATIVE_YIELD_STAKING_PAUSE_TYPE], { account: securityCouncil.account });

      await expectRevertWhenPaused(
        yieldManager,
        yieldManager.write.fundYieldProvider([mockYieldProviderAddress, 1n], { account: nativeYieldOperator.account }),
        NATIVE_YIELD_STAKING_PAUSE_TYPE,
      );
    });

    it("Should revert when the GENERAL pause type is activated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.write.pauseByType([GENERAL_PAUSE_TYPE], { account: securityCouncil.account });

      await expectRevertWhenPaused(
        yieldManager,
        yieldManager.write.fundYieldProvider([mockYieldProviderAddress, 1n], { account: nativeYieldOperator.account }),
        GENERAL_PAUSE_TYPE,
      );
    });

    it("Should revert when sending to an unknown YieldProvider", async () => {
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.fundYieldProvider([randomAddress(), 1n], { account: nativeYieldOperator.account }),
        "UnknownYieldProvider",
      );
    });

    it("Should revert when the withdrawal reserve is in deficit", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.fundYieldProvider([mockYieldProviderAddress, 1n], { account: nativeYieldOperator.account }),
        "InsufficientWithdrawalReserve",
      );
    });

    it("should successfully send ETH to the YieldProvider, update state and emit the expected event", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      const l1MessageService = mockLinethRollup.address;
      const yieldManagerAddress = yieldManager.address;

      const minimumReserveAmount = await yieldManager.read.getMinimumWithdrawalReserveAmount();
      await setBalance(l1MessageService, minimumReserveAmount);
      const transferAmount = 40n;
      await setBalance(yieldManagerAddress, transferAmount);

      await expectEvent(
        yieldManager,
        yieldManager.write.fundYieldProvider([mockYieldProviderAddress, transferAmount], {
          account: nativeYieldOperator.account,
        }),
        "YieldProviderFunded",
        [mockYieldProviderAddress, transferAmount],
      );

      const yieldProviderData = await yieldManager.read.getYieldProviderData([mockYieldProviderAddress]);
      expect(yieldProviderData.userFunds).to.equal(transferAmount);

      expect(await yieldManager.read.isStakingPaused([mockYieldProviderAddress])).to.be.true;
      expect(await yieldManager.read.userFundsInYieldProvidersTotal()).to.equal(transferAmount);
    });
  });

  describe("reporting yield", () => {
    it("Should revert when the caller is not the YIELD_REPORTER_ROLE", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await expectAccessControlRevert(
        yieldManager.write.reportYield([mockYieldProviderAddress, l2YieldRecipient.address], {
          account: nonAuthorizedAccount.account,
        }),
        nonAuthorizedAccount,
        await yieldManager.read.YIELD_REPORTER_ROLE(),
      );
    });

    it("Should revert when the NATIVE_YIELD_REPORTING pause type is activated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.write.pauseByType([NATIVE_YIELD_REPORTING_PAUSE_TYPE], { account: securityCouncil.account });

      await expectRevertWhenPaused(
        yieldManager,
        yieldManager.write.reportYield([mockYieldProviderAddress, l2YieldRecipient.address], {
          account: nativeYieldOperator.account,
        }),
        NATIVE_YIELD_REPORTING_PAUSE_TYPE,
      );
    });

    it("Should revert when the GENERAL pause type is activated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.write.pauseByType([GENERAL_PAUSE_TYPE], { account: securityCouncil.account });

      await expectRevertWhenPaused(
        yieldManager,
        yieldManager.write.reportYield([mockYieldProviderAddress, l2YieldRecipient.address], {
          account: nativeYieldOperator.account,
        }),
        GENERAL_PAUSE_TYPE,
      );
    });

    it("Should revert when reporting for an unknown YieldProvider", async () => {
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.reportYield([randomAddress(), l2YieldRecipient.address], {
          account: nativeYieldOperator.account,
        }),
        "UnknownYieldProvider",
      );
    });

    it("Should revert when distributing yield to an unknown L2YieldRecipient", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      const unknownRecipient = randomAddress();

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.reportYield([mockYieldProviderAddress, unknownRecipient], {
          account: nativeYieldOperator.account,
        }),
        "UnknownL2YieldRecipient",
      );
    });

    it("Should successfully report positive yield, update state and emit the expected event", async () => {
      // ARRANGE
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      const reportedYield = ONE_ETHER;
      const outstandingNegativeYield = 0n;

      await yieldManager.write.setReportYieldReturnVal_NewReportedYield([mockYieldProviderAddress, reportedYield], {
        account: nativeYieldOperator.account,
      });
      await yieldManager.write.setReportYieldReturnVal_OutstandingNegativeYield(
        [mockYieldProviderAddress, outstandingNegativeYield],
        { account: nativeYieldOperator.account },
      );

      // ACT + ASSERT
      await expectEvent(
        yieldManager,
        yieldManager.write.reportYield([mockYieldProviderAddress, l2YieldRecipient.address], {
          account: nativeYieldOperator.account,
        }),
        "NativeYieldReported",
        [mockYieldProviderAddress, l2YieldRecipient.address, reportedYield, outstandingNegativeYield],
      );

      const providerData = await yieldManager.read.getYieldProviderData([mockYieldProviderAddress]);
      expect(providerData.userFunds).to.equal(reportedYield);
      expect(providerData.yieldReportedCumulative).to.equal(reportedYield);
      expect(await yieldManager.read.userFunds([mockYieldProviderAddress])).to.equal(reportedYield);
      expect(await yieldManager.read.userFundsInYieldProvidersTotal()).to.equal(reportedYield);
      expect(await yieldManager.read.getYieldProviderLastReportedNegativeYield([mockYieldProviderAddress])).to.equal(
        outstandingNegativeYield,
      );
    });

    it("Should successfully report negative yield, update state and emit the expected event", async () => {
      // ARRANGE
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      const reportedYield = 0n;
      const outstandingNegativeYield = ONE_ETHER * 2n;

      await yieldManager.write.setReportYieldReturnVal_NewReportedYield([mockYieldProviderAddress, reportedYield], {
        account: nativeYieldOperator.account,
      });
      await yieldManager.write.setReportYieldReturnVal_OutstandingNegativeYield(
        [mockYieldProviderAddress, outstandingNegativeYield],
        { account: nativeYieldOperator.account },
      );

      // ACT + ASSERT
      await expectEvent(
        yieldManager,
        yieldManager.write.reportYield([mockYieldProviderAddress, l2YieldRecipient.address], {
          account: nativeYieldOperator.account,
        }),
        "NativeYieldReported",
        [mockYieldProviderAddress, l2YieldRecipient.address, reportedYield, outstandingNegativeYield],
      );

      const providerData = await yieldManager.read.getYieldProviderData([mockYieldProviderAddress]);
      expect(providerData.userFunds).to.equal(reportedYield);
      expect(providerData.yieldReportedCumulative).to.equal(reportedYield);
      expect(await yieldManager.read.userFunds([mockYieldProviderAddress])).to.equal(reportedYield);
      expect(await yieldManager.read.userFundsInYieldProvidersTotal()).to.equal(reportedYield);
      expect(await yieldManager.read.getYieldProviderLastReportedNegativeYield([mockYieldProviderAddress])).to.equal(
        outstandingNegativeYield,
      );
    });
  });

  describe("permissioned unstake", () => {
    it("Should revert when the caller is not the YIELD_PROVIDER_UNSTAKER_ROLE", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await expectAccessControlRevert(
        yieldManager.write.unstake([mockYieldProviderAddress, mockWithdrawalParams], {
          account: nonAuthorizedAccount.account,
        }),
        nonAuthorizedAccount,
        await yieldManager.read.YIELD_PROVIDER_UNSTAKER_ROLE(),
      );
    });

    it("Should revert when the GENERAL pause type is activated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      await yieldManager.write.pauseByType([GENERAL_PAUSE_TYPE], { account: securityCouncil.account });

      await expectRevertWhenPaused(
        yieldManager,
        yieldManager.write.unstake([mockYieldProviderAddress, mockWithdrawalParams], {
          account: nativeYieldOperator.account,
        }),
        GENERAL_PAUSE_TYPE,
      );
    });

    it("Should revert when the NATIVE_YIELD_UNSTAKING pause type is activated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      await yieldManager.write.pauseByType([NATIVE_YIELD_UNSTAKING_PAUSE_TYPE], { account: securityCouncil.account });

      await expectRevertWhenPaused(
        yieldManager,
        yieldManager.write.unstake([mockYieldProviderAddress, mockWithdrawalParams], {
          account: nativeYieldOperator.account,
        }),
        NATIVE_YIELD_UNSTAKING_PAUSE_TYPE,
      );
    });

    it("Should revert when unstaking from an unknown YieldProvider", async () => {
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.unstake([randomAddress(), mockWithdrawalParams], { account: nativeYieldOperator.account }),
        "UnknownYieldProvider",
      );
    });

    it("Should successfully unstake from a YieldProvider", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.write.unstake([mockYieldProviderAddress, mockWithdrawalParams], {
        account: nativeYieldOperator.account,
      });
    });
  });

  describe("permissionless unstake", () => {
    it("Should revert when the GENERAL pause type is activated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      await yieldManager.write.pauseByType([GENERAL_PAUSE_TYPE], { account: securityCouncil.account });

      await expectRevertWhenPaused(
        yieldManager,
        yieldManager.write.unstakePermissionless(
          [mockYieldProviderAddress, mockValidatorIndex, mockSlot, mockWithdrawalParams, mockWithdrawalParamsProof],
          { account: nativeYieldOperator.account },
        ),
        GENERAL_PAUSE_TYPE,
      );
    });

    it("Should revert when the NATIVE_YIELD_PERMISSIONLESS_ACTIONS pause type is activated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      await yieldManager.write.pauseByType([NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE], {
        account: securityCouncil.account,
      });

      await expectRevertWhenPaused(
        yieldManager,
        yieldManager.write.unstakePermissionless(
          [mockYieldProviderAddress, mockValidatorIndex, mockSlot, mockWithdrawalParams, mockWithdrawalParamsProof],
          { account: nativeYieldOperator.account },
        ),
        NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE,
      );
    });

    it("Should revert when unstaking from an unknown YieldProvider", async () => {
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.unstakePermissionless(
          [randomAddress(), mockValidatorIndex, mockSlot, mockWithdrawalParams, mockWithdrawalParamsProof],
          { account: nativeYieldOperator.account },
        ),
        "UnknownYieldProvider",
      );
    });

    it("Should revert when the withdrawal reserve is not in deficit", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      const minimumReserveAmount = await yieldManager.read.getMinimumWithdrawalReserveAmount();
      await setBalance(mockLinethRollup.address, minimumReserveAmount);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.unstakePermissionless(
          [mockYieldProviderAddress, mockValidatorIndex, mockSlot, mockWithdrawalParams, mockWithdrawalParamsProof],
          { account: nativeYieldOperator.account },
        ),
        "WithdrawalReserveNotInDeficit",
      );
    });

    it("Should ignore msg.value for withdrawal reserve deficit check", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      // Arrange - %-based minimum at 20%
      await yieldManager.write.setWithdrawalReserveParameters(
        [buildSetWithdrawalReserveParams(initializationData, { minAmount: 0n })],
        { account: securityCouncil.account },
      );
      await setBalance(mockLinethRollup.address, 20n * ONE_ETHER);
      await setBalance(yieldManager.address, 80n * ONE_ETHER);

      const msgValue = ONE_ETHER * 100n;
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.unstakePermissionless(
          [mockYieldProviderAddress, mockValidatorIndex, mockSlot, mockWithdrawalParams, mockWithdrawalParamsProof],
          { account: nativeYieldOperator.account, value: msgValue },
        ),
        "WithdrawalReserveNotInDeficit",
      );
    });

    it("Should revert when the slot is too close to the last proven slot for the validator", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      const SLOTS_PER_HISTORICAL_ROOT = 8192n;

      // Arrange - Set up withdrawal reserve in deficit
      await setBalance(mockLinethRollup.address, 0n);

      // Arrange - Set up mocks for successful first call
      const unstakeAmount = ONE_ETHER;
      await yieldManager.write.setWithdrawableValueReturnVal([mockYieldProviderAddress, 0n]);
      await yieldManager.write.setUnstakePermissionlessReturnVal([mockYieldProviderAddress, unstakeAmount]);

      // Arrange - First call with slot = 100000, which sets lastProvenSlot[validatorIndex] = 100000
      const firstSlot = 100000n;
      await yieldManager.write.unstakePermissionless(
        [mockYieldProviderAddress, mockValidatorIndex, firstSlot, mockWithdrawalParams, mockWithdrawalParamsProof],
        { account: nativeYieldOperator.account },
      );

      // Arrange - Try to call again with slot that's too close (<= lastProvenSlot + SLOTS_PER_HISTORICAL_ROOT)
      // lastProvenSlot = 100000, so slot must be > 100000 + 8192 = 108192
      // Using slot = 108192 should revert (boundary case)
      const tooCloseSlot = firstSlot + SLOTS_PER_HISTORICAL_ROOT;

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.unstakePermissionless(
          [mockYieldProviderAddress, mockValidatorIndex, tooCloseSlot, mockWithdrawalParams, mockWithdrawalParamsProof],
          { account: nativeYieldOperator.account },
        ),
        "SlotTooCloseToLastProvenSlot",
        [mockValidatorIndex, firstSlot, tooCloseSlot],
      );
    });

    it("Should revert when requiredUnstakeAmountWei is 0", async () => {
      // Arrange - Put targetDeficit on YieldProvider
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      const targetReserveAmount = await yieldManager.read.getTargetWithdrawalReserveAmount();
      await setBalance(mockLinethRollup.address, 0n);
      await setBalance(yieldManager.address, 10n + targetReserveAmount / 3n);
      await yieldManager.write.setWithdrawableValueReturnVal([mockYieldProviderAddress, targetReserveAmount / 3n]);
      await yieldManager.write.setYieldProviderUserFunds([mockYieldProviderAddress, targetReserveAmount / 3n]);
      await yieldManager.write.setPendingPermissionlessUnstake([targetReserveAmount / 3n]);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.unstakePermissionless(
          [mockYieldProviderAddress, mockValidatorIndex, mockSlot, mockWithdrawalParams, mockWithdrawalParamsProof],
          { account: nativeYieldOperator.account },
        ),
        "NoRequirementToUnstakePermissionless",
      );
    });

    it("Should revert when requiredUnstakeAmountWei is 0, ignoring msg.value", async () => {
      // Arrange - Put targetDeficit on YieldProvider
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      const targetReserveAmount = await yieldManager.read.getTargetWithdrawalReserveAmount();
      await setBalance(mockLinethRollup.address, 0n);
      await setBalance(yieldManager.address, 10n + targetReserveAmount / 3n);
      await yieldManager.write.setWithdrawableValueReturnVal([mockYieldProviderAddress, targetReserveAmount / 3n]);
      await yieldManager.write.setYieldProviderUserFunds([mockYieldProviderAddress, targetReserveAmount / 3n]);
      await yieldManager.write.setPendingPermissionlessUnstake([targetReserveAmount / 3n]);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.unstakePermissionless(
          [mockYieldProviderAddress, mockValidatorIndex, mockSlot, mockWithdrawalParams, mockWithdrawalParamsProof],
          { account: nativeYieldOperator.account, value: ONE_ETHER * 8000n },
        ),
        "NoRequirementToUnstakePermissionless",
      );
    });

    it("Should revert when the YieldProvider returns 0 unstake amount", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      await yieldManager.write.setUnstakePermissionlessReturnVal([mockYieldProviderAddress, 0n], {
        account: nativeYieldOperator.account,
      });

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.unstakePermissionless(
          [mockYieldProviderAddress, mockValidatorIndex, mockSlot, mockWithdrawalParams, mockWithdrawalParamsProof],
          { account: nativeYieldOperator.account },
        ),
        "YieldProviderReturnedZeroUnstakeAmount",
      );
    });

    it("Should revert when unstake amount > required unstake amount", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      const targetDeficit = await yieldManager.read.getTargetReserveDeficit();
      await yieldManager.write.setUnstakePermissionlessReturnVal([mockYieldProviderAddress, targetDeficit * 2n], {
        account: nativeYieldOperator.account,
      });

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.unstakePermissionless(
          [mockYieldProviderAddress, mockValidatorIndex, mockSlot, mockWithdrawalParams, mockWithdrawalParamsProof],
          { account: nativeYieldOperator.account },
        ),
        "UnstakedAmountExceedsRequired",
        [targetDeficit * 2n, targetDeficit],
      );
    });

    it("Should successfully submit the unstake request, change state and emit the expected event", async () => {
      // Arrange - Set up withdrawal reserve in deficit
      await setBalance(mockLinethRollup.address, 0n);

      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      const targetReserveAmount = await yieldManager.read.getTargetWithdrawalReserveAmount();

      // Set up balances so requiredUnstakeAmountWei is non-zero
      // requiredUnstakeAmountWei = targetDeficit - (YieldManager.balance + withdrawableValue + pendingPermissionlessUnstake)
      // We want requiredUnstakeAmountWei > 0, so we set balances to be less than targetDeficit
      const yieldManagerBalance = targetReserveAmount / 3n;
      await setBalance(yieldManager.address, yieldManagerBalance);
      await yieldManager.write.setWithdrawableValueReturnVal([mockYieldProviderAddress, 0n]);
      await yieldManager.write.setPendingPermissionlessUnstake([0n]);

      // Calculate expected requiredUnstakeAmountWei
      const targetDeficit = await yieldManager.read.getTargetReserveDeficit();
      const expectedRequiredUnstakeAmountWei = targetDeficit - yieldManagerBalance;

      // Set unstakedAmount to be non-zero and less than or equal to requiredUnstakeAmountWei
      const unstakedAmount = expectedRequiredUnstakeAmountWei;
      await yieldManager.write.setUnstakePermissionlessReturnVal([mockYieldProviderAddress, unstakedAmount], {
        account: nativeYieldOperator.account,
      });

      await expectEvent(
        yieldManager,
        yieldManager.write.unstakePermissionless(
          [mockYieldProviderAddress, mockValidatorIndex, mockSlot, mockWithdrawalParams, mockWithdrawalParamsProof],
          { account: nativeYieldOperator.account },
        ),
        "UnstakePermissionlessRequest",
        [
          mockYieldProviderAddress,
          mockValidatorIndex,
          mockSlot,
          expectedRequiredUnstakeAmountWei,
          unstakedAmount,
          mockWithdrawalParams,
        ],
      );

      expect(await yieldManager.read.pendingPermissionlessUnstake()).to.equal(unstakedAmount);
      expect(await yieldManager.read.lastProvenSlot([mockValidatorIndex])).to.equal(mockSlot);
    });

    it("After submitting one unstake request that restores the reserve deficit, the next permissionless request reverts", async () => {
      // Arrange - Set up withdrawal reserve in deficit
      await setBalance(mockLinethRollup.address, 0n);

      // Arrange - First do unstake permissionless up to maximum capacity
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      const targetReserveAmount = await yieldManager.read.getTargetWithdrawalReserveAmount();
      const unstakeAmount = targetReserveAmount;

      await yieldManager.write.setWithdrawableValueReturnVal([mockYieldProviderAddress, 0n]);
      await yieldManager.write.setUnstakePermissionlessReturnVal([mockYieldProviderAddress, unstakeAmount], {
        account: nativeYieldOperator.account,
      });

      const firstSlot = mockSlot;
      await yieldManager.write.unstakePermissionless(
        [mockYieldProviderAddress, mockValidatorIndex, firstSlot, mockWithdrawalParams, mockWithdrawalParamsProof],
        { account: nativeYieldOperator.account },
      );

      expect(await yieldManager.read.pendingPermissionlessUnstake()).to.equal(unstakeAmount);

      // Arrange - Then do unstake of 1
      const secondUnstakeAmount = 1n;
      await yieldManager.write.setUnstakePermissionlessReturnVal([mockYieldProviderAddress, secondUnstakeAmount], {
        account: nativeYieldOperator.account,
      });

      // Use a slot that's far enough from the first slot (firstSlot + SLOTS_PER_HISTORICAL_ROOT + 1)
      const secondSlot = firstSlot + 8192n + 1n;

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.unstakePermissionless(
          [mockYieldProviderAddress, mockValidatorIndex, secondSlot, mockWithdrawalParams, mockWithdrawalParamsProof],
          { account: nativeYieldOperator.account },
        ),
        "NoRequirementToUnstakePermissionless",
      );
    });
  });

  describe("delegatecall helper for withdraw from yield provider helper", () => {
    it("Should revert if the YieldProvider does not have sufficient balance", async () => {
      const { mockYieldProviderAddress, mockWithdrawTarget } = await addMockYieldProvider(yieldManager);
      const call = yieldManager.write.delegatecallWithdrawFromYieldProvider([mockYieldProviderAddress, 1n], {
        account: nativeYieldOperator.account,
      });

      await expectRevertWithCustomError(mockWithdrawTarget, call, "MockWithdrawFailed");
    });

    it("Delegatecalls successfully and makes correct state transitions", async () => {
      const { mockYieldProviderAddress, mockYieldProvider } = await addMockYieldProvider(yieldManager);
      const withdrawAmount = ONE_ETHER;
      await fundYieldProviderForWithdrawal(yieldManager, mockYieldProvider, nativeYieldOperator, withdrawAmount);

      const beforeYieldProviderUserFunds = await yieldManager.read.userFunds([mockYieldProviderAddress]);
      const beforeUserFundsInYieldProvidersTotal = await yieldManager.read.userFundsInYieldProvidersTotal();

      // Act
      await yieldManager.write.delegatecallWithdrawFromYieldProvider([mockYieldProviderAddress, withdrawAmount], {
        account: nativeYieldOperator.account,
      });

      // Assert
      const afterYieldProviderUserFunds = await yieldManager.read.userFunds([mockYieldProviderAddress]);
      const afterUserFundsInYieldProvidersTotal = await yieldManager.read.userFundsInYieldProvidersTotal();

      expect(afterYieldProviderUserFunds).to.equal(beforeYieldProviderUserFunds - withdrawAmount);
      expect(afterUserFundsInYieldProvidersTotal).to.equal(beforeUserFundsInYieldProvidersTotal - withdrawAmount);
    });
  });

  describe("decrementPendingPermissionlessUnstake helper", () => {
    it("if pendingPermissionlessUnstake = 0, should be a no-op", async () => {
      expect(await yieldManager.read.getPendingPermissionlessUnstake()).to.equal(0n);

      await yieldManager.write.decrementPendingPermissionlessUnstake([ONE_ETHER], {
        account: nativeYieldOperator.account,
      });

      expect(await yieldManager.read.getPendingPermissionlessUnstake()).to.equal(0n);
    });

    it("if pendingPermissionlessUnstake <= _amount, should reduce pendingPermissionlessUnstake to 0", async () => {
      await yieldManager.write.setPendingPermissionlessUnstake([ONE_ETHER]);

      await yieldManager.write.decrementPendingPermissionlessUnstake([ONE_THOUSAND_ETHER], {
        account: nativeYieldOperator.account,
      });

      expect(await yieldManager.read.getPendingPermissionlessUnstake()).to.equal(0n);
    });

    it("if pendingPermissionlessUnstake > _amount, should reduce pendingPermissionlessUnstake accordingly", async () => {
      const startingPending = ONE_THOUSAND_ETHER;
      await yieldManager.write.setPendingPermissionlessUnstake([startingPending]);

      await yieldManager.write.decrementPendingPermissionlessUnstake([ONE_ETHER], {
        account: nativeYieldOperator.account,
      });

      expect(await yieldManager.read.getPendingPermissionlessUnstake()).to.equal(startingPending - ONE_ETHER);
    });
  });

  describe("safe withdraw from yield provider", () => {
    it("Should revert when the GENERAL pause type is activated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.write.pauseByType([GENERAL_PAUSE_TYPE], { account: securityCouncil.account });

      await expectRevertWhenPaused(
        yieldManager,
        yieldManager.write.safeWithdrawFromYieldProvider([mockYieldProviderAddress, 1n], {
          account: nativeYieldOperator.account,
        }),
        GENERAL_PAUSE_TYPE,
      );
    });

    it("Should revert when the NATIVE_YIELD_UNSTAKING pause type is activated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.write.pauseByType([NATIVE_YIELD_UNSTAKING_PAUSE_TYPE], { account: securityCouncil.account });

      await expectRevertWhenPaused(
        yieldManager,
        yieldManager.write.safeWithdrawFromYieldProvider([mockYieldProviderAddress, 1n], {
          account: nativeYieldOperator.account,
        }),
        NATIVE_YIELD_UNSTAKING_PAUSE_TYPE,
      );
    });

    it("Should revert when unstaking from an unknown YieldProvider", async () => {
      const unknownYieldProvider = randomAddress();

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.safeWithdrawFromYieldProvider([unknownYieldProvider, 1n], {
          account: nativeYieldOperator.account,
        }),
        "UnknownYieldProvider",
      );
    });

    it("Should revert when the caller does not have YIELD_PROVIDER_UNSTAKER_ROLE role", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      const unstakerRole = await yieldManager.read.YIELD_PROVIDER_UNSTAKER_ROLE();

      await expectAccessControlRevert(
        yieldManager.write.safeWithdrawFromYieldProvider([mockYieldProviderAddress, 1n], {
          account: nonAuthorizedAccount.account,
        }),
        nonAuthorizedAccount,
        unstakerRole,
      );
    });

    it("If _amount < withdrawableValue, should withdraw _amount", async () => {
      const { mockYieldProviderAddress, mockYieldProvider } = await addMockYieldProvider(yieldManager);
      const withdrawableAmount = ONE_ETHER * 2n;
      const requestedAmount = ONE_ETHER;

      await fundYieldProviderForWithdrawal(yieldManager, mockYieldProvider, nativeYieldOperator, withdrawableAmount);
      await setWithdrawalReserveToTarget(yieldManager);
      expect(await yieldManager.read.getTargetReserveDeficit()).to.equal(0n);

      await expectEvent(
        yieldManager,
        yieldManager.write.safeWithdrawFromYieldProvider([mockYieldProviderAddress, requestedAmount], {
          account: nativeYieldOperator.account,
        }),
        "YieldProviderWithdrawal",
        [mockYieldProviderAddress, requestedAmount, 0n],
      );

      expect(await yieldManager.read.userFunds([mockYieldProviderAddress])).to.equal(
        withdrawableAmount - requestedAmount,
      );
      expect(await yieldManager.read.userFundsInYieldProvidersTotal()).to.equal(withdrawableAmount - requestedAmount);
    });

    it("If withdrawableValue < _amount , should withdraw withdrawableValue", async () => {
      const { mockYieldProviderAddress, mockYieldProvider } = await addMockYieldProvider(yieldManager);
      const withdrawableAmount = ONE_ETHER;
      const requestedAmount = withdrawableAmount * 2n;

      await fundYieldProviderForWithdrawal(yieldManager, mockYieldProvider, nativeYieldOperator, withdrawableAmount);
      await setWithdrawalReserveToTarget(yieldManager);
      expect(await yieldManager.read.getTargetReserveDeficit()).to.equal(0n);

      await expectEvent(
        yieldManager,
        yieldManager.write.safeWithdrawFromYieldProvider([mockYieldProviderAddress, requestedAmount], {
          account: nativeYieldOperator.account,
        }),
        "YieldProviderWithdrawal",
        [mockYieldProviderAddress, withdrawableAmount, 0n],
      );

      expect(await yieldManager.read.userFunds([mockYieldProviderAddress])).to.equal(0n);
      expect(await yieldManager.read.userFundsInYieldProvidersTotal()).to.equal(0n);
    });

    it("With 0 targetDeficit, should successfully withdraw the full _amount to the YieldManager", async () => {
      const { mockYieldProviderAddress, mockYieldProvider } = await addMockYieldProvider(yieldManager);
      const withdrawAmount = ONE_ETHER;
      await fundYieldProviderForWithdrawal(yieldManager, mockYieldProvider, nativeYieldOperator, withdrawAmount);

      const l1MessageService = mockLinethRollup.address;
      const targetReserveAmount = await yieldManager.read.getTargetWithdrawalReserveAmount();
      const yieldManagerAddress = yieldManager.address;

      await setBalance(l1MessageService, targetReserveAmount);
      await setBalance(yieldManagerAddress, withdrawAmount);

      expect(await yieldManager.read.getTargetReserveDeficit()).to.equal(0n);

      await expectEvent(
        yieldManager,
        yieldManager.write.safeWithdrawFromYieldProvider([mockYieldProviderAddress, withdrawAmount], {
          account: nativeYieldOperator.account,
        }),
        "YieldProviderWithdrawal",
        [mockYieldProviderAddress, withdrawAmount, 0n],
      );

      expect(await yieldManager.read.userFunds([mockYieldProviderAddress])).to.equal(0n);
      expect(await yieldManager.read.userFundsInYieldProvidersTotal()).to.equal(0n);
      expect(await yieldManager.read.isStakingPaused([mockYieldProviderAddress])).to.be.false;
      expect(await publicClient.getBalance({ address: l1MessageService })).to.equal(targetReserveAmount);
    });

    it("With targetDeficit > _amount, should withdraw the full _amount to the reserve and pause staking", async () => {
      const { mockYieldProviderAddress, mockYieldProvider } = await addMockYieldProvider(yieldManager);
      const withdrawAmount = ONE_ETHER;
      await fundYieldProviderForWithdrawal(yieldManager, mockYieldProvider, nativeYieldOperator, withdrawAmount);

      const targetReserveAmount = await yieldManager.read.getTargetWithdrawalReserveAmount();
      const l1MessageService = mockLinethRollup.address;
      const yieldManagerAddress = yieldManager.address;
      const targetDeficit = withdrawAmount * 2n;
      const reserveBalanceBefore = targetReserveAmount - targetDeficit;

      await setBalance(l1MessageService, reserveBalanceBefore);
      await setBalance(yieldManagerAddress, withdrawAmount);

      expect(await yieldManager.read.getTargetReserveDeficit()).to.equal(targetDeficit);
      expect(targetDeficit).to.be.above(withdrawAmount);

      await expectEvent(
        yieldManager,
        yieldManager.write.safeWithdrawFromYieldProvider([mockYieldProviderAddress, withdrawAmount], {
          account: nativeYieldOperator.account,
        }),
        "YieldProviderWithdrawal",
        [mockYieldProviderAddress, withdrawAmount, withdrawAmount],
      );

      expect(await publicClient.getBalance({ address: l1MessageService })).to.equal(
        reserveBalanceBefore + withdrawAmount,
      );
      expect(await yieldManager.read.userFunds([mockYieldProviderAddress])).to.equal(0n);
      expect(await yieldManager.read.userFundsInYieldProvidersTotal()).to.equal(0n);
      expect(await yieldManager.read.isStakingPaused([mockYieldProviderAddress])).to.be.true;
    });

    it("With targetDeficit < _amount, should send only the target deficit to the reserve and not pause staking", async () => {
      const { mockYieldProviderAddress, mockYieldProvider } = await addMockYieldProvider(yieldManager);
      const withdrawAmount = ONE_ETHER;
      await fundYieldProviderForWithdrawal(yieldManager, mockYieldProvider, nativeYieldOperator, withdrawAmount);

      const targetReserveAmount = await yieldManager.read.getTargetWithdrawalReserveAmount();
      const l1MessageService = mockLinethRollup.address;
      const yieldManagerAddress = yieldManager.address;
      const targetDeficit = withdrawAmount / 4n;
      const reserveBalanceBefore = targetReserveAmount - targetDeficit;

      await setBalance(l1MessageService, reserveBalanceBefore);
      await setBalance(yieldManagerAddress, withdrawAmount);

      expect(await yieldManager.read.getTargetReserveDeficit()).to.equal(targetDeficit);

      await expectEvent(
        yieldManager,
        yieldManager.write.safeWithdrawFromYieldProvider([mockYieldProviderAddress, withdrawAmount], {
          account: nativeYieldOperator.account,
        }),
        "YieldProviderWithdrawal",
        [mockYieldProviderAddress, withdrawAmount, targetDeficit],
      );

      expect(await publicClient.getBalance({ address: l1MessageService })).to.equal(
        reserveBalanceBefore + targetDeficit,
      );
      expect(await yieldManager.read.userFunds([mockYieldProviderAddress])).to.equal(0n);
      expect(await yieldManager.read.userFundsInYieldProvidersTotal()).to.equal(0n);
      expect(await yieldManager.read.isStakingPaused([mockYieldProviderAddress])).to.be.false;
    });
  });

  describe("safely adding to withdrawal reserve", () => {
    it("Should revert when the GENERAL pause type is activated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      await yieldManager.write.pauseByType([GENERAL_PAUSE_TYPE], { account: securityCouncil.account });
      await expectRevertWhenPaused(
        yieldManager,
        yieldManager.write.safeAddToWithdrawalReserve([mockYieldProviderAddress, 1n], {
          account: nativeYieldOperator.account,
        }),
        GENERAL_PAUSE_TYPE,
      );
    });
    it("Should revert when the NATIVE_YIELD_UNSTAKING pause type is activated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      await yieldManager.write.pauseByType([NATIVE_YIELD_UNSTAKING_PAUSE_TYPE], { account: securityCouncil.account });
      await expectRevertWhenPaused(
        yieldManager,
        yieldManager.write.safeAddToWithdrawalReserve([mockYieldProviderAddress, 1n], {
          account: nativeYieldOperator.account,
        }),
        NATIVE_YIELD_UNSTAKING_PAUSE_TYPE,
      );
    });
    it("Should revert when rebalancing from an unknown YieldProvider", async () => {
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.safeAddToWithdrawalReserve([randomAddress(), 1n], { account: nativeYieldOperator.account }),
        "UnknownYieldProvider",
      );
    });
    it("Should revert when the caller does not have YIELD_PROVIDER_UNSTAKER_ROLE role", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      await expectAccessControlRevert(
        yieldManager.write.safeAddToWithdrawalReserve([mockYieldProviderAddress, 1n], {
          account: nonAuthorizedAccount.account,
        }),
        nonAuthorizedAccount,
        await yieldManager.read.YIELD_PROVIDER_UNSTAKER_ROLE(),
      );
    });
    it("With _amount min, _amount < _withdrawableValue, will send _amount from YieldManager to reserve", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      // Arrange - Setup YieldManager balance
      const yieldManagerBalance = ONE_ETHER * 4n;
      const yieldManagerAddress = yieldManager.address;
      await setBalance(yieldManagerAddress, yieldManagerBalance);

      // Act
      const rebalanceAmount = ONE_ETHER * 2n;
      await expectEvent(
        yieldManager,
        yieldManager.write.safeAddToWithdrawalReserve([mockYieldProviderAddress, rebalanceAmount], {
          account: nativeYieldOperator.account,
        }),
        "WithdrawalReserveAugmented",
        [mockYieldProviderAddress, rebalanceAmount, rebalanceAmount, 0n],
      );

      expect(await publicClient.getBalance({ address: yieldManagerAddress })).to.equal(
        yieldManagerBalance - rebalanceAmount,
      );
      expect(await publicClient.getBalance({ address: mockLinethRollup.address })).to.equal(rebalanceAmount);
    });
    it("With _withdrawableValue min, _withdrawableValue <= YieldManager.balance, will send _withdrawableValue from YieldManager to reserve", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      // Arrange - Setup YieldManager balance
      const yieldManagerBalance = ONE_ETHER * 2n;
      const yieldManagerAddress = yieldManager.address;
      await setBalance(yieldManagerAddress, yieldManagerBalance);
      const l1MessageServiceBalanceBefore = await getBalance(mockLinethRollup);

      // Act
      const rebalanceAmount = ONE_ETHER * 4n;
      await expectEvent(
        yieldManager,
        yieldManager.write.safeAddToWithdrawalReserve([mockYieldProviderAddress, rebalanceAmount], {
          account: nativeYieldOperator.account,
        }),
        "WithdrawalReserveAugmented",
        [mockYieldProviderAddress, yieldManagerBalance, yieldManagerBalance, 0n],
      );

      expect(await publicClient.getBalance({ address: yieldManagerAddress })).to.equal(0n);
      expect(await getBalance(mockLinethRollup)).to.equal(l1MessageServiceBalanceBefore + yieldManagerBalance);
    });
    it("With _amount min, _amount > YieldManager.balance, will withdraw from YieldProvider", async () => {
      const { mockYieldProvider, mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      // Arrange - Setup withdrawableValue = 3 ETH
      await fundYieldProviderForWithdrawal(yieldManager, mockYieldProvider, nativeYieldOperator, ONE_ETHER * 3n);
      await yieldManager.write.setWithdrawableValueReturnVal([mockYieldProviderAddress, ONE_ETHER * 3n]);

      // Setup YieldManager balance = 1 ETH
      const yieldManagerBalance = ONE_ETHER * 1n;
      const yieldManagerAddress = yieldManager.address;
      await setBalance(yieldManagerAddress, yieldManagerBalance);

      const l1MessageServiceBalanceBefore = await getBalance(mockLinethRollup);

      // Act
      const rebalanceAmount = ONE_ETHER * 2n;
      await expectEvent(
        yieldManager,
        yieldManager.write.safeAddToWithdrawalReserve([mockYieldProviderAddress, rebalanceAmount], {
          account: nativeYieldOperator.account,
        }),
        "WithdrawalReserveAugmented",
        [mockYieldProviderAddress, rebalanceAmount, yieldManagerBalance, rebalanceAmount - yieldManagerBalance],
      );

      expect(await publicClient.getBalance({ address: yieldManagerAddress })).to.equal(0n);
      expect(await publicClient.getBalance({ address: mockLinethRollup.address })).to.equal(
        l1MessageServiceBalanceBefore + rebalanceAmount,
      );
    });
    it("With _withdrawableValue min, _withdrawableValue > YieldManager.balance, will withdraw from YieldProvider", async () => {
      const { mockYieldProvider, mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      // Arrange - Setup withdrawableValue = 3 ETH
      const withdrawableValue = ONE_ETHER * 3n;
      await fundYieldProviderForWithdrawal(yieldManager, mockYieldProvider, nativeYieldOperator, withdrawableValue);
      await yieldManager.write.setWithdrawableValueReturnVal([mockYieldProviderAddress, withdrawableValue]);

      // Setup YieldManager balance = 1 ETH
      const yieldManagerBalance = ONE_ETHER * 1n;
      const yieldManagerAddress = yieldManager.address;
      await setBalance(yieldManagerAddress, yieldManagerBalance);

      const l1MessageServiceBalanceBefore = await getBalance(mockLinethRollup);

      // Act
      const rebalanceAmount = ONE_ETHER * 5n;
      await expectEvent(
        yieldManager,
        yieldManager.write.safeAddToWithdrawalReserve([mockYieldProviderAddress, rebalanceAmount], {
          account: nativeYieldOperator.account,
        }),
        "WithdrawalReserveAugmented",
        [mockYieldProviderAddress, withdrawableValue + yieldManagerBalance, yieldManagerBalance, withdrawableValue],
      );

      expect(await publicClient.getBalance({ address: yieldManagerAddress })).to.equal(0n);
      expect(await publicClient.getBalance({ address: mockLinethRollup.address })).to.equal(
        l1MessageServiceBalanceBefore + withdrawableValue + yieldManagerBalance,
      );
    });
    it("With YieldManager balance > _amount and targetDeficit > _amount, will send _amount from YieldManager to reserve and pause staking", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      const rebalanceAmount = ONE_ETHER;
      const yieldManagerAddress = yieldManager.address;
      await setBalance(yieldManagerAddress, rebalanceAmount * 2n);
      await expectEvent(
        yieldManager,
        yieldManager.write.safeAddToWithdrawalReserve([mockYieldProviderAddress, rebalanceAmount], {
          account: nativeYieldOperator.account,
        }),
        "WithdrawalReserveAugmented",
        [mockYieldProviderAddress, rebalanceAmount, rebalanceAmount, 0n],
      );

      expect(await publicClient.getBalance({ address: yieldManagerAddress })).to.equal(rebalanceAmount);
      expect(await publicClient.getBalance({ address: mockLinethRollup.address })).to.equal(rebalanceAmount);
      expect(await yieldManager.read.isStakingPaused([mockYieldProviderAddress])).eq(true);
    });
    it("With YieldManager balance > _amount and targetDeficit < _amount, will send _amount from YieldManager to reserve and not pause staking", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      const rebalanceAmount = ONE_ETHER;
      const yieldManagerAddress = yieldManager.address;
      await setBalance(yieldManagerAddress, rebalanceAmount * 2n);
      await setWithdrawalReserveToTarget(yieldManager);
      const l1MessageServiceBalanceBefore = await getBalance(mockLinethRollup);
      await expectEvent(
        yieldManager,
        yieldManager.write.safeAddToWithdrawalReserve([mockYieldProviderAddress, rebalanceAmount], {
          account: nativeYieldOperator.account,
        }),
        "WithdrawalReserveAugmented",
        [mockYieldProviderAddress, rebalanceAmount, rebalanceAmount, 0n],
      );

      expect(await publicClient.getBalance({ address: yieldManagerAddress })).to.equal(rebalanceAmount);
      expect(await publicClient.getBalance({ address: mockLinethRollup.address })).to.equal(
        l1MessageServiceBalanceBefore + rebalanceAmount,
      );
      expect(await yieldManager.read.isStakingPaused([mockYieldProviderAddress])).eq(false);
    });
    it("With YieldManager balance < _amount and targetDeficit > _amount, should withdraw from YieldProvider to the reserve and pause staking", async () => {
      const rebalanceAmount = ONE_ETHER * 2n;
      // Arrange - setup remainder of rebalanceAmount on YieldProvider
      const { mockYieldProviderAddress, mockYieldProvider, mockWithdrawTarget } =
        await addMockYieldProvider(yieldManager);
      await fundYieldProviderForWithdrawal(yieldManager, mockYieldProvider, nativeYieldOperator, rebalanceAmount / 2n);

      // Arrange - setup insufficient YieldManager balance
      const yieldManagerAddress = yieldManager.address;
      await incrementBalance(yieldManagerAddress, rebalanceAmount / 2n);
      // Arrange - Get before
      await setWithdrawalReserveToMinimum(yieldManager);
      const l1MessageServiceBalanceBefore = await getBalance(mockLinethRollup);

      // Act
      await expectEvent(
        yieldManager,
        yieldManager.write.safeAddToWithdrawalReserve([mockYieldProviderAddress, rebalanceAmount], {
          account: nativeYieldOperator.account,
        }),
        "WithdrawalReserveAugmented",
        [mockYieldProviderAddress, rebalanceAmount, rebalanceAmount / 2n, rebalanceAmount / 2n],
      );

      expect(await getBalance(mockLinethRollup)).to.equal(l1MessageServiceBalanceBefore + rebalanceAmount);
      expect(await publicClient.getBalance({ address: yieldManager.address })).to.equal(0n);
      expect(await publicClient.getBalance({ address: mockWithdrawTarget.address })).to.equal(0n);
      expect(await yieldManager.read.isStakingPaused([mockYieldProviderAddress])).eq(true);
    });
    it("With YieldManager balance < _amount and targetDeficit < _amount, should withdraw from YieldProvider to the reserve and not pause staking", async () => {
      const rebalanceAmount = ONE_ETHER * 2n;
      // Arrange - setup remainder of rebalanceAmount on YieldProvider
      const { mockYieldProviderAddress, mockYieldProvider, mockWithdrawTarget } =
        await addMockYieldProvider(yieldManager);
      await fundYieldProviderForWithdrawal(yieldManager, mockYieldProvider, nativeYieldOperator, rebalanceAmount / 2n);
      // Arrange - setup insufficient YieldManager balance
      const yieldManagerAddress = yieldManager.address;
      await incrementBalance(yieldManagerAddress, rebalanceAmount / 2n);
      // Arrange - Get before
      await setWithdrawalReserveToTarget(yieldManager);
      const l1MessageServiceBalanceBefore = await getBalance(mockLinethRollup);

      // Act
      await expectEvent(
        yieldManager,
        yieldManager.write.safeAddToWithdrawalReserve([mockYieldProviderAddress, rebalanceAmount], {
          account: nativeYieldOperator.account,
        }),
        "WithdrawalReserveAugmented",
        [mockYieldProviderAddress, rebalanceAmount, rebalanceAmount / 2n, rebalanceAmount / 2n],
      );

      expect(await getBalance(mockLinethRollup)).to.equal(l1MessageServiceBalanceBefore + rebalanceAmount);
      expect(await publicClient.getBalance({ address: yieldManager.address })).to.equal(0n);
      expect(await publicClient.getBalance({ address: mockWithdrawTarget.address })).to.equal(0n);
      expect(await yieldManager.read.isStakingPaused([mockYieldProviderAddress])).eq(false);
    });
  });

  describe("permissionlessly replenishing the withdrawal reserve", () => {
    it("Should revert when the GENERAL pause type is activated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.write.pauseByType([GENERAL_PAUSE_TYPE], { account: securityCouncil.account });

      await expectRevertWhenPaused(
        yieldManager,
        yieldManager.write.replenishWithdrawalReserve([mockYieldProviderAddress], {
          account: nativeYieldOperator.account,
        }),
        GENERAL_PAUSE_TYPE,
      );
    });

    it("Should revert when the NATIVE_YIELD_PERMISSIONLESS_ACTIONS pause type is activated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.write.pauseByType([NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE], {
        account: securityCouncil.account,
      });

      await expectRevertWhenPaused(
        yieldManager,
        yieldManager.write.replenishWithdrawalReserve([mockYieldProviderAddress], {
          account: nativeYieldOperator.account,
        }),
        NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE,
      );
    });

    it("Should revert when rebalancing from an unknown YieldProvider", async () => {
      const unknownYieldProvider = randomAddress();

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.replenishWithdrawalReserve([unknownYieldProvider], { account: nativeYieldOperator.account }),
        "UnknownYieldProvider",
      );
    });

    it("Should revert when there is no reserve deficit", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      const l1MessageService = mockLinethRollup.address;
      const minimumReserve = await yieldManager.read.getEffectiveMinimumWithdrawalReserve();

      await setBalance(l1MessageService, minimumReserve);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.replenishWithdrawalReserve([mockYieldProviderAddress], {
          account: nativeYieldOperator.account,
        }),
        "WithdrawalReserveNotInDeficit",
      );
    });

    it("Should revert if there is 0 available balance on YieldManager and YieldProvider", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      const l1MessageService = mockLinethRollup.address;
      const yieldManagerAddress = yieldManager.address;

      // Arrange balances
      await setBalance(l1MessageService, 0n);
      await setBalance(yieldManagerAddress, 0n);
      await setBalance(mockYieldProviderAddress, 0n);

      // Act
      const call = yieldManager.write.replenishWithdrawalReserve([mockYieldProviderAddress], {
        account: nativeYieldOperator.account,
      });

      // Assert
      await expectRevertWithCustomError(yieldManager, call, "NoAvailableFundsToReplenishWithdrawalReserve");
    });

    it("If YieldManager balance > targetDeficit, settle targetDeficit from YieldManager balance", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      const l1MessageService = mockLinethRollup.address;
      const yieldManagerAddress = yieldManager.address;

      // Arrange - L1MessageService balance = 0
      const beforeL1MessageServiceBalance = 0n;
      await setBalance(l1MessageService, beforeL1MessageServiceBalance);
      // Arrange - YieldManager balance
      const targetDeficit = await yieldManager.read.getTargetReserveDeficit();
      const beforeYieldManagerBalance = targetDeficit + ONE_ETHER;
      await setBalance(yieldManagerAddress, beforeYieldManagerBalance);

      await expectEvent(
        yieldManager,
        yieldManager.write.replenishWithdrawalReserve([mockYieldProviderAddress], {
          account: nativeYieldOperator.account,
        }),
        "WithdrawalReserveReplenished",
        [mockYieldProviderAddress, targetDeficit, targetDeficit, targetDeficit, 0n],
      );
      expect(await yieldManager.read.isStakingPaused([mockYieldProviderAddress])).to.be.false;
      expect(await publicClient.getBalance({ address: l1MessageService })).eq(
        beforeL1MessageServiceBalance + targetDeficit,
      );
      expect(await publicClient.getBalance({ address: yieldManagerAddress })).eq(
        beforeYieldManagerBalance - targetDeficit,
      );
    });

    it("If YieldManager balance < targetDeficit, settle remainder by withdrawing available value from YieldProvider to reserve", async () => {
      const targetReserveAmount = await yieldManager.read.getTargetWithdrawalReserveAmount();
      // Arrange - put half of target reserve amount on YieldProvider
      const { mockYieldProviderAddress, mockYieldProvider, mockWithdrawTarget } =
        await addMockYieldProvider(yieldManager);
      await fundYieldProviderForWithdrawal(
        yieldManager,
        mockYieldProvider,
        nativeYieldOperator,
        targetReserveAmount / 2n,
      );
      const mockWithdrawTargetAddress = mockWithdrawTarget.address;
      const beforeMockWithdrawTargetBalance = await publicClient.getBalance({ address: mockWithdrawTargetAddress });
      // Arrange - put other half on YieldManager
      const yieldManagerAddress = yieldManager.address;
      await setBalance(yieldManagerAddress, targetReserveAmount / 2n);
      const beforeYieldManagerBalance = await publicClient.getBalance({ address: yieldManagerAddress });
      // Arrange - 0 balance on L1MessageService
      const l1MessageService = mockLinethRollup.address;
      await setBalance(l1MessageService, 0n);
      expect(await yieldManager.read.getTargetReserveDeficit()).eq(targetReserveAmount);
      const beforeL1MessageServiceBalance = await publicClient.getBalance({ address: l1MessageService });

      // Act
      await expectEvent(
        yieldManager,
        yieldManager.write.replenishWithdrawalReserve([mockYieldProviderAddress], {
          account: nativeYieldOperator.account,
        }),
        "WithdrawalReserveReplenished",
        [
          mockYieldProviderAddress,
          targetReserveAmount,
          targetReserveAmount,
          targetReserveAmount / 2n,
          targetReserveAmount / 2n,
        ],
      );

      expect(await yieldManager.read.isStakingPaused([mockYieldProviderAddress])).to.be.false;
      expect(await publicClient.getBalance({ address: l1MessageService })).eq(
        beforeL1MessageServiceBalance + targetReserveAmount,
      );
      expect(await publicClient.getBalance({ address: yieldManagerAddress })).eq(
        beforeYieldManagerBalance - targetReserveAmount / 2n,
      );
      expect(await publicClient.getBalance({ address: mockWithdrawTargetAddress })).eq(
        beforeMockWithdrawTargetBalance - targetReserveAmount / 2n,
      );
    });

    it("If YieldManager + YieldProvider balance > targetDeficit, should only rebalance required to restore targetDeficit", async () => {
      const targetReserveAmount = await yieldManager.read.getTargetWithdrawalReserveAmount();
      // Arrange - put target reserve amount on YieldProvider
      const { mockYieldProviderAddress, mockYieldProvider, mockWithdrawTarget } =
        await addMockYieldProvider(yieldManager);
      await fundYieldProviderForWithdrawal(yieldManager, mockYieldProvider, nativeYieldOperator, targetReserveAmount);
      const mockWithdrawTargetAddress = mockWithdrawTarget.address;
      const beforeMockWithdrawTargetBalance = await publicClient.getBalance({ address: mockWithdrawTargetAddress });
      // Arrange - put half of target reserve amount on YieldManager
      const yieldManagerAddress = yieldManager.address;
      await setBalance(yieldManagerAddress, targetReserveAmount / 2n);
      const beforeYieldManagerBalance = await publicClient.getBalance({ address: yieldManagerAddress });
      // Arrange - 0 balance on L1MessageService
      const l1MessageService = mockLinethRollup.address;
      await setBalance(l1MessageService, 0n);
      expect(await yieldManager.read.getTargetReserveDeficit()).eq(targetReserveAmount);
      const beforeL1MessageServiceBalance = await publicClient.getBalance({ address: l1MessageService });

      // Act
      await expectEvent(
        yieldManager,
        yieldManager.write.replenishWithdrawalReserve([mockYieldProviderAddress], {
          account: nativeYieldOperator.account,
        }),
        "WithdrawalReserveReplenished",
        [
          mockYieldProviderAddress,
          targetReserveAmount,
          targetReserveAmount,
          targetReserveAmount / 2n,
          targetReserveAmount / 2n,
        ],
      );

      expect(await yieldManager.read.isStakingPaused([mockYieldProviderAddress])).to.be.false;
      expect(await publicClient.getBalance({ address: l1MessageService })).eq(
        beforeL1MessageServiceBalance + targetReserveAmount,
      );
      expect(await publicClient.getBalance({ address: yieldManagerAddress })).eq(
        beforeYieldManagerBalance - targetReserveAmount / 2n,
      );
      expect(await publicClient.getBalance({ address: mockWithdrawTargetAddress })).eq(
        beforeMockWithdrawTargetBalance - targetReserveAmount / 2n,
      );
    });

    it("If there is remaining target deficit, should pause staking", async () => {
      const targetReserveAmount = await yieldManager.read.getTargetWithdrawalReserveAmount();
      // Arrange - put half target reserve amount on YieldProvider
      const { mockYieldProviderAddress, mockYieldProvider, mockWithdrawTarget } =
        await addMockYieldProvider(yieldManager);
      await fundYieldProviderForWithdrawal(
        yieldManager,
        mockYieldProvider,
        nativeYieldOperator,
        targetReserveAmount / 2n,
      );
      const mockWithdrawTargetAddress = mockWithdrawTarget.address;
      const beforeMockWithdrawTargetBalance = await publicClient.getBalance({ address: mockWithdrawTargetAddress });
      // Arrange - put quarter of target reserve amount on YieldManager
      const yieldManagerAddress = yieldManager.address;
      await setBalance(yieldManagerAddress, targetReserveAmount / 4n);
      const beforeYieldManagerBalance = await publicClient.getBalance({ address: yieldManagerAddress });
      // Arrange - 0 balance on L1MessageService
      const l1MessageService = mockLinethRollup.address;
      await setBalance(l1MessageService, 0n);
      expect(await yieldManager.read.getTargetReserveDeficit()).eq(targetReserveAmount);
      const beforeL1MessageServiceBalance = await publicClient.getBalance({ address: l1MessageService });

      // Act
      await expectEvent(
        yieldManager,
        yieldManager.write.replenishWithdrawalReserve([mockYieldProviderAddress], {
          account: nativeYieldOperator.account,
        }),
        "WithdrawalReserveReplenished",
        [
          mockYieldProviderAddress,
          targetReserveAmount,
          (targetReserveAmount * 3n) / 4n,
          targetReserveAmount / 4n,
          targetReserveAmount / 2n,
        ],
      );

      expect(await yieldManager.read.isStakingPaused([mockYieldProviderAddress])).to.be.true;
      expect(await publicClient.getBalance({ address: l1MessageService })).eq(
        beforeL1MessageServiceBalance + (targetReserveAmount * 3n) / 4n,
      );
      expect(await publicClient.getBalance({ address: yieldManagerAddress })).eq(
        beforeYieldManagerBalance - targetReserveAmount / 4n,
      );
      expect(await publicClient.getBalance({ address: mockWithdrawTargetAddress })).eq(
        beforeMockWithdrawTargetBalance - targetReserveAmount / 2n,
      );
    });
  });

  describe("withdraw LST", () => {
    it("Should revert when the GENERAL pause type is activated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.write.pauseByType([GENERAL_PAUSE_TYPE], { account: securityCouncil.account });

      await expectRevertWhenPaused(
        yieldManager,
        yieldManager.write.withdrawLST([mockYieldProviderAddress, 0n, zeroAddress], {
          account: nativeYieldOperator.account,
        }),
        GENERAL_PAUSE_TYPE,
      );
    });

    it("Should revert when the NATIVE_YIELD_PERMISSIONLESS_ACTIONS pause type is activated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      await yieldManager.write.pauseByType([NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE], {
        account: securityCouncil.account,
      });

      await expectRevertWhenPaused(
        yieldManager,
        yieldManager.write.withdrawLST([mockYieldProviderAddress, 0n, zeroAddress], {
          account: nativeYieldOperator.account,
        }),
        NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE,
      );
    });

    it("Should revert when choosing an unknown YieldProvider", async () => {
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.withdrawLST([randomAddress(), 0n, zeroAddress], { account: nativeYieldOperator.account }),
        "UnknownYieldProvider",
      );
    });

    it("Should revert if the sender is not the L1MessageService", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.withdrawLST([mockYieldProviderAddress, 0n, zeroAddress], {
          account: nativeYieldOperator.account,
        }),
        "SenderNotL1MessageService",
      );
    });

    it("Should revert if L1MessageService does not have withdraw LST flag toggled", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      const l1MessageService = mockLinethRollup.address;

      await mockLinethRollup.write.setWithdrawLSTAllowed([false]);
      // Arrange - set gas funds for L1MessageService to be signer
      await setBalance(l1MessageService, ONE_ETHER);
      const l1Signer = await getImpersonatedAccount(l1MessageService);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.withdrawLST([mockYieldProviderAddress, 0n, zeroAddress], { account: l1Signer.account }),
        "LSTWithdrawalNotAllowed",
      );

      await mockLinethRollup.write.setWithdrawLSTAllowed([true]);
      await (await network.getOrCreate()).networkHelpers.stopImpersonatingAccount(l1MessageService);
    });

    it("Should revert if LST withdraw amount > userFunds for yield provider", async () => {
      const { mockYieldProviderAddress, mockYieldProvider } = await addMockYieldProvider(yieldManager);
      const l1MessageService = mockLinethRollup.address;
      const withdrawAmount = ONE_ETHER;

      await fundYieldProviderForWithdrawal(yieldManager, mockYieldProvider, nativeYieldOperator, withdrawAmount / 2n);

      // Arrange - set gas funds for L1MessageService to be signer
      await setBalance(l1MessageService, ONE_ETHER);
      const l1Signer = await getImpersonatedAccount(l1MessageService);
      await mockLinethRollup.write.setWithdrawLSTAllowed([true]);

      // Arrange Setup

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.withdrawLST([mockYieldProviderAddress, withdrawAmount, zeroAddress], {
          account: l1Signer.account,
        }),
        "LSTWithdrawalExceedsYieldProviderFunds",
      );

      await (await network.getOrCreate()).networkHelpers.stopImpersonatingAccount(l1MessageService);
    });

    it("Should revert if LST withdraw amount + lastReportedNegativeYield > userFunds for yield provider", async () => {
      const { mockYieldProviderAddress, mockYieldProvider } = await addMockYieldProvider(yieldManager);
      const l1MessageService = mockLinethRollup.address;
      const fundAmount = ONE_ETHER * 10n;
      await fundYieldProviderForWithdrawal(yieldManager, mockYieldProvider, nativeYieldOperator, fundAmount);
      // Arrange - Setup lstPrincipalAmount
      const withdrawAmount = fundAmount;
      const negativeYield = 1n;
      await yieldManager.write.setYieldProviderLastReportedNegativeYield([mockYieldProviderAddress, negativeYield]);
      // Arrange - set gas funds for L1MessageService to be signer
      await setBalance(l1MessageService, withdrawAmount);
      const l1Signer = await getImpersonatedAccount(l1MessageService);
      await mockLinethRollup.write.setWithdrawLSTAllowed([true]);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.withdrawLST([mockYieldProviderAddress, withdrawAmount, zeroAddress], {
          account: l1Signer.account,
        }),
        "LSTWithdrawalExceedsYieldProviderFunds",
      );

      await (await network.getOrCreate()).networkHelpers.stopImpersonatingAccount(l1MessageService);
    });

    it("Should successfully withdraw LST, pause staking and emit the expected event", async () => {
      const { mockYieldProviderAddress, mockYieldProvider } = await addMockYieldProvider(yieldManager);
      const l1MessageService = mockLinethRollup.address;
      const withdrawAmount = ONE_ETHER;
      const recipient = randomAddress();

      await fundYieldProviderForWithdrawal(yieldManager, mockYieldProvider, nativeYieldOperator, withdrawAmount);

      await setBalance(l1MessageService, ONE_ETHER);
      const l1Signer = await getImpersonatedAccount(l1MessageService);
      await mockLinethRollup.write.setWithdrawLSTAllowed([true]);

      // Arrange - Get before
      const userFundsInYieldProvidersTotalBefore = await yieldManager.read.userFundsInYieldProvidersTotal();
      const userFundsBefore = await yieldManager.read.userFunds([mockYieldProviderAddress]);
      const lstLiabilityPrincipalBefore = await yieldManager.read.getYieldProviderLstLiabilityPrincipal([
        mockYieldProviderAddress,
      ]);

      // Act
      await expectEvent(
        yieldManager,
        yieldManager.write.withdrawLST([mockYieldProviderAddress, withdrawAmount, recipient], {
          account: l1Signer.account,
        }),
        "LSTMinted",
        [mockYieldProviderAddress, recipient, withdrawAmount],
      );

      expect(await yieldManager.read.isStakingPaused([mockYieldProviderAddress])).to.be.true;
      expect(await yieldManager.read.userFundsInYieldProvidersTotal()).eq(
        userFundsInYieldProvidersTotalBefore - withdrawAmount,
      );
      expect(await yieldManager.read.userFunds([mockYieldProviderAddress])).eq(userFundsBefore - withdrawAmount);
      expect(await yieldManager.read.getYieldProviderLstLiabilityPrincipal([mockYieldProviderAddress])).eq(
        lstLiabilityPrincipalBefore + withdrawAmount,
      );
      await (await network.getOrCreate()).networkHelpers.stopImpersonatingAccount(l1MessageService);
    });
  });
});
