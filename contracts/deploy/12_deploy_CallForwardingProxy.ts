import { ethers } from "hardhat";
import { DeployFunction } from "hardhat-deploy/types";
import { deployFromFactory } from "../scripts/hardhat/utils";
import { getRequiredEnvVar, tryVerifyContractWithConstructorArgs } from "../common/helpers";

const func: DeployFunction = async function () {
  const contractName = "CallForwardingProxy";

  const provider = ethers.provider;

  // This should be the LineaRollup
  const targetAddress = getRequiredEnvVar("LINEA_ROLLUP_ADDRESS");

  const contract = await deployFromFactory(contractName, provider, targetAddress);
  const contractAddress = await contract.getAddress();

  console.log(`${contractName} deployed at ${contractAddress}`);

  const deployTx = contract.deploymentTransaction();
  if (!deployTx) {
    throw "Deployment transaction not found.";
  }

  const args = [targetAddress];

  await tryVerifyContractWithConstructorArgs(
    contractAddress,
    "contracts/lib/CallForwardingProxy.sol:CallForwardingProxy",
    args,
  );
};
export default func;
func.tags = ["CallForwardingProxy"];
