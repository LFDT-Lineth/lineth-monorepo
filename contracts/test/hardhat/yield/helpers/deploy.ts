import { network } from "hardhat";

import { buildVendorInitializationData } from "./mocks.js";
import { incrementBalance } from "./setup.js";
import {
  YIELD_MANAGER_PAUSE_TYPES_ROLES,
  YIELD_MANAGER_UNPAUSE_TYPES_ROLES,
  YIELD_MANAGER_OPERATOR_ROLES,
  YIELD_MANAGER_SECURITY_COUNCIL_ROLES,
  YIELD_MANAGER_INITIALIZE_SIGNATURE,
} from "../../../../common/constants/index.js";
import { generateRoleAssignments } from "../../../../common/helpers/index.js";
import {
  MINIMUM_WITHDRAWAL_RESERVE_PERCENTAGE_BPS,
  TARGET_WITHDRAWAL_RESERVE_PERCENTAGE_BPS,
  MINIMUM_WITHDRAWAL_RESERVE_AMOUNT,
  TARGET_WITHDRAWAL_RESERVE_AMOUNT,
  GI_FIRST_VALIDATOR,
  GI_PENDING_PARTIAL_WITHDRAWALS_ROOT,
  YIELD_PROVIDER_STAKING_ROLE,
  ONE_ETHER,
} from "../../common/constants/index.js";
import { deployFromFactory, deployUpgradableWithConstructorArgs, getUpgrades } from "../../common/deployment.js";
import { type TestAccount, getAccountsFixture } from "../../common/helpers/index.js";
import { deployLinethRollupFixture } from "../../rollup/helpers/deploy.js";

import type { YieldManagerInitializationData } from "./types.js";
import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";
import type { Address, Hex } from "viem";

async function getYieldManagerRoleAddressesFixture(): Promise<
  {
    role: Hex;
    addressWithRole: Address;
  }[]
> {
  const { networkHelpers } = await network.getOrCreate();
  const { nativeYieldOperator, securityCouncil } = await networkHelpers.loadFixture(getAccountsFixture);
  const yieldManagerOpereratorRoleAssignments = generateRoleAssignments(
    YIELD_MANAGER_OPERATOR_ROLES,
    nativeYieldOperator.address,
    [],
  );
  const securityCouncilRoleAssignments = generateRoleAssignments(
    YIELD_MANAGER_SECURITY_COUNCIL_ROLES,
    securityCouncil.address,
    [],
  );
  return [...yieldManagerOpereratorRoleAssignments, ...securityCouncilRoleAssignments];
}

export async function deployMockLinethRollup(): Promise<ContractReturnType<"MockLinethRollup">> {
  return deployFromFactory("MockLinethRollup");
}

export async function deployMockWithdrawTarget(): Promise<ContractReturnType<"MockWithdrawTarget">> {
  return deployFromFactory("MockWithdrawTarget");
}

export async function deployMockYieldProvider(): Promise<ContractReturnType<"MockYieldProvider">> {
  return deployFromFactory("MockYieldProvider");
}

export async function deployValidatorContainerProofVerifier(): Promise<
  ContractReturnType<"ValidatorContainerProofVerifier">
> {
  const { admin } = await getAccountsFixture();
  return deployFromFactory("ValidatorContainerProofVerifier", [
    admin.address,
    GI_FIRST_VALIDATOR,
    GI_PENDING_PARTIAL_WITHDRAWALS_ROOT,
  ]);
}

export async function deployTestValidatorContainerProofVerifier(): Promise<
  ContractReturnType<"TestValidatorContainerProofVerifier">
> {
  const { admin } = await getAccountsFixture();
  return deployFromFactory("TestValidatorContainerProofVerifier", [
    admin.address,
    GI_FIRST_VALIDATOR,
    GI_PENDING_PARTIAL_WITHDRAWALS_ROOT,
  ]);
}

export async function deploySSZMerkleTree(): Promise<ContractReturnType<"SSZMerkleTree">> {
  return deployFromFactory("SSZMerkleTree", [GI_FIRST_VALIDATOR]);
}

