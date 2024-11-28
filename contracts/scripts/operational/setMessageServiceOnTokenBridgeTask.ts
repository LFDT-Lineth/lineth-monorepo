// import { ethers, network, upgrades } from "hardhat";
import { task } from "hardhat/config";
import { TokenBridge } from "../../typechain-types";
import { getTaskCliOrEnvValue } from "../../common/helpers/environmentHelper";
import { getDeployedContractOnNetwork } from "../../common/helpers/readAddress";

/*
    *******************************************************************************************
    1. Deploy the TokenBridge + MessageService contracts on both networks and get the addresses
    2. Run this script on both addresses with the correct variables set.
    *******************************************************************************************
    SEPOLIA_PRIVATE_KEY=<key> \
    INFURA_API_KEY=<key> \
    npx hardhat setMessageServiceOnTokenBridge \
    --message-service-address <address> \
    --token-bridge-address <address> \
    --network sepolia
    *******************************************************************************************
*/

task("setMessageServiceOnTokenBridge", "Sets The Message Service On A TokenBridge")
  .addOptionalParam("messageServiceAddress")
  .addOptionalParam("tokenBridgeAddress")
  .setAction(async (taskArgs, hre) => {
    const ethers = hre.ethers;

    const messageServiceAddress = getTaskCliOrEnvValue(taskArgs, "messageServiceAddress", "MESSAGE_SERVICE_ADDRESS");
    if (!messageServiceAddress) {
      throw "messageServiceAddress is undefined";
    }

    let tokenBridgeAddress = getTaskCliOrEnvValue(taskArgs, "tokenBridgeAddress", "TOKEN_BRIDGE_ADDRESS");
    if (!tokenBridgeAddress) {
      tokenBridgeAddress = await getDeployedContractOnNetwork(hre.network.name, "TokenBridge");
      if (!tokenBridgeAddress) {
        throw "tokenBridgeAddress is undefined";
      }
    }

    const chainId = (await ethers.provider.getNetwork()).chainId;
    console.log(`Current network's chainId is ${chainId}`);

    const TokenBridge = await ethers.getContractFactory("TokenBridge");
    const tokenBridge = TokenBridge.attach(tokenBridgeAddress) as TokenBridge;
    const tx = await tokenBridge.setMessageService(messageServiceAddress!);

    await tx.wait();

    console.log(`MessageService set for the TokenBridge on: ${hre.network.name}`);
  });
