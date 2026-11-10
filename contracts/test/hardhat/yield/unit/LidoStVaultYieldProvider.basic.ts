import { expect } from "chai";
import hre, { network } from "hardhat";
import { describe, it, before, beforeEach } from "node:test";
import { type Address, type Hex, zeroAddress } from "viem";

import { encodeData, randomBytes32 } from "../../../../common/helpers/encoding.js";
import {
  ONE_ETHER,
  ZERO_VALUE,
  EMPTY_CALLDATA,
  ONE_GWEI,
  ProgressOssificationResult,
  YieldProviderVendor,
  OperationType,
  BEACON_PROOF_WITNESS_TYPE,
} from "../../common/constants/index.js";
import { deployFromFactory } from "../../common/deployment.js";
import {
  type TestAccount,
  generateRandomBytes,
  getAccountsFixture,
  hex,
  expectZeroAddressRevert,
  expectEvent,
  expectRevertWithCustomError,
} from "../../common/helpers/index.js";
import {
  buildVendorExitData,
  buildVendorInitializationData,
  deployAndAddSingleLidoStVaultYieldProvider,
  expectRevert,
  fundLidoStVaultYieldProvider,
  getBalance,
  getImpersonatedAccount,
  getWithdrawLSTCall,
  incrementBalance,
  ossifyYieldProvider,
  randomAddress,
  setupLSTPrincipalDecrementForPaxMaximumPossibleLSTLiability,
  setBalance,
  setWithdrawalReserveToMinimum,
} from "../helpers/index.js";
import { generateLidoUnstakePermissionlessWitness } from "../helpers/proof.js";

import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

type MockVaultHub = ContractReturnType<"MockVaultHub">;
type MockVaultFactory = ContractReturnType<"MockVaultFactory">;
type MockSTETH = ContractReturnType<"MockSTETH">;
type MockLinethRollup = ContractReturnType<"MockLinethRollup">;
type TestYieldManager = ContractReturnType<"TestYieldManager">;
type MockDashboard = ContractReturnType<"MockDashboard">;
type MockStakingVault = ContractReturnType<"MockStakingVault">;
type TestLidoStVaultYieldProvider = ContractReturnType<"TestLidoStVaultYieldProvider">;
type ValidatorContainerProofVerifier = ContractReturnType<"ValidatorContainerProofVerifier">;
type SSZMerkleTree = ContractReturnType<"SSZMerkleTree">;
type TestValidatorContainerProofVerifier = ContractReturnType<"TestValidatorContainerProofVerifier">;

/** Asserts that the transaction `asyncCall` resolves to was mined successfully (chai-matchers' `.to.not.be.reverted`). */
async function expectNotReverted(asyncCall: Promise<Hex>): Promise<void> {
  const { viem } = await network.getOrCreate();
  const publicClient = await viem.getPublicClient();
  const receipt = await publicClient.waitForTransactionReceipt({ hash: await asyncCall });
  expect(receipt.status).to.equal("success");
}

