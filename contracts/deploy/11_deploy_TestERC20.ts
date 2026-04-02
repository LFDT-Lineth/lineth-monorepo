import { ethers } from "hardhat";
import { HardhatRuntimeEnvironment } from "hardhat/types";
import { DeployFunction } from "hardhat-deploy/types";

import { getRequiredEnvVar, LogContractDeployment, tryVerifyContractWithConstructorArgs } from "../common/helpers";
import { getUiSigner, setUiTransactionContext, withSignerUiSession } from "../scripts/hardhat/signer-ui-bridge";

const func: DeployFunction = withSignerUiSession(
  "11_deploy_TestERC20.ts",
  async function (hre: HardhatRuntimeEnvironment) {
    const contractName = "TestERC20";
    const signer = await getUiSigner(hre);

    const tokenName = getRequiredEnvVar("TEST_ERC20_NAME");
    const tokenSymbol = getRequiredEnvVar("TEST_ERC20_SYMBOL");
    const initialSupply = getRequiredEnvVar("TEST_ERC20_INITIAL_SUPPLY");

    const TestERC20Factory = await ethers.getContractFactory(contractName, signer);
    const supplyWei = ethers.parseEther(initialSupply);
    setUiTransactionContext({
      contractName,
      constructorArgs: [tokenName, tokenSymbol, supplyWei.toString()],
      notes: `TEST_ERC20_INITIAL_SUPPLY env: ${initialSupply} (interpreted as ether, passed as wei to the contract)`,
    });
    const contract = await TestERC20Factory.deploy(tokenName, tokenSymbol, supplyWei);

    await LogContractDeployment(contractName, contract);
    const contractAddress = await contract.getAddress();

    const args = [tokenName, tokenSymbol, ethers.parseEther(initialSupply)];
    await tryVerifyContractWithConstructorArgs(
      contractAddress,
      "src/_testing/mocks/tokens/TestERC20.sol:TestERC20",
      args,
    );
  },
);

export default func;
func.tags = ["TestERC20"];
