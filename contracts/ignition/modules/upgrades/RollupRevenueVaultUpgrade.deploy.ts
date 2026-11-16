import { requireAddressFromRegistryOrEnv } from "../../../common/helpers/readAddress.js";

import type { DeployCompanion } from "../../../hardhat-plugins/deploy/types.js";

/** `proxy`: the existing RollupRevenueVault proxy (address registry `RollupRevenueVault`, else `ROLLUP_REVENUE_VAULT_ADDRESS`). */
export const parameters: NonNullable<DeployCompanion["parameters"]> = async ({ networkName }) => ({
  RollupRevenueVaultUpgrade: {
    proxy: requireAddressFromRegistryOrEnv(networkName, "RollupRevenueVault", "ROLLUP_REVENUE_VAULT_ADDRESS"),
  },
});
