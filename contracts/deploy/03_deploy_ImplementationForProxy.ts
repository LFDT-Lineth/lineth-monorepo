import { ethers, upgrades } from "hardhat";
import { DeployFunction } from "hardhat-deploy/types";
import { HardhatRuntimeEnvironment } from "hardhat/types";
import { getDeployedContractAddress, getRequiredEnvVar, tryVerifyContract } from "../common/helpers";

const func: DeployFunction = async function (hre: HardhatRuntimeEnvironment) {
  const { deployments } = hre;

  const contractName = getRequiredEnvVar("CONTRACT_NAME");
  const existingContractAddress = await getDeployedContractAddress(contractName, deployments);

  const proxyAddress = getRequiredEnvVar("PROXY_ADDRESS");

  const factory = await ethers.getContractFactory(contractName);

  if (existingContractAddress === undefined) {
    console.log(`Deploying initial version, NB: the address will be saved if env SAVE_ADDRESS=true.`);
  } else {
    console.log(`Deploying new version, NB: ${existingContractAddress} will be overwritten if env SAVE_ADDRESS=true.`);
  }

  console.log("Deploying Contract...");
  const newContract = await upgrades.deployImplementation(factory, {
    kind: "transparent",
  });

  const contract = newContract.toString();

  console.log(`Contract deployed at ${contract}`);

  const upgradeCallUsingSecurityCouncil = ethers.concat([
    "0x99a88ec4",
    ethers.AbiCoder.defaultAbiCoder().encode(["address", "address"], [proxyAddress, newContract]),
  ]);

  console.log("Encoded Tx Upgrade from Security Council:", "\n", upgradeCallUsingSecurityCouncil);

  console.log("\n");

  await tryVerifyContract(contract);
};

export default func;
func.tags = ["ImplementationForProxy"];