// Deploys with MockLinethRollup and MockYieldProvider
export async function deployYieldManagerForUnitTest() {
  const { networkHelpers } = await network.getOrCreate();
  (await getUpgrades()).silenceWarnings();
  const { securityCouncil, l2YieldRecipient } = await networkHelpers.loadFixture(getAccountsFixture);
  const roleAddresses = await networkHelpers.loadFixture(getYieldManagerRoleAddressesFixture);

  const mockLinethRollup = await deployMockLinethRollup();

  const initializationData: YieldManagerInitializationData = {
    pauseTypeRoles: YIELD_MANAGER_PAUSE_TYPES_ROLES,
    unpauseTypeRoles: YIELD_MANAGER_UNPAUSE_TYPES_ROLES,
    roleAddresses,
    initialL2YieldRecipients: [l2YieldRecipient.address],
    defaultAdmin: securityCouncil.address,
    initialMinimumWithdrawalReservePercentageBps: MINIMUM_WITHDRAWAL_RESERVE_PERCENTAGE_BPS,
    initialTargetWithdrawalReservePercentageBps: TARGET_WITHDRAWAL_RESERVE_PERCENTAGE_BPS,
    initialMinimumWithdrawalReserveAmount: MINIMUM_WITHDRAWAL_RESERVE_AMOUNT,
    initialTargetWithdrawalReserveAmount: TARGET_WITHDRAWAL_RESERVE_AMOUNT,
  };

  const yieldManager = await deployUpgradableWithConstructorArgs(
    "TestYieldManager",
    [mockLinethRollup.address],
    [initializationData],
    {
      initializer: YIELD_MANAGER_INITIALIZE_SIGNATURE,
      unsafeAllow: ["constructor", "incorrect-initializer-order", "state-variable-immutable", "delegatecall"],
    },
  );

  return { mockLinethRollup, yieldManager, initializationData };
}

export async function deployYieldManagerForUnitTestWithMutatedInitData(
  mutatedInitData: YieldManagerInitializationData,
) {
  (await getUpgrades()).silenceWarnings();
  const mockLinethRollup = await deployMockLinethRollup();
  await deployUpgradableWithConstructorArgs("TestYieldManager", [mockLinethRollup.address], [mutatedInitData], {
    // initializer: "initialize",
    initializer: YIELD_MANAGER_INITIALIZE_SIGNATURE,
    unsafeAllow: ["constructor", "incorrect-initializer-order", "state-variable-immutable", "delegatecall"],
  });
}

export async function deployMockVaultHub(): Promise<ContractReturnType<"MockVaultHub">> {
  return deployFromFactory("MockVaultHub");
}

export async function deployMockVaultFactory(): Promise<ContractReturnType<"MockVaultFactory">> {
  return deployFromFactory("MockVaultFactory");
}

export async function deployMockSTETH(): Promise<ContractReturnType<"MockSTETH">> {
  return deployFromFactory("MockSTETH");
}

export async function deployMockDashboard(): Promise<ContractReturnType<"MockDashboard">> {
  return deployFromFactory("MockDashboard");
}

export async function deployMockStakingVault(): Promise<ContractReturnType<"MockStakingVault">> {
  return deployFromFactory("MockStakingVault");
}

export async function deployLidoStVaultYieldProviderFactory() {
  const { networkHelpers } = await network.getOrCreate();
  const { mockLinethRollup, yieldManager } = await networkHelpers.loadFixture(deployYieldManagerForUnitTest);
  const mockVaultHub = await deployMockVaultHub();
  const mockVaultFactory = await deployMockVaultFactory();
  const mockSTETH = await deployMockSTETH();
  const verifier = await deployValidatorContainerProofVerifier();

  const verifierAddress = verifier.address;

  const lidoStVaultYieldProviderFactory = await deployFromFactory("LidoStVaultYieldProviderFactory", [
    mockLinethRollup.address,
    yieldManager.address,
    mockVaultHub.address,
    mockVaultFactory.address,
    mockSTETH.address,
    verifierAddress,
  ]);

  return {
    mockLinethRollup,
    yieldManager,
    mockVaultHub,
    mockVaultFactory,
    mockSTETH,
    lidoStVaultYieldProviderFactory,
    verifier,
    verifierAddress,
  };
}

async function deployLidoStVaultYieldProviderDependenciesFixture() {
  const { securityCouncil } = await getAccountsFixture();
  const { mockLinethRollup, yieldManager } = await deployYieldManagerForUnitTest();
  const mockVaultHub = await deployMockVaultHub();
  const mockVaultFactory = await deployMockVaultFactory();
  const mockSTETH = await deployMockSTETH();
  const mockDashboard = await deployMockDashboard();
  const mockStakingVault = await deployMockStakingVault();
  const sszMerkleTree = await deploySSZMerkleTree();
  const verifier = await deployValidatorContainerProofVerifier();
  const testVerifier = await deployTestValidatorContainerProofVerifier();

  return {
    securityCouncil,
    mockLinethRollup,
    yieldManager,
    mockVaultHub,
    mockSTETH,
    mockVaultFactory,
    mockDashboard,
    mockStakingVault,
    sszMerkleTree,
    verifier,
    testVerifier,
  };
}