describe("LidoStVaultYieldProvider contract - basic operations", () => {
  let yieldProvider: TestLidoStVaultYieldProvider;
  let nativeYieldOperator: TestAccount;
  let securityCouncil: TestAccount;
  let mockVaultHub: MockVaultHub;
  let mockVaultFactory: MockVaultFactory;
  let mockSTETH: MockSTETH;
  let mockLinethRollup: MockLinethRollup;
  let yieldManager: TestYieldManager;
  let mockDashboard: MockDashboard;
  let mockStakingVault: MockStakingVault;
  let sszMerkleTree: SSZMerkleTree;
  let verifier: ValidatorContainerProofVerifier;
  let testVerifier: TestValidatorContainerProofVerifier;

  let l1MessageServiceAddress: Address;
  let yieldManagerAddress: Address;
  let vaultHubAddress: Address;
  let vaultFactoryAddress: Address;
  let stethAddress: Address;
  let mockDashboardAddress: Address;
  let mockStakingVaultAddress: Address;
  let yieldProviderAddress: Address;
  let verifierAddress: Address;

  before(async () => {
    const { networkHelpers } = await network.getOrCreate();
    ({ nativeYieldOperator, securityCouncil } = await networkHelpers.loadFixture(getAccountsFixture));
  });

  beforeEach(async () => {
    const { networkHelpers } = await network.getOrCreate();
    ({
      yieldProvider,
      yieldProviderAddress,
      mockDashboard,
      mockStakingVault,
      yieldManager,
      mockVaultHub,
      mockVaultFactory,
      mockSTETH,
      mockLinethRollup,
      sszMerkleTree,
      verifier,
      testVerifier,
    } = await networkHelpers.loadFixture(deployAndAddSingleLidoStVaultYieldProvider));

    l1MessageServiceAddress = mockLinethRollup.address;
    yieldManagerAddress = yieldManager.address;
    vaultHubAddress = mockVaultHub.address;
    vaultFactoryAddress = mockVaultFactory.address;
    stethAddress = mockSTETH.address;
    mockDashboardAddress = mockDashboard.address;
    mockStakingVaultAddress = mockStakingVault.address;
    verifierAddress = verifier.address;
  });

  describe("Constructor", () => {
    // Configuration for constructor arguments
    interface ConstructorConfig {
      l1MessageService: Address;
      yieldManager: Address;
      vaultHub: Address;
      vaultFactory: Address;
      steth: Address;
      validatorContainerProofVerifier: Address;
    }

    // Convert config to args tuple
    const toArgs = (config: ConstructorConfig): [Address, Address, Address, Address, Address, Address] => [
      config.l1MessageService,
      config.yieldManager,
      config.vaultHub,
      config.vaultFactory,
      config.steth,
      config.validatorContainerProofVerifier,
    ];

    // Parameterized zero address validation tests
    const zeroAddressValidationCases: Array<{
      name: string;
      field: keyof ConstructorConfig;
    }> = [
      { name: "_l1MessageService", field: "l1MessageService" },
      { name: "_yieldManager", field: "yieldManager" },
      { name: "_vaultHub", field: "vaultHub" },
      { name: "_vaultFactory", field: "vaultFactory" },
      { name: "_steth", field: "steth" },
      { name: "_validatorContainerProofVerifier", field: "validatorContainerProofVerifier" },
    ];

    zeroAddressValidationCases.forEach(({ name, field }) => {
      it(`Should revert if 0 address provided for ${name}`, async () => {
        const { abi } = await hre.artifacts.readArtifact("TestLidoStVaultYieldProvider");

        const defaultConfig: ConstructorConfig = {
          l1MessageService: l1MessageServiceAddress,
          yieldManager: yieldManagerAddress,
          vaultHub: vaultHubAddress,
          vaultFactory: vaultFactoryAddress,
          steth: stethAddress,
          validatorContainerProofVerifier: verifierAddress,
        };

        const args = toArgs({ ...defaultConfig, [field]: zeroAddress });

        await expectZeroAddressRevert({
          contract: { abi },
          deployOrInitCall: deployFromFactory("TestLidoStVaultYieldProvider", args),
        });
      });
    });

    it("Should succeed and emit the correct deployment event", async () => {
      const { viem } = await network.getOrCreate();
      const { contract, deploymentTransaction } = await viem.sendDeploymentTransaction("TestLidoStVaultYieldProvider", [
        l1MessageServiceAddress,
        yieldManagerAddress,
        vaultHubAddress,
        vaultFactoryAddress,
        stethAddress,
        verifierAddress,
      ]);
      await expectEvent(contract, Promise.resolve(deploymentTransaction.hash), "LidoStVaultYieldProviderDeployed", [
        l1MessageServiceAddress,
        yieldManagerAddress,
        vaultHubAddress,
        vaultFactoryAddress,
        stethAddress,
        verifierAddress,
      ]);
    });
  });

  describe("Immutables", () => {
    it("Should deploy with correct VaultHub address", async () => {
      expect(await yieldProvider.read.VAULT_HUB()).eq(mockVaultHub.address);
    });
    it("Should deploy with correct STETH address", async () => {
      expect(await yieldProvider.read.STETH()).eq(mockSTETH.address);
    });
    it("Should deploy with correct L1MessageService address", async () => {
      expect(await yieldProvider.read.L1_MESSAGE_SERVICE()).eq(mockLinethRollup.address);
    });
    it("Should deploy with correct YieldManager address", async () => {
      expect(await yieldProvider.read.YIELD_MANAGER()).eq(yieldManager.address);
    });
    it("Should deploy with correct ValidatorContainerProofVerifier address", async () => {
      expect(await yieldProvider.read.VALIDATOR_CONTAINER_PROOF_VERIFIER()).eq(verifier.address);
    });
  });

  describe("withdrawableValue", () => {
    it("Should revert if not invoked via delegatecall", async () => {
      const call = yieldProvider.write.withdrawableValue([yieldProviderAddress]);
      await expectRevertWithCustomError(yieldProvider, call, "ContextIsNotYieldManager");
    });
    it("If not ossified, should return withdrawable value from Dashboard", async () => {
      // Arrange
      const expectedWithdrawableValue = 99n;
      await mockDashboard.write.setWithdrawableValueReturn([expectedWithdrawableValue]);
      await yieldManager.write.setYieldProviderUserFunds([yieldProviderAddress, expectedWithdrawableValue + 10n]);
      // Act
      const withdrawableValue = (await yieldManager.simulate.withdrawableValue([yieldProviderAddress])).result;
      // Assert
      expect(withdrawableValue).eq(expectedWithdrawableValue);
    });
    it("If ossified, should return staking vault balance", async () => {
      // Arrange - Ossify
      await ossifyYieldProvider(yieldManager, yieldProviderAddress, securityCouncil);
      // Arrange - Set Staking Vault Balance
      const expectedWithdrawableValue = 99n;
      await incrementBalance(mockStakingVaultAddress, expectedWithdrawableValue);
      await yieldManager.write.setYieldProviderUserFunds([yieldProviderAddress, expectedWithdrawableValue + 10n]);
      // Act
      const withdrawableValue = (await yieldManager.simulate.withdrawableValue([yieldProviderAddress])).result;
      // Assert
      expect(withdrawableValue).eq(expectedWithdrawableValue);
    });
  });

  describe("getEntrypointContract", () => {
    it("If not ossified, should return the Dashboard address", async () => {
      const entryPoint = (await yieldManager.simulate.getEntrypointContract([yieldProviderAddress])).result;
      expect(entryPoint).eq(mockDashboardAddress);
    });
    it("If ossified, should return the Vault address", async () => {
      await ossifyYieldProvider(yieldManager, yieldProviderAddress, securityCouncil);
      const entryPoint = (await yieldManager.simulate.getEntrypointContract([yieldProviderAddress])).result;
      expect(entryPoint).eq(mockStakingVaultAddress);
    });
  });

  describe("getDashboard", () => {
    it("should return the Dashboard address", async () => {
      const contract = (await yieldManager.simulate.getEntrypointContract([yieldProviderAddress])).result;
      expect(contract).eq(mockDashboardAddress);
    });
  });

  describe("getVault", () => {
    it("should return the Dashboard address", async () => {
      const contract = (await yieldManager.simulate.getVault([yieldProviderAddress])).result;
      expect(contract).eq(mockStakingVaultAddress);
    });
  });

  describe("fund YieldProvider", () => {
    it("Should revert if not invoked via delegatecall", async () => {
      const call = yieldProvider.write.fundYieldProvider([yieldProviderAddress, BigInt(ZERO_VALUE)], {
        account: securityCouncil.account,
      });
      await expectRevertWithCustomError(yieldProvider, call, "ContextIsNotYieldManager");
    });
    it("Should revert if ossification initiatied", async () => {
      await yieldManager.write.setYieldProviderIsOssificationInitiated([yieldProviderAddress, true]);
      const fundAmount = ONE_ETHER;
      const call = fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, fundAmount);
      await expectRevertWithCustomError(yieldProvider, call, "OperationNotSupportedDuringOssification", [
        OperationType.FUND_YIELD_PROVIDER,
      ]);
    });

    it("Should revert if staking paused", async () => {
      await yieldManager.write.pauseStaking([yieldProviderAddress], { account: securityCouncil.account });
      const fundAmount = ONE_ETHER;
      const call = fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, fundAmount);
      await expectRevertWithCustomError(yieldProvider, call, "OperationNotSupportedDuringStakingPause", [
        OperationType.FUND_YIELD_PROVIDER,
      ]);
    });

    it("Should revert if initiated ossification", async () => {
      await yieldManager.write.initiateOssification([yieldProviderAddress], { account: securityCouncil.account });
      await yieldManager.write.setYieldProviderIsStakingPaused([yieldProviderAddress, false], {
        account: securityCouncil.account,
      });
      const fundAmount = ONE_ETHER;
      const call = fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, fundAmount);
      await expectRevertWithCustomError(yieldProvider, call, "OperationNotSupportedDuringOssification", [
        OperationType.FUND_YIELD_PROVIDER,
      ]);
    });

    it("Should revert if ossified", async () => {
      await ossifyYieldProvider(yieldManager, yieldProviderAddress, securityCouncil);
      await yieldManager.write.setYieldProviderIsStakingPaused([yieldProviderAddress, false], {
        account: securityCouncil.account,
      });
      const fundAmount = ONE_ETHER;
      const call = fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, fundAmount);
      await expectRevertWithCustomError(yieldProvider, call, "OperationNotSupportedDuringOssification", [
        OperationType.FUND_YIELD_PROVIDER,
      ]);
    });
    it("If not ossified, should fund the Dashboard and pay max LST liability", async () => {
      const beforeVaultBalance = await getBalance({ address: mockStakingVaultAddress });
      const fundAmount = ONE_ETHER;

      // Setup LST liability principal < fundAmount
      const lstLiabilityPrincipal = ONE_ETHER / 2n;
      await yieldManager.write.setYieldProviderLstLiabilityPrincipal([yieldProviderAddress, lstLiabilityPrincipal]);
      await setupLSTPrincipalDecrementForPaxMaximumPossibleLSTLiability(
        lstLiabilityPrincipal,
        yieldManager,
        yieldProviderAddress,
        mockSTETH,
        mockDashboard,
      );

      // Do funding
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, fundAmount);
      expect(await getBalance({ address: mockStakingVaultAddress })).eq(beforeVaultBalance + fundAmount);
      expect(await yieldManager.read.getYieldProviderLstLiabilityPrincipal([yieldProviderAddress])).eq(0n);
    });
  });

  describe("withdraw from YieldProvider", () => {
    it("Should revert if not invoked via delegatecall", async () => {
      const call = yieldProvider.write.withdrawFromYieldProvider([yieldProviderAddress, BigInt(ZERO_VALUE)], {
        account: securityCouncil.account,
      });
      await expectRevertWithCustomError(yieldProvider, call, "ContextIsNotYieldManager");
    });
    it("Should successfully withdraw when not ossifed", async () => {
      const withdrawAmount = ONE_ETHER;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, withdrawAmount);
      await yieldManager.write.safeWithdrawFromYieldProvider([yieldProviderAddress, withdrawAmount], {
        account: nativeYieldOperator.account,
      });
    });
    it("Should successfully withdraw when ossified", async () => {
      const withdrawAmount = ONE_ETHER;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, withdrawAmount);
      await ossifyYieldProvider(yieldManager, yieldProviderAddress, securityCouncil);
      await yieldManager.write.safeWithdrawFromYieldProvider([yieldProviderAddress, withdrawAmount], {
        account: nativeYieldOperator.account,
      });
    });
    it("If VaultHub is connected, should perform max possible LST liability", async () => {
      // Setup LST liability principal < fundAmount
      const lstLiabilityPrincipal = ONE_ETHER / 2n;
      await yieldManager.write.setYieldProviderLstLiabilityPrincipal([yieldProviderAddress, lstLiabilityPrincipal]);
      await setupLSTPrincipalDecrementForPaxMaximumPossibleLSTLiability(
        lstLiabilityPrincipal,
        yieldManager,
        yieldProviderAddress,
        mockSTETH,
        mockDashboard,
      );
      // Setup VaultHub connected
      await mockVaultHub.write.setIsVaultConnectedReturn([true]);

      // Act
      const withdrawAmount = ONE_ETHER;
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, withdrawAmount);
      await yieldManager.write.safeWithdrawFromYieldProvider([yieldProviderAddress, withdrawAmount], {
        account: nativeYieldOperator.account,
      });
      expect(await yieldManager.read.getYieldProviderLstLiabilityPrincipal([yieldProviderAddress])).eq(0n);
    });
  });

  describe("pause staking", () => {
    it("Should revert if not invoked via delegatecall", async () => {
      const call = yieldProvider.write.pauseStaking([yieldProviderAddress], { account: securityCouncil.account });
      await expectRevertWithCustomError(yieldProvider, call, "ContextIsNotYieldManager");
    });
    it("Should successfully pause when not ossifed", async () => {
      await yieldManager.write.pauseStaking([yieldProviderAddress], { account: securityCouncil.account });
    });
    it("Should successfully pause when ossifed", async () => {
      await yieldManager.write.setYieldProviderIsOssified([yieldProviderAddress, true], {
        account: securityCouncil.account,
      });
      await yieldManager.write.pauseStaking([yieldProviderAddress], { account: securityCouncil.account });
    });
  });

  describe("unpause staking", () => {
    it("Should revert if not invoked via delegatecall", async () => {
      const call = yieldProvider.write.unpauseStaking([yieldProviderAddress], { account: securityCouncil.account });
      await expectRevertWithCustomError(yieldProvider, call, "ContextIsNotYieldManager");
    });
    it("Should successfully unpause when not ossifed", async () => {
      await yieldManager.write.pauseStaking([yieldProviderAddress], { account: securityCouncil.account });
      await setWithdrawalReserveToMinimum(yieldManager);
      await yieldManager.write.unpauseStaking([yieldProviderAddress], { account: securityCouncil.account });
    });
    it("Should revert unpause when ossifed", async () => {
      await yieldManager.write.pauseStaking([yieldProviderAddress], { account: securityCouncil.account });
      await yieldManager.write.setYieldProviderIsOssified([yieldProviderAddress, true], {
        account: securityCouncil.account,
      });
      await setWithdrawalReserveToMinimum(yieldManager);
      const call = yieldManager.write.unpauseStaking([yieldProviderAddress], { account: securityCouncil.account });
      await expectRevertWithCustomError(yieldProvider, call, "UnpauseStakingForbiddenWhenOssified");
    });
    it("Should successfully sync LST liability", async () => {
      await yieldManager.write.pauseStaking([yieldProviderAddress], { account: nativeYieldOperator.account });
      await yieldManager.write.setYieldProviderLstLiabilityPrincipal([yieldProviderAddress, 1n]);
      await expectRevert(
        yieldManager.write.unpauseStaking([yieldProviderAddress], { account: nativeYieldOperator.account }),
      );
      // Will sync to this if below current lstLiabilityPrincipal.
      await mockSTETH.write.setPooledEthBySharesRoundUpReturn([0n]);
      // If it didn't sync lstLiabilityPrincipal to 0, the below will revert.
      // Not awaited (as in the ethers version); the rejection is swallowed so node:test does not abort the run on an
      // unhandled rejection.
      yieldManager.write
        .unpauseStaking([yieldProviderAddress], { account: nativeYieldOperator.account })
        .catch(() => undefined);
    });
  });

  describe("withdraw LST", () => {
    it("Should revert if not invoked via delegatecall", async () => {
      const call = yieldProvider.write.withdrawLST([yieldProviderAddress, BigInt(ZERO_VALUE), zeroAddress], {
        account: securityCouncil.account,
      });
      await expectRevertWithCustomError(yieldProvider, call, "ContextIsNotYieldManager");
    });
    it("Should successfully withdraw LST when not ossifed, and update state", async () => {
      const withdrawAmount = ONE_ETHER;
      const lstPrincipalLiabilityBefore = await yieldManager.read.getYieldProviderLstLiabilityPrincipal([
        yieldProviderAddress,
      ]);
      const call = getWithdrawLSTCall(
        mockLinethRollup,
        yieldManager,
        yieldProvider,
        nativeYieldOperator,
        withdrawAmount,
      );
      await call;
      expect(await yieldManager.read.getYieldProviderLstLiabilityPrincipal([yieldProviderAddress])).eq(
        lstPrincipalLiabilityBefore + withdrawAmount,
      );
    });
    it("Should revert withdraw LST when ossification pending", async () => {
      // Setup fund
      const withdrawAmount = ONE_ETHER;
      const recipient = randomAddress();
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, withdrawAmount);

      // Add L1MessageService balance
      const l1MessageService = await yieldManager.read.L1_MESSAGE_SERVICE();
      await setBalance(l1MessageService, ONE_ETHER);
      const l1Signer = await getImpersonatedAccount(l1MessageService);
      await mockLinethRollup.write.setWithdrawLSTAllowed([true]);

      // Initiate ossification
      await yieldManager.write.initiateOssification([yieldProviderAddress], { account: securityCouncil.account });

      const call = yieldManager.write.withdrawLST([yieldProvider.address, withdrawAmount, recipient], {
        account: l1Signer.account,
      });
      await expectRevertWithCustomError(yieldProvider, call, "MintLSTDisabledDuringOssification");
    });
    it("Should revert withdraw LST when ossifed", async () => {
      const withdrawAmount = ONE_ETHER;
      const recipient = randomAddress();
      await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, nativeYieldOperator, withdrawAmount);

      // Add gas fees
      const l1MessageService = await yieldManager.read.L1_MESSAGE_SERVICE();
      await setBalance(l1MessageService, ONE_ETHER);
      const l1Signer = await getImpersonatedAccount(l1MessageService);
      await mockLinethRollup.write.setWithdrawLSTAllowed([true]);

      await yieldManager.write.setYieldProviderIsOssified([yieldProviderAddress, true], {
        account: securityCouncil.account,
      });

      // Act
      const call = yieldManager.write.withdrawLST([yieldProvider.address, withdrawAmount, recipient], {
        account: l1Signer.account,
      });

      // Assert
      await expectRevertWithCustomError(yieldProvider, call, "MintLSTDisabledDuringOssification");
    });
  });

  describe("initiate ossification", () => {
    it("Should revert if not invoked via delegatecall", async () => {
      const call = yieldProvider.write.initiateOssification([yieldProviderAddress], {
        account: securityCouncil.account,
      });
      await expectRevertWithCustomError(yieldProvider, call, "ContextIsNotYieldManager");
    });
    it("Should succeed", async () => {
      await yieldManager.write.initiateOssification([yieldProviderAddress], { account: securityCouncil.account });
    });
    it("Should succeed even if dashboard.voluntaryDisconnect call fails", async () => {
      await mockDashboard.write.setIsVoluntaryDisconnectRevert([true]);
      await yieldManager.write.initiateOssification([yieldProviderAddress], { account: securityCouncil.account });
    });
  });

  describe("progress pending ossification", () => {
    it("Should revert if not invoked via delegatecall", async () => {
      const call = yieldProvider.write.progressPendingOssification([yieldProviderAddress], {
        account: securityCouncil.account,
      });
      await expectRevertWithCustomError(yieldProvider, call, "ContextIsNotYieldManager");
    });
    it("If vault is disconnected, should succeed and return complete", async () => {
      // Arrange
      await yieldManager.write.initiateOssification([yieldProviderAddress], { account: securityCouncil.account });
      await mockVaultHub.write.setIsVaultConnectedReturn([false]);
      // Act
      const isOssificationComplete = (
        await yieldManager.simulate.progressPendingOssification([yieldProviderAddress], {
          account: securityCouncil.address,
        })
      ).result;
      const call = yieldManager.write.progressPendingOssification([yieldProviderAddress], {
        account: securityCouncil.account,
      });

      // Assert
      await expectNotReverted(call);
      expect(isOssificationComplete).eq(ProgressOssificationResult.COMPLETE);
    });
    it("If vault is connected and pending disconnect, should succeed with no-op and return false", async () => {
      // Arrange
      await yieldManager.write.initiateOssification([yieldProviderAddress], { account: securityCouncil.account });
      await mockVaultHub.write.setIsVaultConnectedReturn([true]);
      await mockVaultHub.write.setIsPendingDisconnectReturn([true]);
      // Act
      const isOssificationComplete = (
        await yieldManager.simulate.progressPendingOssification([yieldProviderAddress], {
          account: securityCouncil.address,
        })
      ).result;
      const call = yieldManager.write.progressPendingOssification([yieldProviderAddress], {
        account: securityCouncil.account,
      });

      // Assert
      await expectNotReverted(call);
      expect(isOssificationComplete).eq(ProgressOssificationResult.NOOP);
    });
    it("If vault is connected and not pending disconnect, should successfully redo initiate ossification and return false", async () => {
      // Arrange
      await yieldManager.write.initiateOssification([yieldProviderAddress], { account: securityCouncil.account });
      await mockVaultHub.write.setIsVaultConnectedReturn([true]);
      await mockVaultHub.write.setIsPendingDisconnectReturn([false]);
      // Act
      const isOssificationComplete = (
        await yieldManager.simulate.progressPendingOssification([yieldProviderAddress], {
          account: securityCouncil.address,
        })
      ).result;
      const call = yieldManager.write.progressPendingOssification([yieldProviderAddress], {
        account: securityCouncil.account,
      });

      // Assert
      await expectNotReverted(call);
      expect(isOssificationComplete).eq(ProgressOssificationResult.REINITIATED);
    });
  });

  describe("vendor initialization", () => {
    it("Should revert if not invoked via delegatecall", async () => {
      const call = yieldProvider.write.initializeVendorContracts([EMPTY_CALLDATA], {
        account: securityCouncil.account,
      });
      await expectRevertWithCustomError(yieldProvider, call, "ContextIsNotYieldManager");
    });
    it("Should revert if incorrect type for _vendorInitializationData", async () => {
      const call = yieldManager.write.initializeVendorContracts([yieldProviderAddress, EMPTY_CALLDATA], {
        account: securityCouncil.account,
      });
      await expectRevert(call);
    });
    it("Should succeed with expected return values, and transfer 1 ether to Lido contracts", async () => {
      await yieldManager.write.setYieldProviderUserFunds([yieldProviderAddress, 0n]);
      await yieldManager.write.removeYieldProvider([yieldProviderAddress, buildVendorExitData()], {
        account: securityCouncil.account,
      });
      const expectedVaultAddress = randomAddress();
      const expectedDashboardAddress = randomAddress();
      await mockVaultFactory.write.setReturnValues([expectedVaultAddress, expectedDashboardAddress]);
      await incrementBalance(yieldManagerAddress, ONE_ETHER);
      const beforeYieldManagerBalance = await getBalance(yieldManager);
      const beforeVaultFactoryBalance = await getBalance(mockVaultFactory);

      const registrationData = (
        await yieldManager.simulate.initializeVendorContracts([yieldProviderAddress, buildVendorInitializationData()], {
          account: securityCouncil.address,
        })
      ).result;

      const call = yieldManager.write.initializeVendorContracts(
        [yieldProviderAddress, buildVendorInitializationData()],
        { account: securityCouncil.account },
      );
      await expectNotReverted(call);
      expect(registrationData.yieldProviderVendor).eq(YieldProviderVendor.LIDO_ST_VAULT_YIELD_PROVIDER_VENDOR);
      expect(registrationData.primaryEntrypoint).eq(expectedDashboardAddress);
      expect(registrationData.ossifiedEntrypoint).eq(expectedVaultAddress);
      expect(await getBalance(yieldManager)).eq(beforeYieldManagerBalance - ONE_ETHER);
      expect(await getBalance(mockVaultFactory)).eq(beforeVaultFactoryBalance + ONE_ETHER);
    });
    it("Should revert if YieldManager lacks the CONNECT_DEPOSIT of 1 ether", async () => {
      await yieldManager.write.setYieldProviderUserFunds([yieldProviderAddress, 0n]);
      await yieldManager.write.removeYieldProvider([yieldProviderAddress, buildVendorExitData()], {
        account: securityCouncil.account,
      });
      const expectedVaultAddress = randomAddress();
      const expectedDashboardAddress = randomAddress();
      await mockVaultFactory.write.setReturnValues([expectedVaultAddress, expectedDashboardAddress]);

      const call = yieldManager.write.initializeVendorContracts(
        [yieldProviderAddress, buildVendorInitializationData()],
        { account: securityCouncil.account },
      );
      await expectRevert(call);
    });
  });

  describe("vendor exit", () => {
    it("Should revert if not invoked via delegatecall", async () => {
      const call = yieldProvider.write.exitVendorContracts([yieldProviderAddress, EMPTY_CALLDATA], {
        account: securityCouncil.account,
      });
      await expectRevertWithCustomError(yieldProvider, call, "ContextIsNotYieldManager");
    });
    it("Should revert with empty vendorExitData", async () => {
      await yieldManager.write.setYieldProviderUserFunds([yieldProviderAddress, 0n]);
      const call = yieldManager.write.removeYieldProvider([yieldProviderAddress, EMPTY_CALLDATA], {
        account: securityCouncil.account,
      });
      await expectRevertWithCustomError(yieldProvider, call, "NoVendorExitDataProvided");
    });
    it("Should revert if newVaultAddress = 0", async () => {
      const call = yieldManager.write.exitVendorContracts(
        [yieldProviderAddress, buildVendorExitData({ newVaultOwner: zeroAddress })],
        { account: securityCouncil.account },
      );
      await expectRevertWithCustomError(yieldManager, call, "ZeroAddressNotAllowed");
    });
    it("When non-ossified, should succeed with call to Dashboard", async () => {
      await yieldManager.write.setYieldProviderIsOssified([yieldProviderAddress, false]);
      await yieldManager.write.setYieldProviderUserFunds([yieldProviderAddress, 0n]);
      const call = yieldManager.write.removeYieldProvider([yieldProviderAddress, buildVendorExitData()], {
        account: securityCouncil.account,
      });
      await expectNotReverted(call);
      expect(await mockDashboard.read.transferVaultOwnershipCallCount()).eq(1n);
      expect(await mockStakingVault.read.transferOwnershipCallCount()).eq(0n);
    });
    it("When ossified, should succeed with call to StakingVault", async () => {
      await yieldManager.write.setYieldProviderIsOssified([yieldProviderAddress, true]);
      await yieldManager.write.setYieldProviderUserFunds([yieldProviderAddress, 0n]);
      const call = yieldManager.write.removeYieldProvider([yieldProviderAddress, buildVendorExitData()], {
        account: securityCouncil.account,
      });
      await expectNotReverted(call);
      expect(await mockDashboard.read.transferVaultOwnershipCallCount()).eq(0n);
      expect(await mockStakingVault.read.transferOwnershipCallCount()).eq(1n);
    });
  });

  describe("unstake", () => {
    it("Should revert if not invoked via delegatecall", async () => {
      const mockWithdrawalParams = generateRandomBytes(8);
      const call = yieldProvider.write.unstake([yieldProviderAddress, mockWithdrawalParams], {
        account: securityCouncil.account,
      });
      await expectRevertWithCustomError(yieldProvider, call, "ContextIsNotYieldManager");
    });
    it("Should revert if incorrect withdrawal params type", async () => {
      const mockWithdrawalParams = generateRandomBytes(8);
      const call = yieldManager.write.unstake([yieldProviderAddress, mockWithdrawalParams], {
        account: securityCouncil.account,
      });
      await expectRevert(call);
    });
    it("Should succeed with valid withdrawal params type", async () => {
      const pubkey = "0x" + "a".repeat(96); // 48 bytes
      const amounts = [32000000000n]; // 32 ETH in Gwei
      const refundRecipient = nativeYieldOperator.address;
      const withdrawalParams = encodeData(["bytes", "uint64[]", "address"], [pubkey, amounts, refundRecipient]);

      const call = yieldManager.write.unstake([yieldProviderAddress, withdrawalParams], {
        account: securityCouncil.account,
      });
      await call;
    });
  });

  describe("unstakePermissionless", () => {
    it("Should revert if not invoked via delegatecall", async () => {
      const mockWithdrawalParams = generateRandomBytes(8);
      const mockWithdrawalParamsProof = generateRandomBytes(8);
      const mockRequiredUnstakeAmountWei = ONE_ETHER;
      const mockValidatorIndex = 0n;
      const mockSlot = 100000n;
      const call = yieldProvider.write.unstakePermissionless(
        [
          yieldProviderAddress,
          mockRequiredUnstakeAmountWei,
          mockValidatorIndex,
          mockSlot,
          mockWithdrawalParams,
          mockWithdrawalParamsProof,
        ],
        { account: securityCouncil.account },
      );
      await expectRevertWithCustomError(yieldProvider, call, "ContextIsNotYieldManager");
    });
    it("Should revert if incorrect withdrawal params type", async () => {
      const mockWithdrawalParams = generateRandomBytes(8);
      const mockWithdrawalParamsProof = generateRandomBytes(8);
      const mockValidatorIndex = 0n;
      const mockSlot = 100000n;
      const call = yieldManager.write.unstakePermissionless(
        [yieldProviderAddress, mockValidatorIndex, mockSlot, mockWithdrawalParams, mockWithdrawalParamsProof],
        { account: securityCouncil.account },
      );
      await expectRevert(call);
    });
    it("Should succeed", async () => {
      // Arrange - Set up withdrawal reserve in deficit
      await setBalance(l1MessageServiceAddress, 0n);

      const { eip4788Witness, pubkey, validatorIndex, slot } = await generateLidoUnstakePermissionlessWitness(
        sszMerkleTree,
        testVerifier,
        mockStakingVaultAddress,
        ONE_GWEI * 100n,
      );

      const refundAddress = nativeYieldOperator.address;
      const withdrawalParams = encodeData(["bytes", "address"], [pubkey, refundAddress]);
      const withdrawalParamsProof = encodeData([BEACON_PROOF_WITNESS_TYPE], [eip4788Witness.beaconProofWitness]);

      // Assert - Check return value using staticcall
      // Calculate expected unstake amount: effectiveBalance (100 gwei) - MIN_ACTIVATION_BALANCE (32 gwei) = 68 gwei
      const expectedUnstakeAmountWei = 68n * ONE_ETHER; // 100 - 32 = 68 gwei

      // Use staticcall to simulate the call (won't modify state)
      // Note: YieldManager's unstakePermissionless doesn't return a value, but the yield provider's function does
      // The yield provider's unstakePermissionless returns: min(requiredUnstakeAmountWei, effectiveBalance - MIN_ACTIVATION_BALANCE - pendingWithdrawals)
      // Since effectiveBalance = 100 gwei, MIN_ACTIVATION_BALANCE = 32 gwei, and no pending withdrawals:
      // maxUnstakeable = 100 - 32 - 0 = 68 gwei
      await yieldManager.write.unstakePermissionless(
        [yieldProviderAddress, validatorIndex, slot, withdrawalParams, withdrawalParamsProof],
        { account: securityCouncil.account },
      );

      // Assert - Verify the actual unstaked amount matches expected return value
      // The yield provider returns unstakedAmountWei (68 gwei), which YieldManager uses to update pendingPermissionlessUnstake
      expect(await yieldManager.read.pendingPermissionlessUnstake()).to.equal(expectedUnstakeAmountWei);
    });
    it.skip("Should succeed with 2000 pending partial withdrawals", async () => {
      // Arrange - Set up withdrawal reserve in deficit
      const { viem } = await network.getOrCreate();
      await setBalance(l1MessageServiceAddress, 0n);
      const numPendingPartialWithdrawals = 2000;

      const { eip4788Witness, pubkey, validatorIndex, slot } = await generateLidoUnstakePermissionlessWitness(
        sszMerkleTree,
        testVerifier,
        mockStakingVaultAddress,
        ONE_GWEI * 100n,
        [],
        numPendingPartialWithdrawals,
      );

      const refundAddress = nativeYieldOperator.address;
      const withdrawalParams = encodeData(["bytes", "address"], [pubkey, refundAddress]);
      const withdrawalParamsProof = encodeData([BEACON_PROOF_WITNESS_TYPE], [eip4788Witness.beaconProofWitness]);

      // Assert - Check return value using staticcall
      // Calculate expected unstake amount: effectiveBalance (100 gwei) - MIN_ACTIVATION_BALANCE (32 gwei) = 68 gwei
      const expectedUnstakeAmountWei = 68n * ONE_ETHER; // 100 - 32 = 68 gwei

      // Use staticcall to simulate the call (won't modify state)
      // Note: YieldManager's unstakePermissionless doesn't return a value, but the yield provider's function does
      // The yield provider's unstakePermissionless returns: min(requiredUnstakeAmountWei, effectiveBalance - MIN_ACTIVATION_BALANCE - pendingWithdrawals)
      // Since effectiveBalance = 100 gwei, MIN_ACTIVATION_BALANCE = 32 gwei, and no pending withdrawals:
      // maxUnstakeable = 100 - 32 - 0 = 68 gwei
      const publicClient = await viem.getPublicClient();
      const txHash = await yieldManager.write.unstakePermissionless(
        [yieldProviderAddress, validatorIndex, slot, withdrawalParams, withdrawalParamsProof],
        { account: securityCouncil.account },
      );

      const receipt = await publicClient.waitForTransactionReceipt({ hash: txHash });
      console.log(
        `Gas used for unstakePermissionless with ${numPendingPartialWithdrawals} pending partial withdrawals:`,
        receipt?.gasUsed.toString(),
      );

      // Assert - Verify the actual unstaked amount matches expected return value
      // The yield provider returns unstakedAmountWei (68 gwei), which YieldManager uses to update pendingPermissionlessUnstake
      expect(await yieldManager.read.pendingPermissionlessUnstake()).to.equal(expectedUnstakeAmountWei);
    });
  });

  describe("validateUnstakePermissionless", () => {
    it("Should revert if pubkeys argument is not 48 bytes exactly", async () => {
      const invalidPubkeys = hex("0x" + "22".repeat(32));
      const call = yieldManager.write.validateUnstakePermissionlessRequestHarness(
        [yieldProviderAddress, 1n, invalidPubkeys, 0n, 0n, EMPTY_CALLDATA],
        { account: securityCouncil.account },
      );

      await expectRevertWithCustomError(yieldProvider, call, "SingleValidatorOnlyForUnstakePermissionless");
    });

    it("Should revert if incorrect type is provided for proof", async () => {
      const pubkeys = hex("0x" + "11".repeat(48));
      const call = yieldManager.write.validateUnstakePermissionlessRequestHarness(
        [yieldProviderAddress, 1n, pubkeys, 0n, 0n, EMPTY_CALLDATA],
        { account: securityCouncil.account },
      );

      await expectRevert(call);
    });

    it("Should revert if incorrect validator container proof", async () => {
      const { eip4788Witness, pubkey, validatorIndex, slot } = await generateLidoUnstakePermissionlessWitness(
        sszMerkleTree,
        testVerifier,
        mockStakingVaultAddress,
        ONE_GWEI * 100n,
      );

      eip4788Witness.beaconProofWitness.validatorContainerWitness.proof[0] = randomBytes32();
      const withdrawalParamsProof = encodeData([BEACON_PROOF_WITNESS_TYPE], [eip4788Witness.beaconProofWitness]);

      const call = yieldManager.write.validateUnstakePermissionlessRequestHarness(
        [yieldProviderAddress, 1n, pubkey, validatorIndex, slot, withdrawalParamsProof],
        { account: securityCouncil.account },
      );

      await expectRevert(call);
    });
    it("Should revert if incorrect pending partial withdrawals proof", async () => {
      const { eip4788Witness, pubkey, validatorIndex, slot } = await generateLidoUnstakePermissionlessWitness(
        sszMerkleTree,
        testVerifier,
        mockStakingVaultAddress,
        ONE_GWEI * 100n,
      );

      eip4788Witness.beaconProofWitness.pendingPartialWithdrawalsWitness.proof[0] = randomBytes32();
      const withdrawalParamsProof = encodeData([BEACON_PROOF_WITNESS_TYPE], [eip4788Witness.beaconProofWitness]);

      const call = yieldManager.write.validateUnstakePermissionlessRequestHarness(
        [yieldProviderAddress, 1n, pubkey, validatorIndex, slot, withdrawalParamsProof],
        { account: securityCouncil.account },
      );

      await expectRevert(call);
    });
    it("Should return min of requiredUnstake and withdrawable validator balance", async () => {
      const unstakeAmountWei = ONE_ETHER * 100n;

      const { eip4788Witness, pubkey, validatorIndex, slot } = await generateLidoUnstakePermissionlessWitness(
        sszMerkleTree,
        testVerifier,
        mockStakingVaultAddress,
        ONE_GWEI * 100n,
      );

      const withdrawalParamsProof = encodeData([BEACON_PROOF_WITNESS_TYPE], [eip4788Witness.beaconProofWitness]);

      const unstakedAmountGwei = (
        await yieldManager.simulate.validateUnstakePermissionlessRequestHarness(
          [yieldProviderAddress, unstakeAmountWei, pubkey, validatorIndex, slot, withdrawalParamsProof],
          { account: securityCouncil.address },
        )
      ).result;

      await yieldManager.write.validateUnstakePermissionlessRequestHarness(
        [yieldProviderAddress, unstakeAmountWei, pubkey, validatorIndex, slot, withdrawalParamsProof],
        { account: securityCouncil.account },
      );

      expect(unstakedAmountGwei).eq(100n * ONE_GWEI - 32n * ONE_GWEI);
    });
    it("Should return 0 if validator balance is less than MIN_ACTIVATION_BALANCE", async () => {
      const unstakeAmountWei = ONE_ETHER * 100n;

      const { eip4788Witness, pubkey, validatorIndex, slot } = await generateLidoUnstakePermissionlessWitness(
        sszMerkleTree,
        testVerifier,
        mockStakingVaultAddress,
        ONE_GWEI * 31n,
      );

      const withdrawalParamsProof = encodeData([BEACON_PROOF_WITNESS_TYPE], [eip4788Witness.beaconProofWitness]);

      const unstakedAmountGwei = (
        await yieldManager.simulate.validateUnstakePermissionlessRequestHarness(
          [yieldProviderAddress, unstakeAmountWei, pubkey, validatorIndex, slot, withdrawalParamsProof],
          { account: securityCouncil.address },
        )
      ).result;

      await yieldManager.write.validateUnstakePermissionlessRequestHarness(
        [yieldProviderAddress, unstakeAmountWei, pubkey, validatorIndex, slot, withdrawalParamsProof],
        { account: securityCouncil.account },
      );

      expect(unstakedAmountGwei).eq(0n);
    });
    it("Should clamp unstaked amount by pending partial withdrawals", async () => {
      const unstakeAmountWei = ONE_ETHER * 1000n;
      const effectiveBalance = ONE_GWEI * 1000n;
      const pendingPartialWithdrawalsGwei = [ONE_GWEI * 10n, ONE_GWEI * 20n];

      const { eip4788Witness, pubkey, validatorIndex, slot } = await generateLidoUnstakePermissionlessWitness(
        sszMerkleTree,
        testVerifier,
        mockStakingVaultAddress,
        effectiveBalance,
        pendingPartialWithdrawalsGwei,
        1,
      );

      const withdrawalParamsProof = encodeData([BEACON_PROOF_WITNESS_TYPE], [eip4788Witness.beaconProofWitness]);

      const unstakedAmountGwei = (
        await yieldManager.simulate.validateUnstakePermissionlessRequestHarness(
          [yieldProviderAddress, unstakeAmountWei, pubkey, validatorIndex, slot, withdrawalParamsProof],
          { account: securityCouncil.address },
        )
      ).result;

      await yieldManager.write.validateUnstakePermissionlessRequestHarness(
        [yieldProviderAddress, unstakeAmountWei, pubkey, validatorIndex, slot, withdrawalParamsProof],
        { account: securityCouncil.account },
      );

      expect(unstakedAmountGwei).eq(
        effectiveBalance - 32n * ONE_GWEI - pendingPartialWithdrawalsGwei[0] - pendingPartialWithdrawalsGwei[1],
      );
    });
  });
});
