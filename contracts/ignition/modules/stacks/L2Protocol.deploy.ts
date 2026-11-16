import { timelockParameters } from "../governance/Timelock.deploy.js";
import { l2MessageServiceParameters } from "../messaging/L2MessageService.deploy.js";

import type { DeployCompanion } from "../../../hardhat-plugins/deploy/types.js";

export const parameters: NonNullable<DeployCompanion["parameters"]> = async (context) => ({
  L2MessageService: l2MessageServiceParameters(context),
  Timelock: timelockParameters(context),
});
