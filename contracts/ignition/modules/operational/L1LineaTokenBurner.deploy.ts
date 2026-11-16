import { requireAddressFromRegistryOrEnv } from "../../../common/helpers/readAddress.js";

import type { DeployCompanion } from "../../../hardhat-plugins/deploy/types.js";

export const parameters: NonNullable<DeployCompanion["parameters"]> = async (context) => {
  const messageService = requireAddressFromRegistryOrEnv(context.networkName, "LinethRollup", "LINETH_ROLLUP_ADDRESS");
  const lineaToken = requireAddressFromRegistryOrEnv(context.networkName, "LINEA_TOKEN", "LINEA_TOKEN");
  return { L1LineaTokenBurner: { messageService, lineaToken } };
};
