import {
  OPERATOR_ROLE,
  VALIDIUM_PAUSE_TYPES_ROLES,
  VALIDIUM_ROLES,
  VALIDIUM_UNPAUSE_TYPES_ROLES,
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

/** `03_deploy_Validium.ts`. */
export function validiumParameters(context: ParameterContext): ModuleParameters {
  const { networkName } = context;
  const verifierAddress = requireAddressFromRegistryOrEnv(networkName, "PlonkVerifier", "VERIFIER_ADDRESS");
  const validiumInitialStateRootHash = getRequiredEnvVar("INITIAL_L2_STATE_ROOT_HASH");
  const validiumInitialL2BlockNumber = getRequiredEnvVar("INITIAL_L2_BLOCK_NUMBER");
  const validiumSecurityCouncil = requireAddressFromRegistryOrEnv(
    networkName,
    "L1_SECURITY_COUNCIL",
    "L1_SECURITY_COUNCIL",
  );
  const validiumOperators = requireAddressesFromRegistryOrEnv(networkName, "VALIDIUM_OPERATORS", "VALIDIUM_OPERATORS");
  const validiumRateLimitPeriodInSeconds = getRequiredEnvVar("VALIDIUM_RATE_LIMIT_PERIOD");
  const validiumRateLimitAmountInWei = getRequiredEnvVar("VALIDIUM_RATE_LIMIT_AMOUNT");
  const validiumGenesisTimestamp = getRequiredEnvVar("L2_GENESIS_TIMESTAMP");

  const pauseTypeRoles = getEnvVarOrDefault("VALIDIUM_PAUSE_TYPES_ROLES", VALIDIUM_PAUSE_TYPES_ROLES);
  const unpauseTypeRoles = getEnvVarOrDefault("VALIDIUM_UNPAUSE_TYPES_ROLES", VALIDIUM_UNPAUSE_TYPES_ROLES);
  const defaultRoleAddresses = generateRoleAssignments(VALIDIUM_ROLES, validiumSecurityCouncil, [
    { role: OPERATOR_ROLE, addresses: validiumOperators },
  ]);
  const roleAddresses = getEnvVarOrDefault("VALIDIUM_ROLE_ADDRESSES", defaultRoleAddresses);
  const addressFilter = requireAddressFromRegistryOrEnv(networkName, "AddressFilter", "LINETH_ROLLUP_ADDRESS_FILTER");

  return {
    verifier: verifierAddress,
    initialStateRootHash: validiumInitialStateRootHash,
    initialL2BlockNumber: toUint("INITIAL_L2_BLOCK_NUMBER", validiumInitialL2BlockNumber),
    genesisTimestamp: toUint("L2_GENESIS_TIMESTAMP", validiumGenesisTimestamp),
    rateLimitPeriodInSeconds: toUint("VALIDIUM_RATE_LIMIT_PERIOD", validiumRateLimitPeriodInSeconds),
    rateLimitAmountInWei: toUint("VALIDIUM_RATE_LIMIT_AMOUNT", validiumRateLimitAmountInWei),
    roleAddresses: toSolidityParameter(roleAddresses, "VALIDIUM_ROLE_ADDRESSES"),
    pauseTypeRoles: toSolidityParameter(pauseTypeRoles, "VALIDIUM_PAUSE_TYPES_ROLES"),
    unpauseTypeRoles: toSolidityParameter(unpauseTypeRoles, "VALIDIUM_UNPAUSE_TYPES_ROLES"),
    securityCouncil: validiumSecurityCouncil,
    addressFilter,
  };
}

export const parameters: NonNullable<DeployCompanion["parameters"]> = async (context) => ({
  Validium: validiumParameters(context),
});
