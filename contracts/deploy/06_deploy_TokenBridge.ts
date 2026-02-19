import { ethers, network, upgrades } from "hardhat";
import { DeployFunction } from "hardhat-deploy/types";
import {
  TOKEN_BRIDGE_PAUSE_TYPES_ROLES,
  TOKEN_BRIDGE_ROLES,
  TOKEN_BRIDGE_UNPAUSE_TYPES_ROLES,
} from "../common/constants";
import {
  generateRoleAssignments,
  getEnvVarOrDefault,
  tryVerifyContract,
  getRequiredEnvVar,
  LogContractDeployment,
} from "../common/helpers";
import { get1559Fees } from "../scripts/utils";

const func: DeployFunction = async function () {
  const contractName = "TokenBridge";

  const l2MessageServiceAddress = getRequiredEnvVar("L2_MESSAGE_SERVICE_ADDRESS");
  const lineaRollupAddress = getRequiredEnvVar("LINEA_ROLLUP_ADDRESS");
  const remoteChainId = getRequiredEnvVar("REMOTE_CHAIN_ID");
  const pauseTypeRoles = getEnvVarOrDefault("TOKEN_BRIDGE_PAUSE_TYPES_ROLES", TOKEN_BRIDGE_PAUSE_TYPES_ROLES);
  const unpauseTypeRoles = getEnvVarOrDefault("TOKEN_BRIDGE_UNPAUSE_TYPES_ROLES", TOKEN_BRIDGE_UNPAUSE_TYPES_ROLES);
  const remoteSender = getRequiredEnvVar("REMOTE_SENDER_ADDRESS");

  let securityCouncilAddress;

  const chainId = (await ethers.provider.getNetwork()).chainId;

  console.log(`Current network's chainId is ${chainId}. Remote (target) network's chainId is ${remoteChainId}`);

  let deployingChainMessageService = l2MessageServiceAddress;
  let reservedAddresses = process.env.L2_RESERVED_TOKEN_ADDRESSES
    ? process.env.L2_RESERVED_TOKEN_ADDRESSES.split(",")
    : [];

  if (process.env.TOKEN_BRIDGE_L1 === "true") {
    securityCouncilAddress = getRequiredEnvVar("L1_SECURITY_COUNCIL");
    console.log(
      `TOKEN_BRIDGE_L1=${process.env.TOKEN_BRIDGE_L1}. Deploying TokenBridge on L1, using L1_RESERVED_TOKEN_ADDRESSES environment variable`,
    );
    deployingChainMessageService = lineaRollupAddress;
    reservedAddresses = process.env.L1_RESERVED_TOKEN_ADDRESSES
      ? process.env.L1_RESERVED_TOKEN_ADDRESSES.split(",")
      : [];
  } else {
    securityCouncilAddress = getRequiredEnvVar("L2_SECURITY_COUNCIL");
    console.log(
      `TOKEN_BRIDGE_L1=${process.env.TOKEN_BRIDGE_L1}. Deploying TokenBridge on L2, using L2_RESERVED_TOKEN_ADDRESSES environment variable`,
    );
  }

  const defaultRoleAddresses = generateRoleAssignments(TOKEN_BRIDGE_ROLES, securityCouncilAddress, []);
  const roleAddresses = getEnvVarOrDefault("TOKEN_BRIDGE_ROLE_ADDRESSES", defaultRoleAddresses);

  const bridgedTokenAddress = getRequiredEnvVar("BRIDGED_TOKEN_ADDRESS");

  // Deploying TokenBridge
  const TokenBridgeFactory = await ethers.getContractFactory(contractName);

  const { maxPriorityFeePerGas, maxFeePerGas } = await get1559Fees(ethers.provider);

  const tokenBridge = await upgrades.deployProxy(
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

  await LogContractDeployment(contractName, tokenBridge);

  const tokenBridgeAddress = await tokenBridge.getAddress();

  if (process.env.TOKEN_BRIDGE_L1 === "true") {
    console.log(`L1 TokenBridge deployed on ${network.name}, at address: ${tokenBridgeAddress}`);
  } else {
    console.log(`L2 TokenBridge deployed on ${network.name}, at address: ${tokenBridgeAddress}`);
  }
  await tryVerifyContract(tokenBridgeAddress);
};
export default func;
func.tags = ["TokenBridge"];
