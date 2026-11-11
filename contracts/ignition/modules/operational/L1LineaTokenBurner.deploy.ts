import { requireAddressFromRegistryOrEnv } from "../../../common/helpers/readAddress.js";

import type { DeployCompanion, ParameterContext } from "../../../hardhat-plugins/deploy/types.js";
import type { ModuleParameters } from "@nomicfoundation/ignition-core";

/** `19_deploy_L1LineaTokenBurner.ts`. */
export function l1LineaTokenBurnerParameters(context: ParameterContext): ModuleParameters {
  const messageService = requireAddressFromRegistryOrEnv(context.networkName, "LinethRollup", "LINETH_ROLLUP_ADDRESS");
  const lineaToken = requireAddressFromRegistryOrEnv(context.networkName, "LINEA_TOKEN", "LINEA_TOKEN");
  return { messageService, lineaToken };
}

export const parameters: NonNullable<DeployCompanion["parameters"]> = async (context) => ({
  L1LineaTokenBurner: l1LineaTokenBurnerParameters(context),
});
