import * as dotenv from "dotenv";
import { ethers } from "ethers";
import fs from "fs";
import path from "path";

import { abi as LineaRollupV8Abi, bytecode as LineaRollupV8Bytecode } from "./dynamic-artifacts/LineaRollupV8.json";
import {
  contractName as AddressFilterContractName,
  abi as AddressFilterAbi,
  bytecode as AddressFilterBytecode,
} from "./static-artifacts/AddressFilter.json";
import {
  abi as ForcedTransactionGatewayAbi,
  bytecode as ForcedTransactionGatewayBytecode,
} from "./static-artifacts/ForcedTransactionGateway.json";
import {
  contractName as MimcAddressContractName,
  abi as MimcAddressAbi,
  bytecode as MimcAddressFilterBytecode,
} from "./static-artifacts/Mimc.json";
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
  LINEA_ROLLUP_V8_PAUSE_TYPES_ROLES,
  LINEA_ROLLUP_V8_UNPAUSE_TYPES_ROLES,
  LINEA_ROLLUP_V8_ROLES,
  OPERATOR_ROLE,
  YIELD_PROVIDER_STAKING_ROLE,
  ADDRESS_ZERO,
  PRECOMPILES_ADDRESSES,
  FORCED_TRANSACTION_SENDER_ROLE,
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
  const lineaRollupInitialStateRootHash = getRequiredEnvVar("INITIAL_L2_STATE_ROOT_HASH");
  const lineaRollupInitialL2BlockNumber = getRequiredEnvVar("INITIAL_L2_BLOCK_NUMBER");
  const lineaRollupSecurityCouncil = getRequiredEnvVar("L1_SECURITY_COUNCIL");
  const lineaRollupOperators = getRequiredEnvVar("LINEA_ROLLUP_OPERATORS").split(",");
  const lineaRollupRateLimitPeriodInSeconds = getRequiredEnvVar("LINEA_ROLLUP_RATE_LIMIT_PERIOD");
  const lineaRollupRateLimitAmountInWei = getRequiredEnvVar("LINEA_ROLLUP_RATE_LIMIT_AMOUNT");
  const lineaRollupGenesisTimestamp = getRequiredEnvVar("L2_GENESIS_TIMESTAMP");

  // Forced Transaction Gateway
  const destinationChainId = getRequiredEnvVar("FORCED_TRANSACTION_GATEWAY_L2_CHAIN_ID");
  const l2BlockBuffer = getRequiredEnvVar("FORCED_TRANSACTION_GATEWAY_L2_BLOCK_BUFFER");
  const maxGasLimit = getRequiredEnvVar("FORCED_TRANSACTION_GATEWAY_MAX_GAS_LIMIT");
  const maxInputLengthBuffer = getRequiredEnvVar("FORCED_TRANSACTION_GATEWAY_MAX_INPUT_LENGTH_BUFFER");

  const l2BlockDurationSeconds = getRequiredEnvVar("FORCED_TRANSACTION_L2_BLOCK_DURATION_SECONDS");
  const blockNumberDeadlineBuffer = getRequiredEnvVar("FORCED_TRANSACTION_BLOCK_NUMBER_DEADLINE_BUFFER");

  const multiCallAddress = "0xcA11bde05977b3631167028862bE2a173976CA11";
  const lineaRollupName = "LineaRollupV8";
  const lineaRollupImplementationName = "LineaRollupV8Implementation";
  const forcedTransactionGatewayName = "ForcedTransactionGateway";

  const pauseTypeRoles = getEnvVarOrDefault("LINEA_ROLLUP_PAUSE_TYPE_ROLES", LINEA_ROLLUP_V8_PAUSE_TYPES_ROLES);
  const unpauseTypeRoles = getEnvVarOrDefault("LINEA_ROLLUP_UNPAUSE_TYPE_ROLES", LINEA_ROLLUP_V8_UNPAUSE_TYPES_ROLES);

  const securityCouncilPrivateKey = getRequiredEnvVar("SECURITY_COUNCIL_PRIVATE_KEY");

  // Use random hardcoded address until we introduce YieldManager E2E tests
  const automationServiceAddress = "0x3A9f0c2b8e7D4F6e1b5a9C2e0Fd7a4B6C8e9F1A2";
  const defaultRoleAddresses = [
    ...generateRoleAssignments(LINEA_ROLLUP_V8_ROLES, lineaRollupSecurityCouncil, [
      { role: OPERATOR_ROLE, addresses: lineaRollupOperators },
    ]),
    { role: YIELD_PROVIDER_STAKING_ROLE, addressWithRole: automationServiceAddress },
  ];
  const roleAddresses = getEnvVarOrDefault("LINEA_ROLLUP_ROLE_ADDRESSES", defaultRoleAddresses);

  const verifierArtifacts = findContractArtifacts(path.join(__dirname, "./dynamic-artifacts"), verifierName);

  const provider = new ethers.JsonRpcProvider(process.env.RPC_URL);

  const wallet = new ethers.Wallet(process.env.DEPLOYER_PRIVATE_KEY!, provider);

  const { gasPrice } = await get1559Fees(provider);

  let walletNonce;

  if (!process.env.L1_NONCE) {
    walletNonce = await wallet.getNonce();
  } else {
    walletNonce = parseInt(process.env.L1_NONCE);
  }

  const [verifier, lineaRollupImplementation, proxyAdmin, addressFilter] = await Promise.all([
    deployContractFromArtifacts(verifierName, verifierArtifacts.abi, verifierArtifacts.bytecode, wallet, {
      nonce: walletNonce,
      gasPrice,
    }),
    deployContractFromArtifacts(lineaRollupImplementationName, LineaRollupV8Abi, LineaRollupV8Bytecode, wallet, {
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
      lineaRollupSecurityCouncil,
      PRECOMPILES_ADDRESSES,
      {
        nonce: walletNonce + 3,
        gasPrice,
      },
    ),
  ]);

  const [proxyAdminAddress, verifierAddress, lineaRollupImplementationAddress, addressFilterAddress] =
    await Promise.all([
      proxyAdmin.getAddress(),
      verifier.getAddress(),
      lineaRollupImplementation.getAddress(),
      addressFilter.getAddress(),
    ]);

  const initializer = getInitializerData(LineaRollupV8Abi, "initialize", [
    {
      initialStateRootHash: lineaRollupInitialStateRootHash,
      initialL2BlockNumber: lineaRollupInitialL2BlockNumber,
      genesisTimestamp: lineaRollupGenesisTimestamp,
      defaultVerifier: verifierAddress,
      rateLimitPeriodInSeconds: lineaRollupRateLimitPeriodInSeconds,
      rateLimitAmountInWei: lineaRollupRateLimitAmountInWei,
      roleAddresses,
      pauseTypeRoles,
      unpauseTypeRoles,
      defaultAdmin: lineaRollupSecurityCouncil,
      shnarfProvider: ADDRESS_ZERO,
      addressFilter: addressFilterAddress,
    },
    // Liveness recovery operator
    multiCallAddress,
    // Use random hardcoded address temporarily until we introduce YieldManager to E2E tests
    "0xB7De4A2cf9E1c6a0B5f8d3e7a9C4B1a2e6d0f5C8",
  ]);

  const [lineaRollupContract, mimc] = await Promise.all([
    deployContractFromArtifacts(
      lineaRollupName,
      TransparentUpgradeableProxyAbi,
      TransparentUpgradeableProxyBytecode,
      wallet,
      lineaRollupImplementationAddress,
      proxyAdminAddress,
      initializer,
      {
        nonce: walletNonce + 4,
        gasPrice,
      },
    ),
    deployContractFromArtifacts(MimcAddressContractName, MimcAddressAbi, MimcAddressFilterBytecode, wallet, {
      nonce: walletNonce + 5,
      gasPrice,
    }),
  ]);

  const [lineaRollupAddress, mimcAddress] = await Promise.all([lineaRollupContract.getAddress(), mimc.getAddress()]);

  const args = [
    lineaRollupAddress,
    destinationChainId,
    l2BlockBuffer,
    maxGasLimit,
    maxInputLengthBuffer,
    lineaRollupSecurityCouncil,
    addressFilterAddress,
    l2BlockDurationSeconds,
    blockNumberDeadlineBuffer,
  ];

  const forcedTransactionGateway = await deployContractFromArtifacts(
    forcedTransactionGatewayName,
    ForcedTransactionGatewayAbi,
    ForcedTransactionGatewayBytecode,
    wallet,
    { libraries: { "src/libraries/Mimc.sol:Mimc": mimcAddress } },
    ...args,
    {
      nonce: walletNonce + 6,
      gasPrice,
    },
  );

  const forcedTransactionGatewayAddress = await forcedTransactionGateway.getAddress();
  const securityCouncilWallet = new ethers.Wallet(securityCouncilPrivateKey, provider);
  const lineaRollup = new ethers.Contract(lineaRollupAddress, LineaRollupV8Abi, securityCouncilWallet);

  console.log(
    `Granting FORCED_TRANSACTION_SENDER_ROLE to ForcedTransactionGateway at ${forcedTransactionGatewayAddress}...`,
  );
  const grantRoleTx = await lineaRollup.grantRole(FORCED_TRANSACTION_SENDER_ROLE, forcedTransactionGatewayAddress, {
    gasPrice,
  });
  await grantRoleTx.wait();
  console.log(`FORCED_TRANSACTION_SENDER_ROLE granted to ForcedTransactionGateway`);
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
