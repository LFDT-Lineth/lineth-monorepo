// Test scenarios with LinethRollup + YieldManager + LidoStVaultYieldProvider
import { expect } from "chai";
import { network } from "hardhat";
import { describe, it, before, beforeEach } from "node:test";
import { type Address, encodeFunctionData, keccak256 } from "viem";

import { encodeSendMessage } from "../../../../common/helpers/encoding.js";
import { EMPTY_CALLDATA, ONE_ETHER, ZERO_VALUE, CONNECT_DEPOSIT } from "../../common/constants/index.js";
import {
  type TestAccount,
  getAccountsFixture,
  expectRevertWithCustomError,
  expectEvent,
} from "../../common/helpers/index.js";
import {
  decrementBalance,
  deployAndAddAdditionalLidoStVaultYieldProvider,
  deployYieldManagerIntegrationTestFixture,
  executeUnstakePermissionless,
  fundLidoStVaultYieldProvider,
  getBalance,
  incrementBalance,
  incrementMockDashboardTotalValue,
  incurNegativeYield,
  incurPositiveYield,
  setBalance,
  setupLinethRollupMessageMerkleTree,
  setWithdrawalReserveToMinimum,
  setWithdrawalReserveToTarget,
  withdrawLST,
  setupMaxLSTLiabilityPaymentForWithdrawal,
  setupLSTPrincipalDecrementForPaxMaximumPossibleLSTLiability,
  buildVendorExitData,
  buildSetWithdrawalReserveParams,
  type YieldManagerInitializationData,
} from "../helpers/index.js";

import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

