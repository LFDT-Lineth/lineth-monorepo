import { timelockParameters } from "../governance/Timelock.deploy.js";
import { linethRollupParameters } from "../rollup/LinethRollup.deploy.js";
import { afterDeploy as reportSetVerifierAddress, plonkVerifierParameters } from "../verifiers/PlonkVerifier.deploy.js";

import type { DeployCompanion } from "../../../hardhat-plugins/deploy/types.js";

export const parameters: NonNullable<DeployCompanion["parameters"]> = async (context) => ({
  PlonkVerifier: await plonkVerifierParameters(context),
  L1Protocol: linethRollupParameters(context, { includeVerifier: false }),
  Timelock: timelockParameters(context),
});

export const afterDeploy = reportSetVerifierAddress;
