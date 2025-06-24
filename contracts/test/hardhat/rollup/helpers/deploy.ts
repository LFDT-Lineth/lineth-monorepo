import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { ethers } from "hardhat";

import firstCompressedDataContent from "../../_testData/compressedData/blocks-1-46.json";

import { LINEA_ROLLUP_PAUSE_TYPES_ROLES, LINEA_ROLLUP_UNPAUSE_TYPES_ROLES } from "contracts/common/constants";
import { CallForwardingProxy, TestLineaRollup } from "contracts/typechain-types";
import { getAccountsFixture, getRoleAddressesFixture } from "./";
import {
  DEFAULT_LAST_FINALIZED_TIMESTAMP,
  FALLBACK_OPERATOR_ADDRESS,
  INITIAL_WITHDRAW_LIMIT,
  LINEA_ROLLUP_INITIALIZE_SIGNATURE,
  ONE_DAY_IN_SECONDS,
} from "../../common/constants";
import { deployUpgradableFromFactory } from "../../common/deployment";

export async function deployRevertingVerifier(scenario: bigint): Promise<string> {
  const revertingVerifierFactory = await ethers.getContractFactory("RevertingVerifier");
  const verifier = await revertingVerifierFactory.deploy(scenario);
  await verifier.waitForDeployment();
  return await verifier.getAddress();
}

export async function deployPlonkVerifierSepoliaFull(): Promise<string> {
  const plonkVerifierSepoliaFull = await ethers.getContractFactory("PlonkVerifierSepoliaFull");
  const verifier = await plonkVerifierSepoliaFull.deploy();
  await verifier.waitForDeployment();
  return await verifier.getAddress();
}

export async function deployPlonkVerifierMainnetFull(): Promise<string> {
  const plonkVerifierMainnetFull = await ethers.getContractFactory("PlonkVerifierMainnetFull");
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

export async function deployLineaRollupFixture() {
  const { securityCouncil } = await loadFixture(getAccountsFixture);
  const roleAddresses = await loadFixture(getRoleAddressesFixture);

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
    pauseTypeRoles: LINEA_ROLLUP_PAUSE_TYPES_ROLES,
    unpauseTypeRoles: LINEA_ROLLUP_UNPAUSE_TYPES_ROLES,
    fallbackOperator: FALLBACK_OPERATOR_ADDRESS,
    defaultAdmin: securityCouncil.address,
  };

  const lineaRollup = (await deployUpgradableFromFactory("TestLineaRollup", [initializationData], {
    initializer: LINEA_ROLLUP_INITIALIZE_SIGNATURE,
    unsafeAllow: ["constructor", "incorrect-initializer-order"],
  })) as unknown as TestLineaRollup;

  return { verifier, lineaRollup };
}

async function deployTestPlonkVerifierForDataAggregation(): Promise<string> {
  const plonkVerifierSepoliaFull = await ethers.getContractFactory("TestPlonkVerifierForDataAggregation");
  const verifier = await plonkVerifierSepoliaFull.deploy();
  await verifier.waitForDeployment();
  return await verifier.getAddress();
}
