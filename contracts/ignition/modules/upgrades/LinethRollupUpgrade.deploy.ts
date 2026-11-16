import { requireAddressFromRegistryOrEnv } from "../../../common/helpers/readAddress.js";

import type { DeployCompanion } from "../../../hardhat-plugins/deploy/types.js";

/** `proxy`: the existing LinethRollup proxy (address registry `LinethRollup`, else `LINETH_ROLLUP_ADDRESS`). */
export const parameters: NonNullable<DeployCompanion["parameters"]> = async ({ networkName }) => ({
  LinethRollupUpgrade: { proxy: requireAddressFromRegistryOrEnv(networkName, "LinethRollup", "LINETH_ROLLUP_ADDRESS") },
});
