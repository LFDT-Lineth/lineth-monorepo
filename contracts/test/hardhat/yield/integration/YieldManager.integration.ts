// Test scenarios with LineaRollup + YieldManager + LidoStVaultYieldProvider
import { loadFixture, setBalance } from "@nomicfoundation/hardhat-network-helpers";
import { encodeSendMessage, expectRevertWithCustomError, getAccountsFixture } from "../../common/helpers";
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
  setupLineaRollupMessageMerkleTree,
  setWithdrawalReserveToMinimum,
  setWithdrawalReserveToTarget,
  withdrawLST,
  setupMaxLSTLiabilityPaymentForWithdrawal,
  setupLSTPrincipalDecrementForPaxMaximumPossibleLSTLiability,
  buildVendorExitData,
  buildSetWithdrawalReserveParams,
  YieldManagerInitializationData,
} from "../helpers";
import {
  TestYieldManager,
  TestLineaRollup,
  TestLidoStVaultYieldProvider,
  MockDashboard,
  MockStakingVault,
  TestLidoStVaultYieldProviderFactory,
  SSZMerkleTree,
  TestValidatorContainerProofVerifier,
  MockSTETH,
  MockVaultHub,
  MockVaultFactory,
} from "contracts/typechain-types";
import { expect } from "chai";
import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { ethers } from "hardhat";
import { EMPTY_CALLDATA, ONE_ETHER, ZERO_VALUE, CONNECT_DEPOSIT } from "../../common/constants";

