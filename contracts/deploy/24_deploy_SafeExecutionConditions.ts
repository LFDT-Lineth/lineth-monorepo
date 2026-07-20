import { HardhatRuntimeEnvironment } from "hardhat/types";
import { DeployFunction } from "hardhat-deploy/types";

import { LogContractDeployment, tryVerifyContract } from "../common/helpers";
import { getUiSigner, withSignerUiSession } from "../scripts/hardhat/signer-ui-bridge";
import { deployFromFactory } from "../scripts/hardhat/utils";

const func: DeployFunction = withSignerUiSession(
  "24_deploy_SafeExecutionConditions.ts",
  async function (hre: HardhatRuntimeEnvironment) {
    const contractName = "SafeExecutionConditions";
    const signer = await getUiSigner(hre);

    // No constructor args: the contract is stateless.
    const contract = await deployFromFactory(contractName, signer);

    await LogContractDeployment(contractName, contract);
    const contractAddress = await contract.getAddress();

    await tryVerifyContract(contractAddress, "src/operational/SafeExecutionConditions.sol:SafeExecutionConditions");
  },
);

export default func;
func.tags = ["SafeExecutionConditions"];
