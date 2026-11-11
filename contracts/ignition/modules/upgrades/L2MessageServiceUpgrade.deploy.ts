import { requireAddressFromRegistryOrEnv } from "../../../common/helpers/readAddress.js";

import type { DeployCompanion } from "../../../hardhat-plugins/deploy/types.js";

/** `proxy`: the existing L2MessageService proxy (address registry `L2MessageService`, else `L2_MESSAGE_SERVICE_ADDRESS`). */
export const parameters: NonNullable<DeployCompanion["parameters"]> = async ({ networkName }) => ({
  L2MessageServiceUpgrade: {
    proxy: requireAddressFromRegistryOrEnv(networkName, "L2MessageService", "L2_MESSAGE_SERVICE_ADDRESS"),
  },
});
