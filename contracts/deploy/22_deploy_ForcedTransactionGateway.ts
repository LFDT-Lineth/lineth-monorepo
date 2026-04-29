import { ethers } from "hardhat";
import { DeployFunction } from "hardhat-deploy/types";

import { LogContractDeployment, getRequiredEnvVar, tryVerifyContractWithConstructorArgs } from "../common/helpers";

const func: DeployFunction = async function () {
  const contractName = "ForcedTransactionGateway";

  const lineaRollupAddress = getRequiredEnvVar("LINEA_ROLLUP_ADDRESS");
  const destinationChainId = getRequiredEnvVar("FORCED_TRANSACTION_GATEWAY_L2_CHAIN_ID");
  const l2BlockBuffer = getRequiredEnvVar("FORCED_TRANSACTION_GATEWAY_L2_BLOCK_BUFFER");
  const maxGasLimit = getRequiredEnvVar("FORCED_TRANSACTION_GATEWAY_MAX_GAS_LIMIT");
  const maxInputLengthBuffer = getRequiredEnvVar("FORCED_TRANSACTION_GATEWAY_MAX_INPUT_LENGTH_BUFFER");
  const defaultAdmin = getRequiredEnvVar("L1_SECURITY_COUNCIL");
  const addressFilter = getRequiredEnvVar("FORCED_TRANSACTION_ADDRESS_FILTER");
  const mimcLibraryAddress = getRequiredEnvVar("MIMC_LIBRARY_ADDRESS");

  const factory = await ethers.getContractFactory("ForcedTransactionGateway", {
    libraries: { Mimc: mimcLibraryAddress },
  });
  const l2BlockDurationSeconds = getRequiredEnvVar("FORCED_TRANSACTION_L2_BLOCK_DURATION_SECONDS");
  const blockNumberDeadlineBuffer = getRequiredEnvVar("FORCED_TRANSACTION_BLOCK_NUMBER_DEADLINE_BUFFER");

  const contract = await factory.deploy(
    lineaRollupAddress,
    destinationChainId,
    l2BlockBuffer,
    maxGasLimit,
    maxInputLengthBuffer,
    defaultAdmin,
    addressFilter,
    l2BlockDurationSeconds,
    blockNumberDeadlineBuffer,
  );

  await LogContractDeployment(contractName, contract);
  const contractAddress = await contract.getAddress();

  const args = [
    lineaRollupAddress,
    destinationChainId,
    l2BlockBuffer,
    maxGasLimit,
    maxInputLengthBuffer,
    defaultAdmin,
    addressFilter,
    l2BlockDurationSeconds,
    blockNumberDeadlineBuffer,
  ];
  await tryVerifyContractWithConstructorArgs(
    contractAddress,
    "src/rollup/forcedTransactions/ForcedTransactionGateway.sol:ForcedTransactionGateway",
    args,
  );
};

export default func;
func.tags = ["ForcedTransactionGateway"];
