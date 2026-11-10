import { yieldManagerParameters } from "./YieldManager.deploy.js";

import type { DeployCompanion } from "../../../hardhat-plugins/deploy/types.js";

export const parameters: NonNullable<DeployCompanion["parameters"]> = async (context) => ({
  YieldManagerArtifacts: yieldManagerParameters(context),
});
