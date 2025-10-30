import { ethers } from "hardhat";
import { DeployFunction } from "hardhat-deploy/types";
import { HardhatRuntimeEnvironment } from "hardhat/types";
import {
  getRequiredEnvVar,
  getDeployedContractAddress,
  LogContractDeployment,
  tryVerifyContractWithConstructorArgs,
} from "../common/helpers";

const func: DeployFunction = async function (hre: HardhatRuntimeEnvironment) {
  const { deployments } = hre;

  const contractName = "L1LineaTokenBurner";
  const existingContractAddress = await getDeployedContractAddress(contractName, deployments);

  const messageService = getRequiredEnvVar("LINEA_TOKEN_BURNER_MESSAGE_SERVICE");
  const lineaToken = getRequiredEnvVar("LINEA_TOKEN_BURNER_LINEA_TOKEN");

  if (!existingContractAddress) {
    console.log(`Deploying initial version, NB: the address will be saved if env SAVE_ADDRESS=true.`);
  } else {
    console.log(`Deploying new version, NB: ${existingContractAddress} will be overwritten if env SAVE_ADDRESS=true.`);
  }

  const factory = await ethers.getContractFactory(contractName);
  const contract = await factory.deploy(messageService, lineaToken);

  await LogContractDeployment(contractName, contract);
  const contractAddress = await contract.getAddress();

  const args = [messageService, lineaToken];
  await tryVerifyContractWithConstructorArgs(
    contractAddress,
    "src/operational/L1LineaTokenBurner.sol:L1LineaTokenBurner",
    args,
  );
};

export default func;
func.tags = ["L1LineaTokenBurner"];
