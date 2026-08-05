import * as dotenv from "dotenv";
import { ethers } from "ethers";
import path from "path";

import { abi as LinethRollupV7Abi, bytecode as LinethRollupV7Bytecode } from "./dynamic-artifacts/LinethRollupV7.json";
import {
  contractName as ProxyAdminContractName,
  abi as ProxyAdminAbi,
  bytecode as ProxyAdminBytecode,
} from "./static-artifacts/ProxyAdmin.json";
import {
  abi as TransparentUpgradeableProxyAbi,
  bytecode as TransparentUpgradeableProxyBytecode,
} from "./static-artifacts/TransparentUpgradeableProxy.json";
import {
  LINETH_ROLLUP_V7_PAUSE_TYPES_ROLES,
  LINETH_ROLLUP_V7_UNPAUSE_TYPES_ROLES,
  LINETH_ROLLUP_V7_ROLES,
  OPERATOR_ROLE,
  YIELD_PROVIDER_STAKING_ROLE,
} from "../common/constants";
import {
  deployContractFromArtifacts,
  getDeployNonceFromEnv,
  getInitializerData,
  loadArtifactFromDirectory,
} from "../common/helpers/deployments";
import { getEnvVarOrDefault, getRequiredEnvVar } from "../common/helpers/environment";
import {
  getDeploymentNetworkName,
  requireAddressesFromRegistryOrEnv,
  requireAddressFromRegistryOrEnv,
} from "../common/helpers/readAddress";
import { generateRoleAssignments } from "../common/helpers/roles";
import { get1559Fees } from "../scripts/utils";

dotenv.config();

async function main() {
  const networkName = getDeploymentNetworkName();
  const verifierName = getRequiredEnvVar("VERIFIER_CONTRACT_NAME");
  const linethRollupInitialStateRootHash = getRequiredEnvVar("INITIAL_L2_STATE_ROOT_HASH");
  const linethRollupInitialL2BlockNumber = getRequiredEnvVar("INITIAL_L2_BLOCK_NUMBER");
  const linethRollupSecurityCouncil = requireAddressFromRegistryOrEnv(
    networkName,
    "L1_SECURITY_COUNCIL",
    "L1_SECURITY_COUNCIL",
  );
  const linethRollupOperators = requireAddressesFromRegistryOrEnv(
    networkName,
    "LINETH_ROLLUP_OPERATORS",
    "LINETH_ROLLUP_OPERATORS",
  );
  const linethRollupRateLimitPeriodInSeconds = getRequiredEnvVar("LINETH_ROLLUP_RATE_LIMIT_PERIOD");
  const linethRollupRateLimitAmountInWei = getRequiredEnvVar("LINETH_ROLLUP_RATE_LIMIT_AMOUNT");
  const linethRollupGenesisTimestamp = getRequiredEnvVar("L2_GENESIS_TIMESTAMP");
  const multiCallAddress = "0xcA11bde05977b3631167028862bE2a173976CA11";
  const linethRollupName = "LinethRollupV7";
  const linethRollupImplementationName = "LinethRollupV7Implementation";

  const pauseTypeRoles = getEnvVarOrDefault("LINETH_ROLLUP_PAUSE_TYPES_ROLES", LINETH_ROLLUP_V7_PAUSE_TYPES_ROLES);
  const unpauseTypeRoles = getEnvVarOrDefault(
    "LINETH_ROLLUP_UNPAUSE_TYPES_ROLES",
    LINETH_ROLLUP_V7_UNPAUSE_TYPES_ROLES,
  );
  // Use random hardcoded address until we introduce YieldManager E2E tests
  const automationServiceAddress = "0x3A9f0c2b8e7D4F6e1b5a9C2e0Fd7a4B6C8e9F1A2";
  const defaultRoleAddresses = [
    ...generateRoleAssignments(LINETH_ROLLUP_V7_ROLES, linethRollupSecurityCouncil, [
      { role: OPERATOR_ROLE, addresses: linethRollupOperators },
    ]),
    { role: YIELD_PROVIDER_STAKING_ROLE, addressWithRole: automationServiceAddress },
  ];
  const roleAddresses = getEnvVarOrDefault("LINETH_ROLLUP_ROLE_ADDRESSES", defaultRoleAddresses);

  const verifierArtifacts = loadArtifactFromDirectory(path.join(__dirname, "./dynamic-artifacts"), verifierName);

  const provider = new ethers.JsonRpcProvider(process.env.RPC_URL);

  const wallet = new ethers.Wallet(process.env.DEPLOYER_PRIVATE_KEY!, provider);

  const { gasPrice } = await get1559Fees(provider);

  const walletNonce = await getDeployNonceFromEnv(wallet, "L1_NONCE");

  const [verifier, linethRollupImplementation, proxyAdmin] = await Promise.all([
    deployContractFromArtifacts(verifierName, verifierArtifacts.abi, verifierArtifacts.bytecode, wallet, {
      nonce: walletNonce,
      gasPrice,
    }),
    deployContractFromArtifacts(linethRollupImplementationName, LinethRollupV7Abi, LinethRollupV7Bytecode, wallet, {
      nonce: walletNonce + 1,
      gasPrice,
    }),
    deployContractFromArtifacts(ProxyAdminContractName, ProxyAdminAbi, ProxyAdminBytecode, wallet, {
      nonce: walletNonce + 2,
      gasPrice,
    }),
  ]);

  const proxyAdminAddress = await proxyAdmin.getAddress();
  const verifierAddress = await verifier.getAddress();
  const linethRollupImplementationAddress = await linethRollupImplementation.getAddress();

  const initializer = getInitializerData(LinethRollupV7Abi, "initialize", [
    {
      initialStateRootHash: linethRollupInitialStateRootHash,
      initialL2BlockNumber: linethRollupInitialL2BlockNumber,
      genesisTimestamp: linethRollupGenesisTimestamp,
      defaultVerifier: verifierAddress,
      rateLimitPeriodInSeconds: linethRollupRateLimitPeriodInSeconds,
      rateLimitAmountInWei: linethRollupRateLimitAmountInWei,
      roleAddresses,
      pauseTypeRoles,
      unpauseTypeRoles,
      // Use random hardcoded address temporarily until we introduce YieldManager to E2E tests
      initialYieldManager: "0xB7De4A2cf9E1c6a0B5f8d3e7a9C4B1a2e6d0f5C8",
      fallbackOperator: multiCallAddress,
      defaultAdmin: linethRollupSecurityCouncil,
    },
  ]);

  await deployContractFromArtifacts(
    linethRollupName,
    TransparentUpgradeableProxyAbi,
    TransparentUpgradeableProxyBytecode,
    wallet,
    linethRollupImplementationAddress,
    proxyAdminAddress,
    initializer,
    { gasPrice },
  );
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
