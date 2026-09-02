import * as dotenv from "dotenv";
import { ethers } from "ethers";
import path from "path";

import { abi as ValidiumV1Abi, bytecode as ValidiumV1Bytecode } from "./dynamic-artifacts/ValidiumV1.json";
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
  VALIDIUM_PAUSE_TYPES_ROLES,
  VALIDIUM_ROLES,
  VALIDIUM_UNPAUSE_TYPES_ROLES,
  OPERATOR_ROLE,
} from "../common/constants";
import { ADDRESS_ZERO } from "../common/constants/general";
import {
  deployContractFromArtifacts,
  getDeployNonceFromEnv,
  getInitializerData,
  loadArtifactFromDirectory,
} from "../common/helpers/deployments";
import { getEnvVarOrDefault, getRequiredEnvVar } from "../common/helpers/environment";
import { getDeploymentNetworkName, requireAddressFromRegistryOrEnv } from "../common/helpers/readAddress";
import { generateRoleAssignments } from "../common/helpers/roles";
import { get1559Fees } from "../scripts/utils";

dotenv.config();

async function main() {
  const networkName = getDeploymentNetworkName();
  const verifierName = getRequiredEnvVar("VERIFIER_CONTRACT_NAME");
  const validiumInitialStateRootHash = getRequiredEnvVar("INITIAL_L2_STATE_ROOT_HASH");
  const validiumInitialL2BlockNumber = getRequiredEnvVar("INITIAL_L2_BLOCK_NUMBER");
  const validiumSecurityCouncil = requireAddressFromRegistryOrEnv(
    networkName,
    "L1_SECURITY_COUNCIL",
    "L1_SECURITY_COUNCIL",
  );
  const validiumOperators = getRequiredEnvVar("VALIDIUM_OPERATORS").split(",");
  const validiumRateLimitPeriodInSeconds = getRequiredEnvVar("VALIDIUM_RATE_LIMIT_PERIOD");
  const validiumRateLimitAmountInWei = getRequiredEnvVar("VALIDIUM_RATE_LIMIT_AMOUNT");
  const validiumGenesisTimestamp = getRequiredEnvVar("L2_GENESIS_TIMESTAMP");
  const validiumName = "Validium";
  const validiumImplementationName = "Validium";

  const pauseTypeRoles = getEnvVarOrDefault("VALIDIUM_PAUSE_TYPES_ROLES", VALIDIUM_PAUSE_TYPES_ROLES);
  const unpauseTypeRoles = getEnvVarOrDefault("VALIDIUM_UNPAUSE_TYPES_ROLES", VALIDIUM_UNPAUSE_TYPES_ROLES);
  const defaultRoleAddresses = generateRoleAssignments(VALIDIUM_ROLES, validiumSecurityCouncil, [
    { role: OPERATOR_ROLE, addresses: validiumOperators },
  ]);
  const roleAddresses = getEnvVarOrDefault("VALIDIUM_ROLE_ADDRESSES", defaultRoleAddresses);

  const verifierArtifacts = loadArtifactFromDirectory(path.join(__dirname, "./dynamic-artifacts"), verifierName);

  const provider = new ethers.JsonRpcProvider(process.env.RPC_URL);

  const wallet = new ethers.Wallet(process.env.DEPLOYER_PRIVATE_KEY!, provider);

  const { gasPrice } = await get1559Fees(provider);

  const walletNonce = await getDeployNonceFromEnv(wallet, "L1_NONCE");

  const [verifier, validiumImplementation, proxyAdmin] = await Promise.all([
    deployContractFromArtifacts(verifierName, verifierArtifacts.abi, verifierArtifacts.bytecode, wallet, {
      nonce: walletNonce,
      gasPrice,
    }),
    deployContractFromArtifacts(validiumImplementationName, ValidiumV1Abi, ValidiumV1Bytecode, wallet, {
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
  const validiumImplementationAddress = await validiumImplementation.getAddress();

  const initializer = getInitializerData(ValidiumV1Abi, "initialize", [
    {
      initialStateRootHash: validiumInitialStateRootHash,
      initialL2BlockNumber: validiumInitialL2BlockNumber,
      genesisTimestamp: validiumGenesisTimestamp,
      defaultVerifier: verifierAddress,
      rateLimitPeriodInSeconds: validiumRateLimitPeriodInSeconds,
      rateLimitAmountInWei: validiumRateLimitAmountInWei,
      roleAddresses,
      pauseTypeRoles,
      unpauseTypeRoles,
      defaultAdmin: validiumSecurityCouncil,
      shnarfProvider: ADDRESS_ZERO,
    },
  ]);

  await deployContractFromArtifacts(
    validiumName,
    TransparentUpgradeableProxyAbi,
    TransparentUpgradeableProxyBytecode,
    wallet,
    validiumImplementationAddress,
    proxyAdminAddress,
    initializer,
    { gasPrice },
  );
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