describe("Integration tests with LineaRollup, YieldManager and LidoStVaultYieldProvider", () => {
  let nativeYieldOperator: SignerWithAddress;
  let nonAuthorizedAccount: SignerWithAddress;
  let l2YieldRecipient: SignerWithAddress;
  let securityCouncil: SignerWithAddress;

  let lineaRollup: TestLineaRollup;
  let yieldManager: TestYieldManager;
  let yieldProvider: TestLidoStVaultYieldProvider;
  let mockDashboard: MockDashboard;
  let mockStakingVault: MockStakingVault;
  let mockSTETH: MockSTETH;
  let mockVaultHub: MockVaultHub;
  let mockVaultFactory: MockVaultFactory;
  let lidoStVaultYieldProviderFactory: TestLidoStVaultYieldProviderFactory;
  let sszMerkleTree: SSZMerkleTree;
  let testVerifier: TestValidatorContainerProofVerifier;
  let initializationData: YieldManagerInitializationData;

  let l1MessageServiceAddress: string;
  let yieldManagerAddress: string;
  let yieldProviderAddress: string;
  let mockStakingVaultAddress: string;

  before(async () => {
    ({ nativeYieldOperator, nonAuthorizedAccount, l2YieldRecipient, securityCouncil } =
      await loadFixture(getAccountsFixture));
  });

  beforeEach(async () => {
    ({
      lineaRollup,
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
    } = await loadFixture(deployYieldManagerIntegrationTestFixture));
    l1MessageServiceAddress = await lineaRollup.getAddress();
    yieldManagerAddress = await yieldManager.getAddress();
    mockStakingVaultAddress = await mockStakingVault.getAddress();
  });

  describe("Initial state", () => {
    it("userFunds should be equivalent to CONNECT_DEPOSIT amount", async () => {
      const connectDeposit = await yieldProvider.CONNECT_DEPOSIT();
      expect(await yieldManager.userFunds(yieldProviderAddress)).eq(connectDeposit);
      expect(await yieldManager.userFundsInYieldProvidersTotal()).eq(connectDeposit);
    });
  });

  describe("Transfering to the YieldManager", () => {
    it("Should successfully transfer from LineaRollup to the YieldManager", async () => {
      const fundAmount = ONE_ETHER;
      await setWithdrawalReserveToMinimum(yieldManager);
      await incrementBalance(l1MessageServiceAddress, fundAmount);
      const rollupBalanceBefore = await ethers.provider.getBalance(l1MessageServiceAddress);
      const yieldManagerBalanceBefore = await ethers.provider.getBalance(yieldManagerAddress);
      // Act
      await lineaRollup.connect(nativeYieldOperator).transferFundsForNativeYield(fundAmount);
      // Assert
      const rollupBalanceAfter = await ethers.provider.getBalance(l1MessageServiceAddress);
      const yieldManagerBalanceAfter = await ethers.provider.getBalance(yieldManagerAddress);
      expect(rollupBalanceAfter).eq(rollupBalanceBefore - fundAmount);
      expect(yieldManagerBalanceAfter).eq(yieldManagerBalanceBefore + fundAmount);
    });
    it("Should revert when withdrawal reserve at minimum (including msg.value in the reserve computation)", async () => {
      // Arrange - %-based minimum at 20%
      await yieldManager
        .connect(securityCouncil)
        .setWithdrawalReserveParameters(buildSetWithdrawalReserveParams(initializationData, { minAmount: 0n }));
      await setBalance(await lineaRollup.getAddress(), 21n * ONE_ETHER);
      await setBalance(await yieldManager.getAddress(), 79n * ONE_ETHER);

      // Act
      const withdrawAmount = ONE_ETHER + 1n;
      const call = lineaRollup.connect(nativeYieldOperator).transferFundsForNativeYield(withdrawAmount);

      // Assert
      await expectRevertWithCustomError(yieldManager, call, "InsufficientWithdrawalReserve");
    });
    it("Should successfully accept cross-chain user transfer via receiveFundsFromReserve", async () => {
      // Arrange - Setup L1MessageService balance
      const transferAmount = ONE_ETHER;
      await setWithdrawalReserveToMinimum(yieldManager);
      await incrementBalance(l1MessageServiceAddress, transferAmount);
      // Arrange - setup L1MessageService message
      const recipientAddress = await nonAuthorizedAccount.getAddress();
      const calldata = await yieldManager.interface.encodeFunctionData("receiveFundsFromReserve", []);
      const reserveBalanceBefore = await getBalance(lineaRollup);
      const yieldManagerBalanceBefore = await getBalance(yieldManager);

      const claimParams = await setupLineaRollupMessageMerkleTree(
        lineaRollup,
        recipientAddress,
        yieldManagerAddress,
        transferAmount,
        calldata,
        securityCouncil,
      );

      // Act
      const claimCall = lineaRollup.connect(nonAuthorizedAccount).claimMessageWithProof(claimParams);
      await expect(claimCall).to.not.be.reverted;
      expect(await getBalance(lineaRollup)).eq(reserveBalanceBefore - transferAmount);
      expect(await getBalance(yieldManager)).eq(yieldManagerBalanceBefore + transferAmount);
    });
  });

  describe("Withdraw LST", () => {
    it("Should allow LST withdrawal when in deficit", async () => {
      // Arrange - setup user funds
      const initialFundAmount = ONE_ETHER;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, initialFundAmount);
      // Arrange - setup withdrawal reserve deficit
      await setBalance(l1MessageServiceAddress, ZERO_VALUE);
      // Arrange - setup L1MessageService message
      const withdrawAmount = initialFundAmount / 2n;
      const recipientAddress = await nonAuthorizedAccount.getAddress();
      const claimParams = await setupLineaRollupMessageMerkleTree(
        lineaRollup,
        recipientAddress,
        recipientAddress,
        withdrawAmount,
        EMPTY_CALLDATA,
        securityCouncil,
      );
      // Arrange - Get before figures
      const lstPrincipalBefore = await yieldManager.getYieldProviderLstLiabilityPrincipal(yieldProviderAddress);

      // Act
      const claimCall = lineaRollup
        .connect(nonAuthorizedAccount)
        .claimMessageWithProofAndWithdrawLST(claimParams, yieldProviderAddress);

      // Assert
      await expect(claimCall).to.not.be.reverted;
      const lstPrincipalAfter = await yieldManager.getYieldProviderLstLiabilityPrincipal(yieldProviderAddress);
      expect(lstPrincipalAfter).eq(lstPrincipalBefore + withdrawAmount);
    });
    it("Should not be allowed from claimMessage", async () => {
      // Arrange - setup user funds
      const initialFundAmount = ONE_ETHER;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, initialFundAmount);
      // Arrange - setup withdrawal reserve deficit
      await setBalance(l1MessageServiceAddress, ZERO_VALUE);
      // Arrange - setup L1MessageService message
      const withdrawAmount = 0n;
      const recipientAddress = await nonAuthorizedAccount.getAddress();
      const calldata = await yieldProvider.interface.encodeFunctionData("withdrawLST", [
        yieldProviderAddress,
        withdrawAmount,
        recipientAddress,
      ]);

      const claimParams = await setupLineaRollupMessageMerkleTree(
        lineaRollup,
        recipientAddress,
        yieldManagerAddress,
        withdrawAmount,
        calldata,
        securityCouncil,
      );

      // Act
      const claimCall = lineaRollup.connect(nonAuthorizedAccount).claimMessageWithProof(claimParams);
      await expectRevertWithCustomError(yieldManager, claimCall, "LSTWithdrawalNotAllowed");
    });
    it("Should not allow LST withdrawal amount > fund amount", async () => {
      // Arrange - setup user funds
      const initialFundAmount = ONE_ETHER;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, initialFundAmount);
      // Arrange - setup withdrawal reserve deficit
      await setBalance(l1MessageServiceAddress, ZERO_VALUE);
      // Arrange - setup L1MessageService message
      const withdrawAmount = initialFundAmount * 2n + CONNECT_DEPOSIT;
      const recipientAddress = await nonAuthorizedAccount.getAddress();
      const claimParams = await setupLineaRollupMessageMerkleTree(
        lineaRollup,
        recipientAddress,
        recipientAddress,
        withdrawAmount,
        EMPTY_CALLDATA,
        securityCouncil,
      );

      // Act
      const claimCall = lineaRollup
        .connect(nonAuthorizedAccount)
        .claimMessageWithProofAndWithdrawLST(claimParams, yieldProviderAddress);
      await expectRevertWithCustomError(yieldManager, claimCall, "LSTWithdrawalExceedsYieldProviderFunds");
    });
    it("Should revert if LST withdrawal is for another recipient", async () => {
      // Arrange - setup user funds
      const initialFundAmount = ONE_ETHER;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, initialFundAmount);
      // Arrange - setup withdrawal reserve deficit
      await setBalance(l1MessageServiceAddress, ZERO_VALUE);
      // Arrange - setup L1MessageService message
      const withdrawAmount = initialFundAmount / 2n;
      const recipientAddress = await nonAuthorizedAccount.getAddress();
      const claimParams = await setupLineaRollupMessageMerkleTree(
        lineaRollup,
        recipientAddress,
        recipientAddress,
        withdrawAmount,
        EMPTY_CALLDATA,
        securityCouncil,
      );

      // Act - Execute with different sender than _to
      const claimCall = lineaRollup
        .connect(l2YieldRecipient)
        .claimMessageWithProofAndWithdrawLST(claimParams, yieldProviderAddress);

      // Assert
      await expectRevertWithCustomError(lineaRollup, claimCall, "CallerNotLSTWithdrawalRecipient");
    });
    it("Should revert if LST withdrawal > rate limit", async () => {
      const rateLimit = await lineaRollup.limitInWei();

      const recipientAddress = await nonAuthorizedAccount.getAddress();
      const claimParams = await setupLineaRollupMessageMerkleTree(
        lineaRollup,
        recipientAddress,
        recipientAddress,
        rateLimit + 1n,
        EMPTY_CALLDATA,
        securityCouncil,
      );
      const call = lineaRollup
        .connect(nonAuthorizedAccount)
        .claimMessageWithProofAndWithdrawLST(claimParams, yieldProviderAddress);
      await expectRevertWithCustomError(lineaRollup, call, "RateLimitExceeded");
    });
    it("LST withdrawal should use previous rate limit", async () => {
      // Regular claim message for rate limit
      const rateLimit = await lineaRollup.limitInWei();
      await incrementBalance(l1MessageServiceAddress, rateLimit);
      const recipientAddress = await nonAuthorizedAccount.getAddress();
      const claimParams = await setupLineaRollupMessageMerkleTree(
        lineaRollup,
        recipientAddress,
        recipientAddress,
        rateLimit,
        EMPTY_CALLDATA,
        securityCouncil,
      );
      await lineaRollup.connect(nonAuthorizedAccount).claimMessageWithProof(claimParams);
      // Next claim message with LST withdrawal
      const claimParams2 = await setupLineaRollupMessageMerkleTree(
        lineaRollup,
        recipientAddress,
        recipientAddress,
        1n,
        EMPTY_CALLDATA,
        securityCouncil,
      );
      const call = lineaRollup
        .connect(nonAuthorizedAccount)
        .claimMessageWithProofAndWithdrawLST(claimParams2, yieldProviderAddress);
      await expectRevertWithCustomError(lineaRollup, call, "RateLimitExceeded");
    });
  });

  describe("Yield reporting", () => {
    it("Should report positive yield successfully", async () => {
      // Arrange - setup user funds
      const initialFundAmount = ONE_ETHER;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, initialFundAmount);
      // Arrange - Setup positive yield
      const yieldEarned = ONE_ETHER / 10n;
      await mockDashboard.setTotalValueReturn(initialFundAmount + CONNECT_DEPOSIT + yieldEarned);
      // Arrange - Get message params
      const nextMessageNumberBefore = await lineaRollup.nextMessageNumber();

      const expectedBytes = await encodeSendMessage(
        l1MessageServiceAddress,
        await l2YieldRecipient.getAddress(),
        0n,
        yieldEarned,
        nextMessageNumberBefore,
        EMPTY_CALLDATA,
      );
      const messageHash = ethers.keccak256(expectedBytes);

      // Act
      const [newReportedYield, outstandingNegativeYield] = await yieldManager
        .connect(nativeYieldOperator)
        .reportYield.staticCall(yieldProviderAddress, l2YieldRecipient);
      const call = await yieldManager.connect(nativeYieldOperator).reportYield(yieldProviderAddress, l2YieldRecipient);

      // Assert
      await expect(call)
        .to.emit(lineaRollup, "MessageSent")
        .withArgs(
          yieldManagerAddress,
          l2YieldRecipient,
          0,
          yieldEarned,
          nextMessageNumberBefore,
          EMPTY_CALLDATA,
          messageHash,
        );
      expect(newReportedYield).eq(yieldEarned);
      expect(outstandingNegativeYield).eq(0);
    });
    it("Should report negative yield successfully", async () => {
      // Arrange - setup user funds
      const initialFundAmount = ONE_ETHER;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, initialFundAmount);
      // Arrange - Setup negative yield
      const yieldEarned = 0n;
      await mockDashboard.setTotalValueReturn(yieldEarned + CONNECT_DEPOSIT);
      // Arrange - Get message params
      const nextMessageNumberBefore = await lineaRollup.nextMessageNumber();

      const expectedBytes = await encodeSendMessage(
        l1MessageServiceAddress,
        await l2YieldRecipient.getAddress(),
        0n,
        yieldEarned,
        nextMessageNumberBefore,
        EMPTY_CALLDATA,
      );
      const messageHash = ethers.keccak256(expectedBytes);

      // Act
      const [newReportedYield, outstandingNegativeYield] = await yieldManager
        .connect(nativeYieldOperator)
        .reportYield.staticCall(yieldProviderAddress, l2YieldRecipient);
      const call = await yieldManager.connect(nativeYieldOperator).reportYield(yieldProviderAddress, l2YieldRecipient);

      // Assert
      await expect(call)
        .to.emit(lineaRollup, "MessageSent")
        .withArgs(
          yieldManagerAddress,
          l2YieldRecipient,
          0,
          yieldEarned,
          nextMessageNumberBefore,
          EMPTY_CALLDATA,
          messageHash,
        );
      expect(newReportedYield).eq(0n);
      expect(outstandingNegativeYield).eq(initialFundAmount - yieldEarned);
    });
    it("Immediate subsequent positive yield report with no fund movement should have 0 incremental earned yield", async () => {
      // Arrange - setup user funds
      const initialFundAmount = ONE_ETHER;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, initialFundAmount);
      // Arrange - Setup positive yield
      const yieldEarned = ONE_ETHER / 10n;
      await mockDashboard.setTotalValueReturn(initialFundAmount + CONNECT_DEPOSIT + yieldEarned);
      // Arrange - Get message params
      const nextMessageNumberBefore = await lineaRollup.nextMessageNumber();

      const expectedBytes = await encodeSendMessage(
        l1MessageServiceAddress,
        await l2YieldRecipient.getAddress(),
        0n,
        yieldEarned,
        nextMessageNumberBefore,
        EMPTY_CALLDATA,
      );
      const messageHash = ethers.keccak256(expectedBytes);

      // Act
      const [newReportedYield, outstandingNegativeYield] = await yieldManager
        .connect(nativeYieldOperator)
        .reportYield.staticCall(yieldProviderAddress, l2YieldRecipient);
      const call = await yieldManager.connect(nativeYieldOperator).reportYield(yieldProviderAddress, l2YieldRecipient);
      const [newReportedYield2, outstandingNegativeYield2] = await yieldManager
        .connect(nativeYieldOperator)
        .reportYield.staticCall(yieldProviderAddress, l2YieldRecipient);

      // Assert
      await expect(call)
        .to.emit(lineaRollup, "MessageSent")
        .withArgs(
          yieldManagerAddress,
          l2YieldRecipient,
          0,
          yieldEarned,
          nextMessageNumberBefore,
          EMPTY_CALLDATA,
          messageHash,
        );
      expect(newReportedYield).eq(yieldEarned);
      expect(outstandingNegativeYield).eq(0);
      expect(newReportedYield2).eq(0);
      expect(outstandingNegativeYield2).eq(0);
    });
    it("Immediate subsequent negative yield report with no fund movement should have same outstanding negative yield", async () => {
      // Arrange - setup user funds
      const initialFundAmount = ONE_ETHER;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, initialFundAmount);
      // Arrange - Setup negative yield
      const yieldEarned = 0n;
      await mockDashboard.setTotalValueReturn(yieldEarned + CONNECT_DEPOSIT);
      // Arrange - Get message params
      const nextMessageNumberBefore = await lineaRollup.nextMessageNumber();

      const expectedBytes = await encodeSendMessage(
        l1MessageServiceAddress,
        await l2YieldRecipient.getAddress(),
        0n,
        yieldEarned,
        nextMessageNumberBefore,
        EMPTY_CALLDATA,
      );
      const messageHash = ethers.keccak256(expectedBytes);

      // Act
      const [newReportedYield, outstandingNegativeYield] = await yieldManager
        .connect(nativeYieldOperator)
        .reportYield.staticCall(yieldProviderAddress, l2YieldRecipient);
      const call = await yieldManager.connect(nativeYieldOperator).reportYield(yieldProviderAddress, l2YieldRecipient);
      const [newReportedYield2, outstandingNegativeYield2] = await yieldManager
        .connect(nativeYieldOperator)
        .reportYield.staticCall(yieldProviderAddress, l2YieldRecipient);

      // Assert
      await expect(call)
        .to.emit(lineaRollup, "MessageSent")
        .withArgs(
          yieldManagerAddress,
          l2YieldRecipient,
          0,
          yieldEarned,
          nextMessageNumberBefore,
          EMPTY_CALLDATA,
          messageHash,
        );
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
      await mockDashboard.setTotalValueReturn(initialFundAmount + yieldEarned);
      await yieldManager.connect(nativeYieldOperator).reportYield(yieldProviderAddress, l2YieldRecipient);

      // Act
      const call = yieldManager.connect(securityCouncil).removeYieldProvider(yieldProviderAddress, EMPTY_CALLDATA);

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
      await mockDashboard.setTotalValueReturn(initialFundAmount + yieldEarned);
      await yieldManager.connect(nativeYieldOperator).reportYield(yieldProviderAddress, l2YieldRecipient);
      // Arrange - withdraw vault funds
      await incrementBalance(await mockStakingVault.getAddress(), ONE_ETHER);
      await setWithdrawalReserveToTarget(yieldManager);
      await mockDashboard.setWithdrawableValueReturn(ONE_ETHER * 2n);
      await yieldManager
        .connect(nativeYieldOperator)
        .safeWithdrawFromYieldProvider(yieldProviderAddress, ONE_ETHER * 2n);
      // Arrange - Remove yield provider
      await yieldManager.connect(securityCouncil).removeYieldProvider(yieldProviderAddress, buildVendorExitData());
      // Arrange - Add new yield provider
      const { yieldProviderAddress: yieldProvider2Address } = await deployAndAddAdditionalLidoStVaultYieldProvider(
        lidoStVaultYieldProviderFactory,
        yieldManager,
        securityCouncil,
        mockVaultFactory,
      );

      // Act - Move funds to new YieldProvider
      await yieldManager.connect(nativeYieldOperator).fundYieldProvider(yieldProvider2Address, ONE_ETHER);
    });
  });

  describe("Withdrawals", () => {
    it("safeAddToWithdrawalReserve should paydown LST liability", async () => {
      // Arrange - initial 10 ETH fund
      const initialFundAmount = ONE_ETHER * 10n;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, initialFundAmount);
      // Arrange - 5 ETH LST withdrawal
      await setBalance(l1MessageServiceAddress, ZERO_VALUE);
      const withdrawLSTAmount = ONE_ETHER * 5n;
      const recipientAddress = await nonAuthorizedAccount.getAddress();
      const claimParams = await setupLineaRollupMessageMerkleTree(
        lineaRollup,
        recipientAddress,
        recipientAddress,
        withdrawLSTAmount,
        EMPTY_CALLDATA,
        securityCouncil,
      );
      await lineaRollup
        .connect(nonAuthorizedAccount)
        .claimMessageWithProofAndWithdrawLST(claimParams, yieldProviderAddress);
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
      await mockDashboard.setWithdrawableValueReturn(withdrawAmount);
      // Arrange - Before figures
      const lstLiabilityPrincipalBefore =
        await yieldManager.getYieldProviderLstLiabilityPrincipal(yieldProviderAddress);

      // Act
      await yieldManager.connect(nativeYieldOperator).safeAddToWithdrawalReserve(yieldProviderAddress, withdrawAmount);

      // Assert
      expect(await yieldManager.getYieldProviderLstLiabilityPrincipal(yieldProviderAddress)).eq(
        lstLiabilityPrincipalBefore - withdrawAmount,
      );
    });
    it("safeWithdrawFromYieldProvider should paydown LST liability", async () => {
      // Arrange - initial 10 ETH fund
      const initialFundAmount = ONE_ETHER * 10n;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, initialFundAmount);
      // Arrange - 5 ETH LST withdrawal
      await setBalance(l1MessageServiceAddress, ZERO_VALUE);
      const withdrawLSTAmount = ONE_ETHER * 5n;
      const recipientAddress = await nonAuthorizedAccount.getAddress();
      const claimParams = await setupLineaRollupMessageMerkleTree(
        lineaRollup,
        recipientAddress,
        recipientAddress,
        withdrawLSTAmount,
        EMPTY_CALLDATA,
        securityCouncil,
      );
      await lineaRollup
        .connect(nonAuthorizedAccount)
        .claimMessageWithProofAndWithdrawLST(claimParams, yieldProviderAddress);
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
      await mockDashboard.setWithdrawableValueReturn(withdrawAmount);
      // Arrange - Before figures
      const lstLiabilityPrincipalBefore =
        await yieldManager.getYieldProviderLstLiabilityPrincipal(yieldProviderAddress);

      // Act
      await yieldManager
        .connect(nativeYieldOperator)
        .safeWithdrawFromYieldProvider(yieldProviderAddress, withdrawAmount);

      // Assert
      expect(await yieldManager.getYieldProviderLstLiabilityPrincipal(yieldProviderAddress)).eq(
        lstLiabilityPrincipalBefore - withdrawAmount,
      );
    });
    it("replenishWithdrawalReserve should not paydown LST liability", async () => {
      // Arrange - initial 10 ETH fund
      const initialFundAmount = ONE_ETHER * 10n;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, initialFundAmount);
      // Arrange - 5 ETH LST withdrawal
      await setBalance(l1MessageServiceAddress, ZERO_VALUE);
      const withdrawLSTAmount = ONE_ETHER * 5n;
      const recipientAddress = await nonAuthorizedAccount.getAddress();
      const claimParams = await setupLineaRollupMessageMerkleTree(
        lineaRollup,
        recipientAddress,
        recipientAddress,
        withdrawLSTAmount,
        EMPTY_CALLDATA,
        securityCouncil,
      );
      await lineaRollup
        .connect(nonAuthorizedAccount)
        .claimMessageWithProofAndWithdrawLST(claimParams, yieldProviderAddress);
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
      await mockDashboard.setWithdrawableValueReturn(withdrawAmount);
      // Arrange - Before figures
      const lstLiabilityPrincipalBefore =
        await yieldManager.getYieldProviderLstLiabilityPrincipal(yieldProviderAddress);

      // Act
      await yieldManager.replenishWithdrawalReserve(yieldProviderAddress);

      // Assert
      expect(await yieldManager.getYieldProviderLstLiabilityPrincipal(yieldProviderAddress)).eq(
        lstLiabilityPrincipalBefore,
      );
    });
  });

  describe("Native yield resilience scenarios", () => {
    it("Should not reduce user funds after repeated negative yield reports", async () => {
      // Arrange - setup user funds
      const initialFundAmount = ONE_ETHER * 10n;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, initialFundAmount);
      const userFundsInYieldProvidersTotalBefore = await yieldManager.userFundsInYieldProvidersTotal();
      const userFundsBefore = await yieldManager.userFunds(yieldProviderAddress);
      // Arrange - Setup first negative yield
      const firstNegativeYield = ONE_ETHER * 5n;
      const secondNegativeYield = ONE_ETHER * 5n;
      await mockDashboard.setTotalValueReturn(
        initialFundAmount + CONNECT_DEPOSIT - firstNegativeYield - secondNegativeYield,
      );

      // Act
      const [newReportedYield, outstandingNegativeYield] = await yieldManager
        .connect(nativeYieldOperator)
        .reportYield.staticCall(yieldProviderAddress, l2YieldRecipient);
      await yieldManager.connect(nativeYieldOperator).reportYield(yieldProviderAddress, l2YieldRecipient);

      // Assert
      expect(await yieldManager.userFundsInYieldProvidersTotal()).eq(userFundsInYieldProvidersTotalBefore);
      expect(await yieldManager.userFunds(yieldProviderAddress)).eq(userFundsBefore);
      expect(newReportedYield).eq(0);
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
      await mockDashboard.setTotalValueReturn(initialFundAmount - firstNegativeYield);
      await decrementBalance(mockStakingVaultAddress, firstNegativeYield);
      {
        const [newReportedYield, outstandingNegativeYield] = await yieldManager
          .connect(nativeYieldOperator)
          .reportYield.staticCall(yieldProviderAddress, l2YieldRecipient);
        expect(newReportedYield).eq(0);
        expect(outstandingNegativeYield).eq(firstNegativeYield);
      }
      await yieldManager.connect(nativeYieldOperator).reportYield(yieldProviderAddress, l2YieldRecipient);

      // Drain all L1MessageService funds
      await setBalance(l1MessageServiceAddress, ZERO_VALUE);

      // Max LST withdrawal for 5 ETH
      await lineaRollup.connect(securityCouncil).resetRateLimitAmount(ONE_ETHER * 100n);
      const withdrawLSTAmount = ONE_ETHER * 5n;
      const recipientAddress = await nonAuthorizedAccount.getAddress();
      const claimParams = await setupLineaRollupMessageMerkleTree(
        lineaRollup,
        recipientAddress,
        recipientAddress,
        withdrawLSTAmount,
        EMPTY_CALLDATA,
        securityCouncil,
      );
      await lineaRollup
        .connect(nonAuthorizedAccount)
        .claimMessageWithProofAndWithdrawLST(claimParams, yieldProviderAddress);
      await mockSTETH.setPooledEthBySharesRoundUpReturn(withdrawLSTAmount);

      // Assert - Cannot do another LST withdrawal
      const claimParams2 = await setupLineaRollupMessageMerkleTree(
        lineaRollup,
        recipientAddress,
        recipientAddress,
        1n,
        EMPTY_CALLDATA,
        securityCouncil,
      );
      const secondWithdrawLSTCall = lineaRollup
        .connect(nonAuthorizedAccount)
        .claimMessageWithProofAndWithdrawLST(claimParams2, yieldProviderAddress);
      await expectRevertWithCustomError(yieldManager, secondWithdrawLSTCall, "LSTWithdrawalExceedsYieldProviderFunds");
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.replenishWithdrawalReserve(yieldProviderAddress),
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
      let stakingVaultBalance = await ethers.provider.getBalance(mockStakingVaultAddress);
      await mockDashboard.setTotalValueReturn(stakingVaultBalance);
      let userFunds = await yieldManager.userFunds(yieldProviderAddress);
      {
        const [newReportedYield, outstandingNegativeYield] = await yieldManager
          .connect(nativeYieldOperator)
          .reportYield.staticCall(yieldProviderAddress, l2YieldRecipient);
        expect(newReportedYield).eq(0);
        expect(outstandingNegativeYield).eq(firstNegativeYield + withdrawLSTAmount - firstPositiveYield);
      }
      await yieldManager.connect(nativeYieldOperator).reportYield(yieldProviderAddress, l2YieldRecipient);
      expect(await yieldManager.userFunds(yieldProviderAddress)).eq(
        userFunds + firstPositiveYield - firstNegativeYield,
      );
      expect(await yieldManager.getYieldProviderYieldReportedCumulative(yieldProviderAddress)).eq(0);
      expect(await yieldManager.userFundsInYieldProvidersTotal()).eq(
        await yieldManager.userFunds(yieldProviderAddress),
      );
      console.log(await yieldManager.getYieldProviderLstLiabilityPrincipal(yieldProviderAddress));

      // Add to withdrawal reserve (expect route all funds to L1MessageService)
      stakingVaultBalance = await ethers.provider.getBalance(mockStakingVaultAddress);
      userFunds = await yieldManager.userFunds(yieldProviderAddress);
      await mockDashboard.setWithdrawableValueReturn(stakingVaultBalance);
      const l1MessageBalance = await ethers.provider.getBalance(l1MessageServiceAddress);
      await yieldManager
        .connect(nativeYieldOperator)
        .safeAddToWithdrawalReserve(yieldProviderAddress, stakingVaultBalance);
      expect(await yieldManager.userFunds(yieldProviderAddress)).eq(0);
      expect(await ethers.provider.getBalance(l1MessageServiceAddress)).eq(l1MessageBalance + userFunds);

      // Donation to replenish reserve to target
      await lineaRollup.connect(securityCouncil).fund({ value: await yieldManager.getTargetReserveDeficit() });

      // Fresh deposit of 10 ETH -> should clear all existing LST principal
      const secondFundAmount = ONE_ETHER * 10n;
      await lineaRollup.connect(nativeYieldOperator).transferFundsForNativeYield(secondFundAmount);
      await yieldManager.connect(nativeYieldOperator).fundYieldProvider(yieldProviderAddress, secondFundAmount);

      expect(await yieldManager.getYieldProviderLstLiabilityPrincipal(yieldProviderAddress)).eq(0);
    });

    it("Should be able to safely handle different yield report situations", async () => {
      // Initial funding with 10 ETH
      const initialFundAmount = ONE_ETHER * 10n;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, initialFundAmount);

      // First negative yield event for -5 ETH
      const firstNegativeYield = ONE_ETHER * 5n;
      await mockDashboard.setTotalValueReturn(initialFundAmount + CONNECT_DEPOSIT - firstNegativeYield);
      await decrementBalance(mockStakingVaultAddress, firstNegativeYield);
      {
        const [newReportedYield, outstandingNegativeYield] = await yieldManager
          .connect(nativeYieldOperator)
          .reportYield.staticCall(yieldProviderAddress, l2YieldRecipient);
        expect(newReportedYield).eq(0);
        expect(outstandingNegativeYield).eq(firstNegativeYield);
      }
      await yieldManager.connect(nativeYieldOperator).reportYield(yieldProviderAddress, l2YieldRecipient);

      // Second negative yield of -2 ETH due to obligation settlement
      const firstObligationsPaid = ONE_ETHER * 2n;
      await mockDashboard.setObligationsFeesToSettleReturn(firstObligationsPaid);
      const mockStakingVaultBalance = await getBalance(mockStakingVault);
      await mockVaultHub.setIsSettleLidoFeesWithdrawingFromVault(true);
      await mockVaultHub.setSettleVaultObligationAmount(firstObligationsPaid);
      {
        const [newReportedYield, outstandingNegativeYield] = await yieldManager
          .connect(nativeYieldOperator)
          .reportYield.staticCall(yieldProviderAddress, l2YieldRecipient);
        expect(newReportedYield).eq(0);
        expect(outstandingNegativeYield).eq(firstNegativeYield + firstObligationsPaid);
      }
      await yieldManager.connect(nativeYieldOperator).reportYield(yieldProviderAddress, l2YieldRecipient);
      expect(await yieldManager.userFunds(yieldProviderAddress)).eq(initialFundAmount + CONNECT_DEPOSIT);
      expect(await getBalance(mockStakingVault)).eq(mockStakingVaultBalance - firstObligationsPaid);
      await mockDashboard.setTotalValueReturn((await mockDashboard.totalValue()) - firstObligationsPaid);
      await mockDashboard.setObligationsFeesToSettleReturn(0n);
      await mockVaultHub.setSettleVaultObligationAmount(0n);

      // First positive yield of 4 ETH, not enough to recover from all negative yield incurred so far
      const firstPositiveYield = ONE_ETHER * 4n;
      await mockDashboard.setTotalValueReturn((await mockDashboard.totalValue()) + firstPositiveYield);
      await incrementBalance(mockStakingVaultAddress, firstPositiveYield);
      {
        const [newReportedYield, outstandingNegativeYield] = await yieldManager
          .connect(nativeYieldOperator)
          .reportYield.staticCall(yieldProviderAddress, l2YieldRecipient);
        expect(newReportedYield).eq(0);
        expect(outstandingNegativeYield).eq(firstNegativeYield + firstObligationsPaid - firstPositiveYield);
      }
      await yieldManager.connect(nativeYieldOperator).reportYield(yieldProviderAddress, l2YieldRecipient);
      expect(await yieldManager.getYieldProviderYieldReportedCumulative(yieldProviderAddress)).eq(0n);

      // Second positive yield of 4 ETH
      // +1 ETH in Vault relative to start
      const secondPositiveYield = ONE_ETHER * 4n;
      await mockDashboard.setTotalValueReturn((await mockDashboard.totalValue()) + secondPositiveYield);
      await incrementBalance(mockStakingVaultAddress, secondPositiveYield);
      {
        const [newReportedYield, outstandingNegativeYield] = await yieldManager
          .connect(nativeYieldOperator)
          .reportYield.staticCall(yieldProviderAddress, l2YieldRecipient);
        expect(newReportedYield).eq(ONE_ETHER);
        expect(outstandingNegativeYield).eq(0n);
      }
      await yieldManager.connect(nativeYieldOperator).reportYield(yieldProviderAddress, l2YieldRecipient);
      expect(await yieldManager.getYieldProviderYieldReportedCumulative(yieldProviderAddress)).eq(ONE_ETHER);

      // Third positive yield of 3 ETH
      // -2 ETH of node operators fees
      // -3 ETH of protocol fees
      // Expect end with -2 ETH
      const thirdPositiveYield = ONE_ETHER * 3n;
      await mockDashboard.setTotalValueReturn((await mockDashboard.totalValue()) + thirdPositiveYield);
      await incrementBalance(mockStakingVaultAddress, thirdPositiveYield);
      await mockDashboard.setObligationsFeesToSettleReturn(ONE_ETHER * 3n);
      await mockVaultHub.setIsSettleLidoFeesWithdrawingFromVault(true);
      await mockVaultHub.setSettleVaultObligationAmount(ONE_ETHER * 3n);
      await mockDashboard.setIsDisburseFeeWithdrawingFromVault(true);
      await mockDashboard.setAccruedFeeReturn(ONE_ETHER * 2n);
      {
        const [newReportedYield, outstandingNegativeYield] = await yieldManager
          .connect(nativeYieldOperator)
          .reportYield.staticCall(yieldProviderAddress, l2YieldRecipient);
        expect(newReportedYield).eq(0);
        // Obligations was paid, node operator fees was not
        expect(outstandingNegativeYield).eq(ONE_ETHER * 2n);
      }
      await yieldManager.connect(nativeYieldOperator).reportYield(yieldProviderAddress, l2YieldRecipient);
      await mockDashboard.setTotalValueReturn((await mockDashboard.totalValue()) - ONE_ETHER * 5n);
      await mockVaultHub.setSettleVaultObligationAmount(0n);
      await mockDashboard.setObligationsFeesToSettleReturn(0n);
      await mockDashboard.setAccruedFeeReturn(0n);
      expect(await yieldManager.getYieldProviderYieldReportedCumulative(yieldProviderAddress)).eq(ONE_ETHER);

      // Fourth positive yield of 7 ETH
      // LST liability of -2 ETH
      await mockSTETH.setPooledEthBySharesRoundUpReturn(ONE_ETHER * 2n);
      await mockDashboard.setRebalanceVaultWithSharesWithdrawingFromVault(true);
      const fourthPositiveYield = ONE_ETHER * 7n;
      await mockDashboard.setTotalValueReturn((await mockDashboard.totalValue()) + fourthPositiveYield);
      await incrementBalance(mockStakingVaultAddress, fourthPositiveYield);
      {
        const [newReportedYield, outstandingNegativeYield] = await yieldManager
          .connect(nativeYieldOperator)
          .reportYield.staticCall(yieldProviderAddress, l2YieldRecipient);
        expect(newReportedYield).eq(ONE_ETHER * 3n);
        // Obligations was paid, node operator fees was not
        expect(outstandingNegativeYield).eq(0n);
      }
      await yieldManager.connect(nativeYieldOperator).reportYield(yieldProviderAddress, l2YieldRecipient);
      await mockDashboard.setTotalValueReturn((await mockDashboard.totalValue()) - ONE_ETHER * 2n);
      await mockSTETH.setPooledEthBySharesRoundUpReturn(0);
      expect(await yieldManager.getYieldProviderYieldReportedCumulative(yieldProviderAddress)).eq(ONE_ETHER * 4n);
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
      await lineaRollup.connect(nativeYieldOperator).transferFundsForNativeYield(secondFundAmount);
      await yieldManager.connect(nativeYieldOperator).fundYieldProvider(yieldProviderAddress, secondFundAmount);
      await incrementMockDashboardTotalValue(mockDashboard, secondFundAmount);

      // Earn 5 ETH positive yield
      const firstPositiveYield = ONE_ETHER * 5n;
      await mockDashboard.setTotalValueReturn((await mockDashboard.totalValue()) + CONNECT_DEPOSIT);
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
        lineaRollup.connect(nativeYieldOperator).transferFundsForNativeYield(1n),
        "InsufficientWithdrawalReserve",
      );

      // Create LST withdrawal - 5 ETH. Should cause staking pause.
      await setBalance(l1MessageServiceAddress, (await lineaRollup.limitInWei()) - 1n);
      const lstWithdrawalAmount = await lineaRollup.limitInWei();
      await withdrawLST(lineaRollup, nonAuthorizedAccount, yieldProviderAddress, lstWithdrawalAmount, securityCouncil);
      expect(await yieldManager.isStakingPaused(yieldProviderAddress)).eq(true);

      // Do permissionless rebalance (should not paydown LST liability)
      const stakingVaultBalance = await getBalance(mockStakingVault);
      const l1MessageServiceBalance = await getBalance(lineaRollup);
      const withdrawableValue = stakingVaultBalance - lstWithdrawalAmount; // Simulate locked balance
      await mockDashboard.setWithdrawableValueReturn(withdrawableValue); // Simulate locked balance
      await setupMaxLSTLiabilityPaymentForWithdrawal(
        yieldManager,
        mockDashboard,
        mockVaultHub,
        mockSTETH,
        yieldProviderAddress,
        lstWithdrawalAmount,
      );
      await yieldManager.connect(nonAuthorizedAccount).replenishWithdrawalReserve(yieldProviderAddress);
      expect(await yieldManager.getYieldProviderLstLiabilityPrincipal(yieldProviderAddress)).eq(lstWithdrawalAmount);
      expect(await getBalance(mockStakingVault)).eq(lstWithdrawalAmount);
      await mockDashboard.setTotalValueReturn(0);
      expect(await getBalance(lineaRollup)).eq(l1MessageServiceBalance + withdrawableValue);

      // Call unstakePermissionless
      await executeUnstakePermissionless(
        sszMerkleTree,
        testVerifier,
        yieldManager,
        yieldProviderAddress,
        mockStakingVaultAddress,
        await nativeYieldOperator.getAddress(),
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
      await yieldManager.connect(securityCouncil).initiateOssification(yieldProviderAddress);

      // Complete ossification
      await mockVaultHub.setIsVaultConnectedReturn(false);
      await yieldManager.connect(nativeYieldOperator).progressPendingOssification(yieldProviderAddress);
    });
  });
});
