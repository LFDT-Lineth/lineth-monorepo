import { HardhatRuntimeEnvironment } from "hardhat/types";
import { DeployFunction } from "hardhat-deploy/types";

import { getRequiredEnvVar, LogContractDeployment, tryVerifyContractWithConstructorArgs } from "../common/helpers";
import { getUiSigner, withSignerUiSession } from "../scripts/hardhat/signer-ui-bridge";
import { deployFromFactory } from "../scripts/hardhat/utils";

const func: DeployFunction = withSignerUiSession(
  "22_deploy_YieldManagerUpgrade.ts",
  async function (hre: HardhatRuntimeEnvironment) {
    const lineaRollupAddress = getRequiredEnvVar("LINEA_ROLLUP_ADDRESS");

    console.log("Deploying Contract...");
    const signer = await getUiSigner(hre);
    const contractName = "YieldManager";
    const contract = await deployFromFactory(contractName, signer, lineaRollupAddress);
    const yieldManagerAddress = await contract.getAddress();
    await LogContractDeployment(contractName, contract);
    await tryVerifyContractWithConstructorArgs(yieldManagerAddress, "src/yield/YieldManager.sol:YieldManager", [
      lineaRollupAddress,
    ]);
  },
);

export default func;
func.tags = ["YieldManagerImplementation"];
