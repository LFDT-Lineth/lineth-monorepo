import { ethers, upgrades } from "hardhat";
import { HardhatRuntimeEnvironment } from "hardhat/types";
import { DeployFunction } from "hardhat-deploy/types";

import { getRequiredEnvVar, tryVerifyContract } from "../common/helpers";
import { getUiSigner, withSignerUiSession } from "../scripts/hardhat/signer-ui-bridge";

const func: DeployFunction = withSignerUiSession(
  "03_deploy_ImplementationForProxy.ts",
  async function (hre: HardhatRuntimeEnvironment) {
    const signer = await getUiSigner(hre);
    const contractName = getRequiredEnvVar("CONTRACT_NAME");

    const proxyAddress = getRequiredEnvVar("PROXY_ADDRESS");

    const factory = await ethers.getContractFactory(contractName, signer);

    console.log("Deploying Contract...");
    const newContract = await upgrades.deployImplementation(factory, {
      kind: "transparent",
    });

    const contract = newContract.toString();

    console.log(`Contract deployed at ${contract}`);

    const upgradeCallUsingSecurityCouncil = ethers.concat([
      "0x99a88ec4",
      ethers.AbiCoder.defaultAbiCoder().encode(["address", "address"], [proxyAddress, newContract]),
    ]);

    console.log("Encoded Tx Upgrade from Security Council:", "\n", upgradeCallUsingSecurityCouncil);

    console.log("\n");

    await tryVerifyContract(contract);
  },
);

export default func;
func.tags = ["ImplementationForProxy"];
