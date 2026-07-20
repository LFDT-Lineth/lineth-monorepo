import { LineaRollup__factory } from "contracts/typechain-types";
import { ethers, upgrades } from "hardhat";
import { HardhatRuntimeEnvironment } from "hardhat/types";
import { DeployFunction } from "hardhat-deploy/types";

import { tryVerifyContract, requireAddressFromRegistryOrEnv } from "../common/helpers";
import { getUiSigner, withSignerUiSession } from "../scripts/hardhat/signer-ui-bridge";

const func: DeployFunction = withSignerUiSession(
  "04_deploy_LineaRollupV9WithReinitialization.ts",
  async function (hre: HardhatRuntimeEnvironment) {
    const signer = await getUiSigner(hre);

    const proxyAddress = requireAddressFromRegistryOrEnv(hre.network.name, "LineaRollup", "LINEA_ROLLUP_ADDRESS");

    const contractName = "LineaRollup";

    const factory = await ethers.getContractFactory(contractName, signer);

    console.log("Deploying new LineaRollup implementation...");
    const newImplementation = await upgrades.deployImplementation(factory, {
      kind: "transparent",
    });

    const implementationAddress = newImplementation.toString();
    console.log(`Implementation deployed at ${implementationAddress}`);

    // Encoded calldata for upgradeAndCall via ProxyAdmin (selector 0x9623609d).
    // Submit this through the Security Council Safe using upgradeAndCall on the ProxyAdmin.
    // See: https://www.4byte.directory/signatures/?bytes4_signature=0x9623609d
    const upgradeCallWithReinitialization = ethers.concat([
      "0x9623609d",
      ethers.AbiCoder.defaultAbiCoder().encode(
        ["address", "address", "bytes"],
        [
          proxyAddress,
          newImplementation,
          LineaRollup__factory.createInterface().encodeFunctionData("reinitializeLineaRollupV10"),
        ],
      ),
    ]);

    console.log("Encoded upgradeAndCall calldata for reinitializeLineaRollupV10 (ABI version bump only):");
    console.log("\n", upgradeCallWithReinitialization, "\n");

    await tryVerifyContract(implementationAddress);
  },
);

export default func;
func.tags = ["LineaRollupV9WithReinitialization"];
