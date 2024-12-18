import { ethers } from "hardhat";
import { DeployFunction } from "hardhat-deploy/types";
import { deployFromFactory } from "../scripts/hardhat/utils";
import { get1559Fees } from "../scripts/utils";
import { LogContractDeployment } from "contracts/common/helpers";

const func: DeployFunction = async function () {
  const contractName = "TestEIP4844";

  const provider = ethers.provider;

  const contract = await deployFromFactory(contractName, provider, await get1559Fees(provider));
  await LogContractDeployment(contractName, contract);
};
export default func;
func.tags = ["TestEIP4844"];
