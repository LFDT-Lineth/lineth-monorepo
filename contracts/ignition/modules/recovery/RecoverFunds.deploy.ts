import { requireAddressFromRegistryOrEnv } from "../../../common/helpers/readAddress.js";

import type { DeployCompanion, ParameterContext } from "../../../hardhat-plugins/deploy/types.js";
import type { ModuleParameters } from "@nomicfoundation/ignition-core";

/** `10_deploy_RecoverFunds.ts`. */
export function recoverFundsParameters(context: ParameterContext): ModuleParameters {
  const { networkName } = context;
  const recoverFundsSecurityCouncil = requireAddressFromRegistryOrEnv(
    networkName,
    "L1_SECURITY_COUNCIL",
    "L1_SECURITY_COUNCIL",
  );
  const recoverFundsExecutorAddress = requireAddressFromRegistryOrEnv(
    networkName,
    "RECOVERFUNDS_EXECUTOR_ADDRESS",
    "RECOVERFUNDS_EXECUTOR_ADDRESS",
  );
  return { securityCouncil: recoverFundsSecurityCouncil, executor: recoverFundsExecutorAddress };
}

export const parameters: NonNullable<DeployCompanion["parameters"]> = async (context) => ({
  RecoverFunds: recoverFundsParameters(context),
});
