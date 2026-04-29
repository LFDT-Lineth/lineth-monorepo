import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { PRECOMPILES_ADDRESSES } from "contracts/common/constants";
import {
  LINEA_ROLLUP_V8_PAUSE_TYPES_ROLES,
  LINEA_ROLLUP_V8_UNPAUSE_TYPES_ROLES,
  VALIDIUM_PAUSE_TYPES_ROLES,
  VALIDIUM_UNPAUSE_TYPES_ROLES,
} from "contracts/common/constants/pauseTypes";
import {
  AddressFilter,
  CallForwardingProxy,
  ForcedTransactionGateway,
  Mimc,
  TestLineaRollup,
  TestValidium,
} from "contracts/typechain-types";
import { toBeHex } from "ethers";
import { ethers } from "hardhat";

import { getAccountsFixture, getRoleAddressesFixture, getValidiumRoleAddressesFixture } from "./before";
import firstCompressedDataContent from "../../_testData/compressedData/blocks-1-46.json";
import {
  ADDRESS_ZERO,
  BLOCK_NUMBER_DEADLINE_BUFFER,
  DEFAULT_LAST_FINALIZED_TIMESTAMP,
  FALLBACK_OPERATOR_ADDRESS,
  INITIAL_WITHDRAW_LIMIT,
  L2_BLOCK_DURATION_SECONDS,
  LINEA_MAINNET_CHAIN_ID,
  LINEA_ROLLUP_INITIALIZE_SIGNATURE,
  MAX_FORCED_TRANSACTION_GAS_LIMIT,
  MAX_INPUT_LENGTH_LIMIT,
  ONE_DAY_IN_SECONDS,
  THREE_DAYS_IN_SECONDS,
  VALIDIUM_INITIALIZE_SIGNATURE,
} from "../../common/constants";
import { deployFromFactory, deployUpgradableFromFactory } from "../../common/deployment";
import { LineaRollupInitializationData, PauseTypeRole } from "../../common/types";

export async function deployRevertingVerifier(scenario: bigint): Promise<string> {
  const revertingVerifierFactory = await ethers.getContractFactory("RevertingVerifier");
  const verifier = await revertingVerifierFactory.deploy(scenario);
  await verifier.waitForDeployment();
  return await verifier.getAddress();
}

export async function deployCallForwardingProxy(target: string): Promise<CallForwardingProxy> {
  const callForwardingProxyFactory = await ethers.getContractFactory("CallForwardingProxy");
  const callForwardingProxy = await callForwardingProxyFactory.deploy(target);
  await callForwardingProxy.waitForDeployment();
  return callForwardingProxy;
}
export async function deployValidiumFixture() {
  const { securityCouncil, nonAuthorizedAccount } = await loadFixture(getAccountsFixture);
  const roleAddresses = await loadFixture(getValidiumRoleAddressesFixture);

  const { addressFilter } = await deployAddressFilter(securityCouncil.address, [nonAuthorizedAccount.address]);

  const verifier = await deployTestPlonkVerifierForDataAggregation();
  const { parentStateRootHash } = firstCompressedDataContent;

  const initializationData = {
    initialStateRootHash: parentStateRootHash,
    initialL2BlockNumber: 0,
    genesisTimestamp: DEFAULT_LAST_FINALIZED_TIMESTAMP,
    defaultVerifier: verifier,
    rateLimitPeriodInSeconds: ONE_DAY_IN_SECONDS,
    rateLimitAmountInWei: INITIAL_WITHDRAW_LIMIT,
    roleAddresses,
    pauseTypeRoles: VALIDIUM_PAUSE_TYPES_ROLES,
    unpauseTypeRoles: VALIDIUM_UNPAUSE_TYPES_ROLES,
    defaultAdmin: securityCouncil.address,
    shnarfProvider: ADDRESS_ZERO,
    addressFilter: await addressFilter.getAddress(),
  };

  const validium = (await deployUpgradableFromFactory("TestValidium", [initializationData], {
    initializer: VALIDIUM_INITIALIZE_SIGNATURE,
    unsafeAllow: ["constructor", "incorrect-initializer-order"],
  })) as unknown as TestValidium;

  return { verifier, validium, addressFilter };
}

export async function deployMockYieldManager(): Promise<string> {
  const mockYieldManagerFactory = await ethers.getContractFactory("MockYieldManager");
  const mockYieldManager = await mockYieldManagerFactory.deploy();
  await mockYieldManager.waitForDeployment();
  return await mockYieldManager.getAddress();
}

