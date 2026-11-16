import { requireAddressFromRegistryOrEnv } from "../../../common/helpers/readAddress.js";

import type { DeployCompanion } from "../../../hardhat-plugins/deploy/types.js";

/** `proxy` is the existing YieldManager proxy. */
export const parameters: NonNullable<DeployCompanion["parameters"]> = async (context) => {
  const linethRollupAddress = requireAddressFromRegistryOrEnv(
    context.networkName,
    "LinethRollup",
    "LINETH_ROLLUP_ADDRESS",
  );
  const proxy = requireAddressFromRegistryOrEnv(context.networkName, "YieldManager", "YIELD_MANAGER_ADDRESS");
  return { YieldManagerUpgrade: { proxy, linethRollup: linethRollupAddress } };
};
