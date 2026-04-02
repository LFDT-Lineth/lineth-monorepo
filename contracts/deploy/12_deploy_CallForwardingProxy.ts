import { HardhatRuntimeEnvironment } from "hardhat/types";
import { DeployFunction } from "hardhat-deploy/types";

import { LogContractDeployment, getRequiredEnvVar, tryVerifyContractWithConstructorArgs } from "../common/helpers";
import { getUiSigner, withSignerUiSession } from "../scripts/hardhat/signer-ui-bridge";
import { deployFromFactory } from "../scripts/hardhat/utils";

const func: DeployFunction = withSignerUiSession(
  "12_deploy_CallForwardingProxy.ts",
  async function (hre: HardhatRuntimeEnvironment) {
    const contractName = "CallForwardingProxy";
    const signer = await getUiSigner(hre);

    // This should be the LineaRollup
    const targetAddress = getRequiredEnvVar("LINEA_ROLLUP_ADDRESS");

    const contract = await deployFromFactory(contractName, signer, targetAddress);

    await LogContractDeployment(contractName, contract);
    const contractAddress = await contract.getAddress();

    const args = [targetAddress];

    await tryVerifyContractWithConstructorArgs(
      contractAddress,
      "contracts/lib/CallForwardingProxy.sol:CallForwardingProxy",
      args,
    );
  },
);
export default func;
func.tags = ["CallForwardingProxy"];
