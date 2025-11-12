import { ethers } from "hardhat";
import { DeployFunction } from "hardhat-deploy/types";
import { getRequiredEnvVar, LogContractDeployment, tryVerifyContractWithConstructorArgs } from "../common/helpers";

const func: DeployFunction = async function () {
  const contractName = "TestERC20";

  const tokenName = getRequiredEnvVar("TEST_ERC20_NAME");
  const tokenSymbol = getRequiredEnvVar("TEST_ERC20_SYMBOL");
  const initialSupply = getRequiredEnvVar("TEST_ERC20_INITIAL_SUPPLY");

  const TestERC20Factory = await ethers.getContractFactory(contractName);
  const contract = await TestERC20Factory.deploy(tokenName, tokenSymbol, ethers.parseEther(initialSupply));

  await LogContractDeployment(contractName, contract);
  const contractAddress = await contract.getAddress();

  const args = [tokenName, tokenSymbol, ethers.parseEther(initialSupply)];
  await tryVerifyContractWithConstructorArgs(
    contractAddress,
    "src/_testing/mocks/tokens/TestERC20.sol:TestERC20",
    args,
  );
};

export default func;
func.tags = ["TestERC20"];
