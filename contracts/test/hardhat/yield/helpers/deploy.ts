import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { ethers, upgrades } from "hardhat";

import {
  YIELD_MANAGER_PAUSE_TYPES_ROLES,
  YIELD_MANAGER_UNPAUSE_TYPES_ROLES,
  YIELD_MANAGER_OPERATOR_ROLES,
  YIELD_MANAGER_SECURITY_COUNCIL_ROLES,
  YIELD_MANAGER_INITIALIZE_SIGNATURE,
} from "../../../../common/constants";
import {
  MINIMUM_WITHDRAWAL_RESERVE_PERCENTAGE_BPS,
  TARGET_WITHDRAWAL_RESERVE_PERCENTAGE_BPS,
  MINIMUM_WITHDRAWAL_RESERVE_AMOUNT,
  TARGET_WITHDRAWAL_RESERVE_AMOUNT,
  GI_FIRST_VALIDATOR,
  GI_PENDING_PARTIAL_WITHDRAWALS_ROOT,
  YIELD_PROVIDER_STAKING_ROLE,
  ONE_ETHER,
} from "../../common/constants";
import { generateRoleAssignments } from "../../../../common/helpers";
import { deployUpgradableWithConstructorArgs } from "../../common/deployment";
import {
  TestYieldManager,
  MockLineaRollup,
  MockYieldProvider,
  MockWithdrawTarget,
  MockVaultHub,
  MockVaultFactory,
  MockSTETH,
  MockDashboard,
  MockStakingVault,
  TestLidoStVaultYieldProvider,
  TestValidatorContainerProofVerifier,
  ValidatorContainerProofVerifier,
  SSZMerkleTree,
  TestLidoStVaultYieldProviderFactory,
} from "contracts/typechain-types";
import { YieldManagerInitializationData } from "./types";

import { getAccountsFixture } from "../../common/helpers";
import { deployLineaRollupFixture } from "../../rollup/helpers/deploy";
import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { buildVendorInitializationData } from "./mocks";
import { incrementBalance } from "./setup";

async function getYieldManagerRoleAddressesFixture(): Promise<
  {
    role: string;
    addressWithRole: string;
  }[]
> {
  const { nativeYieldOperator, securityCouncil } = await loadFixture(getAccountsFixture);
  const yieldManagerOpereratorRoleAssignments = generateRoleAssignments(
    YIELD_MANAGER_OPERATOR_ROLES,
    await nativeYieldOperator.getAddress(),
    [],
  );
  const securityCouncilRoleAssignments = generateRoleAssignments(
    YIELD_MANAGER_SECURITY_COUNCIL_ROLES,
    await securityCouncil.getAddress(),
    [],
  );
  return [...yieldManagerOpereratorRoleAssignments, ...securityCouncilRoleAssignments];
}

export async function deployMockLineaRollup(): Promise<MockLineaRollup> {
  const mockYieldManagerFactory = await ethers.getContractFactory("MockLineaRollup");
  const mockYieldManager = await mockYieldManagerFactory.deploy();
  await mockYieldManager.waitForDeployment();
  return await mockYieldManager;
}

export async function deployMockWithdrawTarget(): Promise<MockWithdrawTarget> {
  const factory = await ethers.getContractFactory("MockWithdrawTarget");
  const mockWithdrawTarget = await factory.deploy();
  await mockWithdrawTarget.waitForDeployment();
  return await mockWithdrawTarget;
}

export async function deployMockYieldProvider(): Promise<MockYieldProvider> {
  const mockYieldManagerFactory = await ethers.getContractFactory("MockYieldProvider");
  const mockYieldManager = await mockYieldManagerFactory.deploy();
  await mockYieldManager.waitForDeployment();
  return await mockYieldManager;
}

export async function deployValidatorContainerProofVerifier(): Promise<ValidatorContainerProofVerifier> {
  const [admin] = await ethers.getSigners();
  const factory = await ethers.getContractFactory("ValidatorContainerProofVerifier");
  const contract = await factory.deploy(
    await admin.getAddress(),
    GI_FIRST_VALIDATOR,
    GI_PENDING_PARTIAL_WITHDRAWALS_ROOT,
  );
  await contract.waitForDeployment();
  return contract;
}

