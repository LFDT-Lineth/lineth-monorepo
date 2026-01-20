import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { expectRevertWithCustomError, getAccountsFixture } from "../../common/helpers";
import {
  deployAndAddSingleLidoStVaultYieldProvider,
  fundLidoStVaultYieldProvider,
  getBalance,
  getWithdrawLSTCall,
} from "../helpers";
import {
  MockSTETH,
  MockLineaRollup,
  TestYieldManager,
  MockDashboard,
  MockStakingVault,
  TestLidoStVaultYieldProvider,
} from "contracts/typechain-types";
import { expect } from "chai";
import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { ONE_ETHER, OperationType, ZERO_VALUE, YieldProviderVendor, CONNECT_DEPOSIT } from "../../common/constants";
import { ethers } from "hardhat";

describe("LidoStVaultYieldProvider contract - yield operations", () => {
  let yieldProvider: TestLidoStVaultYieldProvider;
  let nativeYieldOperator: SignerWithAddress;
  let securityCouncil: SignerWithAddress;
  let l2YieldRecipient: SignerWithAddress;
  let mockSTETH: MockSTETH;
  let mockLineaRollup: MockLineaRollup;
  let yieldManager: TestYieldManager;
  let mockDashboard: MockDashboard;
  let mockStakingVault: MockStakingVault;

  let mockStakingVaultAddress: string;
  let yieldProviderAddress: string;
  let l2YieldRecipientAddress: string;

  before(async () => {
    ({ nativeYieldOperator, securityCouncil, l2YieldRecipient } = await loadFixture(getAccountsFixture));
    l2YieldRecipientAddress = await l2YieldRecipient.getAddress();
  });

  beforeEach(async () => {
    ({
      yieldProvider,
      yieldProviderAddress,
      mockDashboard,
      mockStakingVault,
      yieldManager,
      mockSTETH,
      mockLineaRollup,
    } = await loadFixture(deployAndAddSingleLidoStVaultYieldProvider));

    mockStakingVaultAddress = await mockStakingVault.getAddress();
  });

  describe("syncExternalLiabilitySettlement", () => {
    it("If ETH value of Lido liabilityShares >= YieldManager liabilityPrincipal, no-op", async () => {
      // Arrange
      const liabilityShares = ONE_ETHER;
      const ethValueOfLidoLiabilityShares = ONE_ETHER * 2n;
      const liabilityPrincipalBefore = ONE_ETHER;
      await mockSTETH.connect(securityCouncil).setPooledEthBySharesRoundUpReturn(ethValueOfLidoLiabilityShares);
      await yieldManager
        .connect(securityCouncil)
        .setYieldProviderLstLiabilityPrincipal(yieldProviderAddress, liabilityPrincipalBefore);

      const userFundsInYieldProvidersTotalBefore = await yieldManager.userFundsInYieldProvidersTotal();
      const userFundsBefore = await yieldManager.userFunds(yieldProvider);
      await yieldManager.setYieldProviderLstLiabilityPrincipal(yieldProvider, liabilityPrincipalBefore);

      // Act
      const lstLiabilityPrincipalSynced = await yieldManager
        .connect(securityCouncil)
        .syncExternalLiabilitySettlement.staticCall(yieldProviderAddress, liabilityShares, liabilityPrincipalBefore);
      await expect(
        yieldManager
          .connect(securityCouncil)
          .syncExternalLiabilitySettlement(yieldProviderAddress, liabilityShares, liabilityPrincipalBefore),
      ).to.not.emit(yieldManager, "LSTLiabilityPrincipalSynced");

      // Assert
      expect(lstLiabilityPrincipalSynced).eq(liabilityPrincipalBefore);
      expect(await yieldManager.userFundsInYieldProvidersTotal()).eq(userFundsInYieldProvidersTotalBefore);
      expect(await yieldManager.userFunds(yieldProvider)).eq(userFundsBefore);
      expect(await yieldManager.getYieldProviderLstLiabilityPrincipal(yieldProvider)).eq(liabilityPrincipalBefore);
    });
    it("If ETH value of Lido liabilityShares < YieldManager liabilityPrincipal, will decrement liabilityPrincipal to sync", async () => {
      // Arrange liabilityShares and ETH value
      const liabilityShares = ZERO_VALUE;
      const ethValueOfLidoLiabilityShares = ZERO_VALUE;
      await mockSTETH.connect(securityCouncil).setPooledEthBySharesRoundUpReturn(ethValueOfLidoLiabilityShares);
      // Arrange - Set up userFunds and lstLiabilityPrincipals
      const liabilityPrincipalBefore = ONE_ETHER * 2n;
      const call = getWithdrawLSTCall(
        mockLineaRollup,
        yieldManager,
        yieldProvider,
        nativeYieldOperator,
        liabilityPrincipalBefore,
      );
      await call;

      const userFundsInYieldProvidersTotalBefore = await yieldManager.userFundsInYieldProvidersTotal();
      const userFundsBefore = await yieldManager.userFunds(yieldProvider);
      expect(await yieldManager.getYieldProviderLstLiabilityPrincipal(yieldProvider)).eq(liabilityPrincipalBefore);

      // Act
      const lstLiabilityPrincipalSynced = await yieldManager
        .connect(securityCouncil)
        .syncExternalLiabilitySettlement.staticCall(yieldProviderAddress, liabilityShares, liabilityPrincipalBefore);
      const yieldProviderIndex = await yieldManager.getYieldProviderIndex(yieldProviderAddress);
      await expect(
        yieldManager
          .connect(securityCouncil)
          .syncExternalLiabilitySettlement(yieldProviderAddress, liabilityShares, liabilityPrincipalBefore),
      )
        .to.emit(yieldManager, "LSTLiabilityPrincipalSynced")
        .withArgs(
          YieldProviderVendor.LIDO_ST_VAULT_YIELD_PROVIDER_VENDOR,
          yieldProviderIndex,
          liabilityPrincipalBefore,
          ethValueOfLidoLiabilityShares,
        );

      // Assert
      expect(lstLiabilityPrincipalSynced).eq(ethValueOfLidoLiabilityShares);
      const lstLiabilityPrincipalDecrement = liabilityPrincipalBefore - BigInt(ethValueOfLidoLiabilityShares);
      expect(await yieldManager.userFundsInYieldProvidersTotal()).eq(userFundsInYieldProvidersTotalBefore);
      expect(await yieldManager.userFunds(yieldProvider)).eq(userFundsBefore);
      expect(await yieldManager.getYieldProviderLstLiabilityPrincipal(yieldProvider)).eq(
        liabilityPrincipalBefore - lstLiabilityPrincipalDecrement,
      );
    });
  });

  describe("syncLSTLiabilityPrincipal", () => {
    it("Should revert if not invoked via delegatecall", async () => {
      await expect(yieldProvider.syncLSTLiabilityPrincipal(yieldProviderAddress)).to.be.revertedWithCustomError(
        yieldProvider,
        "ContextIsNotYieldManager",
      );
    });
    it("If ETH value of Lido liabilityShares >= YieldManager liabilityPrincipal, no-op", async () => {
      // Arrange
      const liabilityPrincipalBefore = ONE_ETHER;
      await yieldManager
        .connect(securityCouncil)
        .setYieldProviderLstLiabilityPrincipal(yieldProviderAddress, liabilityPrincipalBefore);
      await yieldManager.setYieldProviderLstLiabilityPrincipal(yieldProvider, liabilityPrincipalBefore);
      const liabilityShares = ONE_ETHER;
      const ethValueOfLiabilityShares = ONE_ETHER * 2n;
      await mockDashboard.connect(securityCouncil).setLiabilitySharesReturn(liabilityShares);
      await mockSTETH.connect(securityCouncil).setPooledEthBySharesRoundUpReturn(ethValueOfLiabilityShares);

      // Act
      await yieldManager.connect(securityCouncil).syncLSTLiabilityPrincipal(yieldProviderAddress);

      // Assert
      expect(await yieldManager.getYieldProviderLstLiabilityPrincipal(yieldProvider)).eq(liabilityPrincipalBefore);
    });
    it("If ETH value of Lido liabilityShares < YieldManager liabilityPrincipal, will decrement liabilityPrincipal to sync", async () => {
      // Arrange
      const liabilityPrincipalBefore = ONE_ETHER * 2n;
      await yieldManager
        .connect(securityCouncil)
        .setYieldProviderLstLiabilityPrincipal(yieldProviderAddress, liabilityPrincipalBefore);
      await yieldManager.setYieldProviderLstLiabilityPrincipal(yieldProvider, liabilityPrincipalBefore);
      const liabilityShares = ONE_ETHER;
      const ethValueOfLiabilityShares = ONE_ETHER / 2n;
      await mockDashboard.connect(securityCouncil).setLiabilitySharesReturn(liabilityShares);
      await mockSTETH.connect(securityCouncil).setPooledEthBySharesRoundUpReturn(ethValueOfLiabilityShares);

      // Act
      await yieldManager.connect(securityCouncil).syncLSTLiabilityPrincipal(yieldProviderAddress);

      // Assert
      expect(await yieldManager.getYieldProviderLstLiabilityPrincipal(yieldProvider)).eq(ethValueOfLiabilityShares);
    });
  });

  describe("payMaximumPossibleLSTLiability", () => {
    it("If ossified, should be a no-op", async () => {
      // Arrange - setup lst liability
      const liabilityPrincipalBefore = ONE_ETHER;
      await getWithdrawLSTCall(
        mockLineaRollup,
        yieldManager,
        yieldProvider,
        nativeYieldOperator,
        liabilityPrincipalBefore,
      );
      const userFundsInYieldProvidersTotalBefore = await yieldManager.userFundsInYieldProvidersTotal();
      const userFundsBefore = await yieldManager.userFunds(yieldProvider);
      expect(await yieldManager.getYieldProviderLstLiabilityPrincipal(yieldProvider)).eq(liabilityPrincipalBefore);
      // Arrange - setup ossified. Note with real Lido contracts 'progressPendingOssification' will not succeed with an LST liability
      await yieldManager.setYieldProviderIsOssified(yieldProviderAddress, true);
      await mockDashboard.setRebalanceVaultWithSharesWithdrawingFromVault(true);
      const vaultBalanceBefore = await getBalance(mockStakingVault);
      // Act
      await yieldManager.connect(securityCouncil).payMaximumPossibleLSTLiability(yieldProvider);
      // Assert
      expect(await getBalance(mockStakingVault)).eq(vaultBalanceBefore);
      expect(await yieldManager.userFundsInYieldProvidersTotal()).eq(userFundsInYieldProvidersTotalBefore);
      expect(await yieldManager.userFunds(yieldProvider)).eq(userFundsBefore);
      expect(await yieldManager.getYieldProviderLstLiabilityPrincipal(yieldProvider)).eq(liabilityPrincipalBefore);
    });
    it("If no Lido liabilities, should not rebalance but should sync external liability settlement", async () => {
      // Arrange - setup lst liability principal
      const liabilityPrincipalBefore = ONE_ETHER;
      await getWithdrawLSTCall(
        mockLineaRollup,
        yieldManager,
        yieldProvider,
        nativeYieldOperator,
        liabilityPrincipalBefore,
      );
      const userFundsInYieldProvidersTotalBefore = await yieldManager.userFundsInYieldProvidersTotal();
      const userFundsBefore = await yieldManager.userFunds(yieldProvider);
      expect(await yieldManager.getYieldProviderLstLiabilityPrincipal(yieldProvider)).eq(liabilityPrincipalBefore);
      const vaultBalanceBefore = await getBalance(mockStakingVault);
      // Arrange - setup 0 Lido liability
      const liabilityShares = ZERO_VALUE;
      await mockDashboard.setRebalanceVaultWithSharesWithdrawingFromVault(true);
      await mockDashboard.connect(securityCouncil).setLiabilitySharesReturn(liabilityShares);
      // Arrange - set sync
      const syncedLiabilityShares = ONE_ETHER / 2n;
      await mockSTETH.setPooledEthBySharesRoundUpReturn(syncedLiabilityShares);
      // Act
      await yieldManager.connect(securityCouncil).payMaximumPossibleLSTLiability(yieldProvider);
      // Assert
      expect(await getBalance(mockStakingVault)).eq(vaultBalanceBefore);
      expect(await yieldManager.userFundsInYieldProvidersTotal()).eq(userFundsInYieldProvidersTotalBefore);
      expect(await yieldManager.userFunds(yieldProvider)).eq(userFundsBefore);
      expect(await yieldManager.getYieldProviderLstLiabilityPrincipal(yieldProvider)).eq(syncedLiabilityShares);
    });
    it("If no Vault balance, should not rebalance but should sync external liability settlement", async () => {
      // Arrange - setup lst liability principal
      const liabilityPrincipalBefore = ONE_ETHER;
      await getWithdrawLSTCall(
        mockLineaRollup,
        yieldManager,
        yieldProvider,
        nativeYieldOperator,
        liabilityPrincipalBefore,
      );
      const userFundsInYieldProvidersTotalBefore = await yieldManager.userFundsInYieldProvidersTotal();
      const userFundsBefore = await yieldManager.userFunds(yieldProvider);
      expect(await yieldManager.getYieldProviderLstLiabilityPrincipal(yieldProvider)).eq(liabilityPrincipalBefore);
      await mockDashboard.setRebalanceVaultWithSharesWithdrawingFromVault(true);
      const vaultBalanceBefore = await getBalance(mockStakingVault);
      // Arrange - setup Lido liability
      const liabilityShares = ONE_ETHER;
      await mockDashboard.connect(securityCouncil).setLiabilitySharesReturn(liabilityShares);
      // Arrange - set sync
      const syncedLiabilityShares = ONE_ETHER / 2n;
      await mockSTETH.setPooledEthBySharesRoundUpReturn(syncedLiabilityShares);
      // Arrange - setup Vault balance (counted in shares)
      await mockSTETH.connect(securityCouncil).setSharesByPooledEthReturn(ZERO_VALUE);
      // Act
      await yieldManager.connect(securityCouncil).payMaximumPossibleLSTLiability(yieldProvider);
      // Assert
      expect(await getBalance(mockStakingVault)).eq(vaultBalanceBefore);
      expect(await yieldManager.userFundsInYieldProvidersTotal()).eq(userFundsInYieldProvidersTotalBefore);
      expect(await yieldManager.userFunds(yieldProvider)).eq(userFundsBefore);
      expect(await yieldManager.getYieldProviderLstLiabilityPrincipal(yieldProvider)).eq(syncedLiabilityShares);
    });
    it("If VAULT_BALANCE >0 and LIDO_LIABILITY_SHARE >0, rebalance with lower of the two (VAULT_BALANCE lower)", async () => {
      // Arrange - setup lst liability principal
      const liabilityPrincipalBefore = ONE_ETHER;
      await getWithdrawLSTCall(
        mockLineaRollup,
        yieldManager,
        yieldProvider,
        nativeYieldOperator,
        liabilityPrincipalBefore,
      );
      const userFundsInYieldProvidersTotalBefore = await yieldManager.userFundsInYieldProvidersTotal();
      const userFundsBefore = await yieldManager.userFunds(yieldProvider);
      expect(await yieldManager.getYieldProviderLstLiabilityPrincipal(yieldProvider)).eq(liabilityPrincipalBefore);
      const vaultBalanceBefore = await ethers.provider.getBalance(mockStakingVaultAddress);
      // Arrange - setup Lido liability
      const liabilityShares = ONE_ETHER * 2n;
      await mockDashboard.connect(securityCouncil).setLiabilitySharesReturn(liabilityShares);
      // Arrange - setup Vault balance (counted in shares)
      await mockDashboard.setRebalanceVaultWithSharesWithdrawingFromVault(true);
      await mockSTETH.connect(securityCouncil).setSharesByPooledEthReturn(ONE_ETHER);
      // Arrange - setup post-rebalance Lido LST liability
      const ethValueOfLidoLiabilitySharesAfterRebalance = ONE_ETHER;
      await mockSTETH
        .connect(securityCouncil)
        .setPooledEthBySharesRoundUpReturn(ethValueOfLidoLiabilitySharesAfterRebalance);

      // Act
      await yieldManager.connect(securityCouncil).payMaximumPossibleLSTLiability(yieldProvider);

      // Assert
      expect(await getBalance(mockStakingVault)).eq(vaultBalanceBefore - ONE_ETHER);
      const syncExternalLiabilitySettlementDifference =
        liabilityPrincipalBefore - ethValueOfLidoLiabilitySharesAfterRebalance;
      expect(await yieldManager.userFundsInYieldProvidersTotal()).eq(
        userFundsInYieldProvidersTotalBefore - syncExternalLiabilitySettlementDifference,
      );
      expect(await yieldManager.userFunds(yieldProvider)).eq(
        userFundsBefore - syncExternalLiabilitySettlementDifference,
      );
      expect(await yieldManager.getYieldProviderLstLiabilityPrincipal(yieldProvider)).eq(
        liabilityPrincipalBefore - syncExternalLiabilitySettlementDifference,
      );
    });
    it("If VAULT_BALANCE >0 and LIDO_LIABILITY_SHARE >0, rebalance with lower of the two (LIDO_LIABILITY_SHARE lower)", async () => {
      // Arrange - setup lst liability principal
      const liabilityPrincipalBefore = (ONE_ETHER * 3n) / 2n;
      await getWithdrawLSTCall(
        mockLineaRollup,
        yieldManager,
        yieldProvider,
        nativeYieldOperator,
        liabilityPrincipalBefore,
      );
      const userFundsInYieldProvidersTotalBefore = await yieldManager.userFundsInYieldProvidersTotal();
      const userFundsBefore = await yieldManager.userFunds(yieldProvider);
      expect(await yieldManager.getYieldProviderLstLiabilityPrincipal(yieldProvider)).eq(liabilityPrincipalBefore);
      const vaultBalanceBefore = await ethers.provider.getBalance(mockStakingVaultAddress);
      // Arrange - setup Lido liability
      const liabilityShares = ONE_ETHER;
      await mockDashboard.connect(securityCouncil).setLiabilitySharesReturn(liabilityShares);
      // Arrange - setup Vault balance (counted in shares)
      await mockDashboard.setRebalanceVaultWithSharesWithdrawingFromVault(true);
      await mockSTETH.connect(securityCouncil).setSharesByPooledEthReturn(ONE_ETHER * 2n);
      // Arrange - setup post-rebalance Lido LST liability
      const ethValueOfLidoLiabilitySharesAfterRebalance = ONE_ETHER;
      await mockSTETH
        .connect(securityCouncil)
        .setPooledEthBySharesRoundUpReturn(ethValueOfLidoLiabilitySharesAfterRebalance);

      // Act
      await yieldManager.connect(securityCouncil).payMaximumPossibleLSTLiability(yieldProvider);

      // Assert
      const syncExternalLiabilitySettlementDifference =
        liabilityPrincipalBefore - ethValueOfLidoLiabilitySharesAfterRebalance;
      expect(await yieldManager.userFundsInYieldProvidersTotal()).eq(userFundsInYieldProvidersTotalBefore);
      expect(await yieldManager.userFunds(yieldProvider)).eq(userFundsBefore);
      expect(await yieldManager.getYieldProviderLstLiabilityPrincipal(yieldProvider)).eq(
        liabilityPrincipalBefore - syncExternalLiabilitySettlementDifference,
      );
      expect(await ethers.provider.getBalance(mockStakingVaultAddress)).eq(vaultBalanceBefore - liabilityShares);
    });
  });

  describe("reportYield", () => {
    it("Should revert if not invoked via delegatecall", async () => {
      const call = yieldProvider.connect(securityCouncil).reportYield(yieldProviderAddress);
      await expectRevertWithCustomError(yieldProvider, call, "ContextIsNotYieldManager");
    });
    it("Should revert if ossified", async () => {
      await yieldManager.connect(securityCouncil).initiateOssification(yieldProviderAddress);
      await yieldManager.connect(securityCouncil).progressPendingOssification(yieldProviderAddress);
      const call = yieldManager.connect(nativeYieldOperator).reportYield(yieldProviderAddress, l2YieldRecipientAddress);
      await expectRevertWithCustomError(yieldProvider, call, "OperationNotSupportedDuringOssification", [
        OperationType.REPORT_YIELD,
      ]);
    });
    it("If vault value > user funds, should report positive yield", async () => {
      // Arrange
      const userFunds = ONE_ETHER;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, userFunds);
      const vaultValue = ONE_ETHER * 2n;
      await mockDashboard.setTotalValueReturn(vaultValue + CONNECT_DEPOSIT);

      // Act
      const [newReportedYield, outstandingNegativeYield] = await yieldManager
        .connect(nativeYieldOperator)
        .reportYield.staticCall(yieldProviderAddress, l2YieldRecipientAddress);
      await yieldManager.connect(nativeYieldOperator).reportYield(yieldProviderAddress, l2YieldRecipientAddress);

      // Assert
      expect(newReportedYield).eq(vaultValue - userFunds);
      expect(outstandingNegativeYield).eq(0);
    });
    it("If vault value < user funds, should report negative yield", async () => {
      // Arrange
      const userFunds = ONE_ETHER * 2n;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, userFunds);
      const vaultValue = ONE_ETHER;
      await mockDashboard.setTotalValueReturn(vaultValue + CONNECT_DEPOSIT);

      // Act
      const [newReportedYield, outstandingNegativeYield] = await yieldManager
        .connect(nativeYieldOperator)
        .reportYield.staticCall(yieldProviderAddress, l2YieldRecipientAddress);
      await yieldManager.connect(nativeYieldOperator).reportYield(yieldProviderAddress, l2YieldRecipientAddress);

      // Assert
      expect(newReportedYield).eq(0);
      expect(outstandingNegativeYield).eq(userFunds - vaultValue);
    });
    it("It should decrement reported yield by liabilities and fees owing, for positive yield scenario", async () => {
      // Arrange
      const userFunds = ONE_ETHER;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, userFunds);
      const lstLiabilities = ONE_ETHER;
      await mockSTETH.setPooledEthBySharesRoundUpReturn(lstLiabilities);
      const lidoFees = ONE_ETHER;
      await mockDashboard.setObligationsFeesToSettleReturn(lidoFees);
      const nodeOperatorFees = ONE_ETHER;
      await mockDashboard.setAccruedFeeReturn(nodeOperatorFees);
      const vaultValue = ONE_ETHER * 5n;
      await mockDashboard.setTotalValueReturn(vaultValue + CONNECT_DEPOSIT);

      // Act
      const [newReportedYield, outstandingNegativeYield] = await yieldManager
        .connect(nativeYieldOperator)
        .reportYield.staticCall(yieldProviderAddress, l2YieldRecipientAddress);
      await yieldManager.connect(nativeYieldOperator).reportYield(yieldProviderAddress, l2YieldRecipientAddress);

      // Assert
      expect(newReportedYield).eq(vaultValue - userFunds - lstLiabilities - lidoFees - nodeOperatorFees);
      expect(outstandingNegativeYield).eq(0);
    });
    it("It should decrement reported yield by liabilities and fees owing, for negative yield scenario", async () => {
      // Arrange
      const userFunds = ONE_ETHER;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, userFunds);
      const lstLiabilities = ONE_ETHER;
      await mockSTETH.setPooledEthBySharesRoundUpReturn(lstLiabilities);
      const lidoFees = ONE_ETHER;
      await mockDashboard.setObligationsFeesToSettleReturn(lidoFees);
      const nodeOperatorFees = ONE_ETHER;
      await mockDashboard.setAccruedFeeReturn(nodeOperatorFees);
      const vaultValue = ONE_ETHER;
      await mockDashboard.setTotalValueReturn(vaultValue + CONNECT_DEPOSIT);

      // Act
      const [newReportedYield, outstandingNegativeYield] = await yieldManager
        .connect(nativeYieldOperator)
        .reportYield.staticCall(yieldProviderAddress, l2YieldRecipientAddress);
      await yieldManager.connect(nativeYieldOperator).reportYield(yieldProviderAddress, l2YieldRecipientAddress);

      // Assert
      expect(newReportedYield).eq(0);
      expect(outstandingNegativeYield).eq(userFunds + lstLiabilities + lidoFees + nodeOperatorFees - vaultValue);
    });
    it("It should perform max lst liability payment", async () => {
      // Arrange
      const userFunds = ONE_ETHER;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, userFunds);
      const lstLiabilities = ONE_ETHER;
      await mockSTETH.setPooledEthBySharesRoundUpReturn(lstLiabilities);
      const lidoFees = ONE_ETHER;
      await mockDashboard.setObligationsFeesToSettleReturn(lidoFees);
      const nodeOperatorFees = ONE_ETHER;
      await mockDashboard.setAccruedFeeReturn(nodeOperatorFees);
      const vaultValue = ONE_ETHER * 5n;
      await mockDashboard.setTotalValueReturn(vaultValue);

      // Arrange - payMaximumPossibleLSTLiability to withdraw from dashboard
      const expectedDashboardWithdrawal = 10n;
      await mockDashboard.setLiabilitySharesReturn(expectedDashboardWithdrawal);
      await mockSTETH.setSharesByPooledEthReturn(expectedDashboardWithdrawal);
      await mockDashboard.setRebalanceVaultWithSharesWithdrawingFromVault(true);

      // Arrange - Before figures
      const vaultBalanceBefore = await getBalance(mockStakingVault);

      // Act
      await yieldManager.connect(nativeYieldOperator).reportYield(yieldProviderAddress, l2YieldRecipientAddress);

      // Assert
      expect(await getBalance(mockStakingVault)).eq(vaultBalanceBefore - expectedDashboardWithdrawal);
    });
  });
});
