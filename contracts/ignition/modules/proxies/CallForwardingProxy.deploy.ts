import { requireAddressFromRegistryOrEnv } from "../../../common/helpers/readAddress.js";

import type { DeployCompanion, ParameterContext } from "../../../hardhat-plugins/deploy/types.js";
import type { ModuleParameters } from "@nomicfoundation/ignition-core";

/** `12_deploy_CallForwardingProxy.ts`. */
export function callForwardingProxyParameters(context: ParameterContext): ModuleParameters {
  // This should be the LinethRollup
  const targetAddress = requireAddressFromRegistryOrEnv(context.networkName, "LinethRollup", "LINETH_ROLLUP_ADDRESS");
  return { target: targetAddress };
}

export const parameters: NonNullable<DeployCompanion["parameters"]> = async (context) => ({
  CallForwardingProxy: callForwardingProxyParameters(context),
});
