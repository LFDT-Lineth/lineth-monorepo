import {
  TOKEN_BRIDGE_PAUSE_TYPES_ROLES,
  TOKEN_BRIDGE_ROLES,
  TOKEN_BRIDGE_UNPAUSE_TYPES_ROLES,
} from "../../../common/constants/index.js";
import {
  getBooleanEnvVarOrDefault,
  getEnvVarOrDefault,
  getRequiredEnvVar,
} from "../../../common/helpers/environment.js";
import { getAddressesFromRegistryOrEnv, requireAddressFromRegistryOrEnv } from "../../../common/helpers/readAddress.js";
import { generateRoleAssignments } from "../../../common/helpers/roles.js";
import { toSolidityParameter, toUint } from "../lib/parameters.js";
import { reportTokenBridge } from "../lib/tokenBridgeReport.js";

import type { DeployCompanion, ParameterContext } from "../../../hardhat-plugins/deploy/types.js";
import type { ModuleParameters } from "@nomicfoundation/ignition-core";

/** `06_deploy_TokenBridge.ts`. `tokenBeacon` is resolved only for the standalone module (the composite deploys one). */
export function tokenBridgeParameters(
  context: ParameterContext,
  options: { includeBeacon: boolean },
): ModuleParameters {
  const { networkName, chainId } = context;
  const deployTokenBridgeOnL1 = getBooleanEnvVarOrDefault("DEPLOY_TOKEN_BRIDGE_ON_L1", false);

  const l2MessageServiceAddress = requireAddressFromRegistryOrEnv(
    networkName,
    "L2MessageService",
    "L2_MESSAGE_SERVICE_ADDRESS",
  );
  const linethRollupAddress = requireAddressFromRegistryOrEnv(networkName, "LinethRollup", "LINETH_ROLLUP_ADDRESS");
  const remoteChainId = getRequiredEnvVar("REMOTE_CHAIN_ID");
  const pauseTypeRoles = getEnvVarOrDefault("TOKEN_BRIDGE_PAUSE_TYPES_ROLES", TOKEN_BRIDGE_PAUSE_TYPES_ROLES);
  const unpauseTypeRoles = getEnvVarOrDefault("TOKEN_BRIDGE_UNPAUSE_TYPES_ROLES", TOKEN_BRIDGE_UNPAUSE_TYPES_ROLES);
  const remoteSender = requireAddressFromRegistryOrEnv(networkName, "REMOTE_SENDER_ADDRESS", "REMOTE_SENDER_ADDRESS");

  console.log(`Current network's chainId is ${chainId}. Remote (target) network's chainId is ${remoteChainId}`);

  let securityCouncilAddress: string;
  let deployingChainMessageService = l2MessageServiceAddress;
  let reservedAddresses: string[];

  if (deployTokenBridgeOnL1) {
    securityCouncilAddress = requireAddressFromRegistryOrEnv(networkName, "L1_SECURITY_COUNCIL", "L1_SECURITY_COUNCIL");
    console.log(
      `DEPLOY_TOKEN_BRIDGE_ON_L1=${process.env.DEPLOY_TOKEN_BRIDGE_ON_L1}. Deploying TokenBridge on L1, using L1_RESERVED_TOKEN_ADDRESSES from registry or env`,
    );
    deployingChainMessageService = linethRollupAddress;
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

  const parameters: ModuleParameters = {};
  if (options.includeBeacon) {
    parameters.tokenBeacon = requireAddressFromRegistryOrEnv(networkName, "BridgedToken", "BRIDGED_TOKEN_ADDRESS");
  }

  return {
    ...parameters,
    securityCouncil: securityCouncilAddress,
    messageService: deployingChainMessageService,
    sourceChainId: chainId,
    targetChainId: toUint("REMOTE_CHAIN_ID", remoteChainId),
    remoteSender,
    reservedTokens: reservedAddresses,
    roleAddresses: toSolidityParameter(roleAddresses, "TOKEN_BRIDGE_ROLE_ADDRESSES"),
    pauseTypeRoles: toSolidityParameter(pauseTypeRoles, "TOKEN_BRIDGE_PAUSE_TYPES_ROLES"),
    unpauseTypeRoles: toSolidityParameter(unpauseTypeRoles, "TOKEN_BRIDGE_UNPAUSE_TYPES_ROLES"),
  };
}

export const parameters: NonNullable<DeployCompanion["parameters"]> = async (context) => ({
  TokenBridge: tokenBridgeParameters(context, { includeBeacon: true }),
});

export const afterDeploy: NonNullable<DeployCompanion["afterDeploy"]> = (context) =>
  reportTokenBridge(context, { tokenBridge: "TokenBridge#TokenBridge" });