export async function deployAndAddSingleLidoStVaultYieldProvider() {
  const { viem, networkHelpers } = await network.getOrCreate();
  const {
    securityCouncil,
    mockLinethRollup,
    yieldManager,
    mockVaultHub,
    mockSTETH,
    mockVaultFactory,
    mockDashboard,
    mockStakingVault,
    sszMerkleTree,
    verifier,
    testVerifier,
  } = await networkHelpers.loadFixture(deployLidoStVaultYieldProviderDependenciesFixture);

  const yieldManagerAddress = yieldManager.address;
  const verifierAddress = verifier.address;

  // Deploy Factory
  const lidoStVaultYieldProviderFactory = await deployFromFactory("TestLidoStVaultYieldProviderFactory", [
    mockLinethRollup.address,
    yieldManagerAddress,
    mockVaultHub.address,
    mockVaultFactory.address,
    mockSTETH.address,
    verifierAddress,
  ]);

  // Create YieldProvider
  const { result: yieldProviderAddress } =
    await lidoStVaultYieldProviderFactory.simulate.createTestLidoStVaultYieldProvider();
  await lidoStVaultYieldProviderFactory.write.createTestLidoStVaultYieldProvider({ account: securityCouncil.account });
  const yieldProvider = await viem.getContractAt("TestLidoStVaultYieldProvider", yieldProviderAddress);

  // Add YieldProvider
  await mockDashboard.write.setStakingVaultReturn([mockStakingVault.address]);
  const mockDashboardAddress = mockDashboard.address;
  const mockStakingVaultAddress = mockStakingVault.address;

  await mockVaultFactory.write.setReturnValues([mockStakingVaultAddress, mockDashboardAddress]);
  await incrementBalance(yieldManagerAddress, ONE_ETHER); // Connect Deposit
  await yieldManager.write.addYieldProvider([yieldProviderAddress, buildVendorInitializationData()], {
    account: securityCouncil.account,
  });
  return {
    mockDashboard,
    mockStakingVault,
    yieldManager,
    yieldProvider,
    yieldProviderAddress,
    mockVaultHub,
    mockSTETH,
    mockVaultFactory,
    mockLinethRollup,
    sszMerkleTree,
    verifier,
    verifierAddress,
    testVerifier,
  };
}

