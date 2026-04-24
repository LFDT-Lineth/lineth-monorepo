import { ethers } from "hardhat";
import { HardhatRuntimeEnvironment } from "hardhat/types";
import { DeployFunction } from "hardhat-deploy/types";

import { getRequiredEnvVar, LogContractDeployment, tryVerifyContractWithConstructorArgs } from "../common/helpers";
import { getUiSigner, withSignerUiSession } from "../scripts/hardhat/signer-ui-bridge";
import { deployFromFactory } from "../scripts/hardhat/utils";

const func: DeployFunction = withSignerUiSession(
  "22_deploy_YieldManagerUpgrade.ts",
  async function (hre: HardhatRuntimeEnvironment) {
    const lineaRollupAddress = getRequiredEnvVar("LINEA_ROLLUP_ADDRESS");
    const yieldManagerProxyAddress = getRequiredEnvVar("YIELD_MANAGER_ADDRESS");

    console.log("Deploying Contract...");
    const signer = await getUiSigner(hre);
    const contractName = "YieldManager";
    const contract = await deployFromFactory(contractName, signer, lineaRollupAddress);
    const newYieldManagerImplementationAddress = await contract.getAddress();
    await LogContractDeployment(contractName, contract);
    await tryVerifyContractWithConstructorArgs(
      newYieldManagerImplementationAddress,
      "src/yield/YieldManager.sol:YieldManager",
      [lineaRollupAddress],
    );

    // Encodes the upgrade calldata to be executed through the Security Council Safe.
    // upgrade(address proxy, address implementation) - selector 0x99a88ec4
    // https://www.4byte.directory/signatures/?bytes4_signature=0x99a88ec4
    const upgradeCallUsingSecurityCouncil = ethers.concat([
      "0x99a88ec4",
      ethers.AbiCoder.defaultAbiCoder().encode(
        ["address", "address"],
        [yieldManagerProxyAddress, newYieldManagerImplementationAddress],
      ),
    ]);

    console.log("Encoded Tx Upgrade from Security Council:", "\n", upgradeCallUsingSecurityCouncil);
    console.log("\n");
  },
);

export default func;
func.tags = ["YieldManagerImplementation"];
