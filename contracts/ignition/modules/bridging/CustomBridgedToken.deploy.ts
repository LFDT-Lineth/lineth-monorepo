import { getRequiredEnvVar } from "../../../common/helpers/environment.js";
import { requireAddressFromRegistryOrEnv } from "../../../common/helpers/readAddress.js";
import { toInteger } from "../lib/parameters.js";

import type { DeployCompanion } from "../../../hardhat-plugins/deploy/types.js";

export const parameters: NonNullable<DeployCompanion["parameters"]> = async (context) => {
  const customTokenBridgeName = getRequiredEnvVar("CUSTOMTOKENBRIDGE_NAME");
  const customTokenBridgeSymbol = getRequiredEnvVar("CUSTOMTOKENBRIDGE_SYMBOL");
  const customTokenBridgeDecimals = getRequiredEnvVar("CUSTOMTOKENBRIDGE_DECIMALS");
  const customTokenBridgeBridgeAddress = requireAddressFromRegistryOrEnv(
    context.networkName,
    "CUSTOMTOKENBRIDGE_BRIDGE_ADDRESS",
    "CUSTOMTOKENBRIDGE_BRIDGE_ADDRESS",
  );
  console.log(`Current network's chainId is ${context.chainId}`);

  return {
    CustomBridgedToken: {
      name: customTokenBridgeName,
      symbol: customTokenBridgeSymbol,
      decimals: toInteger("CUSTOMTOKENBRIDGE_DECIMALS", customTokenBridgeDecimals),
      bridge: customTokenBridgeBridgeAddress,
    },
  };
};
