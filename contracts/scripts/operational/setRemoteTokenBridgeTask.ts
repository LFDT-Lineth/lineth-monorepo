// import { ethers, network, upgrades } from "hardhat";
import { task } from "hardhat/config";
import { TokenBridge } from "../../typechain-types";
import { getTaskCliOrEnvValue } from "../../common/helpers/environmentHelper";
import { getDeployedContractOnNetwork } from "../../common/helpers/readAddress";

/*
    *******************************************************************************************
    1. Deploy the TokenBridge and BridgedToken contracts on both networks and get the addresses
    2. Run this script on both addresses with the correct variables set.
    *******************************************************************************************
    SEPOLIA_PRIVATE_KEY=<key> \
    INFURA_API_KEY=<key> \
    npx hardhat setRemoteTokenBridge \
    --remote-token-bridge-address <address> \
    --token-bridge-address <address> \
    --remote-network sepolia \
    --network linea_sepolia
    *******************************************************************************************
*/

task("setRemoteTokenBridge", "Sets the remoteTokenBridge address.")
  .addOptionalParam("remoteTokenBridgeAddress")
  .addOptionalParam("tokenBridgeAddress")
  .addParam("remoteNetwork")
  .setAction(async (taskArgs, hre) => {
    const ethers = hre.ethers;

    let remoteTokenBridgeAddress = getTaskCliOrEnvValue(
      taskArgs,
      "remoteTokenBridgeAddress",
      "REMOTE_TOKEN_BRIDGE_ADDRESS",
    );

    let tokenBridgeAddress = getTaskCliOrEnvValue(taskArgs, "tokenBridgeAddress", "TOKEN_BRIDGE_ADDRESS");

    if (!tokenBridgeAddress) {
      tokenBridgeAddress = await getDeployedContractOnNetwork(hre.network.name, "TokenBridge");
      if (!tokenBridgeAddress) {
        throw "tokenBridgeAddress is undefined";
      }
    }

    if (!remoteTokenBridgeAddress) {
      remoteTokenBridgeAddress = await getDeployedContractOnNetwork(taskArgs.remoteNetwork, "TokenBridge");
      if (!remoteTokenBridgeAddress) {
        throw "remoteTokenBridgeAddress is undefined";
      }
    }

    const chainId = (await ethers.provider.getNetwork()).chainId;
    console.log(`Current network's chainId is ${chainId}`);

    const TokenBridge = await ethers.getContractFactory("TokenBridge");
    const tokenBridge = TokenBridge.attach(tokenBridgeAddress) as TokenBridge;
    const tx = await tokenBridge.setRemoteTokenBridge(remoteTokenBridgeAddress);

    await tx.wait();

    console.log(`RemoteTokenBridge set for the TokenBridge on: ${hre.network.name}`);
  });
