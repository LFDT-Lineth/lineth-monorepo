import { getRequiredEnvVar } from "../../../common/helpers/environment.js";
import { toInteger } from "../lib/parameters.js";

import type { DeployCompanion } from "../../../hardhat-plugins/deploy/types.js";
import type { ModuleParameters } from "@nomicfoundation/ignition-core";

/** `20_deploy_V3DexSwapAdapter.ts`. */
export function v3DexSwapAdapterParameters(): ModuleParameters {
  const router = getRequiredEnvVar("V3_DEX_SWAP_ADAPTER_ROUTER");
  const wethToken = getRequiredEnvVar("V3_DEX_SWAP_ADAPTER_WETH_TOKEN");
  const lineaToken = getRequiredEnvVar("V3_DEX_SWAP_ADAPTER_LINEA_TOKEN");
  const poolTickSpacing = getRequiredEnvVar("V3_DEX_SWAP_ADAPTER_POOL_TICK_SPACING");
  return {
    router,
    wethToken,
    lineaToken,
    poolTickSpacing: toInteger("V3_DEX_SWAP_ADAPTER_POOL_TICK_SPACING", poolTickSpacing),
  };
}

export const parameters: NonNullable<DeployCompanion["parameters"]> = async () => ({
  V3DexSwapAdapter: v3DexSwapAdapterParameters(),
});