export async function deployYieldManagerIntegrationTestFixture() {
  const { viem, networkHelpers } = await network.getOrCreate();
  const { securityCouncil, l2YieldRecipient, nativeYieldOperator } =
    await networkHelpers.loadFixture(getAccountsFixture);
  const yieldManagerRoleAddresses = await networkHelpers.loadFixture(getYieldManagerRoleAddressesFixture);
  // Deploy LinethRollup
  const { linethRollup } = await networkHelpers.loadFixture(deployLinethRollupFixture);
  const l1MessageServiceAddress = linethRollup.address;

  // Deploy YieldManager
  const initializationData: YieldManagerInitializationData = {
    pauseTypeRoles: YIELD_MANAGER_PAUSE_TYPES_ROLES,
    unpauseTypeRoles: YIELD_MANAGER_UNPAUSE_TYPES_ROLES,
    roleAddresses: yieldManagerRoleAddresses,
    initialL2YieldRecipients: [l2YieldRecipient.address],
    defaultAdmin: securityCouncil.address,
    initialMinimumWithdrawalReservePercentageBps: MINIMUM_WITHDRAWAL_RESERVE_PERCENTAGE_BPS,
    initialTargetWithdrawalReservePercentageBps: TARGET_WITHDRAWAL_RESERVE_PERCENTAGE_BPS,
    initialMinimumWithdrawalReserveAmount: MINIMUM_WITHDRAWAL_RESERVE_AMOUNT,
    initialTargetWithdrawalReserveAmount: TARGET_WITHDRAWAL_RESERVE_AMOUNT,
  };

  (await getUpgrades()).silenceWarnings();
  const yieldManager = await deployUpgradableWithConstructorArgs(
    "TestYieldManager",
    [l1MessageServiceAddress],
    [initializationData],
    {
      initializer: YIELD_MANAGER_INITIALIZE_SIGNATURE,
      unsafeAllow: ["constructor", "incorrect-initializer-order", "state-variable-immutable", "delegatecall"],
    },
  );

  // Deploy LidoStVaultYieldProviderFactory
  const mockVaultHub = await deployMockVaultHub();
  const mockVaultFactory = await deployMockVaultFactory();
  const mockSTETH = await deployMockSTETH();
  const mockDashboard = await deployMockDashboard();
  const mockStakingVault = await deployMockStakingVault();
  const verifier = await deployValidatorContainerProofVerifier();

  const yieldManagerAddress = yieldManager.address;
  const verifierAddress = verifier.address;

  const lidoStVaultYieldProviderFactory = await deployFromFactory("TestLidoStVaultYieldProviderFactory", [
    l1MessageServiceAddress,
    yieldManagerAddress,
    mockVaultHub.address,
    mockVaultFactory.address,
    mockSTETH.address,
    verifierAddress,
  ]);

  // Create YieldProvider
  const { result: yieldProviderAddress } =
    await lidoStVaultYieldProviderFactory.simulate.createTestLidoStVaultYieldProvider();
  await lidoStVaultYieldProviderFactory.write.createTestLidoStVaultYieldProvider({ account: securityCouncil.account });
  const yieldProvider = await viem.getContractAt("TestLidoStVaultYieldProvider", yieldProviderAddress);

  // Add YieldProvider
  await mockDashboard.write.setStakingVaultReturn([mockStakingVault.address]);
  const mockDashboardAddress = mockDashboard.address;
  const mockStakingVaultAddress = mockStakingVault.address;

  await mockVaultFactory.write.setReturnValues([mockStakingVaultAddress, mockDashboardAddress]);
  await incrementBalance(yieldManagerAddress, ONE_ETHER); // Connect Deposit
  await yieldManager.write.addYieldProvider([yieldProviderAddress, buildVendorInitializationData()], {
    account: securityCouncil.account,
  });

  await linethRollup.write.setYieldManager([yieldManagerAddress], { account: securityCouncil.account });
  await linethRollup.write.grantRole([YIELD_PROVIDER_STAKING_ROLE, nativeYieldOperator.address], {
    account: securityCouncil.account,
  });

  // Deploy EIP-4788 Beacon Proof utils
  const sszMerkleTree = await deploySSZMerkleTree();
  const testVerifier = await deployTestValidatorContainerProofVerifier();

  return {
    linethRollup,
    yieldManager,
    yieldManagerAddress,
    yieldProvider,
    yieldProviderAddress,
    mockDashboard,
    mockSTETH,
    mockVaultHub,
    mockVaultFactory,
    mockStakingVault,
    lidoStVaultYieldProviderFactory,
    sszMerkleTree,
    verifier,
    verifierAddress,
    testVerifier,
    initializationData,
  };
}

// Not for use in loadFixture, to add additional YieldProviders
export async function deployAndAddAdditionalLidoStVaultYieldProvider(
  factory: ContractReturnType<"TestLidoStVaultYieldProviderFactory">,
  yieldManager: ContractReturnType<"TestYieldManager">,
  securityCouncil: TestAccount,
  mockVaultFactory: ContractReturnType<"MockVaultFactory">,
) {
  const { viem } = await network.getOrCreate();
  // Create YieldProvider
  const { result: yieldProviderAddress } = await factory.simulate.createTestLidoStVaultYieldProvider();
  await factory.write.createTestLidoStVaultYieldProvider();
  const yieldProvider = await viem.getContractAt("TestLidoStVaultYieldProvider", yieldProviderAddress);

  // Add YieldProvider
  const mockDashboard = await deployMockDashboard();
  const mockStakingVault = await deployMockStakingVault();
  await mockDashboard.write.setStakingVaultReturn([mockStakingVault.address]);
  const mockDashboardAddress = mockDashboard.address;
  const mockStakingVaultAddress = mockStakingVault.address;

  await mockVaultFactory.write.setReturnValues([mockStakingVaultAddress, mockDashboardAddress]);
  await incrementBalance(yieldManager.address, ONE_ETHER); // Connect Deposit
  await yieldManager.write.addYieldProvider([yieldProviderAddress, buildVendorInitializationData()], {
    account: securityCouncil.account,
  });
  return {
    mockDashboard,
    mockStakingVault,
    mockStakingVaultAddress,
    yieldProvider,
    yieldProviderAddress,
  };
}
