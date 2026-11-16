import { getRequiredEnvVar } from "../../../common/helpers/environment.js";
import { requireAddressFromRegistryOrEnv } from "../../../common/helpers/readAddress.js";
import { toUint } from "../lib/parameters.js";

import type { DeployCompanion } from "../../../hardhat-plugins/deploy/types.js";

export const parameters: NonNullable<DeployCompanion["parameters"]> = async (context) => {
  const { networkName } = context;
  const securityCouncil = requireAddressFromRegistryOrEnv(networkName, "L2_SECURITY_COUNCIL", "L2_SECURITY_COUNCIL");
  const l1l2MessageSetter = requireAddressFromRegistryOrEnv(
    networkName,
    "L2_MESSAGE_SERVICE_L1L2_MESSAGE_SETTER",
    "L2_MESSAGE_SERVICE_L1L2_MESSAGE_SETTER",
  );
  const rateLimitPeriod = getRequiredEnvVar("L2_MESSAGE_SERVICE_RATE_LIMIT_PERIOD");
  const rateLimitAmount = getRequiredEnvVar("L2_MESSAGE_SERVICE_RATE_LIMIT_AMOUNT");

  return {
    L2MessageServiceV1: {
      securityCouncil,
      l1l2MessageSetter,
      rateLimitPeriod: toUint("L2_MESSAGE_SERVICE_RATE_LIMIT_PERIOD", rateLimitPeriod),
      rateLimitAmount: toUint("L2_MESSAGE_SERVICE_RATE_LIMIT_AMOUNT", rateLimitAmount),
    },
  };
};
