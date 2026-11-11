import { requireAddressFromRegistryOrEnv } from "../../../common/helpers/readAddress.js";

import type { DeployCompanion, ParameterContext } from "../../../hardhat-plugins/deploy/types.js";
import type { ModuleParameters } from "@nomicfoundation/ignition-core";

/** `22_deploy_YieldManagerUpgrade.ts`. `proxy` is the existing YieldManager proxy. */
export function yieldManagerUpgradeParameters(context: ParameterContext): ModuleParameters {
  const linethRollupAddress = requireAddressFromRegistryOrEnv(
    context.networkName,
    "LinethRollup",
    "LINETH_ROLLUP_ADDRESS",
  );
  const proxy = requireAddressFromRegistryOrEnv(context.networkName, "YieldManager", "YIELD_MANAGER_ADDRESS");
  return { proxy, linethRollup: linethRollupAddress };
}

export const parameters: NonNullable<DeployCompanion["parameters"]> = async (context) => ({
  YieldManagerUpgrade: yieldManagerUpgradeParameters(context),
});
