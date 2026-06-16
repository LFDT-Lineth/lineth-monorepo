import { ethers } from "hardhat";
import { HardhatRuntimeEnvironment } from "hardhat/types";
import { DeployFunction } from "hardhat-deploy/types";

import {
  LogContractDeployment,
  getRequiredEnvVar,
  requireAddressFromRegistryOrEnv,
  tryVerifyContractWithConstructorArgs,
} from "../common/helpers";
import { getUiSigner, withSignerUiSession } from "../scripts/hardhat/signer-ui-bridge";

const func: DeployFunction = withSignerUiSession(
  "22_deploy_ForcedTransactionGateway.ts",
  async function (hre: HardhatRuntimeEnvironment) {
    const contractName = "ForcedTransactionGateway";
    const signer = await getUiSigner(hre);

    const lineaRollupAddress = requireAddressFromRegistryOrEnv(hre.network.name, "LineaRollup", "LINEA_ROLLUP_ADDRESS");
    const destinationChainId = getRequiredEnvVar("FORCED_TRANSACTION_GATEWAY_L2_CHAIN_ID");
    const l2BlockBuffer = getRequiredEnvVar("FORCED_TRANSACTION_GATEWAY_L2_BLOCK_BUFFER");
    const maxGasLimit = getRequiredEnvVar("FORCED_TRANSACTION_GATEWAY_MAX_GAS_LIMIT");
    const maxInputLengthBuffer = getRequiredEnvVar("FORCED_TRANSACTION_GATEWAY_MAX_INPUT_LENGTH_BUFFER");
    const defaultAdmin = requireAddressFromRegistryOrEnv(
      hre.network.name,
      "L1_SECURITY_COUNCIL",
      "L1_SECURITY_COUNCIL",
    );
    const addressFilter = requireAddressFromRegistryOrEnv(
      hre.network.name,
      "AddressFilter",
      "FORCED_TRANSACTION_ADDRESS_FILTER",
    );
    const mimcLibraryAddress = requireAddressFromRegistryOrEnv(
      hre.network.name,
      "MIMC_LIBRARY_ADDRESS",
      "MIMC_LIBRARY_ADDRESS",
    );

    const factory = await ethers.getContractFactory(contractName, {
      libraries: { Mimc: mimcLibraryAddress },
    });
    const l2BlockDurationSeconds = getRequiredEnvVar("FORCED_TRANSACTION_L2_BLOCK_DURATION_SECONDS");
    const blockNumberDeadlineBuffer = getRequiredEnvVar("FORCED_TRANSACTION_BLOCK_NUMBER_DEADLINE_BUFFER");

    const contract = await factory
      .connect(signer)
      .deploy(
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
      { Mimc: mimcLibraryAddress },
    );
  },
);

export default func;
func.tags = ["ForcedTransactionGateway"];
