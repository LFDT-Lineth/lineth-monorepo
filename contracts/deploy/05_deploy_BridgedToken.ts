import { ethers, upgrades } from "hardhat";
import { HardhatRuntimeEnvironment } from "hardhat/types";
import { DeployFunction } from "hardhat-deploy/types";

import { tryVerifyContract } from "../common/helpers";
import {
  clearUiWorkflowStatus,
  getUiSigner,
  setUiWorkflowStatus,
  withSignerUiSession,
} from "../scripts/hardhat/signer-ui-bridge";
import { BridgedToken } from "../typechain-types";

const func: DeployFunction = withSignerUiSession(
  "05_deploy_BridgedToken.ts",
  async function (hre: HardhatRuntimeEnvironment) {
    const signer = await getUiSigner(hre);
    const contractName = "BridgedToken";

    const chainId = (await ethers.provider.getNetwork()).chainId;
    console.log(`Current network's chainId is ${chainId}`);

    // Deploy beacon for bridged token
    const BridgedToken = await ethers.getContractFactory(contractName, signer);

    let bridgedToken: BridgedToken;
    await setUiWorkflowStatus(
      "waiting_for_transaction_receipt",
      `Waiting for transaction receipt for ${contractName}.`,
    );
    try {
      bridgedToken = (await upgrades.deployBeacon(BridgedToken)) as unknown as BridgedToken;
      await bridgedToken.waitForDeployment();
    } finally {
      await clearUiWorkflowStatus();
    }

    const bridgedTokenAddress = await bridgedToken.getAddress();
    process.env.BRIDGED_TOKEN_ADDRESS = bridgedTokenAddress;

    // @ts-expect-error - deployTransaction is not a standard property but exists in this plugin's return type
    const deployTx = bridgedToken.deployTransaction;
    if (!deployTx) {
      throw "Contract deployment transaction receipt not found.";
    }

    if (process.env.TOKEN_BRIDGE_L1 === "true") {
      console.log(`L1 BridgedToken beacon deployed on ${hre.network.name}, at address:`, bridgedTokenAddress);
    } else {
      console.log(`L2 BridgedToken beacon deployed on ${hre.network.name}, at address:`, bridgedTokenAddress);
    }

    await tryVerifyContract(bridgedTokenAddress);
  },
);
export default func;
func.tags = ["BridgedToken"];
