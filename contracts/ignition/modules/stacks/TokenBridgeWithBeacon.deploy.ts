import { tokenBridgeParameters } from "../bridging/TokenBridge.deploy.js";
import { reportTokenBridge } from "../lib/tokenBridgeReport.js";

import type { DeployCompanion } from "../../../hardhat-plugins/deploy/types.js";

export const parameters: NonNullable<DeployCompanion["parameters"]> = async (context) => ({
  TokenBridgeWithBeacon: tokenBridgeParameters(context, { includeBeacon: false }),
});

export const afterDeploy: NonNullable<DeployCompanion["afterDeploy"]> = (context) =>
  reportTokenBridge(context, {
    beacon: "BridgedToken#BridgedTokenBeacon",
    tokenBridge: "TokenBridgeWithBeacon#TokenBridge",
  });
