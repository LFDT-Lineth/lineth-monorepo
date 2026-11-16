import { getBooleanEnvVarOrDefault } from "../../../common/helpers/environment.js";

import type { AfterDeployContext } from "../../../hardhat-plugins/deploy/types.js";

/** Prints the BridgedToken beacon and TokenBridge addresses, labelled L1 or L2 by `DEPLOY_TOKEN_BRIDGE_ON_L1`. */
export function reportTokenBridge(
  { contracts, networkName }: AfterDeployContext,
  futureIds: { beacon?: string; tokenBridge?: string },
): void {
  const layer = getBooleanEnvVarOrDefault("DEPLOY_TOKEN_BRIDGE_ON_L1", false) ? "L1" : "L2";
  if (futureIds.beacon !== undefined) {
    console.log(`${layer} BridgedToken beacon deployed on ${networkName}, at address:`, contracts[futureIds.beacon]);
  }
  if (futureIds.tokenBridge !== undefined) {
    console.log(`${layer} TokenBridge deployed on ${networkName}, at address: ${contracts[futureIds.tokenBridge]}`);
  }
}
