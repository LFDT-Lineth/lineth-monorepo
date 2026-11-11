import {
  L1_L2_MESSAGE_SETTER_ROLE,
  L2_MESSAGE_SERVICE_PAUSE_TYPES_ROLES,
  L2_MESSAGE_SERVICE_ROLES,
  L2_MESSAGE_SERVICE_UNPAUSE_TYPES_ROLES,
} from "../../../common/constants/index.js";
import { getEnvVarOrDefault, getRequiredEnvVar } from "../../../common/helpers/environment.js";
import { requireAddressFromRegistryOrEnv } from "../../../common/helpers/readAddress.js";
import { generateRoleAssignments } from "../../../common/helpers/roles.js";
import { toSolidityParameter, toUint } from "../lib/parameters.js";

import type { DeployCompanion, ParameterContext } from "../../../hardhat-plugins/deploy/types.js";
import type { ModuleParameters } from "@nomicfoundation/ignition-core";

/** `04_deploy_L2MessageService.ts`. */
export function l2MessageServiceParameters(context: ParameterContext): ModuleParameters {
  const { networkName } = context;
  const l2MessageServiceSecurityCouncil = requireAddressFromRegistryOrEnv(
    networkName,
    "L2_SECURITY_COUNCIL",
    "L2_SECURITY_COUNCIL",
  );
  const l2MessageServiceL1L2MessageSetter = requireAddressFromRegistryOrEnv(
    networkName,
    "L2_MESSAGE_SERVICE_L1L2_MESSAGE_SETTER",
    "L2_MESSAGE_SERVICE_L1L2_MESSAGE_SETTER",
  );
  const l2MessageServiceRateLimitPeriod = getRequiredEnvVar("L2_MESSAGE_SERVICE_RATE_LIMIT_PERIOD");
  const l2MessageServiceRateLimitAmount = getRequiredEnvVar("L2_MESSAGE_SERVICE_RATE_LIMIT_AMOUNT");

  const pauseTypeRoles = getEnvVarOrDefault(
    "L2_MESSAGE_SERVICE_PAUSE_TYPES_ROLES",
    L2_MESSAGE_SERVICE_PAUSE_TYPES_ROLES,
  );
  const unpauseTypeRoles = getEnvVarOrDefault(
    "L2_MESSAGE_SERVICE_UNPAUSE_TYPES_ROLES",
    L2_MESSAGE_SERVICE_UNPAUSE_TYPES_ROLES,
  );
  const defaultRoleAddresses = generateRoleAssignments(L2_MESSAGE_SERVICE_ROLES, l2MessageServiceSecurityCouncil, [
    { role: L1_L2_MESSAGE_SETTER_ROLE, addresses: [l2MessageServiceL1L2MessageSetter] },
  ]);
  const roleAddresses = getEnvVarOrDefault("L2_MESSAGE_SERVICE_ROLE_ADDRESSES", defaultRoleAddresses);

  return {
    rateLimitPeriod: toUint("L2_MESSAGE_SERVICE_RATE_LIMIT_PERIOD", l2MessageServiceRateLimitPeriod),
    rateLimitAmount: toUint("L2_MESSAGE_SERVICE_RATE_LIMIT_AMOUNT", l2MessageServiceRateLimitAmount),
    securityCouncil: l2MessageServiceSecurityCouncil,
    roleAddresses: toSolidityParameter(roleAddresses, "L2_MESSAGE_SERVICE_ROLE_ADDRESSES"),
    pauseTypeRoles: toSolidityParameter(pauseTypeRoles, "L2_MESSAGE_SERVICE_PAUSE_TYPES_ROLES"),
    unpauseTypeRoles: toSolidityParameter(unpauseTypeRoles, "L2_MESSAGE_SERVICE_UNPAUSE_TYPES_ROLES"),
  };
}

export const parameters: NonNullable<DeployCompanion["parameters"]> = async (context) => ({
  L2MessageService: l2MessageServiceParameters(context),
});
