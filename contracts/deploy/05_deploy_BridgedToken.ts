import { upgrades as createUpgrades } from "@openzeppelin/hardhat-upgrades";
import hre, { network as hardhatNetwork } from "hardhat";

import { getBooleanEnvVarOrDefault, tryVerifyContract, setHandoffAddress } from "../common/helpers";
import { deployScript } from "../rocketh/deploy";
import {
  clearUiWorkflowStatus,
  getUiSigner,
  setUiWorkflowStatus,
  withSignerUiSession,
} from "../scripts/hardhat/signer-ui-bridge";
import { BridgedToken } from "../typechain-types";

const hardhatConnection = await hardhatNetwork.getOrCreate();
const { ethers } = hardhatConnection;
const networkName = hardhatConnection.networkName === "default" ? "hardhat" : hardhatConnection.networkName;
const upgrades = await createUpgrades(hre, hardhatConnection);

const func = withSignerUiSession("05_deploy_BridgedToken.ts", async function () {
  const signer = await getUiSigner();
  const contractName = "BridgedToken";

  const chainId = (await ethers.provider.getNetwork()).chainId;
  console.log(`Current network's chainId is ${chainId}`);

  // Deploy beacon for bridged token
  const BridgedToken = await ethers.getContractFactory(contractName, signer);

  let bridgedToken: BridgedToken;
  await setUiWorkflowStatus("waiting_for_transaction_receipt", `Waiting for transaction receipt for ${contractName}.`);
  try {
    bridgedToken = (await upgrades.deployBeacon(BridgedToken)) as unknown as BridgedToken;
    await bridgedToken.waitForDeployment();
  } finally {
    await clearUiWorkflowStatus();
  }

  const bridgedTokenAddress = await bridgedToken.getAddress();
  setHandoffAddress("BRIDGED_TOKEN_ADDRESS", bridgedTokenAddress);

  // @ts-expect-error - deployTransaction is not a standard property but exists in this plugin's return type
  const deployTx = bridgedToken.deployTransaction;
  if (!deployTx) {
    throw "Contract deployment transaction receipt not found.";
  }

  if (getBooleanEnvVarOrDefault("DEPLOY_TOKEN_BRIDGE_ON_L1", false)) {
    console.log(`L1 BridgedToken beacon deployed on ${networkName}, at address:`, bridgedTokenAddress);
  } else {
    console.log(`L2 BridgedToken beacon deployed on ${networkName}, at address:`, bridgedTokenAddress);
  }

  await tryVerifyContract(bridgedTokenAddress);
});
export default deployScript(func, { tags: ["BridgedToken"] });
