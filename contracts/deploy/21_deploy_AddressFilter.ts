import { PRECOMPILES_ADDRESSES } from "contracts/common/constants";
import { ethers } from "hardhat";
import { DeployFunction } from "hardhat-deploy/types";

import { LogContractDeployment, getRequiredEnvVar, tryVerifyContractWithConstructorArgs } from "../common/helpers";

const func: DeployFunction = async function () {
  const contractName = "AddressFilter";

  const lineaRollupSecurityCouncil = getRequiredEnvVar("L1_SECURITY_COUNCIL");
  const filteredAddresses = getRequiredEnvVar("ADDRESS_FILTER_FILTERED_ADDRESSES").split(",");

  const defaultFilterAddresses = [...PRECOMPILES_ADDRESSES, ...filteredAddresses];

  const factory = await ethers.getContractFactory(contractName);
  const contract = await factory.deploy(lineaRollupSecurityCouncil, defaultFilterAddresses);

  await LogContractDeployment(contractName, contract);
  const contractAddress = await contract.getAddress();

  const args = [lineaRollupSecurityCouncil, defaultFilterAddresses];
  await tryVerifyContractWithConstructorArgs(
    contractAddress,
    "src/rollup/forcedTransactions/AddressFilter.sol:AddressFilter",
    args,
  );
};

export default func;
func.tags = ["AddressFilter"];
