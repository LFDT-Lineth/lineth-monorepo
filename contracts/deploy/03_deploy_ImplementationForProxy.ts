import { ethers, upgrades } from "hardhat";
import { DeployFunction } from "hardhat-deploy/types";
import { getRequiredEnvVar, tryVerifyContract } from "../common/helpers";

const func: DeployFunction = async function () {
  const contractName = getRequiredEnvVar("CONTRACT_NAME");

  const proxyAddress = getRequiredEnvVar("PROXY_ADDRESS");

  const factory = await ethers.getContractFactory(contractName);

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
