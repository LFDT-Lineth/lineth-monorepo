import { upgrades as createUpgrades } from "@openzeppelin/hardhat-upgrades";
import hre, { network as hardhatNetwork } from "hardhat";

import {
  TOKEN_BRIDGE_PAUSE_TYPES_ROLES,
  TOKEN_BRIDGE_ROLES,
  TOKEN_BRIDGE_UNPAUSE_TYPES_ROLES,
} from "../common/constants";
import {
  generateRoleAssignments,
  getAddressesFromRegistryOrEnv,
  getBooleanEnvVarOrDefault,
  getEnvVarOrDefault,
  requireAddressFromRegistryOrEnv,
  tryVerifyContract,
  getRequiredEnvVar,
  LogContractDeployment,
} from "../common/helpers";
import { deployScript } from "../rocketh/deploy";
import {
  clearUiWorkflowStatus,
  getUiSigner,
  setUiWorkflowStatus,
  withSignerUiSession,
} from "../scripts/hardhat/signer-ui-bridge";
import { get1559Fees } from "../scripts/utils";

const hardhatConnection = await hardhatNetwork.getOrCreate();
const { ethers } = hardhatConnection;
const networkName = hardhatConnection.networkName === "default" ? "hardhat" : hardhatConnection.networkName;
const upgrades = await createUpgrades(hre, hardhatConnection);

const func = withSignerUiSession("06_deploy_TokenBridge.ts", async function () {
  const signer = await getUiSigner();
  const contractName = "TokenBridge";
  const deployTokenBridgeOnL1 = getBooleanEnvVarOrDefault("DEPLOY_TOKEN_BRIDGE_ON_L1", false);

  const l2MessageServiceAddress = requireAddressFromRegistryOrEnv(
    networkName,
    "L2MessageService",
    "L2_MESSAGE_SERVICE_ADDRESS",
  );
  const lineaRollupAddress = requireAddressFromRegistryOrEnv(networkName, "LineaRollup", "LINEA_ROLLUP_ADDRESS");
  const remoteChainId = getRequiredEnvVar("REMOTE_CHAIN_ID");
  const pauseTypeRoles = getEnvVarOrDefault("TOKEN_BRIDGE_PAUSE_TYPES_ROLES", TOKEN_BRIDGE_PAUSE_TYPES_ROLES);
  const unpauseTypeRoles = getEnvVarOrDefault("TOKEN_BRIDGE_UNPAUSE_TYPES_ROLES", TOKEN_BRIDGE_UNPAUSE_TYPES_ROLES);
  const remoteSender = requireAddressFromRegistryOrEnv(networkName, "REMOTE_SENDER_ADDRESS", "REMOTE_SENDER_ADDRESS");

  let securityCouncilAddress;

  const chainId = (await ethers.provider.getNetwork()).chainId;

  console.log(`Current network's chainId is ${chainId}. Remote (target) network's chainId is ${remoteChainId}`);

  let deployingChainMessageService = l2MessageServiceAddress;
  let reservedAddresses: string[];

  if (deployTokenBridgeOnL1) {
    securityCouncilAddress = requireAddressFromRegistryOrEnv(networkName, "L1_SECURITY_COUNCIL", "L1_SECURITY_COUNCIL");
    console.log(
      `DEPLOY_TOKEN_BRIDGE_ON_L1=${process.env.DEPLOY_TOKEN_BRIDGE_ON_L1}. Deploying TokenBridge on L1, using L1_RESERVED_TOKEN_ADDRESSES from registry or env`,
    );
    deployingChainMessageService = lineaRollupAddress;
    reservedAddresses = getAddressesFromRegistryOrEnv(
      networkName,
      "L1_RESERVED_TOKEN_ADDRESSES",
      "L1_RESERVED_TOKEN_ADDRESSES",
    );
  } else {
    securityCouncilAddress = requireAddressFromRegistryOrEnv(networkName, "L2_SECURITY_COUNCIL", "L2_SECURITY_COUNCIL");
    console.log(
      `DEPLOY_TOKEN_BRIDGE_ON_L1=${process.env.DEPLOY_TOKEN_BRIDGE_ON_L1}. Deploying TokenBridge on L2, using L2_RESERVED_TOKEN_ADDRESSES from registry or env`,
    );
    reservedAddresses = getAddressesFromRegistryOrEnv(
      networkName,
      "L2_RESERVED_TOKEN_ADDRESSES",
      "L2_RESERVED_TOKEN_ADDRESSES",
    );
  }

  const defaultRoleAddresses = generateRoleAssignments(TOKEN_BRIDGE_ROLES, securityCouncilAddress, []);
  const roleAddresses = getEnvVarOrDefault("TOKEN_BRIDGE_ROLE_ADDRESSES", defaultRoleAddresses);

  const bridgedTokenAddress = requireAddressFromRegistryOrEnv(networkName, "BridgedToken", "BRIDGED_TOKEN_ADDRESS");

  // Deploying TokenBridge
  const TokenBridgeFactory = await ethers.getContractFactory(contractName, signer);

  const { maxPriorityFeePerGas, maxFeePerGas } = await get1559Fees(ethers.provider);

  let tokenBridge: Awaited<ReturnType<typeof upgrades.deployProxy>>;
  await setUiWorkflowStatus("waiting_for_transaction_receipt", `Waiting for transaction receipt for ${contractName}.`);
  try {
    tokenBridge = await upgrades.deployProxy(
      TokenBridgeFactory,
      [
        {
          defaultAdmin: securityCouncilAddress,
          messageService: deployingChainMessageService,
          tokenBeacon: bridgedTokenAddress,
          sourceChainId: chainId,
          targetChainId: remoteChainId,
          remoteSender,
          reservedTokens: reservedAddresses,
          roleAddresses,
          pauseTypeRoles,
          unpauseTypeRoles,
        },
      ],
      { txOverrides: { maxPriorityFeePerGas: maxPriorityFeePerGas!, maxFeePerGas: maxFeePerGas! } },
    );
    await tokenBridge.waitForDeployment();
  } finally {
    await clearUiWorkflowStatus();
  }

  await LogContractDeployment(contractName, tokenBridge);

  const tokenBridgeAddress = await tokenBridge.getAddress();

  if (deployTokenBridgeOnL1) {
    console.log(`L1 TokenBridge deployed on ${networkName}, at address: ${tokenBridgeAddress}`);
  } else {
    console.log(`L2 TokenBridge deployed on ${networkName}, at address: ${tokenBridgeAddress}`);
  }
  await tryVerifyContract(tokenBridgeAddress);
});

export default deployScript(func, { tags: ["TokenBridge"] });
