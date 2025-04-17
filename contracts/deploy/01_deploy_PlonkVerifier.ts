import { ethers } from "hardhat";
import { DeployFunction } from "hardhat-deploy/types";
import { HardhatRuntimeEnvironment } from "hardhat/types";
import { deployFromFactory } from "../scripts/hardhat/utils";
import {
  tryVerifyContract,
  getDeployedContractAddress,
  tryStoreAddress,
  getRequiredEnvVar,
  LogContractDeployment,
} from "../common/helpers";

const func: DeployFunction = async function (hre: HardhatRuntimeEnvironment) {
  const { deployments } = hre;

  const contractName = getRequiredEnvVar("PLONKVERIFIER_NAME");
  const verifierIndex = getRequiredEnvVar("PLONKVERIFIER_INDEX");

  const existingContractAddress = await getDeployedContractAddress(contractName, deployments);

  const provider = ethers.provider;

  if (existingContractAddress === undefined) {
    console.log(`Deploying initial version, NB: the address will be saved if env SAVE_ADDRESS=true.`);
  } else {
    console.log(`Deploying new version, NB: ${existingContractAddress} will be overwritten if env SAVE_ADDRESS=true.`);
  }
  const contract = await deployFromFactory(contractName, provider);

  await LogContractDeployment(contractName, contract);
  const contractAddress = await contract.getAddress();

  process.env.PLONKVERIFIER_ADDRESS = contractAddress;
  await tryStoreAddress(hre.network.name, contractName, contractAddress, contract.deploymentTransaction()!.hash);

  const setVerifierAddress = ethers.concat([
    "0xc2116974",
    ethers.AbiCoder.defaultAbiCoder().encode(["address", "uint256"], [contractAddress, verifierIndex]),
  ]);

  console.log("setVerifierAddress calldata:", setVerifierAddress);

  await tryVerifyContract(contractAddress);
};
export default func;
func.tags = ["PlonkVerifier"];
