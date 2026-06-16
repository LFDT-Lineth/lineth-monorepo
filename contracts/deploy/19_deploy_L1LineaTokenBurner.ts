import { ethers } from "hardhat";
import { HardhatRuntimeEnvironment } from "hardhat/types";
import { DeployFunction } from "hardhat-deploy/types";

import {
  requireAddressFromRegistryOrEnv,
  LogContractDeployment,
  tryVerifyContractWithConstructorArgs,
} from "../common/helpers";
import { getUiSigner, withSignerUiSession } from "../scripts/hardhat/signer-ui-bridge";

const func: DeployFunction = withSignerUiSession(
  "19_deploy_L1LineaTokenBurner.ts",
  async function (hre: HardhatRuntimeEnvironment) {
    const contractName = "L1LineaTokenBurner";
    const signer = await getUiSigner(hre);

    const messageService = requireAddressFromRegistryOrEnv(hre.network.name, "LineaRollup", "LINEA_ROLLUP_ADDRESS");
    const lineaToken = requireAddressFromRegistryOrEnv(hre.network.name, "LINEA_TOKEN", "LINEA_TOKEN");

    const factory = await ethers.getContractFactory(contractName, signer);
    const contract = await factory.deploy(messageService, lineaToken);

    await LogContractDeployment(contractName, contract);
    const contractAddress = await contract.getAddress();

    const args = [messageService, lineaToken];
    await tryVerifyContractWithConstructorArgs(
      contractAddress,
      "src/operational/L1LineaTokenBurner.sol:L1LineaTokenBurner",
      args,
    );
  },
);

export default func;
func.tags = ["L1LineaTokenBurner"];