describe("Integration tests with LinethRollup, YieldManager and LidoStVaultYieldProvider", () => {
  let nativeYieldOperator: TestAccount;
  let nonAuthorizedAccount: TestAccount;
  let l2YieldRecipient: TestAccount;
  let securityCouncil: TestAccount;

  let linethRollup: ContractReturnType<"TestLinethRollup">;
  let yieldManager: ContractReturnType<"TestYieldManager">;
  let yieldProvider: ContractReturnType<"TestLidoStVaultYieldProvider">;
  let mockDashboard: ContractReturnType<"MockDashboard">;
  let mockStakingVault: ContractReturnType<"MockStakingVault">;
  let mockSTETH: ContractReturnType<"MockSTETH">;
  let mockVaultHub: ContractReturnType<"MockVaultHub">;
  let mockVaultFactory: ContractReturnType<"MockVaultFactory">;
  let lidoStVaultYieldProviderFactory: ContractReturnType<"TestLidoStVaultYieldProviderFactory">;
  let sszMerkleTree: ContractReturnType<"SSZMerkleTree">;
  let testVerifier: ContractReturnType<"TestValidatorContainerProofVerifier">;
  let initializationData: YieldManagerInitializationData;

  let l1MessageServiceAddress: Address;
  let yieldManagerAddress: Address;
  let yieldProviderAddress: Address;
  let mockStakingVaultAddress: Address;

  before(async () => {
    const { networkHelpers } = await network.getOrCreate();
    ({ nativeYieldOperator, nonAuthorizedAccount, l2YieldRecipient, securityCouncil } =
      await networkHelpers.loadFixture(getAccountsFixture));
  });

  beforeEach(async () => {
    const { networkHelpers } = await network.getOrCreate();
    ({
      linethRollup,
      yieldProvider,
      yieldProviderAddress,
      yieldManager,
      mockDashboard,
      mockStakingVault,
      mockSTETH,
      mockVaultHub,
      mockVaultFactory,
      lidoStVaultYieldProviderFactory,
      sszMerkleTree,
      testVerifier,
      initializationData,
    } = await networkHelpers.loadFixture(deployYieldManagerIntegrationTestFixture));
    l1MessageServiceAddress = linethRollup.address;
    yieldManagerAddress = yieldManager.address;
    mockStakingVaultAddress = mockStakingVault.address;
  });

  describe("Initial state", () => {
    it("userFunds should be equivalent to CONNECT_DEPOSIT amount", async () => {
      const connectDeposit = await yieldProvider.read.CONNECT_DEPOSIT();
      expect(await yieldManager.read.userFunds([yieldProviderAddress])).eq(connectDeposit);
      expect(await yieldManager.read.userFundsInYieldProvidersTotal()).eq(connectDeposit);
    });
  });

  describe("Transfering to the YieldManager", () => {
    it("Should successfully transfer from LinethRollup to the YieldManager", async () => {
      const fundAmount = ONE_ETHER;
      await setWithdrawalReserveToMinimum(yieldManager);
      await incrementBalance(l1MessageServiceAddress, fundAmount);
      const rollupBalanceBefore = await getBalance({ address: l1MessageServiceAddress });
      const yieldManagerBalanceBefore = await getBalance({ address: yieldManagerAddress });
      // Act
      await linethRollup.write.transferFundsForNativeYield([fundAmount], { account: nativeYieldOperator.account });
      // Assert
      const rollupBalanceAfter = await getBalance({ address: l1MessageServiceAddress });
      const yieldManagerBalanceAfter = await getBalance({ address: yieldManagerAddress });
      expect(rollupBalanceAfter).eq(rollupBalanceBefore - fundAmount);
      expect(yieldManagerBalanceAfter).eq(yieldManagerBalanceBefore + fundAmount);
    });
    it("Should revert when withdrawal reserve at minimum (including msg.value in the reserve computation)", async () => {
      // Arrange - %-based minimum at 20%
      await yieldManager.write.setWithdrawalReserveParameters(
        [buildSetWithdrawalReserveParams(initializationData, { minAmount: 0n })],
        { account: securityCouncil.account },
      );
      await setBalance(linethRollup.address, 21n * ONE_ETHER);
      await setBalance(yieldManager.address, 79n * ONE_ETHER);

      // Act
      const withdrawAmount = ONE_ETHER + 1n;
      const call = linethRollup.write.transferFundsForNativeYield([withdrawAmount], {
        account: nativeYieldOperator.account,
      });

      // Assert
      await expectRevertWithCustomError(yieldManager, call, "InsufficientWithdrawalReserve");
    });
    it("Should successfully accept cross-chain user transfer via receiveFundsFromReserve", async () => {
      // Arrange - Setup L1MessageService balance
      const transferAmount = ONE_ETHER;
      await setWithdrawalReserveToMinimum(yieldManager);
      await incrementBalance(l1MessageServiceAddress, transferAmount);
      // Arrange - setup L1MessageService message
      const recipientAddress = nonAuthorizedAccount.address;
      const calldata = encodeFunctionData({ abi: yieldManager.abi, functionName: "receiveFundsFromReserve" });
      const reserveBalanceBefore = await getBalance(linethRollup);
      const yieldManagerBalanceBefore = await getBalance(yieldManager);

      const claimParams = await setupLinethRollupMessageMerkleTree(
        linethRollup,
        recipientAddress,
        yieldManagerAddress,
        transferAmount,
        calldata,
        securityCouncil,
      );

      // Act
      const claimCall = linethRollup.write.claimMessageWithProof([claimParams], {
        account: nonAuthorizedAccount.account,
      });
      await claimCall;
      expect(await getBalance(linethRollup)).eq(reserveBalanceBefore - transferAmount);
      expect(await getBalance(yieldManager)).eq(yieldManagerBalanceBefore + transferAmount);
    });
  });

  describe("Withdraw LST", () => {
    it("Should allow LST withdrawal when in deficit", async () => {
      // Arrange - setup user funds
      const initialFundAmount = ONE_ETHER;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, initialFundAmount);
      // Arrange - setup withdrawal reserve deficit
      await setBalance(l1MessageServiceAddress, BigInt(ZERO_VALUE));
      // Arrange - setup L1MessageService message
      const withdrawAmount = initialFundAmount / 2n;
      const recipientAddress = nonAuthorizedAccount.address;
      const claimParams = await setupLinethRollupMessageMerkleTree(
        linethRollup,
        recipientAddress,
        recipientAddress,
        withdrawAmount,
        EMPTY_CALLDATA,
        securityCouncil,
      );
      // Arrange - Get before figures
      const lstPrincipalBefore = await yieldManager.read.getYieldProviderLstLiabilityPrincipal([yieldProviderAddress]);

      // Act
      const claimCall = linethRollup.write.claimMessageWithProofAndWithdrawLST([claimParams, yieldProviderAddress], {
        account: nonAuthorizedAccount.account,
      });

      // Assert
      await claimCall;
      const lstPrincipalAfter = await yieldManager.read.getYieldProviderLstLiabilityPrincipal([yieldProviderAddress]);
      expect(lstPrincipalAfter).eq(lstPrincipalBefore + withdrawAmount);
    });
    it("Should not be allowed from claimMessage", async () => {
      // Arrange - setup user funds
      const initialFundAmount = ONE_ETHER;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, initialFundAmount);
      // Arrange - setup withdrawal reserve deficit
      await setBalance(l1MessageServiceAddress, BigInt(ZERO_VALUE));
      // Arrange - setup L1MessageService message
      const withdrawAmount = 0n;
      const recipientAddress = nonAuthorizedAccount.address;
      const calldata = encodeFunctionData({
        abi: yieldProvider.abi,
        functionName: "withdrawLST",
        args: [yieldProviderAddress, withdrawAmount, recipientAddress],
      });

      const claimParams = await setupLinethRollupMessageMerkleTree(
        linethRollup,
        recipientAddress,
        yieldManagerAddress,
        withdrawAmount,
        calldata,
        securityCouncil,
      );

      // Act
      const claimCall = linethRollup.write.claimMessageWithProof([claimParams], {
        account: nonAuthorizedAccount.account,
      });
      await expectRevertWithCustomError(yieldManager, claimCall, "LSTWithdrawalNotAllowed");
    });
    it("Should not allow LST withdrawal amount > fund amount", async () => {
      // Arrange - setup user funds
      const initialFundAmount = ONE_ETHER;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, initialFundAmount);
      // Arrange - setup withdrawal reserve deficit
      await setBalance(l1MessageServiceAddress, BigInt(ZERO_VALUE));
      // Arrange - setup L1MessageService message
      const withdrawAmount = initialFundAmount * 2n + CONNECT_DEPOSIT;
      const recipientAddress = nonAuthorizedAccount.address;
      const claimParams = await setupLinethRollupMessageMerkleTree(
        linethRollup,
        recipientAddress,
        recipientAddress,
        withdrawAmount,
        EMPTY_CALLDATA,
        securityCouncil,
      );

      // Act
      const claimCall = linethRollup.write.claimMessageWithProofAndWithdrawLST([claimParams, yieldProviderAddress], {
        account: nonAuthorizedAccount.account,
      });
      await expectRevertWithCustomError(yieldManager, claimCall, "LSTWithdrawalExceedsYieldProviderFunds");
    });
    it("Should revert if LST withdrawal is for another recipient", async () => {
      // Arrange - setup user funds
      const initialFundAmount = ONE_ETHER;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, initialFundAmount);
      // Arrange - setup withdrawal reserve deficit
      await setBalance(l1MessageServiceAddress, BigInt(ZERO_VALUE));
      // Arrange - setup L1MessageService message
      const withdrawAmount = initialFundAmount / 2n;
      const recipientAddress = nonAuthorizedAccount.address;
      const claimParams = await setupLinethRollupMessageMerkleTree(
        linethRollup,
        recipientAddress,
        recipientAddress,
        withdrawAmount,
        EMPTY_CALLDATA,
        securityCouncil,
      );

      // Act - Execute with different sender than _to
      const claimCall = linethRollup.write.claimMessageWithProofAndWithdrawLST([claimParams, yieldProviderAddress], {
        account: l2YieldRecipient.account,
      });

      // Assert
      await expectRevertWithCustomError(linethRollup, claimCall, "CallerNotLSTWithdrawalRecipient");
    });
    it("Should revert if LST withdrawal > rate limit", async () => {
      const rateLimit = await linethRollup.read.limitInWei();

      const recipientAddress = nonAuthorizedAccount.address;
      const claimParams = await setupLinethRollupMessageMerkleTree(
        linethRollup,
        recipientAddress,
        recipientAddress,
        rateLimit + 1n,
        EMPTY_CALLDATA,
        securityCouncil,
      );
      const call = linethRollup.write.claimMessageWithProofAndWithdrawLST([claimParams, yieldProviderAddress], {
        account: nonAuthorizedAccount.account,
      });
      await expectRevertWithCustomError(linethRollup, call, "RateLimitExceeded");
    });
    it("LST withdrawal should use previous rate limit", async () => {
      // Regular claim message for rate limit
      const rateLimit = await linethRollup.read.limitInWei();
      await incrementBalance(l1MessageServiceAddress, rateLimit);
      const recipientAddress = nonAuthorizedAccount.address;
      const claimParams = await setupLinethRollupMessageMerkleTree(
        linethRollup,
        recipientAddress,
        recipientAddress,
        rateLimit,
        EMPTY_CALLDATA,
        securityCouncil,
      );
      await linethRollup.write.claimMessageWithProof([claimParams], { account: nonAuthorizedAccount.account });
      // Next claim message with LST withdrawal
      const claimParams2 = await setupLinethRollupMessageMerkleTree(
        linethRollup,
        recipientAddress,
        recipientAddress,
        1n,
        EMPTY_CALLDATA,
        securityCouncil,
      );
      const call = linethRollup.write.claimMessageWithProofAndWithdrawLST([claimParams2, yieldProviderAddress], {
        account: nonAuthorizedAccount.account,
      });
      await expectRevertWithCustomError(linethRollup, call, "RateLimitExceeded");
    });
  });

  describe("Yield reporting", () => {
    it("Should report positive yield successfully", async () => {
      // Arrange - setup user funds
      const initialFundAmount = ONE_ETHER;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, initialFundAmount);
      // Arrange - Setup positive yield
      const yieldEarned = ONE_ETHER / 10n;
      await mockDashboard.write.setTotalValueReturn([initialFundAmount + CONNECT_DEPOSIT + yieldEarned]);
      // Arrange - Get message params
      const nextMessageNumberBefore = await linethRollup.read.nextMessageNumber();

      const expectedBytes = encodeSendMessage(
        yieldManagerAddress,
        l2YieldRecipient.address,
        0n,
        yieldEarned,
        nextMessageNumberBefore,
        EMPTY_CALLDATA,
      );
      const messageHash = keccak256(expectedBytes);

      // Act
      const [newReportedYield, outstandingNegativeYield] = (
        await yieldManager.simulate.reportYield([yieldProviderAddress, l2YieldRecipient.address], {
          account: nativeYieldOperator.address,
        })
      ).result;
      const call = await yieldManager.write.reportYield([yieldProviderAddress, l2YieldRecipient.address], {
        account: nativeYieldOperator.account,
      });

      // Assert
      await expectEvent(linethRollup, Promise.resolve(call), "MessageSent", [
        yieldManagerAddress,
        l2YieldRecipient.address,
        0n,
        yieldEarned,
        nextMessageNumberBefore,
        EMPTY_CALLDATA,
        messageHash,
      ]);
      expect(newReportedYield).eq(yieldEarned);
      expect(outstandingNegativeYield).eq(0n);
    });
    it("Should report negative yield successfully", async () => {
      // Arrange - setup user funds
      const initialFundAmount = ONE_ETHER;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, initialFundAmount);
      // Arrange - Setup negative yield
      const yieldEarned = 0n;
      await mockDashboard.write.setTotalValueReturn([yieldEarned + CONNECT_DEPOSIT]);
      // Arrange - Get message params
      const nextMessageNumberBefore = await linethRollup.read.nextMessageNumber();

      const expectedBytes = encodeSendMessage(
        yieldManagerAddress,
        l2YieldRecipient.address,
        0n,
        yieldEarned,
        nextMessageNumberBefore,
        EMPTY_CALLDATA,
      );
      const messageHash = keccak256(expectedBytes);

      // Act
      const [newReportedYield, outstandingNegativeYield] = (
        await yieldManager.simulate.reportYield([yieldProviderAddress, l2YieldRecipient.address], {
          account: nativeYieldOperator.address,
        })
      ).result;
      const call = await yieldManager.write.reportYield([yieldProviderAddress, l2YieldRecipient.address], {
        account: nativeYieldOperator.account,
      });

      // Assert
      await expectEvent(linethRollup, Promise.resolve(call), "MessageSent", [
        yieldManagerAddress,
        l2YieldRecipient.address,
        0n,
        yieldEarned,
        nextMessageNumberBefore,
        EMPTY_CALLDATA,
        messageHash,
      ]);
      expect(newReportedYield).eq(0n);
      expect(outstandingNegativeYield).eq(initialFundAmount - yieldEarned);
    });
    it("Immediate subsequent positive yield report with no fund movement should have 0 incremental earned yield", async () => {
      // Arrange - setup user funds
      const initialFundAmount = ONE_ETHER;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, initialFundAmount);
      // Arrange - Setup positive yield
      const yieldEarned = ONE_ETHER / 10n;
      await mockDashboard.write.setTotalValueReturn([initialFundAmount + CONNECT_DEPOSIT + yieldEarned]);
      // Arrange - Get message params
      const nextMessageNumberBefore = await linethRollup.read.nextMessageNumber();

      const expectedBytes = encodeSendMessage(
        yieldManagerAddress,
        l2YieldRecipient.address,
        0n,
        yieldEarned,
        nextMessageNumberBefore,
        EMPTY_CALLDATA,
      );
      const messageHash = keccak256(expectedBytes);

      // Act
      const [newReportedYield, outstandingNegativeYield] = (
        await yieldManager.simulate.reportYield([yieldProviderAddress, l2YieldRecipient.address], {
          account: nativeYieldOperator.address,
        })
      ).result;
      const call = await yieldManager.write.reportYield([yieldProviderAddress, l2YieldRecipient.address], {
        account: nativeYieldOperator.account,
      });
      const [newReportedYield2, outstandingNegativeYield2] = (
        await yieldManager.simulate.reportYield([yieldProviderAddress, l2YieldRecipient.address], {
          account: nativeYieldOperator.address,
        })
      ).result;

      // Assert
      await expectEvent(linethRollup, Promise.resolve(call), "MessageSent", [
        yieldManagerAddress,
        l2YieldRecipient.address,
        0n,
        yieldEarned,
        nextMessageNumberBefore,
        EMPTY_CALLDATA,
        messageHash,
      ]);
      expect(newReportedYield).eq(yieldEarned);
      expect(outstandingNegativeYield).eq(0n);
      expect(newReportedYield2).eq(0n);
      expect(outstandingNegativeYield2).eq(0n);
    });
    it("Immediate subsequent negative yield report with no fund movement should have same outstanding negative yield", async () => {
      // Arrange - setup user funds
      const initialFundAmount = ONE_ETHER;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, initialFundAmount);
      // Arrange - Setup negative yield
      const yieldEarned = 0n;
      await mockDashboard.write.setTotalValueReturn([yieldEarned + CONNECT_DEPOSIT]);
      // Arrange - Get message params
      const nextMessageNumberBefore = await linethRollup.read.nextMessageNumber();

      const expectedBytes = encodeSendMessage(
        yieldManagerAddress,
        l2YieldRecipient.address,
        0n,
        yieldEarned,
        nextMessageNumberBefore,
        EMPTY_CALLDATA,
      );
      const messageHash = keccak256(expectedBytes);

      // Act
      const [newReportedYield, outstandingNegativeYield] = (
        await yieldManager.simulate.reportYield([yieldProviderAddress, l2YieldRecipient.address], {
          account: nativeYieldOperator.address,
        })
      ).result;
      const call = await yieldManager.write.reportYield([yieldProviderAddress, l2YieldRecipient.address], {
        account: nativeYieldOperator.account,
      });
      const [newReportedYield2, outstandingNegativeYield2] = (
        await yieldManager.simulate.reportYield([yieldProviderAddress, l2YieldRecipient.address], {
          account: nativeYieldOperator.address,
        })
      ).result;

      // Assert
      await expectEvent(linethRollup, Promise.resolve(call), "MessageSent", [
        yieldManagerAddress,
        l2YieldRecipient.address,
        0n,
        yieldEarned,
        nextMessageNumberBefore,
        EMPTY_CALLDATA,
        messageHash,
      ]);
      expect(newReportedYield).eq(0n);
      expect(outstandingNegativeYield).eq(initialFundAmount - yieldEarned);
      expect(newReportedYield2).eq(0n);
      expect(outstandingNegativeYield2).eq(outstandingNegativeYield);
    });
  });

  describe("Migrating yield providers", () => {
    it("Cannot remove a yield provider with funds", async () => {
      // Arrange - fund a single vault
      const initialFundAmount = ONE_ETHER;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, initialFundAmount);
      // Arrange - single positive vault report
      const yieldEarned = ONE_ETHER;
      await mockDashboard.write.setTotalValueReturn([initialFundAmount + yieldEarned]);
      await yieldManager.write.reportYield([yieldProviderAddress, l2YieldRecipient.address], {
        account: nativeYieldOperator.account,
      });

      // Act
      const call = yieldManager.write.removeYieldProvider([yieldProviderAddress, EMPTY_CALLDATA], {
        account: securityCouncil.account,
      });

      // Assert
      await expectRevertWithCustomError(yieldManager, call, "YieldProviderHasRemainingFunds", [
        initialFundAmount + yieldEarned,
      ]);
    });
    it("Can migrate funds successfully to a new YieldProvider", async () => {
      // Arrange - fund a single vault
      const initialFundAmount = ONE_ETHER;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, initialFundAmount);
      // Arrange - single positive vault report
      const yieldEarned = ONE_ETHER;
      await mockDashboard.write.setTotalValueReturn([initialFundAmount + yieldEarned]);
      await yieldManager.write.reportYield([yieldProviderAddress, l2YieldRecipient.address], {
        account: nativeYieldOperator.account,
      });
      // Arrange - withdraw vault funds
      await incrementBalance(mockStakingVault.address, ONE_ETHER);
      await setWithdrawalReserveToTarget(yieldManager);
      await mockDashboard.write.setWithdrawableValueReturn([ONE_ETHER * 2n]);
      await yieldManager.write.safeWithdrawFromYieldProvider([yieldProviderAddress, ONE_ETHER * 2n], {
        account: nativeYieldOperator.account,
      });
      // Arrange - Remove yield provider
      await yieldManager.write.removeYieldProvider([yieldProviderAddress, buildVendorExitData()], {
        account: securityCouncil.account,
      });
      // Arrange - Add new yield provider
      const { yieldProviderAddress: yieldProvider2Address } = await deployAndAddAdditionalLidoStVaultYieldProvider(
        lidoStVaultYieldProviderFactory,
        yieldManager,
        securityCouncil,
        mockVaultFactory,
      );

      // Act - Move funds to new YieldProvider
      await yieldManager.write.fundYieldProvider([yieldProvider2Address, ONE_ETHER], {
        account: nativeYieldOperator.account,
      });
    });
  });

  describe("Withdrawals", () => {
    it("safeAddToWithdrawalReserve should paydown LST liability", async () => {
      // Arrange - initial 10 ETH fund
      const initialFundAmount = ONE_ETHER * 10n;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, initialFundAmount);
      // Arrange - 5 ETH LST withdrawal
      await setBalance(l1MessageServiceAddress, BigInt(ZERO_VALUE));
      const withdrawLSTAmount = ONE_ETHER * 5n;
      const recipientAddress = nonAuthorizedAccount.address;
      const claimParams = await setupLinethRollupMessageMerkleTree(
        linethRollup,
        recipientAddress,
        recipientAddress,
        withdrawLSTAmount,
        EMPTY_CALLDATA,
        securityCouncil,
      );
      await linethRollup.write.claimMessageWithProofAndWithdrawLST([claimParams, yieldProviderAddress], {
        account: nonAuthorizedAccount.account,
      });
      // Arrange LST liability paydown
      await setupMaxLSTLiabilityPaymentForWithdrawal(
        yieldManager,
        mockDashboard,
        mockVaultHub,
        mockSTETH,
        yieldProviderAddress,
        withdrawLSTAmount,
      );
      // Arrange withdrawal
      const withdrawAmount = ONE_ETHER * 5n;
      await mockDashboard.write.setWithdrawableValueReturn([withdrawAmount]);
      // Arrange - Before figures
      const lstLiabilityPrincipalBefore = await yieldManager.read.getYieldProviderLstLiabilityPrincipal([
        yieldProviderAddress,
      ]);

      // Act
      await yieldManager.write.safeAddToWithdrawalReserve([yieldProviderAddress, withdrawAmount], {
        account: nativeYieldOperator.account,
      });

      // Assert
      expect(await yieldManager.read.getYieldProviderLstLiabilityPrincipal([yieldProviderAddress])).eq(
        lstLiabilityPrincipalBefore - withdrawAmount,
      );
    });
    it("safeWithdrawFromYieldProvider should paydown LST liability", async () => {
      // Arrange - initial 10 ETH fund
      const initialFundAmount = ONE_ETHER * 10n;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, initialFundAmount);
      // Arrange - 5 ETH LST withdrawal
      await setBalance(l1MessageServiceAddress, BigInt(ZERO_VALUE));
      const withdrawLSTAmount = ONE_ETHER * 5n;
      const recipientAddress = nonAuthorizedAccount.address;
      const claimParams = await setupLinethRollupMessageMerkleTree(
        linethRollup,
        recipientAddress,
        recipientAddress,
        withdrawLSTAmount,
        EMPTY_CALLDATA,
        securityCouncil,
      );
      await linethRollup.write.claimMessageWithProofAndWithdrawLST([claimParams, yieldProviderAddress], {
        account: nonAuthorizedAccount.account,
      });
      // Arrange LST liability paydown
      await setupMaxLSTLiabilityPaymentForWithdrawal(
        yieldManager,
        mockDashboard,
        mockVaultHub,
        mockSTETH,
        yieldProviderAddress,
        withdrawLSTAmount,
      );
      // Arrange withdrawal
      const withdrawAmount = ONE_ETHER * 5n;
      await mockDashboard.write.setWithdrawableValueReturn([withdrawAmount]);
      // Arrange - Before figures
      const lstLiabilityPrincipalBefore = await yieldManager.read.getYieldProviderLstLiabilityPrincipal([
        yieldProviderAddress,
      ]);

      // Act
      await yieldManager.write.safeWithdrawFromYieldProvider([yieldProviderAddress, withdrawAmount], {
        account: nativeYieldOperator.account,
      });

      // Assert
      expect(await yieldManager.read.getYieldProviderLstLiabilityPrincipal([yieldProviderAddress])).eq(
        lstLiabilityPrincipalBefore - withdrawAmount,
      );
    });
    it("replenishWithdrawalReserve should not paydown LST liability", async () => {
      // Arrange - initial 10 ETH fund
      const initialFundAmount = ONE_ETHER * 10n;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, initialFundAmount);
      // Arrange - 5 ETH LST withdrawal
      await setBalance(l1MessageServiceAddress, BigInt(ZERO_VALUE));
      const withdrawLSTAmount = ONE_ETHER * 5n;
      const recipientAddress = nonAuthorizedAccount.address;
      const claimParams = await setupLinethRollupMessageMerkleTree(
        linethRollup,
        recipientAddress,
        recipientAddress,
        withdrawLSTAmount,
        EMPTY_CALLDATA,
        securityCouncil,
      );
      await linethRollup.write.claimMessageWithProofAndWithdrawLST([claimParams, yieldProviderAddress], {
        account: nonAuthorizedAccount.account,
      });
      // Arrange LST liability paydown
      await setupMaxLSTLiabilityPaymentForWithdrawal(
        yieldManager,
        mockDashboard,
        mockVaultHub,
        mockSTETH,
        yieldProviderAddress,
        withdrawLSTAmount,
      );
      // Arrange withdrawal
      const withdrawAmount = ONE_ETHER * 5n;
      await mockDashboard.write.setWithdrawableValueReturn([withdrawAmount]);
      // Arrange - Before figures
      const lstLiabilityPrincipalBefore = await yieldManager.read.getYieldProviderLstLiabilityPrincipal([
        yieldProviderAddress,
      ]);

      // Act
      await yieldManager.write.replenishWithdrawalReserve([yieldProviderAddress]);

      // Assert
      expect(await yieldManager.read.getYieldProviderLstLiabilityPrincipal([yieldProviderAddress])).eq(
        lstLiabilityPrincipalBefore,
      );
    });
  });

  describe("Native yield resilience scenarios", () => {
    it("Should not reduce user funds after repeated negative yield reports", async () => {
      // Arrange - setup user funds
      const initialFundAmount = ONE_ETHER * 10n;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, initialFundAmount);
      const userFundsInYieldProvidersTotalBefore = await yieldManager.read.userFundsInYieldProvidersTotal();
      const userFundsBefore = await yieldManager.read.userFunds([yieldProviderAddress]);
      // Arrange - Setup first negative yield
      const firstNegativeYield = ONE_ETHER * 5n;
      const secondNegativeYield = ONE_ETHER * 5n;
      await mockDashboard.write.setTotalValueReturn([
        initialFundAmount + CONNECT_DEPOSIT - firstNegativeYield - secondNegativeYield,
      ]);

      // Act
      const [newReportedYield, outstandingNegativeYield] = (
        await yieldManager.simulate.reportYield([yieldProviderAddress, l2YieldRecipient.address], {
          account: nativeYieldOperator.address,
        })
      ).result;
      await yieldManager.write.reportYield([yieldProviderAddress, l2YieldRecipient.address], {
        account: nativeYieldOperator.account,
      });

      // Assert
      expect(await yieldManager.read.userFundsInYieldProvidersTotal()).eq(userFundsInYieldProvidersTotalBefore);
      expect(await yieldManager.read.userFunds([yieldProviderAddress])).eq(userFundsBefore);
      expect(newReportedYield).eq(0n);
      expect(outstandingNegativeYield).eq(firstNegativeYield + secondNegativeYield);
    });

    // Test on Hoodi - Because not easy to test reportYield() paying LST liability scenario.
    // STETH.getPooledEthBySharesRoundUp - needs to be mocked to return two different values within the same function call.
    it.skip("Should be able to recover after temporary negative yield and max LST withdrawal", async () => {
      // Initial funding with 10 ETH
      const initialFundAmount = ONE_ETHER * 10n;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, initialFundAmount);

      // Negative yield event for -5 ETH
      const firstNegativeYield = ONE_ETHER * 5n;
      await mockDashboard.write.setTotalValueReturn([initialFundAmount - firstNegativeYield]);
      await decrementBalance(mockStakingVaultAddress, firstNegativeYield);
      {
        const [newReportedYield, outstandingNegativeYield] = (
          await yieldManager.simulate.reportYield([yieldProviderAddress, l2YieldRecipient.address], {
            account: nativeYieldOperator.address,
          })
        ).result;
        expect(newReportedYield).eq(0n);
        expect(outstandingNegativeYield).eq(firstNegativeYield);
      }
      await yieldManager.write.reportYield([yieldProviderAddress, l2YieldRecipient.address], {
        account: nativeYieldOperator.account,
      });

      // Drain all L1MessageService funds
      await setBalance(l1MessageServiceAddress, BigInt(ZERO_VALUE));

      // Max LST withdrawal for 5 ETH
      await linethRollup.write.resetRateLimitAmount([ONE_ETHER * 100n], { account: securityCouncil.account });
      const withdrawLSTAmount = ONE_ETHER * 5n;
      const recipientAddress = nonAuthorizedAccount.address;
      const claimParams = await setupLinethRollupMessageMerkleTree(
        linethRollup,
        recipientAddress,
        recipientAddress,
        withdrawLSTAmount,
        EMPTY_CALLDATA,
        securityCouncil,
      );
      await linethRollup.write.claimMessageWithProofAndWithdrawLST([claimParams, yieldProviderAddress], {
        account: nonAuthorizedAccount.account,
      });
      await mockSTETH.write.setPooledEthBySharesRoundUpReturn([withdrawLSTAmount]);

      // Assert - Cannot do another LST withdrawal
      const claimParams2 = await setupLinethRollupMessageMerkleTree(
        linethRollup,
        recipientAddress,
        recipientAddress,
        1n,
        EMPTY_CALLDATA,
        securityCouncil,
      );
      const secondWithdrawLSTCall = linethRollup.write.claimMessageWithProofAndWithdrawLST(
        [claimParams2, yieldProviderAddress],
        { account: nonAuthorizedAccount.account },
      );
      await expectRevertWithCustomError(yieldManager, secondWithdrawLSTCall, "LSTWithdrawalExceedsYieldProviderFunds");
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.replenishWithdrawalReserve([yieldProviderAddress]),
        "NoAvailableFundsToReplenishWithdrawalReserve",
      );

      // Setup max liability payment
      await setupLSTPrincipalDecrementForPaxMaximumPossibleLSTLiability(
        withdrawLSTAmount,
        yieldManager,
        yieldProviderAddress,
        mockSTETH,
        mockDashboard,
      );
      // Earn 7 ETH positive yield
      const firstPositiveYield = ONE_ETHER * 7n;
      await incrementBalance(mockStakingVaultAddress, firstPositiveYield);
      let stakingVaultBalance = await getBalance({ address: mockStakingVaultAddress });
      await mockDashboard.write.setTotalValueReturn([stakingVaultBalance]);
      let userFunds = await yieldManager.read.userFunds([yieldProviderAddress]);
      {
        const [newReportedYield, outstandingNegativeYield] = (
          await yieldManager.simulate.reportYield([yieldProviderAddress, l2YieldRecipient.address], {
            account: nativeYieldOperator.address,
          })
        ).result;
        expect(newReportedYield).eq(0n);
        expect(outstandingNegativeYield).eq(firstNegativeYield + withdrawLSTAmount - firstPositiveYield);
      }
      await yieldManager.write.reportYield([yieldProviderAddress, l2YieldRecipient.address], {
        account: nativeYieldOperator.account,
      });
      expect(await yieldManager.read.userFunds([yieldProviderAddress])).eq(
        userFunds + firstPositiveYield - firstNegativeYield,
      );
      expect(await yieldManager.read.getYieldProviderYieldReportedCumulative([yieldProviderAddress])).eq(0n);
      expect(await yieldManager.read.userFundsInYieldProvidersTotal()).eq(
        await yieldManager.read.userFunds([yieldProviderAddress]),
      );
      console.log(await yieldManager.read.getYieldProviderLstLiabilityPrincipal([yieldProviderAddress]));

      // Add to withdrawal reserve (expect route all funds to L1MessageService)
      stakingVaultBalance = await getBalance({ address: mockStakingVaultAddress });
      userFunds = await yieldManager.read.userFunds([yieldProviderAddress]);
      await mockDashboard.write.setWithdrawableValueReturn([stakingVaultBalance]);
      const l1MessageBalance = await getBalance({ address: l1MessageServiceAddress });
      await yieldManager.write.safeAddToWithdrawalReserve([yieldProviderAddress, stakingVaultBalance], {
        account: nativeYieldOperator.account,
      });
      expect(await yieldManager.read.userFunds([yieldProviderAddress])).eq(0n);
      expect(await getBalance({ address: l1MessageServiceAddress })).eq(l1MessageBalance + userFunds);

      // Donation to replenish reserve to target
      await linethRollup.write.fund({
        account: securityCouncil.account,
        value: await yieldManager.read.getTargetReserveDeficit(),
      });

      // Fresh deposit of 10 ETH -> should clear all existing LST principal
      const secondFundAmount = ONE_ETHER * 10n;
      await linethRollup.write.transferFundsForNativeYield([secondFundAmount], {
        account: nativeYieldOperator.account,
      });
      await yieldManager.write.fundYieldProvider([yieldProviderAddress, secondFundAmount], {
        account: nativeYieldOperator.account,
      });

      expect(await yieldManager.read.getYieldProviderLstLiabilityPrincipal([yieldProviderAddress])).eq(0n);
    });

    it("Should be able to safely handle different yield report situations", async () => {
      // Initial funding with 10 ETH
      const initialFundAmount = ONE_ETHER * 10n;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, initialFundAmount);

      // First negative yield event for -5 ETH
      const firstNegativeYield = ONE_ETHER * 5n;
      await mockDashboard.write.setTotalValueReturn([initialFundAmount + CONNECT_DEPOSIT - firstNegativeYield]);
      await decrementBalance(mockStakingVaultAddress, firstNegativeYield);
      {
        const [newReportedYield, outstandingNegativeYield] = (
          await yieldManager.simulate.reportYield([yieldProviderAddress, l2YieldRecipient.address], {
            account: nativeYieldOperator.address,
          })
        ).result;
        expect(newReportedYield).eq(0n);
        expect(outstandingNegativeYield).eq(firstNegativeYield);
      }
      await yieldManager.write.reportYield([yieldProviderAddress, l2YieldRecipient.address], {
        account: nativeYieldOperator.account,
      });

      // Second negative yield of -2 ETH due to obligation settlement
      const firstObligationsPaid = ONE_ETHER * 2n;
      await mockDashboard.write.setObligationsFeesToSettleReturn([firstObligationsPaid]);
      const mockStakingVaultBalance = await getBalance(mockStakingVault);
      await mockVaultHub.write.setIsSettleLidoFeesWithdrawingFromVault([true]);
      await mockVaultHub.write.setSettleVaultObligationAmount([firstObligationsPaid]);
      {
        const [newReportedYield, outstandingNegativeYield] = (
          await yieldManager.simulate.reportYield([yieldProviderAddress, l2YieldRecipient.address], {
            account: nativeYieldOperator.address,
          })
        ).result;
        expect(newReportedYield).eq(0n);
        expect(outstandingNegativeYield).eq(firstNegativeYield + firstObligationsPaid);
      }
      await yieldManager.write.reportYield([yieldProviderAddress, l2YieldRecipient.address], {
        account: nativeYieldOperator.account,
      });
      expect(await yieldManager.read.userFunds([yieldProviderAddress])).eq(initialFundAmount + CONNECT_DEPOSIT);
      expect(await getBalance(mockStakingVault)).eq(mockStakingVaultBalance - firstObligationsPaid);
      await mockDashboard.write.setTotalValueReturn([(await mockDashboard.read.totalValue()) - firstObligationsPaid]);
      await mockDashboard.write.setObligationsFeesToSettleReturn([0n]);
      await mockVaultHub.write.setSettleVaultObligationAmount([0n]);

      // First positive yield of 4 ETH, not enough to recover from all negative yield incurred so far
      const firstPositiveYield = ONE_ETHER * 4n;
      await mockDashboard.write.setTotalValueReturn([(await mockDashboard.read.totalValue()) + firstPositiveYield]);
      await incrementBalance(mockStakingVaultAddress, firstPositiveYield);
      {
        const [newReportedYield, outstandingNegativeYield] = (
          await yieldManager.simulate.reportYield([yieldProviderAddress, l2YieldRecipient.address], {
            account: nativeYieldOperator.address,
          })
        ).result;
        expect(newReportedYield).eq(0n);
        expect(outstandingNegativeYield).eq(firstNegativeYield + firstObligationsPaid - firstPositiveYield);
      }
      await yieldManager.write.reportYield([yieldProviderAddress, l2YieldRecipient.address], {
        account: nativeYieldOperator.account,
      });
      expect(await yieldManager.read.getYieldProviderYieldReportedCumulative([yieldProviderAddress])).eq(0n);

      // Second positive yield of 4 ETH
      // +1 ETH in Vault relative to start
      const secondPositiveYield = ONE_ETHER * 4n;
      await mockDashboard.write.setTotalValueReturn([(await mockDashboard.read.totalValue()) + secondPositiveYield]);
      await incrementBalance(mockStakingVaultAddress, secondPositiveYield);
      {
        const [newReportedYield, outstandingNegativeYield] = (
          await yieldManager.simulate.reportYield([yieldProviderAddress, l2YieldRecipient.address], {
            account: nativeYieldOperator.address,
          })
        ).result;
        expect(newReportedYield).eq(ONE_ETHER);
        expect(outstandingNegativeYield).eq(0n);
      }
      await yieldManager.write.reportYield([yieldProviderAddress, l2YieldRecipient.address], {
        account: nativeYieldOperator.account,
      });
      expect(await yieldManager.read.getYieldProviderYieldReportedCumulative([yieldProviderAddress])).eq(ONE_ETHER);

      // Third positive yield of 3 ETH
      // -2 ETH of node operators fees
      // -3 ETH of protocol fees
      // Expect end with -2 ETH
      const thirdPositiveYield = ONE_ETHER * 3n;
      await mockDashboard.write.setTotalValueReturn([(await mockDashboard.read.totalValue()) + thirdPositiveYield]);
      await incrementBalance(mockStakingVaultAddress, thirdPositiveYield);
      await mockDashboard.write.setObligationsFeesToSettleReturn([ONE_ETHER * 3n]);
      await mockVaultHub.write.setIsSettleLidoFeesWithdrawingFromVault([true]);
      await mockVaultHub.write.setSettleVaultObligationAmount([ONE_ETHER * 3n]);
      await mockDashboard.write.setIsDisburseFeeWithdrawingFromVault([true]);
      await mockDashboard.write.setAccruedFeeReturn([ONE_ETHER * 2n]);
      {
        const [newReportedYield, outstandingNegativeYield] = (
          await yieldManager.simulate.reportYield([yieldProviderAddress, l2YieldRecipient.address], {
            account: nativeYieldOperator.address,
          })
        ).result;
        expect(newReportedYield).eq(0n);
        // Obligations was paid, node operator fees was not
        expect(outstandingNegativeYield).eq(ONE_ETHER * 2n);
      }
      await yieldManager.write.reportYield([yieldProviderAddress, l2YieldRecipient.address], {
        account: nativeYieldOperator.account,
      });
      await mockDashboard.write.setTotalValueReturn([(await mockDashboard.read.totalValue()) - ONE_ETHER * 5n]);
      await mockVaultHub.write.setSettleVaultObligationAmount([0n]);
      await mockDashboard.write.setObligationsFeesToSettleReturn([0n]);
      await mockDashboard.write.setAccruedFeeReturn([0n]);
      expect(await yieldManager.read.getYieldProviderYieldReportedCumulative([yieldProviderAddress])).eq(ONE_ETHER);

      // Fourth positive yield of 7 ETH
      // LST liability of -2 ETH
      await mockSTETH.write.setPooledEthBySharesRoundUpReturn([ONE_ETHER * 2n]);
      await mockDashboard.write.setRebalanceVaultWithSharesWithdrawingFromVault([true]);
      const fourthPositiveYield = ONE_ETHER * 7n;
      await mockDashboard.write.setTotalValueReturn([(await mockDashboard.read.totalValue()) + fourthPositiveYield]);
      await incrementBalance(mockStakingVaultAddress, fourthPositiveYield);
      {
        const [newReportedYield, outstandingNegativeYield] = (
          await yieldManager.simulate.reportYield([yieldProviderAddress, l2YieldRecipient.address], {
            account: nativeYieldOperator.address,
          })
        ).result;
        expect(newReportedYield).eq(ONE_ETHER * 3n);
        // Obligations was paid, node operator fees was not
        expect(outstandingNegativeYield).eq(0n);
      }
      await yieldManager.write.reportYield([yieldProviderAddress, l2YieldRecipient.address], {
        account: nativeYieldOperator.account,
      });
      await mockDashboard.write.setTotalValueReturn([(await mockDashboard.read.totalValue()) - ONE_ETHER * 2n]);
      await mockSTETH.write.setPooledEthBySharesRoundUpReturn([0n]);
      expect(await yieldManager.read.getYieldProviderYieldReportedCumulative([yieldProviderAddress])).eq(
        ONE_ETHER * 4n,
      );
    });

    it("Should be able to safely reach ossification state", async () => {
      // Initial funding with 10 ETH
      const initialFundAmount = ONE_ETHER * 10n;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, initialFundAmount);
      await incrementMockDashboardTotalValue(mockDashboard, initialFundAmount);

      // More bridge funds arrive
      const secondFundAmount = ONE_ETHER * 10n;
      await setWithdrawalReserveToTarget(yieldManager);
      await incrementBalance(l1MessageServiceAddress, secondFundAmount);
      await linethRollup.write.transferFundsForNativeYield([secondFundAmount], {
        account: nativeYieldOperator.account,
      });
      await yieldManager.write.fundYieldProvider([yieldProviderAddress, secondFundAmount], {
        account: nativeYieldOperator.account,
      });
      await incrementMockDashboardTotalValue(mockDashboard, secondFundAmount);

      // Earn 5 ETH positive yield
      const firstPositiveYield = ONE_ETHER * 5n;
      await mockDashboard.write.setTotalValueReturn([(await mockDashboard.read.totalValue()) + CONNECT_DEPOSIT]);
      await incurPositiveYield(
        yieldManager,
        mockDashboard,
        mockVaultHub,
        mockSTETH,
        nativeYieldOperator,
        mockStakingVaultAddress,
        yieldProviderAddress,
        l2YieldRecipient,
        firstPositiveYield,
      );
      // Negative yield event for -3 ETH
      const firstNegativeYield = ONE_ETHER * 3n;
      await incurNegativeYield(
        yieldManager,
        mockDashboard,
        mockVaultHub,
        mockSTETH,
        nativeYieldOperator,
        mockStakingVaultAddress,
        yieldProviderAddress,
        l2YieldRecipient,
        firstNegativeYield,
      );

      // Bridge funds decrement to deficit
      await setWithdrawalReserveToMinimum(yieldManager);
      await decrementBalance(l1MessageServiceAddress, ONE_ETHER * 10n);
      await expectRevertWithCustomError(
        yieldManager,
        linethRollup.write.transferFundsForNativeYield([1n], { account: nativeYieldOperator.account }),
        "InsufficientWithdrawalReserve",
      );

      // Create LST withdrawal - 5 ETH. Should cause staking pause.
      await setBalance(l1MessageServiceAddress, (await linethRollup.read.limitInWei()) - 1n);
      const lstWithdrawalAmount = await linethRollup.read.limitInWei();
      await withdrawLST(linethRollup, nonAuthorizedAccount, yieldProviderAddress, lstWithdrawalAmount, securityCouncil);
      expect(await yieldManager.read.isStakingPaused([yieldProviderAddress])).eq(true);

      // Do permissionless rebalance (should not paydown LST liability)
      const stakingVaultBalance = await getBalance(mockStakingVault);
      const l1MessageServiceBalance = await getBalance(linethRollup);
      const withdrawableValue = stakingVaultBalance - lstWithdrawalAmount; // Simulate locked balance
      await mockDashboard.write.setWithdrawableValueReturn([withdrawableValue]); // Simulate locked balance
      await setupMaxLSTLiabilityPaymentForWithdrawal(
        yieldManager,
        mockDashboard,
        mockVaultHub,
        mockSTETH,
        yieldProviderAddress,
        lstWithdrawalAmount,
      );
      await yieldManager.write.replenishWithdrawalReserve([yieldProviderAddress], {
        account: nonAuthorizedAccount.account,
      });
      expect(await yieldManager.read.getYieldProviderLstLiabilityPrincipal([yieldProviderAddress])).eq(
        lstWithdrawalAmount,
      );
      expect(await getBalance(mockStakingVault)).eq(lstWithdrawalAmount);
      await mockDashboard.write.setTotalValueReturn([0n]);
      expect(await getBalance(linethRollup)).eq(l1MessageServiceBalance + withdrawableValue);

      // Call unstakePermissionless
      await executeUnstakePermissionless(
        sszMerkleTree,
        testVerifier,
        yieldManager,
        yieldProviderAddress,
        mockStakingVaultAddress,
        nativeYieldOperator.address,
      );

      // Some more positive yield
      const secondPositiveYield = ONE_ETHER * 10n;
      await incurPositiveYield(
        yieldManager,
        mockDashboard,
        mockVaultHub,
        mockSTETH,
        nativeYieldOperator,
        mockStakingVaultAddress,
        yieldProviderAddress,
        l2YieldRecipient,
        secondPositiveYield,
      );

      // Kick off ossification
      await yieldManager.write.initiateOssification([yieldProviderAddress], { account: securityCouncil.account });

      // Complete ossification
      await mockVaultHub.write.setIsVaultConnectedReturn([false]);
      await yieldManager.write.progressPendingOssification([yieldProviderAddress], {
        account: nativeYieldOperator.account,
      });
    });
  });
});
