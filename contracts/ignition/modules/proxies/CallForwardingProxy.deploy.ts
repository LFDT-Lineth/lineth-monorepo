import { requireAddressFromRegistryOrEnv } from "../../../common/helpers/readAddress.js";

import type { DeployCompanion } from "../../../hardhat-plugins/deploy/types.js";

export const parameters: NonNullable<DeployCompanion["parameters"]> = async (context) => {
  // This should be the LinethRollup
  const targetAddress = requireAddressFromRegistryOrEnv(context.networkName, "LinethRollup", "LINETH_ROLLUP_ADDRESS");
  return { CallForwardingProxy: { target: targetAddress } };
};
