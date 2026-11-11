import { expect } from "chai";
import { network } from "hardhat";
import { describe, it, before, beforeEach } from "node:test";

import {
  ONE_ETHER,
  OperationType,
  ZERO_VALUE,
  YieldProviderVendor,
  CONNECT_DEPOSIT,
} from "../../common/constants/index.js";
import {
  type TestAccount,
  expectNoEvent,
  getAccountsFixture,
  expectEvent,
  expectRevertWithCustomError,
} from "../../common/helpers/index.js";
import {
  deployAndAddSingleLidoStVaultYieldProvider,
  fundLidoStVaultYieldProvider,
  getBalance,
  getWithdrawLSTCall,
} from "../helpers/index.js";

import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";
import type { Address } from "viem";

type MockSTETH = ContractReturnType<"MockSTETH">;
type MockLinethRollup = ContractReturnType<"MockLinethRollup">;
type TestYieldManager = ContractReturnType<"TestYieldManager">;
type MockDashboard = ContractReturnType<"MockDashboard">;
type MockStakingVault = ContractReturnType<"MockStakingVault">;
type TestLidoStVaultYieldProvider = ContractReturnType<"TestLidoStVaultYieldProvider">;

describe("LidoStVaultYieldProvider contract - yield operations", () => {
  let yieldProvider: TestLidoStVaultYieldProvider;
  let nativeYieldOperator: TestAccount;
  let securityCouncil: TestAccount;
  let l2YieldRecipient: TestAccount;
  let mockSTETH: MockSTETH;
  let mockLinethRollup: MockLinethRollup;
  let yieldManager: TestYieldManager;
  let mockDashboard: MockDashboard;
  let mockStakingVault: MockStakingVault;

  let mockStakingVaultAddress: Address;
  let yieldProviderAddress: Address;
  let l2YieldRecipientAddress: Address;

  before(async () => {
    const { networkHelpers } = await network.getOrCreate();
    ({ nativeYieldOperator, securityCouncil, l2YieldRecipient } = await networkHelpers.loadFixture(getAccountsFixture));
    l2YieldRecipientAddress = l2YieldRecipient.address;
  });

  beforeEach(async () => {
    const { networkHelpers } = await network.getOrCreate();
    ({
      yieldProvider,
      yieldProviderAddress,
      mockDashboard,
      mockStakingVault,
      yieldManager,
      mockSTETH,
      mockLinethRollup,
    } = await networkHelpers.loadFixture(deployAndAddSingleLidoStVaultYieldProvider));

    mockStakingVaultAddress = mockStakingVault.address;
  });

  describe("syncExternalLiabilitySettlement", () => {
    it("If ETH value of Lido liabilityShares >= YieldManager liabilityPrincipal, no-op", async () => {
      // Arrange
      const liabilityShares = ONE_ETHER;
      const ethValueOfLidoLiabilityShares = ONE_ETHER * 2n;
      const liabilityPrincipalBefore = ONE_ETHER;
      await mockSTETH.write.setPooledEthBySharesRoundUpReturn([ethValueOfLidoLiabilityShares], {
        account: securityCouncil.account,
      });
      await yieldManager.write.setYieldProviderLstLiabilityPrincipal([yieldProviderAddress, liabilityPrincipalBefore], {
        account: securityCouncil.account,
      });

      const userFundsInYieldProvidersTotalBefore = await yieldManager.read.userFundsInYieldProvidersTotal();
      const userFundsBefore = await yieldManager.read.userFunds([yieldProvider.address]);
      await yieldManager.write.setYieldProviderLstLiabilityPrincipal([yieldProvider.address, liabilityPrincipalBefore]);

      // Act
      const lstLiabilityPrincipalSynced = (
        await yieldManager.simulate.syncExternalLiabilitySettlement(
          [yieldProviderAddress, liabilityShares, liabilityPrincipalBefore],
          { account: securityCouncil.address },
        )
      ).result;
      await expectNoEvent(
        yieldManager,
        yieldManager.write.syncExternalLiabilitySettlement(
          [yieldProviderAddress, liabilityShares, liabilityPrincipalBefore],
          { account: securityCouncil.account },
        ),
        "LSTLiabilityPrincipalSynced",
      );

      // Assert
      expect(lstLiabilityPrincipalSynced).eq(liabilityPrincipalBefore);
      expect(await yieldManager.read.userFundsInYieldProvidersTotal()).eq(userFundsInYieldProvidersTotalBefore);
      expect(await yieldManager.read.userFunds([yieldProvider.address])).eq(userFundsBefore);
      expect(await yieldManager.read.getYieldProviderLstLiabilityPrincipal([yieldProvider.address])).eq(
        liabilityPrincipalBefore,
      );
    });
    it("If ETH value of Lido liabilityShares < YieldManager liabilityPrincipal, will decrement liabilityPrincipal to sync", async () => {
      // Arrange liabilityShares and ETH value
      const liabilityShares = BigInt(ZERO_VALUE);
      const ethValueOfLidoLiabilityShares = BigInt(ZERO_VALUE);
      await mockSTETH.write.setPooledEthBySharesRoundUpReturn([ethValueOfLidoLiabilityShares], {
        account: securityCouncil.account,
      });
      // Arrange - Set up userFunds and lstLiabilityPrincipals
      const liabilityPrincipalBefore = ONE_ETHER * 2n;
      const call = getWithdrawLSTCall(
        mockLinethRollup,
        yieldManager,
        yieldProvider,
        nativeYieldOperator,
        liabilityPrincipalBefore,
      );
      await call;

      const userFundsInYieldProvidersTotalBefore = await yieldManager.read.userFundsInYieldProvidersTotal();
      const userFundsBefore = await yieldManager.read.userFunds([yieldProvider.address]);
      expect(await yieldManager.read.getYieldProviderLstLiabilityPrincipal([yieldProvider.address])).eq(
        liabilityPrincipalBefore,
      );

      // Act
      const lstLiabilityPrincipalSynced = (
        await yieldManager.simulate.syncExternalLiabilitySettlement(
          [yieldProviderAddress, liabilityShares, liabilityPrincipalBefore],
          { account: securityCouncil.address },
        )
      ).result;
      const yieldProviderIndex = await yieldManager.read.getYieldProviderIndex([yieldProviderAddress]);
      await expectEvent(
        yieldManager,
        yieldManager.write.syncExternalLiabilitySettlement(
          [yieldProviderAddress, liabilityShares, liabilityPrincipalBefore],
          { account: securityCouncil.account },
        ),
        "LSTLiabilityPrincipalSynced",
        [
          YieldProviderVendor.LIDO_ST_VAULT_YIELD_PROVIDER_VENDOR,
          yieldProviderIndex,
          liabilityPrincipalBefore,
          ethValueOfLidoLiabilityShares,
        ],
      );

      // Assert
      expect(lstLiabilityPrincipalSynced).eq(ethValueOfLidoLiabilityShares);
      const lstLiabilityPrincipalDecrement = liabilityPrincipalBefore - BigInt(ethValueOfLidoLiabilityShares);
      expect(await yieldManager.read.userFundsInYieldProvidersTotal()).eq(userFundsInYieldProvidersTotalBefore);
      expect(await yieldManager.read.userFunds([yieldProvider.address])).eq(userFundsBefore);
      expect(await yieldManager.read.getYieldProviderLstLiabilityPrincipal([yieldProvider.address])).eq(
        liabilityPrincipalBefore - lstLiabilityPrincipalDecrement,
      );
    });
  });

  describe("syncLSTLiabilityPrincipal", () => {
    it("Should revert if not invoked via delegatecall", async () => {
      await expectRevertWithCustomError(
        yieldProvider,
        yieldProvider.write.syncLSTLiabilityPrincipal([yieldProviderAddress]),
        "ContextIsNotYieldManager",
      );
    });
    it("If ETH value of Lido liabilityShares >= YieldManager liabilityPrincipal, no-op", async () => {
      // Arrange
      const liabilityPrincipalBefore = ONE_ETHER;
      await yieldManager.write.setYieldProviderLstLiabilityPrincipal([yieldProviderAddress, liabilityPrincipalBefore], {
        account: securityCouncil.account,
      });
      await yieldManager.write.setYieldProviderLstLiabilityPrincipal([yieldProvider.address, liabilityPrincipalBefore]);
      const liabilityShares = ONE_ETHER;
      const ethValueOfLiabilityShares = ONE_ETHER * 2n;
      await mockDashboard.write.setLiabilitySharesReturn([liabilityShares], { account: securityCouncil.account });
      await mockSTETH.write.setPooledEthBySharesRoundUpReturn([ethValueOfLiabilityShares], {
        account: securityCouncil.account,
      });

      // Act
      await yieldManager.write.syncLSTLiabilityPrincipal([yieldProviderAddress], { account: securityCouncil.account });

      // Assert
      expect(await yieldManager.read.getYieldProviderLstLiabilityPrincipal([yieldProvider.address])).eq(
        liabilityPrincipalBefore,
      );
    });
    it("If ETH value of Lido liabilityShares < YieldManager liabilityPrincipal, will decrement liabilityPrincipal to sync", async () => {
      // Arrange
      const liabilityPrincipalBefore = ONE_ETHER * 2n;
      await yieldManager.write.setYieldProviderLstLiabilityPrincipal([yieldProviderAddress, liabilityPrincipalBefore], {
        account: securityCouncil.account,
      });
      await yieldManager.write.setYieldProviderLstLiabilityPrincipal([yieldProvider.address, liabilityPrincipalBefore]);
      const liabilityShares = ONE_ETHER;
      const ethValueOfLiabilityShares = ONE_ETHER / 2n;
      await mockDashboard.write.setLiabilitySharesReturn([liabilityShares], { account: securityCouncil.account });
      await mockSTETH.write.setPooledEthBySharesRoundUpReturn([ethValueOfLiabilityShares], {
        account: securityCouncil.account,
      });

      // Act
      await yieldManager.write.syncLSTLiabilityPrincipal([yieldProviderAddress], { account: securityCouncil.account });

      // Assert
      expect(await yieldManager.read.getYieldProviderLstLiabilityPrincipal([yieldProvider.address])).eq(
        ethValueOfLiabilityShares,
      );
    });
  });

  describe("payMaximumPossibleLSTLiability", () => {
    it("If ossified, should be a no-op", async () => {
      // Arrange - setup lst liability
      const liabilityPrincipalBefore = ONE_ETHER;
      await getWithdrawLSTCall(
        mockLinethRollup,
        yieldManager,
        yieldProvider,
        nativeYieldOperator,
        liabilityPrincipalBefore,
      );
      const userFundsInYieldProvidersTotalBefore = await yieldManager.read.userFundsInYieldProvidersTotal();
      const userFundsBefore = await yieldManager.read.userFunds([yieldProvider.address]);
      expect(await yieldManager.read.getYieldProviderLstLiabilityPrincipal([yieldProvider.address])).eq(
        liabilityPrincipalBefore,
      );
      // Arrange - setup ossified. Note with real Lido contracts 'progressPendingOssification' will not succeed with an LST liability
      await yieldManager.write.setYieldProviderIsOssified([yieldProviderAddress, true]);
      await mockDashboard.write.setRebalanceVaultWithSharesWithdrawingFromVault([true]);
      const vaultBalanceBefore = await getBalance(mockStakingVault);
      // Act
      await yieldManager.write.payMaximumPossibleLSTLiability([yieldProvider.address], {
        account: securityCouncil.account,
      });
      // Assert
      expect(await getBalance(mockStakingVault)).eq(vaultBalanceBefore);
      expect(await yieldManager.read.userFundsInYieldProvidersTotal()).eq(userFundsInYieldProvidersTotalBefore);
      expect(await yieldManager.read.userFunds([yieldProvider.address])).eq(userFundsBefore);
      expect(await yieldManager.read.getYieldProviderLstLiabilityPrincipal([yieldProvider.address])).eq(
        liabilityPrincipalBefore,
      );
    });
    it("If no Lido liabilities, should not rebalance but should sync external liability settlement", async () => {
      // Arrange - setup lst liability principal
      const liabilityPrincipalBefore = ONE_ETHER;
      await getWithdrawLSTCall(
        mockLinethRollup,
        yieldManager,
        yieldProvider,
        nativeYieldOperator,
        liabilityPrincipalBefore,
      );
      const userFundsInYieldProvidersTotalBefore = await yieldManager.read.userFundsInYieldProvidersTotal();
      const userFundsBefore = await yieldManager.read.userFunds([yieldProvider.address]);
      expect(await yieldManager.read.getYieldProviderLstLiabilityPrincipal([yieldProvider.address])).eq(
        liabilityPrincipalBefore,
      );
      const vaultBalanceBefore = await getBalance(mockStakingVault);
      // Arrange - setup 0 Lido liability
      const liabilityShares = BigInt(ZERO_VALUE);
      await mockDashboard.write.setRebalanceVaultWithSharesWithdrawingFromVault([true]);
      await mockDashboard.write.setLiabilitySharesReturn([liabilityShares], { account: securityCouncil.account });
      // Arrange - set sync
      const syncedLiabilityShares = ONE_ETHER / 2n;
      await mockSTETH.write.setPooledEthBySharesRoundUpReturn([syncedLiabilityShares]);
      // Act
      await yieldManager.write.payMaximumPossibleLSTLiability([yieldProvider.address], {
        account: securityCouncil.account,
      });
      // Assert
      expect(await getBalance(mockStakingVault)).eq(vaultBalanceBefore);
      expect(await yieldManager.read.userFundsInYieldProvidersTotal()).eq(userFundsInYieldProvidersTotalBefore);
      expect(await yieldManager.read.userFunds([yieldProvider.address])).eq(userFundsBefore);
      expect(await yieldManager.read.getYieldProviderLstLiabilityPrincipal([yieldProvider.address])).eq(
        syncedLiabilityShares,
      );
    });
    it("If no Vault balance, should not rebalance but should sync external liability settlement", async () => {
      // Arrange - setup lst liability principal
      const liabilityPrincipalBefore = ONE_ETHER;
      await getWithdrawLSTCall(
        mockLinethRollup,
        yieldManager,
        yieldProvider,
        nativeYieldOperator,
        liabilityPrincipalBefore,
      );
      const userFundsInYieldProvidersTotalBefore = await yieldManager.read.userFundsInYieldProvidersTotal();
      const userFundsBefore = await yieldManager.read.userFunds([yieldProvider.address]);
      expect(await yieldManager.read.getYieldProviderLstLiabilityPrincipal([yieldProvider.address])).eq(
        liabilityPrincipalBefore,
      );
      await mockDashboard.write.setRebalanceVaultWithSharesWithdrawingFromVault([true]);
      const vaultBalanceBefore = await getBalance(mockStakingVault);
      // Arrange - setup Lido liability
      const liabilityShares = ONE_ETHER;
      await mockDashboard.write.setLiabilitySharesReturn([liabilityShares], { account: securityCouncil.account });
      // Arrange - set sync
      const syncedLiabilityShares = ONE_ETHER / 2n;
      await mockSTETH.write.setPooledEthBySharesRoundUpReturn([syncedLiabilityShares]);
      // Arrange - setup Vault balance (counted in shares)
      await mockSTETH.write.setSharesByPooledEthReturn([BigInt(ZERO_VALUE)], { account: securityCouncil.account });
      // Act
      await yieldManager.write.payMaximumPossibleLSTLiability([yieldProvider.address], {
        account: securityCouncil.account,
      });
      // Assert
      expect(await getBalance(mockStakingVault)).eq(vaultBalanceBefore);
      expect(await yieldManager.read.userFundsInYieldProvidersTotal()).eq(userFundsInYieldProvidersTotalBefore);
      expect(await yieldManager.read.userFunds([yieldProvider.address])).eq(userFundsBefore);
      expect(await yieldManager.read.getYieldProviderLstLiabilityPrincipal([yieldProvider.address])).eq(
        syncedLiabilityShares,
      );
    });
    it("If VAULT_BALANCE >0 and LIDO_LIABILITY_SHARE >0, rebalance with lower of the two (VAULT_BALANCE lower)", async () => {
      // Arrange - setup lst liability principal
      const liabilityPrincipalBefore = ONE_ETHER;
      await getWithdrawLSTCall(
        mockLinethRollup,
        yieldManager,
        yieldProvider,
        nativeYieldOperator,
        liabilityPrincipalBefore,
      );
      const userFundsInYieldProvidersTotalBefore = await yieldManager.read.userFundsInYieldProvidersTotal();
      const userFundsBefore = await yieldManager.read.userFunds([yieldProvider.address]);
      expect(await yieldManager.read.getYieldProviderLstLiabilityPrincipal([yieldProvider.address])).eq(
        liabilityPrincipalBefore,
      );
      const vaultBalanceBefore = await getBalance({ address: mockStakingVaultAddress });
      // Arrange - setup Lido liability
      const liabilityShares = ONE_ETHER * 2n;
      await mockDashboard.write.setLiabilitySharesReturn([liabilityShares], { account: securityCouncil.account });
      // Arrange - setup Vault balance (counted in shares)
      await mockDashboard.write.setRebalanceVaultWithSharesWithdrawingFromVault([true]);
      await mockSTETH.write.setSharesByPooledEthReturn([ONE_ETHER], { account: securityCouncil.account });
      // Arrange - setup post-rebalance Lido LST liability
      const ethValueOfLidoLiabilitySharesAfterRebalance = ONE_ETHER;
      await mockSTETH.write.setPooledEthBySharesRoundUpReturn([ethValueOfLidoLiabilitySharesAfterRebalance], {
        account: securityCouncil.account,
      });

      // Act
      await yieldManager.write.payMaximumPossibleLSTLiability([yieldProvider.address], {
        account: securityCouncil.account,
      });

      // Assert
      expect(await getBalance(mockStakingVault)).eq(vaultBalanceBefore - ONE_ETHER);
      const syncExternalLiabilitySettlementDifference =
        liabilityPrincipalBefore - ethValueOfLidoLiabilitySharesAfterRebalance;
      expect(await yieldManager.read.userFundsInYieldProvidersTotal()).eq(
        userFundsInYieldProvidersTotalBefore - syncExternalLiabilitySettlementDifference,
      );
      expect(await yieldManager.read.userFunds([yieldProvider.address])).eq(
        userFundsBefore - syncExternalLiabilitySettlementDifference,
      );
      expect(await yieldManager.read.getYieldProviderLstLiabilityPrincipal([yieldProvider.address])).eq(
        liabilityPrincipalBefore - syncExternalLiabilitySettlementDifference,
      );
    });
    it("If VAULT_BALANCE >0 and LIDO_LIABILITY_SHARE >0, rebalance with lower of the two (LIDO_LIABILITY_SHARE lower)", async () => {
      // Arrange - setup lst liability principal
      const liabilityPrincipalBefore = (ONE_ETHER * 3n) / 2n;
      await getWithdrawLSTCall(
        mockLinethRollup,
        yieldManager,
        yieldProvider,
        nativeYieldOperator,
        liabilityPrincipalBefore,
      );
      const userFundsInYieldProvidersTotalBefore = await yieldManager.read.userFundsInYieldProvidersTotal();
      const userFundsBefore = await yieldManager.read.userFunds([yieldProvider.address]);
      expect(await yieldManager.read.getYieldProviderLstLiabilityPrincipal([yieldProvider.address])).eq(
        liabilityPrincipalBefore,
      );
      const vaultBalanceBefore = await getBalance({ address: mockStakingVaultAddress });
      // Arrange - setup Lido liability
      const liabilityShares = ONE_ETHER;
      await mockDashboard.write.setLiabilitySharesReturn([liabilityShares], { account: securityCouncil.account });
      // Arrange - setup Vault balance (counted in shares)
      await mockDashboard.write.setRebalanceVaultWithSharesWithdrawingFromVault([true]);
      await mockSTETH.write.setSharesByPooledEthReturn([ONE_ETHER * 2n], { account: securityCouncil.account });
      // Arrange - setup post-rebalance Lido LST liability
      const ethValueOfLidoLiabilitySharesAfterRebalance = ONE_ETHER;
      await mockSTETH.write.setPooledEthBySharesRoundUpReturn([ethValueOfLidoLiabilitySharesAfterRebalance], {
        account: securityCouncil.account,
      });

      // Act
      await yieldManager.write.payMaximumPossibleLSTLiability([yieldProvider.address], {
        account: securityCouncil.account,
      });

      // Assert
      const syncExternalLiabilitySettlementDifference =
        liabilityPrincipalBefore - ethValueOfLidoLiabilitySharesAfterRebalance;
      expect(await yieldManager.read.userFundsInYieldProvidersTotal()).eq(userFundsInYieldProvidersTotalBefore);
      expect(await yieldManager.read.userFunds([yieldProvider.address])).eq(userFundsBefore);
      expect(await yieldManager.read.getYieldProviderLstLiabilityPrincipal([yieldProvider.address])).eq(
        liabilityPrincipalBefore - syncExternalLiabilitySettlementDifference,
      );
      expect(await getBalance({ address: mockStakingVaultAddress })).eq(vaultBalanceBefore - liabilityShares);
    });
  });

  describe("reportYield", () => {
    it("Should revert if not invoked via delegatecall", async () => {
      const call = yieldProvider.write.reportYield([yieldProviderAddress], { account: securityCouncil.account });
      await expectRevertWithCustomError(yieldProvider, call, "ContextIsNotYieldManager");
    });
    it("Should revert if ossified", async () => {
      await yieldManager.write.initiateOssification([yieldProviderAddress], { account: securityCouncil.account });
      await yieldManager.write.progressPendingOssification([yieldProviderAddress], {
        account: securityCouncil.account,
      });
      const call = yieldManager.write.reportYield([yieldProviderAddress, l2YieldRecipientAddress], {
        account: nativeYieldOperator.account,
      });
      await expectRevertWithCustomError(yieldProvider, call, "OperationNotSupportedDuringOssification", [
        OperationType.REPORT_YIELD,
      ]);
    });
    it("If vault value > user funds, should report positive yield", async () => {
      // Arrange
      const userFunds = ONE_ETHER;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, userFunds);
      const vaultValue = ONE_ETHER * 2n;
      await mockDashboard.write.setTotalValueReturn([vaultValue + CONNECT_DEPOSIT]);

      // Act
      const [newReportedYield, outstandingNegativeYield] = (
        await yieldManager.simulate.reportYield([yieldProviderAddress, l2YieldRecipientAddress], {
          account: nativeYieldOperator.address,
        })
      ).result;
      await yieldManager.write.reportYield([yieldProviderAddress, l2YieldRecipientAddress], {
        account: nativeYieldOperator.account,
      });

      // Assert
      expect(newReportedYield).eq(vaultValue - userFunds);
      expect(outstandingNegativeYield).eq(0n);
    });
    it("If vault value < user funds, should report negative yield", async () => {
      // Arrange
      const userFunds = ONE_ETHER * 2n;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, userFunds);
      const vaultValue = ONE_ETHER;
      await mockDashboard.write.setTotalValueReturn([vaultValue + CONNECT_DEPOSIT]);

      // Act
      const [newReportedYield, outstandingNegativeYield] = (
        await yieldManager.simulate.reportYield([yieldProviderAddress, l2YieldRecipientAddress], {
          account: nativeYieldOperator.address,
        })
      ).result;
      await yieldManager.write.reportYield([yieldProviderAddress, l2YieldRecipientAddress], {
        account: nativeYieldOperator.account,
      });

      // Assert
      expect(newReportedYield).eq(0n);
      expect(outstandingNegativeYield).eq(userFunds - vaultValue);
    });
    it("It should decrement reported yield by liabilities and fees owing, for positive yield scenario", async () => {
      // Arrange
      const userFunds = ONE_ETHER;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, userFunds);
      const lstLiabilities = ONE_ETHER;
      await mockSTETH.write.setPooledEthBySharesRoundUpReturn([lstLiabilities]);
      const lidoFees = ONE_ETHER;
      await mockDashboard.write.setObligationsFeesToSettleReturn([lidoFees]);
      const nodeOperatorFees = ONE_ETHER;
      await mockDashboard.write.setAccruedFeeReturn([nodeOperatorFees]);
      const vaultValue = ONE_ETHER * 5n;
      await mockDashboard.write.setTotalValueReturn([vaultValue + CONNECT_DEPOSIT]);

      // Act
      const [newReportedYield, outstandingNegativeYield] = (
        await yieldManager.simulate.reportYield([yieldProviderAddress, l2YieldRecipientAddress], {
          account: nativeYieldOperator.address,
        })
      ).result;
      await yieldManager.write.reportYield([yieldProviderAddress, l2YieldRecipientAddress], {
        account: nativeYieldOperator.account,
      });

      // Assert
      expect(newReportedYield).eq(vaultValue - userFunds - lstLiabilities - lidoFees - nodeOperatorFees);
      expect(outstandingNegativeYield).eq(0n);
    });
    it("It should decrement reported yield by liabilities and fees owing, for negative yield scenario", async () => {
      // Arrange
      const userFunds = ONE_ETHER;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, userFunds);
      const lstLiabilities = ONE_ETHER;
      await mockSTETH.write.setPooledEthBySharesRoundUpReturn([lstLiabilities]);
      const lidoFees = ONE_ETHER;
      await mockDashboard.write.setObligationsFeesToSettleReturn([lidoFees]);
      const nodeOperatorFees = ONE_ETHER;
      await mockDashboard.write.setAccruedFeeReturn([nodeOperatorFees]);
      const vaultValue = ONE_ETHER;
      await mockDashboard.write.setTotalValueReturn([vaultValue + CONNECT_DEPOSIT]);

      // Act
      const [newReportedYield, outstandingNegativeYield] = (
        await yieldManager.simulate.reportYield([yieldProviderAddress, l2YieldRecipientAddress], {
          account: nativeYieldOperator.address,
        })
      ).result;
      await yieldManager.write.reportYield([yieldProviderAddress, l2YieldRecipientAddress], {
        account: nativeYieldOperator.account,
      });

      // Assert
      expect(newReportedYield).eq(0n);
      expect(outstandingNegativeYield).eq(userFunds + lstLiabilities + lidoFees + nodeOperatorFees - vaultValue);
    });
    it("It should perform max lst liability payment", async () => {
      // Arrange
      const userFunds = ONE_ETHER;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, userFunds);
      const lstLiabilities = ONE_ETHER;
      await mockSTETH.write.setPooledEthBySharesRoundUpReturn([lstLiabilities]);
      const lidoFees = ONE_ETHER;
      await mockDashboard.write.setObligationsFeesToSettleReturn([lidoFees]);
      const nodeOperatorFees = ONE_ETHER;
      await mockDashboard.write.setAccruedFeeReturn([nodeOperatorFees]);
      const vaultValue = ONE_ETHER * 5n;
      await mockDashboard.write.setTotalValueReturn([vaultValue]);

      // Arrange - payMaximumPossibleLSTLiability to withdraw from dashboard
      const expectedDashboardWithdrawal = 10n;
      await mockDashboard.write.setLiabilitySharesReturn([expectedDashboardWithdrawal]);
      await mockSTETH.write.setSharesByPooledEthReturn([expectedDashboardWithdrawal]);
      await mockDashboard.write.setRebalanceVaultWithSharesWithdrawingFromVault([true]);

      // Arrange - Before figures
      const vaultBalanceBefore = await getBalance(mockStakingVault);

      // Act
      await yieldManager.write.reportYield([yieldProviderAddress, l2YieldRecipientAddress], {
        account: nativeYieldOperator.account,
      });

      // Assert
      expect(await getBalance(mockStakingVault)).eq(vaultBalanceBefore - expectedDashboardWithdrawal);
    });
  });
});
