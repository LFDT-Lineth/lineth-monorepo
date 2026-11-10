import { getRequiredEnvVar } from "../../../common/helpers/environment.js";
import { requireAddressFromRegistryOrEnv } from "../../../common/helpers/readAddress.js";
import { toUint } from "../lib/parameters.js";

import type { DeployCompanion, ParameterContext } from "../../../hardhat-plugins/deploy/types.js";
import type { ModuleParameters } from "@nomicfoundation/ignition-core";

/** `04_deploy_L2MessageServiceV1.ts`. */
export function l2MessageServiceV1Parameters(context: ParameterContext): ModuleParameters {
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
    securityCouncil,
    l1l2MessageSetter,
    rateLimitPeriod: toUint("L2_MESSAGE_SERVICE_RATE_LIMIT_PERIOD", rateLimitPeriod),
    rateLimitAmount: toUint("L2_MESSAGE_SERVICE_RATE_LIMIT_AMOUNT", rateLimitAmount),
  };
}

export const parameters: NonNullable<DeployCompanion["parameters"]> = async (context) => ({
  L2MessageServiceV1: l2MessageServiceV1Parameters(context),
});
