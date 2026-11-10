import { getRequiredEnvVar } from "../../../common/helpers/environment.js";
import { toBool } from "../lib/parameters.js";

import type { DeployCompanion } from "../../../hardhat-plugins/deploy/types.js";
import type { ModuleParameters } from "@nomicfoundation/ignition-core";

/** `13_deploy_BaseSequencerUptimeFeed.ts`. */
export function lineaSequencerUptimeFeedParameters(): ModuleParameters {
  const initialStatus = getRequiredEnvVar("LINETH_SEQUENCER_UPTIME_FEED_INITIAL_STATUS");
  const adminAddress = getRequiredEnvVar("LINETH_SEQUENCER_UPTIME_FEED_ADMIN");
  const feedUpdaterAddress = getRequiredEnvVar("LINETH_SEQUENCER_UPTIME_FEED_UPDATER");
  return {
    initialStatus: toBool("LINETH_SEQUENCER_UPTIME_FEED_INITIAL_STATUS", initialStatus),
    admin: adminAddress,
    feedUpdater: feedUpdaterAddress,
  };
}

export const parameters: NonNullable<DeployCompanion["parameters"]> = async () => ({
  LineaSequencerUptimeFeed: lineaSequencerUptimeFeedParameters(),
});
