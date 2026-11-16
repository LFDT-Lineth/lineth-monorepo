import { requireAddressFromRegistryOrEnv } from "../../../common/helpers/readAddress.js";

import type { DeployCompanion } from "../../../hardhat-plugins/deploy/types.js";

export const parameters: NonNullable<DeployCompanion["parameters"]> = async (context) => {
  const { networkName } = context;
  const recoverFundsSecurityCouncil = requireAddressFromRegistryOrEnv(
    networkName,
    "L1_SECURITY_COUNCIL",
    "L1_SECURITY_COUNCIL",
  );
  const recoverFundsExecutorAddress = requireAddressFromRegistryOrEnv(
    networkName,
    "RECOVERFUNDS_EXECUTOR_ADDRESS",
    "RECOVERFUNDS_EXECUTOR_ADDRESS",
  );
  return { RecoverFunds: { securityCouncil: recoverFundsSecurityCouncil, executor: recoverFundsExecutorAddress } };
};
