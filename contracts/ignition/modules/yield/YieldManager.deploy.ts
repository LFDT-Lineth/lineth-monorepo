import {
  GI_FIRST_VALIDATOR,
  GI_PENDING_PARTIAL_WITHDRAWALS_ROOT,
  YIELD_MANAGER_OPERATOR_ROLES,
  YIELD_MANAGER_PAUSE_TYPES_ROLES,
  YIELD_MANAGER_SECURITY_COUNCIL_ROLES,
  YIELD_MANAGER_UNPAUSE_TYPES_ROLES,
} from "../../../common/constants/index.js";
import { getEnvVarOrDefault, getRequiredEnvVar } from "../../../common/helpers/environment.js";
import { requireAddressFromRegistryOrEnv } from "../../../common/helpers/readAddress.js";
import { generateRoleAssignments } from "../../../common/helpers/roles.js";
import { toSolidityParameter } from "../lib/parameters.js";

import type { DeployCompanion, ParameterContext } from "../../../hardhat-plugins/deploy/types.js";
import type { ModuleParameters } from "@nomicfoundation/ignition-core";

/** `21_deploy_YieldManager.ts` and `21_deploy_YieldManagerArtifacts.ts` (same inputs). */
export function yieldManagerParameters(context: ParameterContext): ModuleParameters {
  const { networkName } = context;
  const linethRollupAddress = requireAddressFromRegistryOrEnv(networkName, "LinethRollup", "LINETH_ROLLUP_ADDRESS");
  const linethRollupSecurityCouncil = requireAddressFromRegistryOrEnv(
    networkName,
    "L1_SECURITY_COUNCIL",
    "L1_SECURITY_COUNCIL",
  );
  const nativeYieldAutomationServiceAddress = requireAddressFromRegistryOrEnv(
    networkName,
    "NATIVE_YIELD_AUTOMATION_SERVICE_ADDRESS",
    "NATIVE_YIELD_AUTOMATION_SERVICE_ADDRESS",
  );
  const vaultHub = requireAddressFromRegistryOrEnv(networkName, "VAULT_HUB", "VAULT_HUB");
  const vaultFactory = requireAddressFromRegistryOrEnv(networkName, "VAULT_FACTORY", "VAULT_FACTORY");
  const steth = requireAddressFromRegistryOrEnv(networkName, "STETH", "STETH");
  const initialMinimumWithdrawalReservePercentageBps = parseInt(
    getRequiredEnvVar("MINIMUM_WITHDRAWAL_RESERVE_PERCENTAGE_BPS"),
  );
  const initialTargetWithdrawalReservePercentageBps = parseInt(
    getRequiredEnvVar("TARGET_WITHDRAWAL_RESERVE_PERCENTAGE_BPS"),
  );
  const initialMinimumWithdrawalReserveAmount = BigInt(getRequiredEnvVar("MINIMUM_WITHDRAWAL_RESERVE_AMOUNT"));
  const initialTargetWithdrawalReserveAmount = BigInt(getRequiredEnvVar("TARGET_WITHDRAWAL_RESERVE_AMOUNT"));
  const gIFirstValidator = getEnvVarOrDefault("GI_FIRST_VALIDATOR", GI_FIRST_VALIDATOR);
  const gIPendingPartialWithdrawalsRoot = getEnvVarOrDefault(
    "GI_PENDING_PARTIAL_WITHDRAWALS_ROOT",
    GI_PENDING_PARTIAL_WITHDRAWALS_ROOT,
  );
  const verifierAdmin = getEnvVarOrDefault("VALIDATOR_CONTAINER_PROOF_VERIFIER_ADMIN", linethRollupSecurityCouncil);

  const securityCouncilRoles = generateRoleAssignments(
    YIELD_MANAGER_SECURITY_COUNCIL_ROLES,
    linethRollupSecurityCouncil,
    [],
  );
  const automationServiceRoles = generateRoleAssignments(
    YIELD_MANAGER_OPERATOR_ROLES,
    nativeYieldAutomationServiceAddress,
    [],
  );
  const roleAddresses = [...securityCouncilRoles, ...automationServiceRoles];

  const pauseTypeRoles = getEnvVarOrDefault("YIELD_MANAGER_PAUSE_TYPES_ROLES", YIELD_MANAGER_PAUSE_TYPES_ROLES);
  const unpauseTypeRoles = getEnvVarOrDefault("YIELD_MANAGER_UNPAUSE_TYPES_ROLES", YIELD_MANAGER_UNPAUSE_TYPES_ROLES);

  return {
    linethRollup: linethRollupAddress,
    yieldManagerInitializationData: {
      pauseTypeRoles: toSolidityParameter(pauseTypeRoles, "YIELD_MANAGER_PAUSE_TYPES_ROLES"),
      unpauseTypeRoles: toSolidityParameter(unpauseTypeRoles, "YIELD_MANAGER_UNPAUSE_TYPES_ROLES"),
      roleAddresses,
      initialL2YieldRecipients: [],
      defaultAdmin: linethRollupSecurityCouncil,
      initialMinimumWithdrawalReservePercentageBps,
      initialTargetWithdrawalReservePercentageBps,
      initialMinimumWithdrawalReserveAmount,
      initialTargetWithdrawalReserveAmount,
    },
    verifierAdmin: toSolidityParameter(verifierAdmin, "VALIDATOR_CONTAINER_PROOF_VERIFIER_ADMIN"),
    gIFirstValidator: toSolidityParameter(gIFirstValidator, "GI_FIRST_VALIDATOR"),
    gIPendingPartialWithdrawalsRoot: toSolidityParameter(
      gIPendingPartialWithdrawalsRoot,
      "GI_PENDING_PARTIAL_WITHDRAWALS_ROOT",
    ),
    vaultHub,
    vaultFactory,
    steth,
  };
}

export const parameters: NonNullable<DeployCompanion["parameters"]> = async (context) => ({
  YieldManager: yieldManagerParameters(context),
});
