import { getRequiredEnvVar } from "../../../common/helpers/environment.js";
import { requireAddressFromRegistryOrEnv } from "../../../common/helpers/readAddress.js";
import { toUint } from "../lib/parameters.js";

import type { DeployCompanion } from "../../../hardhat-plugins/deploy/types.js";

/** Same inputs as the Hardhat 2 `03_deploy_LinethRollupV8WithReinitialization.ts` script. */
export const parameters: NonNullable<DeployCompanion["parameters"]> = async ({ networkName }) => ({
  LinethRollupReinitializeV9: {
    proxy: requireAddressFromRegistryOrEnv(networkName, "LinethRollup", "LINETH_ROLLUP_ADDRESS"),
    forcedTransactionFeeInWei: toUint(
      "LINETH_ROLLUP_FORCED_TRANSACTION_FEE_IN_WEI",
      getRequiredEnvVar("LINETH_ROLLUP_FORCED_TRANSACTION_FEE_IN_WEI"),
    ),
    addressFilter: requireAddressFromRegistryOrEnv(networkName, "AddressFilter", "LINETH_ROLLUP_ADDRESS_FILTER"),
  },
});
