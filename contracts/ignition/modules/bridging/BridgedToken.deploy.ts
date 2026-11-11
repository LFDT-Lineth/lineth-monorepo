import { reportTokenBridge } from "../lib/tokenBridgeReport.js";

import type { DeployCompanion } from "../../../hardhat-plugins/deploy/types.js";

export const afterDeploy: NonNullable<DeployCompanion["afterDeploy"]> = (context) =>
  reportTokenBridge(context, { beacon: "BridgedToken#BridgedTokenBeacon" });
