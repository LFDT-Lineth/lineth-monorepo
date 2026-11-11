import { getRequiredEnvVar } from "../../../common/helpers/environment.js";
import { requireAddressFromRegistryOrEnv } from "../../../common/helpers/readAddress.js";
import { toUint } from "../lib/parameters.js";

import type { DeployCompanion, ParameterContext } from "../../../hardhat-plugins/deploy/types.js";
import type { ModuleParameters } from "@nomicfoundation/ignition-core";

/** `22_deploy_ForcedTransactionGateway.ts`. */
export function forcedTransactionGatewayParameters(context: ParameterContext): ModuleParameters {
  const { networkName } = context;
  const linethRollupAddress = requireAddressFromRegistryOrEnv(networkName, "LinethRollup", "LINETH_ROLLUP_ADDRESS");
  const destinationChainId = getRequiredEnvVar("FORCED_TRANSACTION_GATEWAY_L2_CHAIN_ID");
  const l2BlockBuffer = getRequiredEnvVar("FORCED_TRANSACTION_GATEWAY_L2_BLOCK_BUFFER");
  const maxGasLimit = getRequiredEnvVar("FORCED_TRANSACTION_GATEWAY_MAX_GAS_LIMIT");
  const maxInputLengthBuffer = getRequiredEnvVar("FORCED_TRANSACTION_GATEWAY_MAX_INPUT_LENGTH_BUFFER");
  const defaultAdmin = requireAddressFromRegistryOrEnv(networkName, "L1_SECURITY_COUNCIL", "L1_SECURITY_COUNCIL");
  const addressFilter = requireAddressFromRegistryOrEnv(
    networkName,
    "AddressFilter",
    "FORCED_TRANSACTION_ADDRESS_FILTER",
  );
  const mimcLibraryAddress = requireAddressFromRegistryOrEnv(
    networkName,
    "MIMC_LIBRARY_ADDRESS",
    "MIMC_LIBRARY_ADDRESS",
  );
  const l2BlockDurationSeconds = getRequiredEnvVar("FORCED_TRANSACTION_L2_BLOCK_DURATION_SECONDS");
  const blockNumberDeadlineBuffer = getRequiredEnvVar("FORCED_TRANSACTION_BLOCK_NUMBER_DEADLINE_BUFFER");

  return {
    mimcLibrary: mimcLibraryAddress,
    linethRollup: linethRollupAddress,
    destinationChainId: toUint("FORCED_TRANSACTION_GATEWAY_L2_CHAIN_ID", destinationChainId),
    l2BlockBuffer: toUint("FORCED_TRANSACTION_GATEWAY_L2_BLOCK_BUFFER", l2BlockBuffer),
    maxGasLimit: toUint("FORCED_TRANSACTION_GATEWAY_MAX_GAS_LIMIT", maxGasLimit),
    maxInputLengthBuffer: toUint("FORCED_TRANSACTION_GATEWAY_MAX_INPUT_LENGTH_BUFFER", maxInputLengthBuffer),
    securityCouncil: defaultAdmin,
    addressFilter,
    l2BlockDurationSeconds: toUint("FORCED_TRANSACTION_L2_BLOCK_DURATION_SECONDS", l2BlockDurationSeconds),
    blockNumberDeadlineBuffer: toUint("FORCED_TRANSACTION_BLOCK_NUMBER_DEADLINE_BUFFER", blockNumberDeadlineBuffer),
  };
}

export const parameters: NonNullable<DeployCompanion["parameters"]> = async (context) => ({
  ForcedTransactionGateway: forcedTransactionGatewayParameters(context),
});
