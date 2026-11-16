import { requireAddressFromRegistryOrEnv } from "../../../common/helpers/readAddress.js";

import type { DeployCompanion } from "../../../hardhat-plugins/deploy/types.js";

/** `proxy`: the existing Validium proxy (address registry `Validium`, else `PROXY_ADDRESS`). */
export const parameters: NonNullable<DeployCompanion["parameters"]> = async ({ networkName }) => ({
  ValidiumUpgrade: { proxy: requireAddressFromRegistryOrEnv(networkName, "Validium", "PROXY_ADDRESS") },
});