export async function deployTestValidatorContainerProofVerifier(): Promise<TestValidatorContainerProofVerifier> {
  const [admin] = await ethers.getSigners();
  const factory = await ethers.getContractFactory("TestValidatorContainerProofVerifier");
  const contract = await factory.deploy(
    await admin.getAddress(),
    GI_FIRST_VALIDATOR,
    GI_PENDING_PARTIAL_WITHDRAWALS_ROOT,
  );
  await contract.waitForDeployment();
  return contract;
}

export async function deploySSZMerkleTree(): Promise<SSZMerkleTree> {
  const factory = await ethers.getContractFactory("SSZMerkleTree");
  const contract = await factory.deploy(GI_FIRST_VALIDATOR);
  await contract.waitForDeployment();
  return contract;
}

// Deploys with MockLineaRollup and MockYieldProvider
export async function deployYieldManagerForUnitTest() {
  upgrades.silenceWarnings();
  const { securityCouncil, l2YieldRecipient } = await loadFixture(getAccountsFixture);
  const roleAddresses = await loadFixture(getYieldManagerRoleAddressesFixture);

  const mockLineaRollup = await deployMockLineaRollup();

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

  const yieldManager = (await deployUpgradableWithConstructorArgs(
    "TestYieldManager",
    [await mockLineaRollup.getAddress()],
    [initializationData],
    {
      initializer: YIELD_MANAGER_INITIALIZE_SIGNATURE,
      unsafeAllow: ["constructor", "incorrect-initializer-order", "state-variable-immutable", "delegatecall"],
    },
  )) as unknown as TestYieldManager;

  return { mockLineaRollup, yieldManager, initializationData };
}

export async function deployYieldManagerForUnitTestWithMutatedInitData(
  mutatedInitData: YieldManagerInitializationData,
) {
  upgrades.silenceWarnings();
  const mockLineaRollup = await deployMockLineaRollup();
  await deployUpgradableWithConstructorArgs(
    "TestYieldManager",
    [await mockLineaRollup.getAddress()],
    [mutatedInitData],
    {
      // initializer: "initialize",
      initializer: YIELD_MANAGER_INITIALIZE_SIGNATURE,
      unsafeAllow: ["constructor", "incorrect-initializer-order", "state-variable-immutable", "delegatecall"],
    },
  );
}

export async function deployMockVaultHub(): Promise<MockVaultHub> {
  const factory = await ethers.getContractFactory("MockVaultHub");
  const contract = await factory.deploy();
  await contract.waitForDeployment();
  return contract;
}

export async function deployMockVaultFactory(): Promise<MockVaultFactory> {
  const factory = await ethers.getContractFactory("MockVaultFactory");
  const contract = await factory.deploy();
  await contract.waitForDeployment();
  return contract;
}

export async function deployMockSTETH(): Promise<MockSTETH> {
  const factory = await ethers.getContractFactory("MockSTETH");
  const contract = await factory.deploy();
  await contract.waitForDeployment();
  return contract;
}

export async function deployMockDashboard(): Promise<MockDashboard> {
  const factory = await ethers.getContractFactory("MockDashboard");
  const contract = await factory.deploy();
  await contract.waitForDeployment();
  return contract;
}

export async function deployMockStakingVault(): Promise<MockStakingVault> {
  const factory = await ethers.getContractFactory("MockStakingVault");
  const contract = await factory.deploy();
  await contract.waitForDeployment();
  return contract;
}

