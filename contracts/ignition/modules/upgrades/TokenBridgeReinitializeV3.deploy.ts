import { getBooleanEnvVarOrDefault } from "../../../common/helpers/environment.js";
import { requireAddressFromRegistryOrEnv } from "../../../common/helpers/readAddress.js";

import type { DeployCompanion } from "../../../hardhat-plugins/deploy/types.js";

/** `proxy`: the existing TokenBridge proxy (registry `TokenBridge_L1` or `TokenBridge_L2` by `DEPLOY_TOKEN_BRIDGE_ON_L1`, else `TOKEN_BRIDGE_ADDRESS`). */
export const parameters: NonNullable<DeployCompanion["parameters"]> = async ({ networkName }) => {
  const registryKey = getBooleanEnvVarOrDefault("DEPLOY_TOKEN_BRIDGE_ON_L1", false)
    ? "TokenBridge_L1"
    : "TokenBridge_L2";
  return {
    TokenBridgeReinitializeV3: {
      proxy: requireAddressFromRegistryOrEnv(networkName, registryKey, "TOKEN_BRIDGE_ADDRESS"),
    },
  };
};
