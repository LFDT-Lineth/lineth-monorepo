import { ethers } from "hardhat";
import { DeployFunction } from "hardhat-deploy/types";
import { getRequiredEnvVar, LogContractDeployment, tryVerifyContractWithConstructorArgs } from "../common/helpers";

const func: DeployFunction = async function () {
  const contractName = "L1LineaTokenBurner";

  const messageService = getRequiredEnvVar("LINEA_TOKEN_BURNER_MESSAGE_SERVICE");
  const lineaToken = getRequiredEnvVar("LINEA_TOKEN_BURNER_LINEA_TOKEN");

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
