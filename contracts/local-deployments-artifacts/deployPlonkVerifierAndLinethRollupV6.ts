import * as dotenv from "dotenv";
import { ethers } from "ethers";
import path from "path";

import linethRollupV6Json from "./dynamic-artifacts/LinethRollupV6.json" with { type: "json" };
import proxyAdminJson from "./static-artifacts/ProxyAdmin.json" with { type: "json" };
import transparentUpgradeableProxyJson from "./static-artifacts/TransparentUpgradeableProxy.json" with { type: "json" };
import {
  LINETH_ROLLUP_V6_PAUSE_TYPES_ROLES,
  LINETH_ROLLUP_V6_UNPAUSE_TYPES_ROLES,
  LINETH_ROLLUP_V6_ROLES,
  OPERATOR_ROLE,
} from "../common/constants/index.js";
import {
  deployContractFromArtifacts,
  getDeployNonceFromEnv,
  getInitializerData,
  loadArtifactFromDirectory,
} from "../common/helpers/deployments.js";
import { getEnvVarOrDefault, getRequiredEnvVar } from "../common/helpers/environment.js";
import {
  getDeploymentNetworkName,
  requireAddressesFromRegistryOrEnv,
  requireAddressFromRegistryOrEnv,
} from "../common/helpers/readAddress.js";
import { generateRoleAssignments } from "../common/helpers/roles.js";
import { get1559Fees } from "../scripts/utils.js";

const { abi: LinethRollupV6Abi, bytecode: LinethRollupV6Bytecode } = linethRollupV6Json;
const { contractName: ProxyAdminContractName, abi: ProxyAdminAbi, bytecode: ProxyAdminBytecode } = proxyAdminJson;
const { abi: TransparentUpgradeableProxyAbi, bytecode: TransparentUpgradeableProxyBytecode } =
  transparentUpgradeableProxyJson;

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
  const linethRollupName = "LinethRollupV6";
  const linethRollupImplementationName = "LinethRollupV6Implementation";

  const pauseTypeRoles = getEnvVarOrDefault("LINETH_ROLLUP_PAUSE_TYPES_ROLES", LINETH_ROLLUP_V6_PAUSE_TYPES_ROLES);
  const unpauseTypeRoles = getEnvVarOrDefault(
    "LINETH_ROLLUP_UNPAUSE_TYPES_ROLES",
    LINETH_ROLLUP_V6_UNPAUSE_TYPES_ROLES,
  );
  const defaultRoleAddresses = generateRoleAssignments(LINETH_ROLLUP_V6_ROLES, linethRollupSecurityCouncil, [
    { role: OPERATOR_ROLE, addresses: linethRollupOperators },
  ]);
  const roleAddresses = getEnvVarOrDefault("LINETH_ROLLUP_ROLE_ADDRESSES", defaultRoleAddresses);

  const verifierArtifacts = loadArtifactFromDirectory(
    path.join(import.meta.dirname, "./dynamic-artifacts"),
    verifierName,
  );

  const provider = new ethers.JsonRpcProvider(process.env.RPC_URL);

  const wallet = new ethers.Wallet(process.env.DEPLOYER_PRIVATE_KEY!, provider);

  const { gasPrice } = await get1559Fees(provider);

  const walletNonce = await getDeployNonceFromEnv(wallet, "L1_NONCE");

  const [verifier, linethRollupImplementation, proxyAdmin] = await Promise.all([
    deployContractFromArtifacts(verifierName, verifierArtifacts.abi, verifierArtifacts.bytecode, wallet, {
      nonce: walletNonce,
      gasPrice,
    }),
    deployContractFromArtifacts(linethRollupImplementationName, LinethRollupV6Abi, LinethRollupV6Bytecode, wallet, {
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

  const initializer = getInitializerData(LinethRollupV6Abi, "initialize", [
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
