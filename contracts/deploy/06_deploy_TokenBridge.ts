import { ethers, network, upgrades } from "hardhat";
import { HardhatRuntimeEnvironment } from "hardhat/types";
import { DeployFunction } from "hardhat-deploy/types";

import {
  TOKEN_BRIDGE_PAUSE_TYPES_ROLES,
  TOKEN_BRIDGE_ROLES,
  TOKEN_BRIDGE_UNPAUSE_TYPES_ROLES,
} from "../common/constants";
import {
  generateRoleAssignments,
  getAddressesFromRegistryOrEnv,
  getEnvVarOrDefault,
  requireAddressFromRegistryOrEnv,
  tryVerifyContract,
  getRequiredEnvVar,
  LogContractDeployment,
} from "../common/helpers";
import {
  clearUiWorkflowStatus,
  getUiSigner,
  setUiWorkflowStatus,
  withSignerUiSession,
} from "../scripts/hardhat/signer-ui-bridge";
import { get1559Fees } from "../scripts/utils";

const func: DeployFunction = withSignerUiSession(
  "06_deploy_TokenBridge.ts",
  async function (hre: HardhatRuntimeEnvironment) {
    const signer = await getUiSigner(hre);
    const contractName = "TokenBridge";

    const l2MessageServiceAddress = requireAddressFromRegistryOrEnv(
      network.name,
      "L2MessageService",
      "L2_MESSAGE_SERVICE_ADDRESS",
    );
    const lineaRollupAddress = requireAddressFromRegistryOrEnv(network.name, "LineaRollup", "LINEA_ROLLUP_ADDRESS");
    const remoteChainId = getRequiredEnvVar("REMOTE_CHAIN_ID");
    const pauseTypeRoles = getEnvVarOrDefault("TOKEN_BRIDGE_PAUSE_TYPES_ROLES", TOKEN_BRIDGE_PAUSE_TYPES_ROLES);
    const unpauseTypeRoles = getEnvVarOrDefault("TOKEN_BRIDGE_UNPAUSE_TYPES_ROLES", TOKEN_BRIDGE_UNPAUSE_TYPES_ROLES);
    const remoteSender = requireAddressFromRegistryOrEnv(
      network.name,
      "REMOTE_SENDER_ADDRESS",
      "REMOTE_SENDER_ADDRESS",
    );

    let securityCouncilAddress;

    const chainId = (await ethers.provider.getNetwork()).chainId;

    console.log(`Current network's chainId is ${chainId}. Remote (target) network's chainId is ${remoteChainId}`);

    let deployingChainMessageService = l2MessageServiceAddress;
    let reservedAddresses: string[];

    if (process.env.DEPLOY_TOKEN_BRIDGE_ON_L1 === "true") {
      securityCouncilAddress = requireAddressFromRegistryOrEnv(
        network.name,
        "L1_SECURITY_COUNCIL",
        "L1_SECURITY_COUNCIL",
      );
      console.log(
        `DEPLOY_TOKEN_BRIDGE_ON_L1=${process.env.DEPLOY_TOKEN_BRIDGE_ON_L1}. Deploying TokenBridge on L1, using L1_RESERVED_TOKEN_ADDRESSES from registry or env`,
      );
      deployingChainMessageService = lineaRollupAddress;
      reservedAddresses = getAddressesFromRegistryOrEnv(
        network.name,
        "L1_RESERVED_TOKEN_ADDRESSES",
        "L1_RESERVED_TOKEN_ADDRESSES",
      );
    } else {
      securityCouncilAddress = requireAddressFromRegistryOrEnv(
        network.name,
        "L2_SECURITY_COUNCIL",
        "L2_SECURITY_COUNCIL",
      );
      console.log(
        `DEPLOY_TOKEN_BRIDGE_ON_L1=${process.env.DEPLOY_TOKEN_BRIDGE_ON_L1}. Deploying TokenBridge on L2, using L2_RESERVED_TOKEN_ADDRESSES from registry or env`,
      );
      reservedAddresses = getAddressesFromRegistryOrEnv(
        network.name,
        "L2_RESERVED_TOKEN_ADDRESSES",
        "L2_RESERVED_TOKEN_ADDRESSES",
      );
    }

    const defaultRoleAddresses = generateRoleAssignments(TOKEN_BRIDGE_ROLES, securityCouncilAddress, []);
    const roleAddresses = getEnvVarOrDefault("TOKEN_BRIDGE_ROLE_ADDRESSES", defaultRoleAddresses);

    const bridgedTokenAddress = requireAddressFromRegistryOrEnv(network.name, "BridgedToken", "BRIDGED_TOKEN_ADDRESS");

    // Deploying TokenBridge
    const TokenBridgeFactory = await ethers.getContractFactory(contractName, signer);

    const { maxPriorityFeePerGas, maxFeePerGas } = await get1559Fees(ethers.provider);

    let tokenBridge: Awaited<ReturnType<typeof upgrades.deployProxy>>;
    await setUiWorkflowStatus(
      "waiting_for_transaction_receipt",
      `Waiting for transaction receipt for ${contractName}.`,
    );
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

    if (process.env.DEPLOY_TOKEN_BRIDGE_ON_L1 === "true") {
      console.log(`L1 TokenBridge deployed on ${network.name}, at address: ${tokenBridgeAddress}`);
    } else {
      console.log(`L2 TokenBridge deployed on ${network.name}, at address: ${tokenBridgeAddress}`);
    }
    await tryVerifyContract(tokenBridgeAddress);
  },
);
export default func;
func.tags = ["TokenBridge"];
