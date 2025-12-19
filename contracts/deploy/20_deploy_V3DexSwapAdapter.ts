import { ethers } from "hardhat";
import { DeployFunction } from "hardhat-deploy/types";
import { getRequiredEnvVar, LogContractDeployment, tryVerifyContractWithConstructorArgs } from "../common/helpers";

const func: DeployFunction = async function () {
  const contractName = "V3DexSwapAdapter";

  const router = getRequiredEnvVar("V3_DEX_SWAP_ADAPTER_ROUTER");
  const wethToken = getRequiredEnvVar("V3_DEX_SWAP_ADAPTER_WETH_TOKEN");
  const lineaToken = getRequiredEnvVar("V3_DEX_SWAP_ADAPTER_LINEA_TOKEN");
  const poolTickSpacing = getRequiredEnvVar("V3_DEX_SWAP_ADAPTER_POOL_TICK_SPACING");

  const factory = await ethers.getContractFactory(contractName);
  const contract = await factory.deploy(router, wethToken, lineaToken, poolTickSpacing);

  await LogContractDeployment(contractName, contract);
  const contractAddress = await contract.getAddress();

  const args = [router, wethToken, lineaToken, poolTickSpacing];
  await tryVerifyContractWithConstructorArgs(
    contractAddress,
    "src/operational/V3DexSwapAdapter.sol:V3DexSwapAdapter",
    args,
  );
};

export default func;
func.tags = ["V3DexSwapAdapter"];
