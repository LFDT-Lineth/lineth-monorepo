import { getRequiredEnvVar } from "../../../common/helpers/environment.js";
import { toInteger } from "../lib/parameters.js";

import type { DeployCompanion } from "../../../hardhat-plugins/deploy/types.js";

export const parameters: NonNullable<DeployCompanion["parameters"]> = async () => {
  const router = getRequiredEnvVar("V3_DEX_SWAP_ADAPTER_ROUTER");
  const wethToken = getRequiredEnvVar("V3_DEX_SWAP_ADAPTER_WETH_TOKEN");
  const lineaToken = getRequiredEnvVar("V3_DEX_SWAP_ADAPTER_LINEA_TOKEN");
  const poolTickSpacing = getRequiredEnvVar("V3_DEX_SWAP_ADAPTER_POOL_TICK_SPACING");
  return {
    V3DexSwapAdapter: {
      router,
      wethToken,
      lineaToken,
      poolTickSpacing: toInteger("V3_DEX_SWAP_ADAPTER_POOL_TICK_SPACING", poolTickSpacing),
    },
  };
};
