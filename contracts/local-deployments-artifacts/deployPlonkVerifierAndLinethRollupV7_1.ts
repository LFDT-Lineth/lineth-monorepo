import * as dotenv from "dotenv";
import { ethers } from "ethers";
import path from "path";

import {
  abi as LinethRollupV7_1Abi,
  bytecode as LinethRollupV7_1Bytecode,
} from "./dynamic-artifacts/LinethRollupV7.1.json";
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
  LINETH_ROLLUP_V8_PAUSE_TYPES_ROLES,
  LINETH_ROLLUP_V8_UNPAUSE_TYPES_ROLES,
  LINETH_ROLLUP_V8_ROLES,
  OPERATOR_ROLE,
  ADDRESS_ZERO,
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
  const linethRollupYieldManager = requireAddressFromRegistryOrEnv(
    networkName,
    "YieldManager",
    "YIELD_MANAGER_ADDRESS",
  );
  const multiCallAddress = "0xcA11bde05977b3631167028862bE2a173976CA11";
  const linethRollupName = "LinethRollupV7.1";
  const linethRollupImplementationName = "LinethRollupV7_1Implementation";

  const pauseTypeRoles = getEnvVarOrDefault("LINETH_ROLLUP_PAUSE_TYPES_ROLES", LINETH_ROLLUP_V8_PAUSE_TYPES_ROLES);
  const unpauseTypeRoles = getEnvVarOrDefault(
    "LINETH_ROLLUP_UNPAUSE_TYPES_ROLES",
    LINETH_ROLLUP_V8_UNPAUSE_TYPES_ROLES,
  );
  const defaultRoleAddresses = generateRoleAssignments(LINETH_ROLLUP_V8_ROLES, linethRollupSecurityCouncil, [
    { role: OPERATOR_ROLE, addresses: linethRollupOperators },
  ]);
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
    deployContractFromArtifacts(linethRollupImplementationName, LinethRollupV7_1Abi, LinethRollupV7_1Bytecode, wallet, {
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

  const initializer = getInitializerData(LinethRollupV7_1Abi, "initialize", [
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
      defaultAdmin: linethRollupSecurityCouncil,
      shnarfProvider: ADDRESS_ZERO,
    },
    multiCallAddress,
    linethRollupYieldManager,
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