export async function deployLidoStVaultYieldProviderFactory() {
  const { mockLineaRollup, yieldManager } = await loadFixture(deployYieldManagerForUnitTest);
  const mockVaultHub = await deployMockVaultHub();
  const mockVaultFactory = await deployMockVaultFactory();
  const mockSTETH = await deployMockSTETH();
  const verifier = await deployValidatorContainerProofVerifier();

  const l1MessageServiceAddress = await mockLineaRollup.getAddress();
  const yieldManagerAddress = await yieldManager.getAddress();
  const mockVaultHubAddress = await mockVaultHub.getAddress();
  const mockVaultFactoryAddress = await mockVaultFactory.getAddress();
  const mockSTETHAddress = await mockSTETH.getAddress();
  const verifierAddress = await verifier.getAddress();

  const yieldProviderFactoryFactory = await ethers.getContractFactory("LidoStVaultYieldProviderFactory");
  const lidoStVaultYieldProviderFactory = await yieldProviderFactoryFactory.deploy(
    l1MessageServiceAddress,
    yieldManagerAddress,
    mockVaultHubAddress,
    mockVaultFactoryAddress,
    mockSTETHAddress,
    verifierAddress,
  );
  await lidoStVaultYieldProviderFactory.waitForDeployment();

  return {
    mockLineaRollup,
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
  const { mockLineaRollup, yieldManager } = await deployYieldManagerForUnitTest();
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
    mockLineaRollup,
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
  const {
    securityCouncil,
    mockLineaRollup,
    yieldManager,
    mockVaultHub,
    mockSTETH,
    mockVaultFactory,
    mockDashboard,
    mockStakingVault,
    sszMerkleTree,
    verifier,
    testVerifier,
  } = await loadFixture(deployLidoStVaultYieldProviderDependenciesFixture);

  const l1MessageServiceAddress = await mockLineaRollup.getAddress();
  const yieldManagerAddress = await yieldManager.getAddress();
  const mockVaultHubAddress = await mockVaultHub.getAddress();
  const mockVaultFactoryAddress = await mockVaultFactory.getAddress();
  const mockSTETHAddress = await mockSTETH.getAddress();
  const verifierAddress = await verifier.getAddress();

  // Deploy Factory
  const yieldProviderFactoryFactory = await ethers.getContractFactory("TestLidoStVaultYieldProviderFactory");
  const lidoStVaultYieldProviderFactory = await yieldProviderFactoryFactory.deploy(
    l1MessageServiceAddress,
    yieldManagerAddress,
    mockVaultHubAddress,
    mockVaultFactoryAddress,
    mockSTETHAddress,
    verifierAddress,
  );
  await lidoStVaultYieldProviderFactory.waitForDeployment();

  // Create YieldProvider
  const yieldProviderAddress = await lidoStVaultYieldProviderFactory.createTestLidoStVaultYieldProvider.staticCall();
  await lidoStVaultYieldProviderFactory.connect(securityCouncil).createTestLidoStVaultYieldProvider();
  const yieldProvider: TestLidoStVaultYieldProvider = (
    await ethers.getContractFactory("TestLidoStVaultYieldProvider")
  ).attach(yieldProviderAddress);

  // Add YieldProvider
  await mockDashboard.setStakingVaultReturn(await mockStakingVault.getAddress());
  const mockDashboardAddress = await mockDashboard.getAddress();
  const mockStakingVaultAddress = await mockStakingVault.getAddress();

  await mockVaultFactory.setReturnValues(mockStakingVaultAddress, mockDashboardAddress);
  await incrementBalance(yieldManagerAddress, ONE_ETHER); // Connect Deposit
  await yieldManager.connect(securityCouncil).addYieldProvider(yieldProviderAddress, buildVendorInitializationData());
  return {
    mockDashboard,
    mockStakingVault,
    yieldManager,
    yieldProvider,
    yieldProviderAddress,
    mockVaultHub,
    mockSTETH,
    mockVaultFactory,
    mockLineaRollup,
    sszMerkleTree,
    verifier,
    verifierAddress,
    testVerifier,
  };
}

export async function deployYieldManagerIntegrationTestFixture() {
  const { securityCouncil, l2YieldRecipient, nativeYieldOperator } = await loadFixture(getAccountsFixture);
  const yieldManagerRoleAddresses = await loadFixture(getYieldManagerRoleAddressesFixture);
  // Deploy LineaRollup
  const { lineaRollup } = await loadFixture(deployLineaRollupFixture);
  const l1MessageServiceAddress = await lineaRollup.getAddress();

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

  upgrades.silenceWarnings();
  const yieldManager = (await deployUpgradableWithConstructorArgs(
    "TestYieldManager",
    [l1MessageServiceAddress],
    [initializationData],
    {
      initializer: YIELD_MANAGER_INITIALIZE_SIGNATURE,
      unsafeAllow: ["constructor", "incorrect-initializer-order", "state-variable-immutable", "delegatecall"],
    },
  )) as unknown as TestYieldManager;

  // Deploy LidoStVaultYieldProviderFactory
  const mockVaultHub = await deployMockVaultHub();
  const mockVaultFactory = await deployMockVaultFactory();
  const mockSTETH = await deployMockSTETH();
  const mockDashboard = await deployMockDashboard();
  const mockStakingVault = await deployMockStakingVault();
  const verifier = await deployValidatorContainerProofVerifier();

  const yieldManagerAddress = await yieldManager.getAddress();
  const mockVaultHubAddress = await mockVaultHub.getAddress();
  const mockVaultFactoryAddress = await mockVaultFactory.getAddress();
  const mockSTETHAddress = await mockSTETH.getAddress();
  const verifierAddress = await verifier.getAddress();

  const yieldProviderFactoryFactory = await ethers.getContractFactory("TestLidoStVaultYieldProviderFactory");
  const lidoStVaultYieldProviderFactory = await yieldProviderFactoryFactory.deploy(
    l1MessageServiceAddress,
    yieldManagerAddress,
    mockVaultHubAddress,
    mockVaultFactoryAddress,
    mockSTETHAddress,
    verifierAddress,
  );
  await lidoStVaultYieldProviderFactory.waitForDeployment();

  // Create YieldProvider
  const yieldProviderAddress = await lidoStVaultYieldProviderFactory.createTestLidoStVaultYieldProvider.staticCall();
  await lidoStVaultYieldProviderFactory.connect(securityCouncil).createTestLidoStVaultYieldProvider();
  const yieldProvider = await ethers.getContractAt("TestLidoStVaultYieldProvider", yieldProviderAddress);

  // Add YieldProvider
  await mockDashboard.setStakingVaultReturn(await mockStakingVault.getAddress());
  const mockDashboardAddress = await mockDashboard.getAddress();
  const mockStakingVaultAddress = await mockStakingVault.getAddress();

  await mockVaultFactory.setReturnValues(mockStakingVaultAddress, mockDashboardAddress);
  await incrementBalance(yieldManagerAddress, ONE_ETHER); // Connect Deposit
  await yieldManager.connect(securityCouncil).addYieldProvider(yieldProviderAddress, buildVendorInitializationData());

  await lineaRollup.connect(securityCouncil).setYieldManager(yieldManagerAddress);
  await lineaRollup
    .connect(securityCouncil)
    .grantRole(YIELD_PROVIDER_STAKING_ROLE, await nativeYieldOperator.getAddress());

  // Deploy EIP-4788 Beacon Proof utils
  const sszMerkleTree = await deploySSZMerkleTree();
  const testVerifier = await deployTestValidatorContainerProofVerifier();

  return {
    lineaRollup,
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
  factory: TestLidoStVaultYieldProviderFactory,
  yieldManager: TestYieldManager,
  securityCouncil: SignerWithAddress,
  mockVaultFactory: MockVaultFactory,
) {
  // Create YieldProvider
  const yieldProviderAddress = await factory.createTestLidoStVaultYieldProvider.staticCall();
  await factory.createTestLidoStVaultYieldProvider();
  const yieldProvider: TestLidoStVaultYieldProvider = (
    await ethers.getContractFactory("TestLidoStVaultYieldProvider")
  ).attach(yieldProviderAddress);

  // Add YieldProvider
  const mockDashboard = await deployMockDashboard();
  const mockStakingVault = await deployMockStakingVault();
  await mockDashboard.setStakingVaultReturn(await mockStakingVault.getAddress());
  const mockDashboardAddress = await mockDashboard.getAddress();
  const mockStakingVaultAddress = await mockStakingVault.getAddress();

  await mockVaultFactory.setReturnValues(mockStakingVaultAddress, mockDashboardAddress);
  await incrementBalance(await yieldManager.getAddress(), ONE_ETHER); // Connect Deposit
  await yieldManager.connect(securityCouncil).addYieldProvider(yieldProviderAddress, buildVendorInitializationData());
  return {
    mockDashboard,
    mockStakingVault,
    mockStakingVaultAddress,
    yieldProvider,
    yieldProviderAddress,
  };
}
