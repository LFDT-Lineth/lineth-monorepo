import {
  LINETH_ROLLUP_V8_PAUSE_TYPES_ROLES,
  LINETH_ROLLUP_V8_ROLES,
  LINETH_ROLLUP_V8_UNPAUSE_TYPES_ROLES,
  OPERATOR_ROLE,
} from "../../../common/constants/index.js";
import { getEnvVarOrDefault, getRequiredEnvVar } from "../../../common/helpers/environment.js";
import {
  requireAddressesFromRegistryOrEnv,
  requireAddressFromRegistryOrEnv,
} from "../../../common/helpers/readAddress.js";
import { generateRoleAssignments } from "../../../common/helpers/roles.js";
import { toSolidityParameter, toUint } from "../lib/parameters.js";

import type { DeployCompanion, ParameterContext } from "../../../hardhat-plugins/deploy/types.js";
import type { ModuleParameters } from "@nomicfoundation/ignition-core";

/** `03_deploy_LinethRollup.ts`. `verifier` is resolved only for the standalone module (the composite deploys one). */
export function linethRollupParameters(
  context: ParameterContext,
  options: { includeVerifier: boolean },
): ModuleParameters {
  const { networkName } = context;
  const parameters: ModuleParameters = {};
  if (options.includeVerifier) {
    parameters.verifier = requireAddressFromRegistryOrEnv(networkName, "PlonkVerifier", "VERIFIER_ADDRESS");
  }
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

  const pauseTypeRoles = getEnvVarOrDefault("LINETH_ROLLUP_PAUSE_TYPES_ROLES", LINETH_ROLLUP_V8_PAUSE_TYPES_ROLES);
  const unpauseTypeRoles = getEnvVarOrDefault(
    "LINETH_ROLLUP_UNPAUSE_TYPES_ROLES",
    LINETH_ROLLUP_V8_UNPAUSE_TYPES_ROLES,
  );
  const defaultRoleAddresses = generateRoleAssignments(LINETH_ROLLUP_V8_ROLES, linethRollupSecurityCouncil, [
    { role: OPERATOR_ROLE, addresses: linethRollupOperators },
  ]);
  const roleAddresses = getEnvVarOrDefault("LINETH_ROLLUP_ROLE_ADDRESSES", defaultRoleAddresses);
  const yieldManagerAddress = requireAddressFromRegistryOrEnv(networkName, "YieldManager", "YIELD_MANAGER_ADDRESS");
  const addressFilter = requireAddressFromRegistryOrEnv(networkName, "AddressFilter", "LINETH_ROLLUP_ADDRESS_FILTER");

  return {
    ...parameters,
    initialStateRootHash: linethRollupInitialStateRootHash,
    initialL2BlockNumber: toUint("INITIAL_L2_BLOCK_NUMBER", linethRollupInitialL2BlockNumber),
    genesisTimestamp: toUint("L2_GENESIS_TIMESTAMP", linethRollupGenesisTimestamp),
    rateLimitPeriodInSeconds: toUint("LINETH_ROLLUP_RATE_LIMIT_PERIOD", linethRollupRateLimitPeriodInSeconds),
    rateLimitAmountInWei: toUint("LINETH_ROLLUP_RATE_LIMIT_AMOUNT", linethRollupRateLimitAmountInWei),
    roleAddresses: toSolidityParameter(roleAddresses, "LINETH_ROLLUP_ROLE_ADDRESSES"),
    pauseTypeRoles: toSolidityParameter(pauseTypeRoles, "LINETH_ROLLUP_PAUSE_TYPES_ROLES"),
    unpauseTypeRoles: toSolidityParameter(unpauseTypeRoles, "LINETH_ROLLUP_UNPAUSE_TYPES_ROLES"),
    securityCouncil: linethRollupSecurityCouncil,
    addressFilter,
    yieldManager: yieldManagerAddress,
  };
}

export const parameters: NonNullable<DeployCompanion["parameters"]> = async (context) => ({
  LinethRollup: linethRollupParameters(context, { includeVerifier: true }),
});