export async function deployLineaRollupFixture() {
  const { securityCouncil, nonAuthorizedAccount } = await loadFixture(getAccountsFixture);
  const roleAddresses = await loadFixture(getRoleAddressesFixture);

  const { addressFilter } = await deployAddressFilter(securityCouncil.address, [nonAuthorizedAccount.address]);

  const verifier = await deployTestPlonkVerifierForDataAggregation();
  const { parentStateRootHash } = firstCompressedDataContent;

  const yieldManager = await deployMockYieldManager();

  const initializationData: LineaRollupInitializationData = {
    initialStateRootHash: parentStateRootHash,
    initialL2BlockNumber: 0n,
    genesisTimestamp: DEFAULT_LAST_FINALIZED_TIMESTAMP,
    defaultVerifier: verifier,
    rateLimitPeriodInSeconds: BigInt(ONE_DAY_IN_SECONDS),
    rateLimitAmountInWei: BigInt(INITIAL_WITHDRAW_LIMIT),
    roleAddresses,
    pauseTypeRoles: LINEA_ROLLUP_V8_PAUSE_TYPES_ROLES as unknown as PauseTypeRole[],
    unpauseTypeRoles: LINEA_ROLLUP_V8_UNPAUSE_TYPES_ROLES as unknown as PauseTypeRole[],
    defaultAdmin: securityCouncil.address,
    shnarfProvider: ADDRESS_ZERO,
    addressFilter: await addressFilter.getAddress(),
  };

  const lineaRollup = (await deployUpgradableFromFactory(
    "TestLineaRollup",
    [initializationData, FALLBACK_OPERATOR_ADDRESS, yieldManager],
    {
      initializer: LINEA_ROLLUP_INITIALIZE_SIGNATURE,
      unsafeAllow: ["constructor", "incorrect-initializer-order"],
    },
  )) as unknown as TestLineaRollup;

  return { verifier, lineaRollup, addressFilter, yieldManager, lineaRollupInitializationData: initializationData };
}

export async function deployAddressFilter(securityCouncil: string, nonAuthorizedAccount: string[]) {
  const AddressFilterFactory = await ethers.getContractFactory("AddressFilter");

  const addressFilter = (await AddressFilterFactory.deploy(securityCouncil, [
    ...PRECOMPILES_ADDRESSES,
    ...nonAuthorizedAccount,
  ])) as unknown as AddressFilter;

  await addressFilter.waitForDeployment();

  return { addressFilter };
}

export async function deployMimcFixture() {
  const mimc = (await deployFromFactory("Mimc")) as unknown as Mimc;
  await mimc.waitForDeployment();
  return { mimc };
}

export async function deployForcedTransactionGatewayFixture() {
  const { securityCouncil } = await loadFixture(getAccountsFixture);
  const { lineaRollup, addressFilter, verifier, yieldManager, lineaRollupInitializationData } =
    await loadFixture(deployLineaRollupFixture);
  const { mimc } = await loadFixture(deployMimcFixture);

  const forcedTransactionGatewayFactory = await ethers.getContractFactory("ForcedTransactionGateway", {
    libraries: { Mimc: await mimc.getAddress() },
  });

  const forcedTransactionGateway = (await forcedTransactionGatewayFactory.deploy(
    await lineaRollup.getAddress(),
    LINEA_MAINNET_CHAIN_ID,
    THREE_DAYS_IN_SECONDS,
    MAX_FORCED_TRANSACTION_GAS_LIMIT,
    MAX_INPUT_LENGTH_LIMIT,
    securityCouncil.address,
    await addressFilter.getAddress(),
    L2_BLOCK_DURATION_SECONDS,
    BLOCK_NUMBER_DEADLINE_BUFFER,
  )) as unknown as ForcedTransactionGateway;

  await forcedTransactionGateway.waitForDeployment();

  return {
    lineaRollup,
    forcedTransactionGateway,
    addressFilter,
    mimc,
    verifier,
    yieldManager,
    lineaRollupInitializationData,
  };
}

export async function deployAddressFilterFixture() {
  const { securityCouncil, nonAuthorizedAccount } = await loadFixture(getAccountsFixture);
  const { addressFilter } = await deployAddressFilter(securityCouncil.address, [nonAuthorizedAccount.address]);
  return { addressFilter };
}

async function deployTestPlonkVerifierForDataAggregation(): Promise<string> {
  const mimc = (await deployFromFactory("Mimc")) as Mimc;
  const testPlonkVerifierFactory = await ethers.getContractFactory("TestPlonkVerifierForDataAggregation", {
    libraries: { Mimc: await mimc.getAddress() },
  });
  const verifier = await testPlonkVerifierFactory.deploy([
    {
      value: toBeHex(59144, 32),
      name: "chainId",
    },
    {
      value: toBeHex(7n, 32),
      name: "baseFee",
    },
    {
      value: toBeHex("0x8f81e2e3f8b46467523463835f965ffe476e1c9e", 32),
      name: "coinbase",
    },
    {
      value: toBeHex("0x508Ca82Df566dCD1B0DE8296e70a96332cD644ec", 32),
      name: "l2MessageServiceAddress",
    },
  ]);
  await verifier.waitForDeployment();
  return await verifier.getAddress();
}
