// Unit tests on functions handling ETH transfer

import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { ethers } from "hardhat";

import { MockLineaRollup, TestYieldManager } from "contracts/typechain-types";
import { deployYieldManagerForUnitTest } from "../helpers/deploy";
import { addMockYieldProvider } from "../helpers/mocks";
import {
  ONE_THOUSAND_ETHER,
  ONE_ETHER,
  GENERAL_PAUSE_TYPE,
  NATIVE_YIELD_STAKING_PAUSE_TYPE,
  NATIVE_YIELD_REPORTING_PAUSE_TYPE,
  NATIVE_YIELD_UNSTAKING_PAUSE_TYPE,
  NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE,
} from "../../common/constants";
import { buildAccessErrorMessage, expectRevertWithCustomError, getAccountsFixture } from "../../common/helpers";
import {
  buildSetWithdrawalReserveParams,
  fundYieldProviderForWithdrawal,
  getBalance,
  incrementBalance,
  setBalance,
  setWithdrawalReserveToMinimum,
  setWithdrawalReserveToTarget,
  YieldManagerInitializationData,
} from "../helpers";

describe("YieldManager contract - ETH transfer operations", () => {
  let yieldManager: TestYieldManager;

  let securityCouncil: SignerWithAddress;
  let nonAuthorizedAccount: SignerWithAddress;
  let nativeYieldOperator: SignerWithAddress;
  let l2YieldRecipient: SignerWithAddress;
  let mockLineaRollup: MockLineaRollup;
  let initializationData: YieldManagerInitializationData;

  const mockWithdrawalParams = ethers.hexlify(ethers.randomBytes(8));
  const mockWithdrawalParamsProof = ethers.hexlify(ethers.randomBytes(8));
  const mockValidatorIndex = 0n;
  const mockSlot = 100000n; // Must be > lastProvenSlot + SLOTS_PER_HISTORICAL_ROOT (8192)

  before(async () => {
    ({
      securityCouncil,
      operator: nonAuthorizedAccount,
      nativeYieldOperator,
      l2YieldRecipient,
    } = await loadFixture(getAccountsFixture));
  });

  beforeEach(async () => {
    ({ yieldManager, mockLineaRollup, initializationData } = await loadFixture(deployYieldManagerForUnitTest));
  });

  describe("receiving ETH from the L1MessageService", () => {
    it("Should revert when the caller is not the L1MessageService", async () => {
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).receiveFundsFromReserve({ value: 1n }),
        "SenderNotL1MessageService",
      );
    });

    it("Should revert when the withdrawal reserve is below minimum", async () => {
      const l1MessageService = await mockLineaRollup.getAddress();
      const minimumEffectiveBalance = await yieldManager.getEffectiveMinimumWithdrawalReserve();
      const yieldManagerAddress = await yieldManager.getAddress();

      await ethers.provider.send("hardhat_setBalance", [l1MessageService, ethers.toBeHex(minimumEffectiveBalance)]);

      // Act - Use helper function on MockLineaRollup
      await expectRevertWithCustomError(
        yieldManager,
        mockLineaRollup.connect(nativeYieldOperator).callReceiveFundsFromReserve(yieldManagerAddress, 1n),
        "InsufficientWithdrawalReserve",
      );
    });

    it("Should successfully receive funds and emit the expected event", async () => {
      const l1MessageService = await mockLineaRollup.getAddress();
      const minimumEffectiveBalance = await yieldManager.getEffectiveMinimumWithdrawalReserve();
      const yieldManagerAddress = await yieldManager.getAddress();

      await ethers.provider.send("hardhat_setBalance", [
        l1MessageService,
        ethers.toBeHex(minimumEffectiveBalance + 1n),
      ]);

      await expect(mockLineaRollup.connect(nativeYieldOperator).callReceiveFundsFromReserve(yieldManagerAddress, 1n))
        .to.emit(yieldManager, "ReserveFundsReceived")
        .withArgs(1n);
    });
  });

  describe("sending ETH to the L1MessageService", () => {
    it("Should revert when the caller is not the YIELD_PROVIDER_UNSTAKER_ROLE", async () => {
      await expect(yieldManager.connect(nonAuthorizedAccount).transferFundsToReserve(1n)).to.be.revertedWith(
        buildAccessErrorMessage(nonAuthorizedAccount, await yieldManager.YIELD_PROVIDER_UNSTAKER_ROLE()),
      );
    });

    it("Should revert when the NATIVE_YIELD_UNSTAKING pause type is activated", async () => {
      await yieldManager.connect(securityCouncil).pauseByType(NATIVE_YIELD_UNSTAKING_PAUSE_TYPE);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).transferFundsToReserve(1n),
        "IsPaused",
        [NATIVE_YIELD_UNSTAKING_PAUSE_TYPE],
      );
    });

    it("Should revert when the GENERAL pause type is activated", async () => {
      await yieldManager.connect(securityCouncil).pauseByType(GENERAL_PAUSE_TYPE);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).transferFundsToReserve(1n),
        "IsPaused",
        [GENERAL_PAUSE_TYPE],
      );
    });

    it("Should successfully send ETH to the L1MessageService", async () => {
      const yieldManagerAddress = await yieldManager.getAddress();
      await ethers.provider.send("hardhat_setBalance", [yieldManagerAddress, ethers.toBeHex(ONE_THOUSAND_ETHER)]);
      const transferAmount = 1_000_000_000_000_000_000n;
      const l1MessageService = await mockLineaRollup.getAddress();

      const reserveBalanceBefore = await ethers.provider.getBalance(l1MessageService);
      const yieldManagerBalanceBefore = await ethers.provider.getBalance(yieldManagerAddress);

      await yieldManager.connect(nativeYieldOperator).transferFundsToReserve(transferAmount);

      const reserveBalanceAfter = await ethers.provider.getBalance(l1MessageService);
      const yieldManagerBalanceAfter = await ethers.provider.getBalance(yieldManagerAddress);

      expect(reserveBalanceAfter).to.equal(reserveBalanceBefore + transferAmount);
      expect(yieldManagerBalanceAfter).to.equal(yieldManagerBalanceBefore - transferAmount);
    });
  });

  describe("sending ETH to a YieldProvider", () => {
    it("Should revert when the caller is not the YIELD_PROVIDER_STAKING_ROLE", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await expect(
        yieldManager.connect(nonAuthorizedAccount).fundYieldProvider(mockYieldProviderAddress, 1n),
      ).to.be.revertedWith(
        buildAccessErrorMessage(nonAuthorizedAccount, await yieldManager.YIELD_PROVIDER_STAKING_ROLE()),
      );
    });

    it("Should revert when the NATIVE_YIELD_STAKING pause type is activated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.connect(securityCouncil).pauseByType(NATIVE_YIELD_STAKING_PAUSE_TYPE);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).fundYieldProvider(mockYieldProviderAddress, 1n),
        "IsPaused",
        [NATIVE_YIELD_STAKING_PAUSE_TYPE],
      );
    });

    it("Should revert when the GENERAL pause type is activated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.connect(securityCouncil).pauseByType(GENERAL_PAUSE_TYPE);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).fundYieldProvider(mockYieldProviderAddress, 1n),
        "IsPaused",
        [GENERAL_PAUSE_TYPE],
      );
    });

    it("Should revert when sending to an unknown YieldProvider", async () => {
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).fundYieldProvider(ethers.Wallet.createRandom().address, 1n),
        "UnknownYieldProvider",
      );
    });

    it("Should revert when the withdrawal reserve is in deficit", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).fundYieldProvider(mockYieldProviderAddress, 1n),
        "InsufficientWithdrawalReserve",
      );
    });

    it("should successfully send ETH to the YieldProvider, update state and emit the expected event", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      const l1MessageService = await mockLineaRollup.getAddress();
      const yieldManagerAddress = await yieldManager.getAddress();

      const minimumReserveAmount = await yieldManager.getMinimumWithdrawalReserveAmount();
      await ethers.provider.send("hardhat_setBalance", [l1MessageService, ethers.toBeHex(minimumReserveAmount)]);
      const transferAmount = 40n;
      await ethers.provider.send("hardhat_setBalance", [yieldManagerAddress, ethers.toBeHex(transferAmount)]);

      await expect(
        yieldManager.connect(nativeYieldOperator).fundYieldProvider(mockYieldProviderAddress, transferAmount),
      )
        .to.emit(yieldManager, "YieldProviderFunded")
        .withArgs(mockYieldProviderAddress, transferAmount);

      const yieldProviderData = await yieldManager.getYieldProviderData(mockYieldProviderAddress);
      expect(yieldProviderData.userFunds).to.equal(transferAmount);

      expect(await yieldManager.isStakingPaused(mockYieldProviderAddress)).to.be.true;
      expect(await yieldManager.userFundsInYieldProvidersTotal()).to.equal(transferAmount);
    });
  });

  describe("reporting yield", () => {
    it("Should revert when the caller is not the YIELD_REPORTER_ROLE", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await expect(
        yieldManager.connect(nonAuthorizedAccount).reportYield(mockYieldProviderAddress, l2YieldRecipient.address),
      ).to.be.revertedWith(buildAccessErrorMessage(nonAuthorizedAccount, await yieldManager.YIELD_REPORTER_ROLE()));
    });

    it("Should revert when the NATIVE_YIELD_REPORTING pause type is activated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.connect(securityCouncil).pauseByType(NATIVE_YIELD_REPORTING_PAUSE_TYPE);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).reportYield(mockYieldProviderAddress, l2YieldRecipient.address),
        "IsPaused",
        [NATIVE_YIELD_REPORTING_PAUSE_TYPE],
      );
    });

    it("Should revert when the GENERAL pause type is activated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.connect(securityCouncil).pauseByType(GENERAL_PAUSE_TYPE);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).reportYield(mockYieldProviderAddress, l2YieldRecipient.address),
        "IsPaused",
        [GENERAL_PAUSE_TYPE],
      );
    });

    it("Should revert when reporting for an unknown YieldProvider", async () => {
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager
          .connect(nativeYieldOperator)
          .reportYield(ethers.Wallet.createRandom().address, l2YieldRecipient.address),
        "UnknownYieldProvider",
      );
    });

    it("Should revert when distributing yield to an unknown L2YieldRecipient", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      const unknownRecipient = ethers.Wallet.createRandom().address;

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).reportYield(mockYieldProviderAddress, unknownRecipient),
        "UnknownL2YieldRecipient",
      );
    });

    it("Should successfully report positive yield, update state and emit the expected event", async () => {
      // ARRANGE
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      const reportedYield = ONE_ETHER;
      const outstandingNegativeYield = 0n;

      await yieldManager
        .connect(nativeYieldOperator)
        .setReportYieldReturnVal_NewReportedYield(mockYieldProviderAddress, reportedYield);
      await yieldManager
        .connect(nativeYieldOperator)
        .setReportYieldReturnVal_OutstandingNegativeYield(mockYieldProviderAddress, outstandingNegativeYield);

      // ACT + ASSERT
      await expect(
        yieldManager.connect(nativeYieldOperator).reportYield(mockYieldProviderAddress, l2YieldRecipient.address),
      )
        .to.emit(yieldManager, "NativeYieldReported")
        .withArgs(mockYieldProviderAddress, l2YieldRecipient.address, reportedYield, outstandingNegativeYield);

      const providerData = await yieldManager.getYieldProviderData(mockYieldProviderAddress);
      expect(providerData.userFunds).to.equal(reportedYield);
      expect(providerData.yieldReportedCumulative).to.equal(reportedYield);
      expect(await yieldManager.userFunds(mockYieldProviderAddress)).to.equal(reportedYield);
      expect(await yieldManager.userFundsInYieldProvidersTotal()).to.equal(reportedYield);
      expect(await yieldManager.getYieldProviderLastReportedNegativeYield(mockYieldProviderAddress)).to.equal(
        outstandingNegativeYield,
      );
    });

    it("Should successfully report negative yield, update state and emit the expected event", async () => {
      // ARRANGE
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      const reportedYield = 0n;
      const outstandingNegativeYield = ONE_ETHER * 2n;

      await yieldManager
        .connect(nativeYieldOperator)
        .setReportYieldReturnVal_NewReportedYield(mockYieldProviderAddress, reportedYield);
      await yieldManager
        .connect(nativeYieldOperator)
        .setReportYieldReturnVal_OutstandingNegativeYield(mockYieldProviderAddress, outstandingNegativeYield);

      // ACT + ASSERT
      await expect(
        yieldManager.connect(nativeYieldOperator).reportYield(mockYieldProviderAddress, l2YieldRecipient.address),
      )
        .to.emit(yieldManager, "NativeYieldReported")
        .withArgs(mockYieldProviderAddress, l2YieldRecipient.address, reportedYield, outstandingNegativeYield);

      const providerData = await yieldManager.getYieldProviderData(mockYieldProviderAddress);
      expect(providerData.userFunds).to.equal(reportedYield);
      expect(providerData.yieldReportedCumulative).to.equal(reportedYield);
      expect(await yieldManager.userFunds(mockYieldProviderAddress)).to.equal(reportedYield);
      expect(await yieldManager.userFundsInYieldProvidersTotal()).to.equal(reportedYield);
      expect(await yieldManager.getYieldProviderLastReportedNegativeYield(mockYieldProviderAddress)).to.equal(
        outstandingNegativeYield,
      );
    });
  });

  describe("permissioned unstake", () => {
    it("Should revert when the caller is not the YIELD_PROVIDER_UNSTAKER_ROLE", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await expect(
        yieldManager.connect(nonAuthorizedAccount).unstake(mockYieldProviderAddress, mockWithdrawalParams),
      ).to.be.revertedWith(
        buildAccessErrorMessage(nonAuthorizedAccount, await yieldManager.YIELD_PROVIDER_UNSTAKER_ROLE()),
      );
    });

    it("Should revert when the GENERAL pause type is activated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      await yieldManager.connect(securityCouncil).pauseByType(GENERAL_PAUSE_TYPE);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).unstake(mockYieldProviderAddress, mockWithdrawalParams),
        "IsPaused",
        [GENERAL_PAUSE_TYPE],
      );
    });

    it("Should revert when the NATIVE_YIELD_UNSTAKING pause type is activated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      await yieldManager.connect(securityCouncil).pauseByType(NATIVE_YIELD_UNSTAKING_PAUSE_TYPE);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).unstake(mockYieldProviderAddress, mockWithdrawalParams),
        "IsPaused",
        [NATIVE_YIELD_UNSTAKING_PAUSE_TYPE],
      );
    });

    it("Should revert when unstaking from an unknown YieldProvider", async () => {
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).unstake(ethers.Wallet.createRandom().address, mockWithdrawalParams),
        "UnknownYieldProvider",
      );
    });

    it("Should successfully unstake from a YieldProvider", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await expect(yieldManager.connect(nativeYieldOperator).unstake(mockYieldProviderAddress, mockWithdrawalParams)).to
        .not.be.reverted;
    });
  });

  describe("permissionless unstake", () => {
    it("Should revert when the GENERAL pause type is activated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      await yieldManager.connect(securityCouncil).pauseByType(GENERAL_PAUSE_TYPE);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager
          .connect(nativeYieldOperator)
          .unstakePermissionless(
            mockYieldProviderAddress,
            mockValidatorIndex,
            mockSlot,
            mockWithdrawalParams,
            mockWithdrawalParamsProof,
          ),
        "IsPaused",
        [GENERAL_PAUSE_TYPE],
      );
    });

    it("Should revert when the NATIVE_YIELD_PERMISSIONLESS_ACTIONS pause type is activated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      await yieldManager.connect(securityCouncil).pauseByType(NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager
          .connect(nativeYieldOperator)
          .unstakePermissionless(
            mockYieldProviderAddress,
            mockValidatorIndex,
            mockSlot,
            mockWithdrawalParams,
            mockWithdrawalParamsProof,
          ),
        "IsPaused",
        [NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE],
      );
    });

    it("Should revert when unstaking from an unknown YieldProvider", async () => {
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager
          .connect(nativeYieldOperator)
          .unstakePermissionless(
            ethers.Wallet.createRandom().address,
            mockValidatorIndex,
            mockSlot,
            mockWithdrawalParams,
            mockWithdrawalParamsProof,
          ),
        "UnknownYieldProvider",
      );
    });

    it("Should revert when the withdrawal reserve is not in deficit", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      const minimumReserveAmount = await yieldManager.getMinimumWithdrawalReserveAmount();
      await ethers.provider.send("hardhat_setBalance", [
        await mockLineaRollup.getAddress(),
        ethers.toBeHex(minimumReserveAmount),
      ]);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager
          .connect(nativeYieldOperator)
          .unstakePermissionless(
            mockYieldProviderAddress,
            mockValidatorIndex,
            mockSlot,
            mockWithdrawalParams,
            mockWithdrawalParamsProof,
          ),
        "WithdrawalReserveNotInDeficit",
      );
    });

    it("Should ignore msg.value for withdrawal reserve deficit check", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      // Arrange - %-based minimum at 20%
      await yieldManager
        .connect(securityCouncil)
        .setWithdrawalReserveParameters(buildSetWithdrawalReserveParams(initializationData, { minAmount: 0n }));
      await setBalance(await mockLineaRollup.getAddress(), 20n * ONE_ETHER);
      await setBalance(await yieldManager.getAddress(), 80n * ONE_ETHER);

      const msgValue = ONE_ETHER * 100n;
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager
          .connect(nativeYieldOperator)
          .unstakePermissionless(
            mockYieldProviderAddress,
            mockValidatorIndex,
            mockSlot,
            mockWithdrawalParams,
            mockWithdrawalParamsProof,
            { value: msgValue },
          ),
        "WithdrawalReserveNotInDeficit",
      );
    });

    it("Should revert when the slot is too close to the last proven slot for the validator", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      const SLOTS_PER_HISTORICAL_ROOT = 8192n;

      // Arrange - Set up withdrawal reserve in deficit
      await ethers.provider.send("hardhat_setBalance", [await mockLineaRollup.getAddress(), ethers.toBeHex(0)]);

      // Arrange - Set up mocks for successful first call
      const unstakeAmount = ONE_ETHER;
      await yieldManager.setWithdrawableValueReturnVal(mockYieldProviderAddress, 0n);
      await yieldManager.setUnstakePermissionlessReturnVal(mockYieldProviderAddress, unstakeAmount);

      // Arrange - First call with slot = 100000, which sets lastProvenSlot[validatorIndex] = 100000
      const firstSlot = 100000n;
      await yieldManager
        .connect(nativeYieldOperator)
        .unstakePermissionless(
          mockYieldProviderAddress,
          mockValidatorIndex,
          firstSlot,
          mockWithdrawalParams,
          mockWithdrawalParamsProof,
        );

      // Arrange - Try to call again with slot that's too close (<= lastProvenSlot + SLOTS_PER_HISTORICAL_ROOT)
      // lastProvenSlot = 100000, so slot must be > 100000 + 8192 = 108192
      // Using slot = 108192 should revert (boundary case)
      const tooCloseSlot = firstSlot + SLOTS_PER_HISTORICAL_ROOT;

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager
          .connect(nativeYieldOperator)
          .unstakePermissionless(
            mockYieldProviderAddress,
            mockValidatorIndex,
            tooCloseSlot,
            mockWithdrawalParams,
            mockWithdrawalParamsProof,
          ),
        "SlotTooCloseToLastProvenSlot",
        [mockValidatorIndex, firstSlot, tooCloseSlot],
      );
    });

    it("Should revert when requiredUnstakeAmountWei is 0", async () => {
      // Arrange - Put targetDeficit on YieldProvider
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      const targetReserveAmount = await yieldManager.getTargetWithdrawalReserveAmount();
      await setBalance(await mockLineaRollup.getAddress(), 0n);
      await setBalance(await yieldManager.getAddress(), 10n + targetReserveAmount / 3n);
      await yieldManager.setWithdrawableValueReturnVal(mockYieldProviderAddress, targetReserveAmount / 3n);
      await yieldManager.setYieldProviderUserFunds(mockYieldProviderAddress, targetReserveAmount / 3n);
      await yieldManager.setPendingPermissionlessUnstake(targetReserveAmount / 3n);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager
          .connect(nativeYieldOperator)
          .unstakePermissionless(
            mockYieldProviderAddress,
            mockValidatorIndex,
            mockSlot,
            mockWithdrawalParams,
            mockWithdrawalParamsProof,
          ),
        "NoRequirementToUnstakePermissionless",
      );
    });

    it("Should revert when requiredUnstakeAmountWei is 0, ignoring msg.value", async () => {
      // Arrange - Put targetDeficit on YieldProvider
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      const targetReserveAmount = await yieldManager.getTargetWithdrawalReserveAmount();
      await setBalance(await mockLineaRollup.getAddress(), 0n);
      await setBalance(await yieldManager.getAddress(), 10n + targetReserveAmount / 3n);
      await yieldManager.setWithdrawableValueReturnVal(mockYieldProviderAddress, targetReserveAmount / 3n);
      await yieldManager.setYieldProviderUserFunds(mockYieldProviderAddress, targetReserveAmount / 3n);
      await yieldManager.setPendingPermissionlessUnstake(targetReserveAmount / 3n);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager
          .connect(nativeYieldOperator)
          .unstakePermissionless(
            mockYieldProviderAddress,
            mockValidatorIndex,
            mockSlot,
            mockWithdrawalParams,
            mockWithdrawalParamsProof,
            { value: ONE_ETHER * 8000n },
          ),
        "NoRequirementToUnstakePermissionless",
      );
    });

    it("Should revert when the YieldProvider returns 0 unstake amount", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      await yieldManager.connect(nativeYieldOperator).setUnstakePermissionlessReturnVal(mockYieldProviderAddress, 0n);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager
          .connect(nativeYieldOperator)
          .unstakePermissionless(
            mockYieldProviderAddress,
            mockValidatorIndex,
            mockSlot,
            mockWithdrawalParams,
            mockWithdrawalParamsProof,
          ),
        "YieldProviderReturnedZeroUnstakeAmount",
      );
    });

    it("Should revert when unstake amount > required unstake amount", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      const targetDeficit = await yieldManager.getTargetReserveDeficit();
      await yieldManager
        .connect(nativeYieldOperator)
        .setUnstakePermissionlessReturnVal(mockYieldProviderAddress, targetDeficit * 2n);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager
          .connect(nativeYieldOperator)
          .unstakePermissionless(
            mockYieldProviderAddress,
            mockValidatorIndex,
            mockSlot,
            mockWithdrawalParams,
            mockWithdrawalParamsProof,
          ),
        "UnstakedAmountExceedsRequired",
        [targetDeficit * 2n, targetDeficit],
      );
    });

    it("Should successfully submit the unstake request, change state and emit the expected event", async () => {
      // Arrange - Set up withdrawal reserve in deficit
      await setBalance(await mockLineaRollup.getAddress(), 0n);

      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      const targetReserveAmount = await yieldManager.getTargetWithdrawalReserveAmount();

      // Set up balances so requiredUnstakeAmountWei is non-zero
      // requiredUnstakeAmountWei = targetDeficit - (YieldManager.balance + withdrawableValue + pendingPermissionlessUnstake)
      // We want requiredUnstakeAmountWei > 0, so we set balances to be less than targetDeficit
      const yieldManagerBalance = targetReserveAmount / 3n;
      await setBalance(await yieldManager.getAddress(), yieldManagerBalance);
      await yieldManager.setWithdrawableValueReturnVal(mockYieldProviderAddress, 0n);
      await yieldManager.setPendingPermissionlessUnstake(0n);

      // Calculate expected requiredUnstakeAmountWei
      const targetDeficit = await yieldManager.getTargetReserveDeficit();
      const expectedRequiredUnstakeAmountWei = targetDeficit - yieldManagerBalance;

      // Set unstakedAmount to be non-zero and less than or equal to requiredUnstakeAmountWei
      const unstakedAmount = expectedRequiredUnstakeAmountWei;
      await yieldManager
        .connect(nativeYieldOperator)
        .setUnstakePermissionlessReturnVal(mockYieldProviderAddress, unstakedAmount);

      await expect(
        yieldManager
          .connect(nativeYieldOperator)
          .unstakePermissionless(
            mockYieldProviderAddress,
            mockValidatorIndex,
            mockSlot,
            mockWithdrawalParams,
            mockWithdrawalParamsProof,
          ),
      )
        .to.emit(yieldManager, "UnstakePermissionlessRequest")
        .withArgs(
          mockYieldProviderAddress,
          mockValidatorIndex,
          mockSlot,
          expectedRequiredUnstakeAmountWei,
          unstakedAmount,
          mockWithdrawalParams,
        );

      expect(await yieldManager.pendingPermissionlessUnstake()).to.equal(unstakedAmount);
      expect(await yieldManager.lastProvenSlot(mockValidatorIndex)).to.equal(mockSlot);
    });

    it("After submitting one unstake request that restores the reserve deficit, the next permissionless request reverts", async () => {
      // Arrange - Set up withdrawal reserve in deficit
      await setBalance(await mockLineaRollup.getAddress(), 0n);

      // Arrange - First do unstake permissionless up to maximum capacity
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      const targetReserveAmount = await yieldManager.getTargetWithdrawalReserveAmount();
      const unstakeAmount = targetReserveAmount;

      await yieldManager.setWithdrawableValueReturnVal(mockYieldProviderAddress, 0n);
      await yieldManager
        .connect(nativeYieldOperator)
        .setUnstakePermissionlessReturnVal(mockYieldProviderAddress, unstakeAmount);

      const firstSlot = mockSlot;
      await yieldManager
        .connect(nativeYieldOperator)
        .unstakePermissionless(
          mockYieldProviderAddress,
          mockValidatorIndex,
          firstSlot,
          mockWithdrawalParams,
          mockWithdrawalParamsProof,
        );

      expect(await yieldManager.pendingPermissionlessUnstake()).to.equal(unstakeAmount);

      // Arrange - Then do unstake of 1
      const secondUnstakeAmount = 1n;
      await yieldManager
        .connect(nativeYieldOperator)
        .setUnstakePermissionlessReturnVal(mockYieldProviderAddress, secondUnstakeAmount);

      // Use a slot that's far enough from the first slot (firstSlot + SLOTS_PER_HISTORICAL_ROOT + 1)
      const secondSlot = firstSlot + 8192n + 1n;

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager
          .connect(nativeYieldOperator)
          .unstakePermissionless(
            mockYieldProviderAddress,
            mockValidatorIndex,
            secondSlot,
            mockWithdrawalParams,
            mockWithdrawalParamsProof,
          ),
        "NoRequirementToUnstakePermissionless",
      );
    });
  });

  describe("delegatecall helper for withdraw from yield provider helper", () => {
    it("Should revert if the YieldProvider does not have sufficient balance", async () => {
      const { mockYieldProviderAddress, mockWithdrawTarget } = await addMockYieldProvider(yieldManager);
      const call = yieldManager
        .connect(nativeYieldOperator)
        .delegatecallWithdrawFromYieldProvider(mockYieldProviderAddress, 1n);

      await expect(call).to.be.revertedWithCustomError(mockWithdrawTarget, "MockWithdrawFailed");
    });

    it("Delegatecalls successfully and makes correct state transitions", async () => {
      const { mockYieldProviderAddress, mockYieldProvider } = await addMockYieldProvider(yieldManager);
      const withdrawAmount = ONE_ETHER;
      await fundYieldProviderForWithdrawal(yieldManager, mockYieldProvider, nativeYieldOperator, withdrawAmount);

      const beforeYieldProviderUserFunds = await yieldManager.userFunds(mockYieldProviderAddress);
      const beforeUserFundsInYieldProvidersTotal = await yieldManager.userFundsInYieldProvidersTotal();

      // Act
      await yieldManager
        .connect(nativeYieldOperator)
        .delegatecallWithdrawFromYieldProvider(mockYieldProviderAddress, withdrawAmount);

      // Assert
      const afterYieldProviderUserFunds = await yieldManager.userFunds(mockYieldProviderAddress);
      const afterUserFundsInYieldProvidersTotal = await yieldManager.userFundsInYieldProvidersTotal();

      expect(afterYieldProviderUserFunds).to.equal(beforeYieldProviderUserFunds - withdrawAmount);
      expect(afterUserFundsInYieldProvidersTotal).to.equal(beforeUserFundsInYieldProvidersTotal - withdrawAmount);
    });
  });

  describe("decrementPendingPermissionlessUnstake helper", () => {
    it("if pendingPermissionlessUnstake = 0, should be a no-op", async () => {
      expect(await yieldManager.getPendingPermissionlessUnstake()).to.equal(0n);

      await yieldManager.connect(nativeYieldOperator).decrementPendingPermissionlessUnstake(ONE_ETHER);

      expect(await yieldManager.getPendingPermissionlessUnstake()).to.equal(0n);
    });

    it("if pendingPermissionlessUnstake <= _amount, should reduce pendingPermissionlessUnstake to 0", async () => {
      await yieldManager.setPendingPermissionlessUnstake(ONE_ETHER);

      await yieldManager.connect(nativeYieldOperator).decrementPendingPermissionlessUnstake(ONE_THOUSAND_ETHER);

      expect(await yieldManager.getPendingPermissionlessUnstake()).to.equal(0n);
    });

    it("if pendingPermissionlessUnstake > _amount, should reduce pendingPermissionlessUnstake accordingly", async () => {
      const startingPending = ONE_THOUSAND_ETHER;
      await yieldManager.setPendingPermissionlessUnstake(startingPending);

      await yieldManager.connect(nativeYieldOperator).decrementPendingPermissionlessUnstake(ONE_ETHER);

      expect(await yieldManager.getPendingPermissionlessUnstake()).to.equal(startingPending - ONE_ETHER);
    });
  });

  describe("safe withdraw from yield provider", () => {
    it("Should revert when the GENERAL pause type is activated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.connect(securityCouncil).pauseByType(GENERAL_PAUSE_TYPE);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).safeWithdrawFromYieldProvider(mockYieldProviderAddress, 1n),
        "IsPaused",
        [GENERAL_PAUSE_TYPE],
      );
    });

    it("Should revert when the NATIVE_YIELD_UNSTAKING pause type is activated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.connect(securityCouncil).pauseByType(NATIVE_YIELD_UNSTAKING_PAUSE_TYPE);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).safeWithdrawFromYieldProvider(mockYieldProviderAddress, 1n),
        "IsPaused",
        [NATIVE_YIELD_UNSTAKING_PAUSE_TYPE],
      );
    });

    it("Should revert when unstaking from an unknown YieldProvider", async () => {
      const unknownYieldProvider = ethers.Wallet.createRandom().address;

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).safeWithdrawFromYieldProvider(unknownYieldProvider, 1n),
        "UnknownYieldProvider",
      );
    });

    it("Should revert when the caller does not have YIELD_PROVIDER_UNSTAKER_ROLE role", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      const unstakerRole = await yieldManager.YIELD_PROVIDER_UNSTAKER_ROLE();

      await expect(
        yieldManager.connect(nonAuthorizedAccount).safeWithdrawFromYieldProvider(mockYieldProviderAddress, 1n),
      ).to.be.revertedWith(buildAccessErrorMessage(nonAuthorizedAccount, unstakerRole));
    });

    it("If _amount < withdrawableValue, should withdraw _amount", async () => {
      const { mockYieldProviderAddress, mockYieldProvider } = await addMockYieldProvider(yieldManager);
      const withdrawableAmount = ONE_ETHER * 2n;
      const requestedAmount = ONE_ETHER;

      await fundYieldProviderForWithdrawal(yieldManager, mockYieldProvider, nativeYieldOperator, withdrawableAmount);
      await setWithdrawalReserveToTarget(yieldManager);
      expect(await yieldManager.getTargetReserveDeficit()).to.equal(0n);

      await expect(
        yieldManager
          .connect(nativeYieldOperator)
          .safeWithdrawFromYieldProvider(mockYieldProviderAddress, requestedAmount),
      )
        .to.emit(yieldManager, "YieldProviderWithdrawal")
        .withArgs(mockYieldProviderAddress, requestedAmount, 0);

      expect(await yieldManager.userFunds(mockYieldProviderAddress)).to.equal(withdrawableAmount - requestedAmount);
      expect(await yieldManager.userFundsInYieldProvidersTotal()).to.equal(withdrawableAmount - requestedAmount);
    });

    it("If withdrawableValue < _amount , should withdraw withdrawableValue", async () => {
      const { mockYieldProviderAddress, mockYieldProvider } = await addMockYieldProvider(yieldManager);
      const withdrawableAmount = ONE_ETHER;
      const requestedAmount = withdrawableAmount * 2n;

      await fundYieldProviderForWithdrawal(yieldManager, mockYieldProvider, nativeYieldOperator, withdrawableAmount);
      await setWithdrawalReserveToTarget(yieldManager);
      expect(await yieldManager.getTargetReserveDeficit()).to.equal(0n);

      await expect(
        yieldManager
          .connect(nativeYieldOperator)
          .safeWithdrawFromYieldProvider(mockYieldProviderAddress, requestedAmount),
      )
        .to.emit(yieldManager, "YieldProviderWithdrawal")
        .withArgs(mockYieldProviderAddress, withdrawableAmount, 0);

      expect(await yieldManager.userFunds(mockYieldProviderAddress)).to.equal(0n);
      expect(await yieldManager.userFundsInYieldProvidersTotal()).to.equal(0n);
    });

    it("With 0 targetDeficit, should successfully withdraw the full _amount to the YieldManager", async () => {
      const { mockYieldProviderAddress, mockYieldProvider } = await addMockYieldProvider(yieldManager);
      const withdrawAmount = ONE_ETHER;
      await fundYieldProviderForWithdrawal(yieldManager, mockYieldProvider, nativeYieldOperator, withdrawAmount);

      const l1MessageService = await mockLineaRollup.getAddress();
      const targetReserveAmount = await yieldManager.getTargetWithdrawalReserveAmount();
      const yieldManagerAddress = await yieldManager.getAddress();

      await ethers.provider.send("hardhat_setBalance", [l1MessageService, ethers.toBeHex(targetReserveAmount)]);
      await ethers.provider.send("hardhat_setBalance", [yieldManagerAddress, ethers.toBeHex(withdrawAmount)]);

      expect(await yieldManager.getTargetReserveDeficit()).to.equal(0n);

      await expect(
        yieldManager
          .connect(nativeYieldOperator)
          .safeWithdrawFromYieldProvider(mockYieldProviderAddress, withdrawAmount),
      )
        .to.emit(yieldManager, "YieldProviderWithdrawal")
        .withArgs(mockYieldProviderAddress, withdrawAmount, 0);

      expect(await yieldManager.userFunds(mockYieldProviderAddress)).to.equal(0n);
      expect(await yieldManager.userFundsInYieldProvidersTotal()).to.equal(0n);
      expect(await yieldManager.isStakingPaused(mockYieldProviderAddress)).to.be.false;
      expect(await ethers.provider.getBalance(l1MessageService)).to.equal(targetReserveAmount);
    });

    it("With targetDeficit > _amount, should withdraw the full _amount to the reserve and pause staking", async () => {
      const { mockYieldProviderAddress, mockYieldProvider } = await addMockYieldProvider(yieldManager);
      const withdrawAmount = ONE_ETHER;
      await fundYieldProviderForWithdrawal(yieldManager, mockYieldProvider, nativeYieldOperator, withdrawAmount);

      const targetReserveAmount = await yieldManager.getTargetWithdrawalReserveAmount();
      const l1MessageService = await mockLineaRollup.getAddress();
      const yieldManagerAddress = await yieldManager.getAddress();
      const targetDeficit = withdrawAmount * 2n;
      const reserveBalanceBefore = targetReserveAmount - targetDeficit;

      await ethers.provider.send("hardhat_setBalance", [l1MessageService, ethers.toBeHex(reserveBalanceBefore)]);
      await ethers.provider.send("hardhat_setBalance", [yieldManagerAddress, ethers.toBeHex(withdrawAmount)]);

      expect(await yieldManager.getTargetReserveDeficit()).to.equal(targetDeficit);
      expect(targetDeficit).to.be.above(withdrawAmount);

      await expect(
        yieldManager
          .connect(nativeYieldOperator)
          .safeWithdrawFromYieldProvider(mockYieldProviderAddress, withdrawAmount),
      )
        .to.emit(yieldManager, "YieldProviderWithdrawal")
        .withArgs(mockYieldProviderAddress, withdrawAmount, withdrawAmount);

      expect(await ethers.provider.getBalance(l1MessageService)).to.equal(reserveBalanceBefore + withdrawAmount);
      expect(await yieldManager.userFunds(mockYieldProviderAddress)).to.equal(0n);
      expect(await yieldManager.userFundsInYieldProvidersTotal()).to.equal(0n);
      expect(await yieldManager.isStakingPaused(mockYieldProviderAddress)).to.be.true;
    });

    it("With targetDeficit < _amount, should send only the target deficit to the reserve and not pause staking", async () => {
      const { mockYieldProviderAddress, mockYieldProvider } = await addMockYieldProvider(yieldManager);
      const withdrawAmount = ONE_ETHER;
      await fundYieldProviderForWithdrawal(yieldManager, mockYieldProvider, nativeYieldOperator, withdrawAmount);

      const targetReserveAmount = await yieldManager.getTargetWithdrawalReserveAmount();
      const l1MessageService = await mockLineaRollup.getAddress();
      const yieldManagerAddress = await yieldManager.getAddress();
      const targetDeficit = withdrawAmount / 4n;
      const reserveBalanceBefore = targetReserveAmount - targetDeficit;

      await ethers.provider.send("hardhat_setBalance", [l1MessageService, ethers.toBeHex(reserveBalanceBefore)]);
      await ethers.provider.send("hardhat_setBalance", [yieldManagerAddress, ethers.toBeHex(withdrawAmount)]);

      expect(await yieldManager.getTargetReserveDeficit()).to.equal(targetDeficit);

      await expect(
        yieldManager
          .connect(nativeYieldOperator)
          .safeWithdrawFromYieldProvider(mockYieldProviderAddress, withdrawAmount),
      )
        .to.emit(yieldManager, "YieldProviderWithdrawal")
        .withArgs(mockYieldProviderAddress, withdrawAmount, targetDeficit);

      expect(await ethers.provider.getBalance(l1MessageService)).to.equal(reserveBalanceBefore + targetDeficit);
      expect(await yieldManager.userFunds(mockYieldProviderAddress)).to.equal(0n);
      expect(await yieldManager.userFundsInYieldProvidersTotal()).to.equal(0n);
      expect(await yieldManager.isStakingPaused(mockYieldProviderAddress)).to.be.false;
    });
  });

  describe("safely adding to withdrawal reserve", () => {
    it("Should revert when the GENERAL pause type is activated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      await yieldManager.connect(securityCouncil).pauseByType(GENERAL_PAUSE_TYPE);
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).safeAddToWithdrawalReserve(mockYieldProviderAddress, 1n),
        "IsPaused",
        [GENERAL_PAUSE_TYPE],
      );
    });
    it("Should revert when the NATIVE_YIELD_UNSTAKING pause type is activated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      await yieldManager.connect(securityCouncil).pauseByType(NATIVE_YIELD_UNSTAKING_PAUSE_TYPE);
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).safeAddToWithdrawalReserve(mockYieldProviderAddress, 1n),
        "IsPaused",
        [NATIVE_YIELD_UNSTAKING_PAUSE_TYPE],
      );
    });
    it("Should revert when rebalancing from an unknown YieldProvider", async () => {
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).safeAddToWithdrawalReserve(ethers.Wallet.createRandom().address, 1n),
        "UnknownYieldProvider",
      );
    });
    it("Should revert when the caller does not have YIELD_PROVIDER_UNSTAKER_ROLE role", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      await expect(
        yieldManager.connect(nonAuthorizedAccount).safeAddToWithdrawalReserve(mockYieldProviderAddress, 1n),
      ).to.be.revertedWith(
        buildAccessErrorMessage(nonAuthorizedAccount, await yieldManager.YIELD_PROVIDER_UNSTAKER_ROLE()),
      );
    });
    it("With _amount min, _amount < _withdrawableValue, will send _amount from YieldManager to reserve", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      // Arrange - Setup YieldManager balance
      const yieldManagerBalance = ONE_ETHER * 4n;
      const yieldManagerAddress = await yieldManager.getAddress();
      await setBalance(yieldManagerAddress, yieldManagerBalance);

      // Act
      const rebalanceAmount = ONE_ETHER * 2n;
      await expect(
        yieldManager.connect(nativeYieldOperator).safeAddToWithdrawalReserve(mockYieldProviderAddress, rebalanceAmount),
      )
        .to.emit(yieldManager, "WithdrawalReserveAugmented")
        .withArgs(mockYieldProviderAddress, rebalanceAmount, rebalanceAmount, 0n);

      expect(await ethers.provider.getBalance(yieldManagerAddress)).to.equal(yieldManagerBalance - rebalanceAmount);
      expect(await ethers.provider.getBalance(await mockLineaRollup.getAddress())).to.equal(rebalanceAmount);
    });
    it("With _withdrawableValue min, _withdrawableValue <= YieldManager.balance, will send _withdrawableValue from YieldManager to reserve", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      // Arrange - Setup YieldManager balance
      const yieldManagerBalance = ONE_ETHER * 2n;
      const yieldManagerAddress = await yieldManager.getAddress();
      await setBalance(yieldManagerAddress, yieldManagerBalance);
      const l1MessageServiceBalanceBefore = await getBalance(mockLineaRollup);

      // Act
      const rebalanceAmount = ONE_ETHER * 4n;
      await expect(
        yieldManager.connect(nativeYieldOperator).safeAddToWithdrawalReserve(mockYieldProviderAddress, rebalanceAmount),
      )
        .to.emit(yieldManager, "WithdrawalReserveAugmented")
        .withArgs(mockYieldProviderAddress, yieldManagerBalance, yieldManagerBalance, 0n);

      expect(await ethers.provider.getBalance(yieldManagerAddress)).to.equal(0n);
      expect(await getBalance(mockLineaRollup)).to.equal(l1MessageServiceBalanceBefore + yieldManagerBalance);
    });
    it("With _amount min, _amount > YieldManager.balance, will withdraw from YieldProvider", async () => {
      const { mockYieldProvider, mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      // Arrange - Setup withdrawableValue = 3 ETH
      await fundYieldProviderForWithdrawal(yieldManager, mockYieldProvider, nativeYieldOperator, ONE_ETHER * 3n);
      await yieldManager.setWithdrawableValueReturnVal(mockYieldProviderAddress, ONE_ETHER * 3n);

      // Setup YieldManager balance = 1 ETH
      const yieldManagerBalance = ONE_ETHER * 1n;
      const yieldManagerAddress = await yieldManager.getAddress();
      await setBalance(yieldManagerAddress, yieldManagerBalance);

      const l1MessageServiceBalanceBefore = await getBalance(mockLineaRollup);

      // Act
      const rebalanceAmount = ONE_ETHER * 2n;
      await expect(
        yieldManager.connect(nativeYieldOperator).safeAddToWithdrawalReserve(mockYieldProviderAddress, rebalanceAmount),
      )
        .to.emit(yieldManager, "WithdrawalReserveAugmented")
        .withArgs(
          mockYieldProviderAddress,
          rebalanceAmount,
          yieldManagerBalance,
          rebalanceAmount - yieldManagerBalance,
        );

      expect(await ethers.provider.getBalance(yieldManagerAddress)).to.equal(0n);
      expect(await ethers.provider.getBalance(await mockLineaRollup.getAddress())).to.equal(
        l1MessageServiceBalanceBefore + rebalanceAmount,
      );
    });
    it("With _withdrawableValue min, _withdrawableValue > YieldManager.balance, will withdraw from YieldProvider", async () => {
      const { mockYieldProvider, mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      // Arrange - Setup withdrawableValue = 3 ETH
      const withdrawableValue = ONE_ETHER * 3n;
      await fundYieldProviderForWithdrawal(yieldManager, mockYieldProvider, nativeYieldOperator, withdrawableValue);
      await yieldManager.setWithdrawableValueReturnVal(mockYieldProviderAddress, withdrawableValue);

      // Setup YieldManager balance = 1 ETH
      const yieldManagerBalance = ONE_ETHER * 1n;
      const yieldManagerAddress = await yieldManager.getAddress();
      await setBalance(yieldManagerAddress, yieldManagerBalance);

      const l1MessageServiceBalanceBefore = await getBalance(mockLineaRollup);

      // Act
      const rebalanceAmount = ONE_ETHER * 5n;
      await expect(
        yieldManager.connect(nativeYieldOperator).safeAddToWithdrawalReserve(mockYieldProviderAddress, rebalanceAmount),
      )
        .to.emit(yieldManager, "WithdrawalReserveAugmented")
        .withArgs(
          mockYieldProviderAddress,
          withdrawableValue + yieldManagerBalance,
          yieldManagerBalance,
          withdrawableValue,
        );

      expect(await ethers.provider.getBalance(yieldManagerAddress)).to.equal(0n);
      expect(await ethers.provider.getBalance(await mockLineaRollup.getAddress())).to.equal(
        l1MessageServiceBalanceBefore + withdrawableValue + yieldManagerBalance,
      );
    });
    it("With YieldManager balance > _amount and targetDeficit > _amount, will send _amount from YieldManager to reserve and pause staking", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      const rebalanceAmount = ONE_ETHER;
      const yieldManagerAddress = await yieldManager.getAddress();
      await ethers.provider.send("hardhat_setBalance", [yieldManagerAddress, ethers.toBeHex(rebalanceAmount * 2n)]);
      await expect(
        yieldManager.connect(nativeYieldOperator).safeAddToWithdrawalReserve(mockYieldProviderAddress, rebalanceAmount),
      )
        .to.emit(yieldManager, "WithdrawalReserveAugmented")
        .withArgs(mockYieldProviderAddress, rebalanceAmount, rebalanceAmount, 0n);

      expect(await ethers.provider.getBalance(yieldManagerAddress)).to.equal(rebalanceAmount);
      expect(await ethers.provider.getBalance(await mockLineaRollup.getAddress())).to.equal(rebalanceAmount);
      expect(await yieldManager.isStakingPaused(mockYieldProviderAddress)).eq(true);
    });
    it("With YieldManager balance > _amount and targetDeficit < _amount, will send _amount from YieldManager to reserve and not pause staking", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      const rebalanceAmount = ONE_ETHER;
      const yieldManagerAddress = await yieldManager.getAddress();
      await ethers.provider.send("hardhat_setBalance", [yieldManagerAddress, ethers.toBeHex(rebalanceAmount * 2n)]);
      await setWithdrawalReserveToTarget(yieldManager);
      const l1MessageServiceBalanceBefore = await getBalance(mockLineaRollup);
      await expect(
        yieldManager.connect(nativeYieldOperator).safeAddToWithdrawalReserve(mockYieldProviderAddress, rebalanceAmount),
      )
        .to.emit(yieldManager, "WithdrawalReserveAugmented")
        .withArgs(mockYieldProviderAddress, rebalanceAmount, rebalanceAmount, 0n);

      expect(await ethers.provider.getBalance(yieldManagerAddress)).to.equal(rebalanceAmount);
      expect(await ethers.provider.getBalance(await mockLineaRollup.getAddress())).to.equal(
        l1MessageServiceBalanceBefore + rebalanceAmount,
      );
      expect(await yieldManager.isStakingPaused(mockYieldProviderAddress)).eq(false);
    });
    it("With YieldManager balance < _amount and targetDeficit > _amount, should withdraw from YieldProvider to the reserve and pause staking", async () => {
      const rebalanceAmount = ONE_ETHER * 2n;
      // Arrange - setup remainder of rebalanceAmount on YieldProvider
      const { mockYieldProviderAddress, mockYieldProvider, mockWithdrawTarget } =
        await addMockYieldProvider(yieldManager);
      await fundYieldProviderForWithdrawal(yieldManager, mockYieldProvider, nativeYieldOperator, rebalanceAmount / 2n);

      // Arrange - setup insufficient YieldManager balance
      const yieldManagerAddress = await yieldManager.getAddress();
      await incrementBalance(yieldManagerAddress, rebalanceAmount / 2n);
      // Arrange - Get before
      await setWithdrawalReserveToMinimum(yieldManager);
      const l1MessageServiceBalanceBefore = await getBalance(mockLineaRollup);

      // Act
      await expect(
        yieldManager.connect(nativeYieldOperator).safeAddToWithdrawalReserve(mockYieldProviderAddress, rebalanceAmount),
      )
        .to.emit(yieldManager, "WithdrawalReserveAugmented")
        .withArgs(mockYieldProviderAddress, rebalanceAmount, rebalanceAmount / 2n, rebalanceAmount / 2n);

      expect(await getBalance(mockLineaRollup)).to.equal(l1MessageServiceBalanceBefore + rebalanceAmount);
      expect(await ethers.provider.getBalance(yieldManager)).to.equal(0);
      expect(await ethers.provider.getBalance(mockWithdrawTarget)).to.equal(0);
      expect(await yieldManager.isStakingPaused(mockYieldProviderAddress)).eq(true);
    });
    it("With YieldManager balance < _amount and targetDeficit < _amount, should withdraw from YieldProvider to the reserve and not pause staking", async () => {
      const rebalanceAmount = ONE_ETHER * 2n;
      // Arrange - setup remainder of rebalanceAmount on YieldProvider
      const { mockYieldProviderAddress, mockYieldProvider, mockWithdrawTarget } =
        await addMockYieldProvider(yieldManager);
      await fundYieldProviderForWithdrawal(yieldManager, mockYieldProvider, nativeYieldOperator, rebalanceAmount / 2n);
      // Arrange - setup insufficient YieldManager balance
      const yieldManagerAddress = await yieldManager.getAddress();
      await incrementBalance(yieldManagerAddress, rebalanceAmount / 2n);
      // Arrange - Get before
      await setWithdrawalReserveToTarget(yieldManager);
      const l1MessageServiceBalanceBefore = await getBalance(mockLineaRollup);

      // Act
      await expect(
        yieldManager.connect(nativeYieldOperator).safeAddToWithdrawalReserve(mockYieldProviderAddress, rebalanceAmount),
      )
        .to.emit(yieldManager, "WithdrawalReserveAugmented")
        .withArgs(mockYieldProviderAddress, rebalanceAmount, rebalanceAmount / 2n, rebalanceAmount / 2n);

      expect(await getBalance(mockLineaRollup)).to.equal(l1MessageServiceBalanceBefore + rebalanceAmount);
      expect(await ethers.provider.getBalance(yieldManager)).to.equal(0);
      expect(await ethers.provider.getBalance(mockWithdrawTarget)).to.equal(0);
      expect(await yieldManager.isStakingPaused(mockYieldProviderAddress)).eq(false);
    });
  });

  describe("permissionlessly replenishing the withdrawal reserve", () => {
    it("Should revert when the GENERAL pause type is activated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.connect(securityCouncil).pauseByType(GENERAL_PAUSE_TYPE);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).replenishWithdrawalReserve(mockYieldProviderAddress),
        "IsPaused",
        [GENERAL_PAUSE_TYPE],
      );
    });

    it("Should revert when the NATIVE_YIELD_PERMISSIONLESS_ACTIONS pause type is activated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.connect(securityCouncil).pauseByType(NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).replenishWithdrawalReserve(mockYieldProviderAddress),
        "IsPaused",
        [NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE],
      );
    });

    it("Should revert when rebalancing from an unknown YieldProvider", async () => {
      const unknownYieldProvider = ethers.Wallet.createRandom().address;

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).replenishWithdrawalReserve(unknownYieldProvider),
        "UnknownYieldProvider",
      );
    });

    it("Should revert when there is no reserve deficit", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      const l1MessageService = await mockLineaRollup.getAddress();
      const minimumReserve = await yieldManager.getEffectiveMinimumWithdrawalReserve();

      await ethers.provider.send("hardhat_setBalance", [l1MessageService, ethers.toBeHex(minimumReserve)]);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).replenishWithdrawalReserve(mockYieldProviderAddress),
        "WithdrawalReserveNotInDeficit",
      );
    });

    it("Should revert if there is 0 available balance on YieldManager and YieldProvider", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      const l1MessageService = await mockLineaRollup.getAddress();
      const yieldManagerAddress = await yieldManager.getAddress();

      // Arrange balances
      await setBalance(l1MessageService, 0n);
      await setBalance(yieldManagerAddress, 0n);
      await setBalance(mockYieldProviderAddress, 0n);

      // Act
      const call = yieldManager.connect(nativeYieldOperator).replenishWithdrawalReserve(mockYieldProviderAddress);

      // Assert
      await expectRevertWithCustomError(yieldManager, call, "NoAvailableFundsToReplenishWithdrawalReserve");
    });

    it("If YieldManager balance > targetDeficit, settle targetDeficit from YieldManager balance", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      const l1MessageService = await mockLineaRollup.getAddress();
      const yieldManagerAddress = await yieldManager.getAddress();

      // Arrange - L1MessageService balance = 0
      const beforeL1MessageServiceBalance = 0n;
      await ethers.provider.send("hardhat_setBalance", [
        l1MessageService,
        ethers.toBeHex(beforeL1MessageServiceBalance),
      ]);
      // Arrange - YieldManager balance
      const targetDeficit = await yieldManager.getTargetReserveDeficit();
      const beforeYieldManagerBalance = targetDeficit + ONE_ETHER;
      await ethers.provider.send("hardhat_setBalance", [
        yieldManagerAddress,
        ethers.toBeHex(beforeYieldManagerBalance),
      ]);

      await expect(yieldManager.connect(nativeYieldOperator).replenishWithdrawalReserve(mockYieldProviderAddress))
        .to.emit(yieldManager, "WithdrawalReserveReplenished")
        .withArgs(mockYieldProviderAddress, targetDeficit, targetDeficit, targetDeficit, 0n);
      expect(await yieldManager.isStakingPaused(mockYieldProviderAddress)).to.be.false;
      expect(await ethers.provider.getBalance(l1MessageService)).eq(beforeL1MessageServiceBalance + targetDeficit);
      expect(await ethers.provider.getBalance(yieldManagerAddress)).eq(beforeYieldManagerBalance - targetDeficit);
    });

    it("If YieldManager balance < targetDeficit, settle remainder by withdrawing available value from YieldProvider to reserve", async () => {
      const targetReserveAmount = await yieldManager.getTargetWithdrawalReserveAmount();
      // Arrange - put half of target reserve amount on YieldProvider
      const { mockYieldProviderAddress, mockYieldProvider, mockWithdrawTarget } =
        await addMockYieldProvider(yieldManager);
      await fundYieldProviderForWithdrawal(
        yieldManager,
        mockYieldProvider,
        nativeYieldOperator,
        targetReserveAmount / 2n,
      );
      const mockWithdrawTargetAddress = await mockWithdrawTarget.getAddress();
      const beforeMockWithdrawTargetBalance = await ethers.provider.getBalance(mockWithdrawTargetAddress);
      // Arrange - put other half on YieldManager
      const yieldManagerAddress = await yieldManager.getAddress();
      await ethers.provider.send("hardhat_setBalance", [yieldManagerAddress, ethers.toBeHex(targetReserveAmount / 2n)]);
      const beforeYieldManagerBalance = await ethers.provider.getBalance(yieldManagerAddress);
      // Arrange - 0 balance on L1MessageService
      const l1MessageService = await mockLineaRollup.getAddress();
      await ethers.provider.send("hardhat_setBalance", [l1MessageService, ethers.toBeHex(0)]);
      expect(await yieldManager.getTargetReserveDeficit()).eq(targetReserveAmount);
      const beforeL1MessageServiceBalance = await ethers.provider.getBalance(l1MessageService);

      // Act
      await expect(yieldManager.connect(nativeYieldOperator).replenishWithdrawalReserve(mockYieldProviderAddress))
        .to.emit(yieldManager, "WithdrawalReserveReplenished")
        .withArgs(
          mockYieldProviderAddress,
          targetReserveAmount,
          targetReserveAmount,
          targetReserveAmount / 2n,
          targetReserveAmount / 2n,
        );

      expect(await yieldManager.isStakingPaused(mockYieldProviderAddress)).to.be.false;
      expect(await ethers.provider.getBalance(l1MessageService)).eq(
        beforeL1MessageServiceBalance + targetReserveAmount,
      );
      expect(await ethers.provider.getBalance(yieldManagerAddress)).eq(
        beforeYieldManagerBalance - targetReserveAmount / 2n,
      );
      expect(await ethers.provider.getBalance(mockWithdrawTargetAddress)).eq(
        beforeMockWithdrawTargetBalance - targetReserveAmount / 2n,
      );
    });

    it("If YieldManager + YieldProvider balance > targetDeficit, should only rebalance required to restore targetDeficit", async () => {
      const targetReserveAmount = await yieldManager.getTargetWithdrawalReserveAmount();
      // Arrange - put target reserve amount on YieldProvider
      const { mockYieldProviderAddress, mockYieldProvider, mockWithdrawTarget } =
        await addMockYieldProvider(yieldManager);
      await fundYieldProviderForWithdrawal(yieldManager, mockYieldProvider, nativeYieldOperator, targetReserveAmount);
      const mockWithdrawTargetAddress = await mockWithdrawTarget.getAddress();
      const beforeMockWithdrawTargetBalance = await ethers.provider.getBalance(mockWithdrawTargetAddress);
      // Arrange - put half of target reserve amount on YieldManager
      const yieldManagerAddress = await yieldManager.getAddress();
      await ethers.provider.send("hardhat_setBalance", [yieldManagerAddress, ethers.toBeHex(targetReserveAmount / 2n)]);
      const beforeYieldManagerBalance = await ethers.provider.getBalance(yieldManagerAddress);
      // Arrange - 0 balance on L1MessageService
      const l1MessageService = await mockLineaRollup.getAddress();
      await ethers.provider.send("hardhat_setBalance", [l1MessageService, ethers.toBeHex(0)]);
      expect(await yieldManager.getTargetReserveDeficit()).eq(targetReserveAmount);
      const beforeL1MessageServiceBalance = await ethers.provider.getBalance(l1MessageService);

      // Act
      await expect(yieldManager.connect(nativeYieldOperator).replenishWithdrawalReserve(mockYieldProviderAddress))
        .to.emit(yieldManager, "WithdrawalReserveReplenished")
        .withArgs(
          mockYieldProviderAddress,
          targetReserveAmount,
          targetReserveAmount,
          targetReserveAmount / 2n,
          targetReserveAmount / 2n,
        );

      expect(await yieldManager.isStakingPaused(mockYieldProviderAddress)).to.be.false;
      expect(await ethers.provider.getBalance(l1MessageService)).eq(
        beforeL1MessageServiceBalance + targetReserveAmount,
      );
      expect(await ethers.provider.getBalance(yieldManagerAddress)).eq(
        beforeYieldManagerBalance - targetReserveAmount / 2n,
      );
      expect(await ethers.provider.getBalance(mockWithdrawTargetAddress)).eq(
        beforeMockWithdrawTargetBalance - targetReserveAmount / 2n,
      );
    });

    it("If there is remaining target deficit, should pause staking", async () => {
      const targetReserveAmount = await yieldManager.getTargetWithdrawalReserveAmount();
      // Arrange - put half target reserve amount on YieldProvider
      const { mockYieldProviderAddress, mockYieldProvider, mockWithdrawTarget } =
        await addMockYieldProvider(yieldManager);
      await fundYieldProviderForWithdrawal(
        yieldManager,
        mockYieldProvider,
        nativeYieldOperator,
        targetReserveAmount / 2n,
      );
      const mockWithdrawTargetAddress = await mockWithdrawTarget.getAddress();
      const beforeMockWithdrawTargetBalance = await ethers.provider.getBalance(mockWithdrawTargetAddress);
      // Arrange - put quarter of target reserve amount on YieldManager
      const yieldManagerAddress = await yieldManager.getAddress();
      await ethers.provider.send("hardhat_setBalance", [yieldManagerAddress, ethers.toBeHex(targetReserveAmount / 4n)]);
      const beforeYieldManagerBalance = await ethers.provider.getBalance(yieldManagerAddress);
      // Arrange - 0 balance on L1MessageService
      const l1MessageService = await mockLineaRollup.getAddress();
      await ethers.provider.send("hardhat_setBalance", [l1MessageService, ethers.toBeHex(0)]);
      expect(await yieldManager.getTargetReserveDeficit()).eq(targetReserveAmount);
      const beforeL1MessageServiceBalance = await ethers.provider.getBalance(l1MessageService);

      // Act
      await expect(yieldManager.connect(nativeYieldOperator).replenishWithdrawalReserve(mockYieldProviderAddress))
        .to.emit(yieldManager, "WithdrawalReserveReplenished")
        .withArgs(
          mockYieldProviderAddress,
          targetReserveAmount,
          (targetReserveAmount * 3n) / 4n,
          targetReserveAmount / 4n,
          targetReserveAmount / 2n,
        );

      expect(await yieldManager.isStakingPaused(mockYieldProviderAddress)).to.be.true;
      expect(await ethers.provider.getBalance(l1MessageService)).eq(
        beforeL1MessageServiceBalance + (targetReserveAmount * 3n) / 4n,
      );
      expect(await ethers.provider.getBalance(yieldManagerAddress)).eq(
        beforeYieldManagerBalance - targetReserveAmount / 4n,
      );
      expect(await ethers.provider.getBalance(mockWithdrawTargetAddress)).eq(
        beforeMockWithdrawTargetBalance - targetReserveAmount / 2n,
      );
    });
  });

  describe("withdraw LST", () => {
    it("Should revert when the GENERAL pause type is activated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.connect(securityCouncil).pauseByType(GENERAL_PAUSE_TYPE);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).withdrawLST(mockYieldProviderAddress, 0n, ethers.ZeroAddress),
        "IsPaused",
        [GENERAL_PAUSE_TYPE],
      );
    });

    it("Should revert when the NATIVE_YIELD_PERMISSIONLESS_ACTIONS pause type is activated", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      await yieldManager.connect(securityCouncil).pauseByType(NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).withdrawLST(mockYieldProviderAddress, 0n, ethers.ZeroAddress),
        "IsPaused",
        [NATIVE_YIELD_PERMISSIONLESS_ACTIONS_PAUSE_TYPE],
      );
    });

    it("Should revert when choosing an unknown YieldProvider", async () => {
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager
          .connect(nativeYieldOperator)
          .withdrawLST(ethers.Wallet.createRandom().address, 0n, ethers.ZeroAddress),
        "UnknownYieldProvider",
      );
    });

    it("Should revert if the sender is not the L1MessageService", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(nativeYieldOperator).withdrawLST(mockYieldProviderAddress, 0n, ethers.ZeroAddress),
        "SenderNotL1MessageService",
      );
    });

    it("Should revert if L1MessageService does not have withdraw LST flag toggled", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      const l1MessageService = await mockLineaRollup.getAddress();

      await mockLineaRollup.setWithdrawLSTAllowed(false);
      // Arrange - set gas funds for L1MessageService to be signer
      await ethers.provider.send("hardhat_setBalance", [l1MessageService, ethers.toBeHex(ONE_ETHER)]);
      const l1Signer = await ethers.getImpersonatedSigner(l1MessageService);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(l1Signer).withdrawLST(mockYieldProviderAddress, 0n, ethers.ZeroAddress),
        "LSTWithdrawalNotAllowed",
      );

      await mockLineaRollup.setWithdrawLSTAllowed(true);
      await ethers.provider.send("hardhat_stopImpersonatingAccount", [l1MessageService]);
    });

    it("Should revert if LST withdraw amount > userFunds for yield provider", async () => {
      const { mockYieldProviderAddress, mockYieldProvider } = await addMockYieldProvider(yieldManager);
      const l1MessageService = await mockLineaRollup.getAddress();
      const withdrawAmount = ONE_ETHER;

      await fundYieldProviderForWithdrawal(yieldManager, mockYieldProvider, nativeYieldOperator, withdrawAmount / 2n);

      // Arrange - set gas funds for L1MessageService to be signer
      await ethers.provider.send("hardhat_setBalance", [l1MessageService, ethers.toBeHex(ONE_ETHER)]);
      const l1Signer = await ethers.getImpersonatedSigner(l1MessageService);
      await mockLineaRollup.setWithdrawLSTAllowed(true);

      // Arrange Setup

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(l1Signer).withdrawLST(mockYieldProviderAddress, withdrawAmount, ethers.ZeroAddress),
        "LSTWithdrawalExceedsYieldProviderFunds",
      );

      await ethers.provider.send("hardhat_stopImpersonatingAccount", [l1MessageService]);
    });

    it("Should revert if LST withdraw amount + lastReportedNegativeYield > userFunds for yield provider", async () => {
      const { mockYieldProviderAddress, mockYieldProvider } = await addMockYieldProvider(yieldManager);
      const l1MessageService = await mockLineaRollup.getAddress();
      const fundAmount = ONE_ETHER * 10n;
      await fundYieldProviderForWithdrawal(yieldManager, mockYieldProvider, nativeYieldOperator, fundAmount);
      // Arrange - Setup lstPrincipalAmount
      const withdrawAmount = fundAmount;
      const negativeYield = 1n;
      await yieldManager.setYieldProviderLastReportedNegativeYield(mockYieldProviderAddress, negativeYield);
      // Arrange - set gas funds for L1MessageService to be signer
      await ethers.provider.send("hardhat_setBalance", [l1MessageService, ethers.toBeHex(withdrawAmount)]);
      const l1Signer = await ethers.getImpersonatedSigner(l1MessageService);
      await mockLineaRollup.setWithdrawLSTAllowed(true);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(l1Signer).withdrawLST(mockYieldProviderAddress, withdrawAmount, ethers.ZeroAddress),
        "LSTWithdrawalExceedsYieldProviderFunds",
      );

      await ethers.provider.send("hardhat_stopImpersonatingAccount", [l1MessageService]);
    });

    it("Should successfully withdraw LST, pause staking and emit the expected event", async () => {
      const { mockYieldProviderAddress, mockYieldProvider } = await addMockYieldProvider(yieldManager);
      const l1MessageService = await mockLineaRollup.getAddress();
      const withdrawAmount = ONE_ETHER;
      const recipient = ethers.Wallet.createRandom().address;

      await fundYieldProviderForWithdrawal(yieldManager, mockYieldProvider, nativeYieldOperator, withdrawAmount);

      await ethers.provider.send("hardhat_setBalance", [l1MessageService, ethers.toBeHex(ONE_ETHER)]);
      const l1Signer = await ethers.getImpersonatedSigner(l1MessageService);
      await mockLineaRollup.setWithdrawLSTAllowed(true);

      // Arrange - Get before
      const userFundsInYieldProvidersTotalBefore = await yieldManager.userFundsInYieldProvidersTotal();
      const userFundsBefore = await yieldManager.userFunds(mockYieldProviderAddress);
      const lstLiabilityPrincipalBefore =
        await yieldManager.getYieldProviderLstLiabilityPrincipal(mockYieldProviderAddress);

      // Act
      await expect(yieldManager.connect(l1Signer).withdrawLST(mockYieldProviderAddress, withdrawAmount, recipient))
        .to.emit(yieldManager, "LSTMinted")
        .withArgs(mockYieldProviderAddress, recipient, withdrawAmount);

      expect(await yieldManager.isStakingPaused(mockYieldProviderAddress)).to.be.true;
      expect(await yieldManager.userFundsInYieldProvidersTotal()).eq(
        userFundsInYieldProvidersTotalBefore - withdrawAmount,
      );
      expect(await yieldManager.userFunds(mockYieldProviderAddress)).eq(userFundsBefore - withdrawAmount);
      expect(await yieldManager.getYieldProviderLstLiabilityPrincipal(mockYieldProviderAddress)).eq(
        lstLiabilityPrincipalBefore + withdrawAmount,
      );
      await ethers.provider.send("hardhat_stopImpersonatingAccount", [l1MessageService]);
    });
  });
});
