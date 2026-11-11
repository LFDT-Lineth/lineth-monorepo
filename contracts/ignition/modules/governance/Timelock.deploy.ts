import { getRequiredEnvVar } from "../../../common/helpers/environment.js";
import { requireAddressFromRegistryOrEnv } from "../../../common/helpers/readAddress.js";
import { splitList, toUint } from "../lib/parameters.js";

import type { DeployCompanion, ParameterContext } from "../../../hardhat-plugins/deploy/types.js";
import type { ModuleParameters } from "@nomicfoundation/ignition-core";

/** `02_deploy_Timelock.ts`. */
export function timelockParameters(context: ParameterContext): ModuleParameters {
  const timeLockProposers = getRequiredEnvVar("TIMELOCK_PROPOSERS");
  const timelockExecutors = getRequiredEnvVar("TIMELOCK_EXECUTORS");
  const adminAddress = requireAddressFromRegistryOrEnv(
    context.networkName,
    "TIMELOCK_ADMIN_ADDRESS",
    "TIMELOCK_ADMIN_ADDRESS",
  );
  const minDelay = process.env.MIN_DELAY || "0";

  return {
    timelockMinDelay: toUint("MIN_DELAY", minDelay),
    timelockProposers: splitList(timeLockProposers),
    timelockExecutors: splitList(timelockExecutors),
    timelockAdmin: adminAddress,
  };
}

export const parameters: NonNullable<DeployCompanion["parameters"]> = async (context) => ({
  Timelock: timelockParameters(context),
});
