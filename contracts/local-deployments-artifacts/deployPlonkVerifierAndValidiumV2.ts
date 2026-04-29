import * as dotenv from "dotenv";
import { ethers } from "ethers";
import fs from "fs";
import path from "path";

import { abi as ValidiumV2Abi, bytecode as ValidiumV2Bytecode } from "./dynamic-artifacts/ValidiumV2.json";
import {
  contractName as AddressFilterContractName,
  abi as AddressFilterAbi,
  bytecode as AddressFilterBytecode,
} from "./static-artifacts/AddressFilter.json";
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
  VALIDIUM_UNPAUSE_TYPES_ROLES,
  VALIDIUM_ROLES,
  OPERATOR_ROLE,
  YIELD_PROVIDER_STAKING_ROLE,
  ADDRESS_ZERO,
  PRECOMPILES_ADDRESSES,
} from "../common/constants";
import { deployContractFromArtifacts, getInitializerData } from "../common/helpers/deployments";
import { getEnvVarOrDefault, getRequiredEnvVar } from "../common/helpers/environment";
import { generateRoleAssignments } from "../common/helpers/roles";
import { get1559Fees } from "../scripts/utils";

dotenv.config();

function findContractArtifacts(
  folderPath: string,
  contractName: string,
): { abi: ethers.InterfaceAbi; bytecode: ethers.BytesLike } {
  const files = fs.readdirSync(folderPath);

  const foundFile = files.find((file) => file === `${contractName}.json`);

  if (!foundFile) {
    // Throw an error if the file is not found
    throw new Error(`Contract "${contractName}" not found in folder "${folderPath}"`);
  }

  // Construct the full file path
  const filePath = path.join(folderPath, foundFile);

  // Read the file content
  const fileContent = fs.readFileSync(filePath, "utf-8").trim();
  const parsedContent = JSON.parse(fileContent);
  return parsedContent;
}

async function main() {
  const verifierName = getRequiredEnvVar("VERIFIER_CONTRACT_NAME");
  const validiumInitialStateRootHash = getRequiredEnvVar("VALIDIUM_INITIAL_STATE_ROOT_HASH");
  const validiumInitialL2BlockNumber = getRequiredEnvVar("VALIDIUM_INITIAL_L2_BLOCK_NUMBER");
  const validiumSecurityCouncil = getRequiredEnvVar("VALIDIUM_SECURITY_COUNCIL");
  const validiumOperators = getRequiredEnvVar("VALIDIUM_OPERATORS").split(",");
  const validiumRateLimitPeriodInSeconds = getRequiredEnvVar("VALIDIUM_RATE_LIMIT_PERIOD");
  const validiumRateLimitAmountInWei = getRequiredEnvVar("VALIDIUM_RATE_LIMIT_AMOUNT");
  const validiumGenesisTimestamp = getRequiredEnvVar("VALIDIUM_GENESIS_TIMESTAMP");

  const validiumName = "Validium";
  const validiumImplementationName = "ValidiumV2Implementation";

  const pauseTypeRoles = getEnvVarOrDefault("VALIDIUM_PAUSE_TYPES_ROLES", VALIDIUM_PAUSE_TYPES_ROLES);
  const unpauseTypeRoles = getEnvVarOrDefault("VALIDIUM_UNPAUSE_TYPES_ROLES", VALIDIUM_UNPAUSE_TYPES_ROLES);

  // Use random hardcoded address until we introduce YieldManager E2E tests
  const automationServiceAddress = "0x3A9f0c2b8e7D4F6e1b5a9C2e0Fd7a4B6C8e9F1A2";
  const defaultRoleAddresses = [
    ...generateRoleAssignments(VALIDIUM_ROLES, validiumSecurityCouncil, [
      { role: OPERATOR_ROLE, addresses: validiumOperators },
    ]),
    { role: YIELD_PROVIDER_STAKING_ROLE, addressWithRole: automationServiceAddress },
  ];
  const roleAddresses = getEnvVarOrDefault("VALIDIUM_ROLE_ADDRESSES", defaultRoleAddresses);

  const verifierArtifacts = findContractArtifacts(path.join(__dirname, "./dynamic-artifacts"), verifierName);

  const provider = new ethers.JsonRpcProvider(process.env.RPC_URL);

  const wallet = new ethers.Wallet(process.env.PRIVATE_KEY!, provider);

  const { gasPrice } = await get1559Fees(provider);

  let walletNonce;

  if (!process.env.L1_NONCE) {
    walletNonce = await wallet.getNonce();
  } else {
    walletNonce = parseInt(process.env.L1_NONCE);
  }

  const [verifier, validiumImplementation, proxyAdmin, addressFilter] = await Promise.all([
    deployContractFromArtifacts(verifierName, verifierArtifacts.abi, verifierArtifacts.bytecode, wallet, {
      nonce: walletNonce,
      gasPrice,
    }),
    deployContractFromArtifacts(validiumImplementationName, ValidiumV2Abi, ValidiumV2Bytecode, wallet, {
      nonce: walletNonce + 1,
      gasPrice,
    }),
    deployContractFromArtifacts(ProxyAdminContractName, ProxyAdminAbi, ProxyAdminBytecode, wallet, {
      nonce: walletNonce + 2,
      gasPrice,
    }),
    deployContractFromArtifacts(
      AddressFilterContractName,
      AddressFilterAbi,
      AddressFilterBytecode,
      wallet,
      validiumSecurityCouncil,
      PRECOMPILES_ADDRESSES,
      {
        nonce: walletNonce + 3,
        gasPrice,
      },
    ),
  ]);

  const [proxyAdminAddress, verifierAddress, validiumImplementationAddress, addressFilterAddress] = await Promise.all([
    proxyAdmin.getAddress(),
    verifier.getAddress(),
    validiumImplementation.getAddress(),
    addressFilter.getAddress(),
  ]);

  const initializer = getInitializerData(ValidiumV2Abi, "initialize", [
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
      addressFilter: addressFilterAddress,
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
    {
      nonce: walletNonce + 4,
      gasPrice,
    },
  );
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
