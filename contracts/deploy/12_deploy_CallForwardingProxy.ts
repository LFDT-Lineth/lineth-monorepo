import { ethers } from "hardhat";
import { DeployFunction } from "hardhat-deploy/types";
import { deployFromFactory } from "../scripts/hardhat/utils";
import {
  LogContractDeployment,
  getRequiredEnvVar,
  tryStoreAddress,
  tryVerifyContractWithConstructorArgs,
} from "../common/helpers";
import { HardhatRuntimeEnvironment } from "hardhat/types";

const func: DeployFunction = async function (hre: HardhatRuntimeEnvironment) {
  const contractName = "CallForwardingProxy";

  const provider = ethers.provider;

  // This should be the LineaRollup
  const targetAddress = getRequiredEnvVar("LINEA_ROLLUP_ADDRESS");

  const contract = await deployFromFactory(contractName, provider, targetAddress);

  await LogContractDeployment(contractName, contract);
  const contractAddress = await contract.getAddress();

  await tryStoreAddress(hre.network.name, contractName, contractAddress, contract.deploymentTransaction()!.hash);

  const args = [targetAddress];

  await tryVerifyContractWithConstructorArgs(
    contractAddress,
    "contracts/lib/CallForwardingProxy.sol:CallForwardingProxy",
    args,
  );
};
export default func;
func.tags = ["CallForwardingProxy"];
