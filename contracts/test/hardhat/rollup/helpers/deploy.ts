import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { ethers } from "hardhat";

import firstCompressedDataContent from "../../_testData/compressedData/blocks-1-46.json";

import {
  LINEA_ROLLUP_V8_PAUSE_TYPES_ROLES,
  LINEA_ROLLUP_V8_UNPAUSE_TYPES_ROLES,
  VALIDIUM_PAUSE_TYPES_ROLES,
  VALIDIUM_UNPAUSE_TYPES_ROLES,
} from "contracts/common/constants";
import { CallForwardingProxy, Mimc, TestLineaRollup, TestValidium } from "contracts/typechain-types";
import { getAccountsFixture, getRoleAddressesFixture, getValidiumRoleAddressesFixture } from "./";
import {
  ADDRESS_ZERO,
  DEFAULT_LAST_FINALIZED_TIMESTAMP,
  FALLBACK_OPERATOR_ADDRESS,
  INITIAL_WITHDRAW_LIMIT,
  LINEA_ROLLUP_INITIALIZE_SIGNATURE,
  ONE_DAY_IN_SECONDS,
  VALIDIUM_INITIALIZE_SIGNATURE,
} from "../../common/constants";
import { deployUpgradableFromFactory, deployFromFactory } from "../../common/deployment";
import { toBeHex } from "ethers";

export async function deployRevertingVerifier(scenario: bigint): Promise<string> {
  const revertingVerifierFactory = await ethers.getContractFactory("RevertingVerifier");
  const verifier = await revertingVerifierFactory.deploy(scenario);
  await verifier.waitForDeployment();
  return await verifier.getAddress();
}

export async function deployPlonkVerifierSepoliaFull(): Promise<string> {
  const plonkVerifierSepoliaFull = await ethers.getContractFactory(
    "src/verifiers/PlonkVerifierSepoliaFull.sol:PlonkVerifierSepoliaFull",
  );
  const verifier = await plonkVerifierSepoliaFull.deploy();
  await verifier.waitForDeployment();
  return await verifier.getAddress();
}

export async function deployPlonkVerifierMainnetFull(): Promise<string> {
  const plonkVerifierMainnetFull = await ethers.getContractFactory(
    "src/verifiers/PlonkVerifierMainnetFull.sol:PlonkVerifierMainnetFull",
  );
  const verifier = await plonkVerifierMainnetFull.deploy();
  await verifier.waitForDeployment();
  return await verifier.getAddress();
}

export async function deployPlonkVerifierDev(): Promise<string> {
  const plonkVerifierDev = await ethers.getContractFactory("PlonkVerifierDev");
  const verifier = await plonkVerifierDev.deploy();
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
  const { securityCouncil } = await loadFixture(getAccountsFixture);
  const roleAddresses = await loadFixture(getValidiumRoleAddressesFixture);

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
  };

  const validium = (await deployUpgradableFromFactory("TestValidium", [initializationData], {
    initializer: VALIDIUM_INITIALIZE_SIGNATURE,
    unsafeAllow: ["constructor", "incorrect-initializer-order"],
  })) as unknown as TestValidium;

  return { verifier, validium };
}

export async function deployMockYieldManager(): Promise<string> {
  const mockYieldManagerFactory = await ethers.getContractFactory("MockYieldManager");
  const mockYieldManager = await mockYieldManagerFactory.deploy();
  await mockYieldManager.waitForDeployment();
  return await mockYieldManager.getAddress();
}

export async function deployLineaRollupFixture() {
  const { securityCouncil } = await loadFixture(getAccountsFixture);
  const roleAddresses = await loadFixture(getRoleAddressesFixture);

  const verifier = await deployTestPlonkVerifierForDataAggregation();
  const { parentStateRootHash } = firstCompressedDataContent;

  const yieldManager = await deployMockYieldManager();

  const initializationData = {
    initialStateRootHash: parentStateRootHash,
    initialL2BlockNumber: 0,
    genesisTimestamp: DEFAULT_LAST_FINALIZED_TIMESTAMP,
    defaultVerifier: verifier,
    rateLimitPeriodInSeconds: ONE_DAY_IN_SECONDS,
    rateLimitAmountInWei: INITIAL_WITHDRAW_LIMIT,
    roleAddresses,
    pauseTypeRoles: LINEA_ROLLUP_V8_PAUSE_TYPES_ROLES,
    unpauseTypeRoles: LINEA_ROLLUP_V8_UNPAUSE_TYPES_ROLES,
    defaultAdmin: securityCouncil.address,
    shnarfProvider: ADDRESS_ZERO,
  };

  const lineaRollup = (await deployUpgradableFromFactory(
    "TestLineaRollup",
    [initializationData, FALLBACK_OPERATOR_ADDRESS, yieldManager],
    {
      initializer: LINEA_ROLLUP_INITIALIZE_SIGNATURE,
      unsafeAllow: ["constructor", "incorrect-initializer-order"],
    },
  )) as unknown as TestLineaRollup;

  return { verifier, lineaRollup, yieldManager, roleAddresses };
}

async function deployTestPlonkVerifierForDataAggregation(): Promise<string> {
  const mimc = (await deployFromFactory("Mimc")) as Mimc;
  const plonkVerifierSepoliaFull = await ethers.getContractFactory("TestPlonkVerifierForDataAggregation", {
    libraries: { Mimc: await mimc.getAddress() },
  });
  const verifier = await plonkVerifierSepoliaFull.deploy([
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
