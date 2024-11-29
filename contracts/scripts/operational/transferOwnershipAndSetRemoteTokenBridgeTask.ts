// import { ethers, network, upgrades } from "hardhat";
import { task } from "hardhat/config";
import { ProxyAdminReplica, TokenBridge } from "../../typechain-types";
import { getTaskCliOrEnvValue } from "../../common/helpers/environmentHelper";
import { getDeployedContractOnNetwork } from "../../common/helpers/readAddress";

/*
    *******************************************************************************************
    1. Deploy the TokenBridge and BridgedToken contracts on both networks and get the addresses
    2. Run this script on both addresses with the correct variables set.
    *******************************************************************************************
    SEPOLIA_PRIVATE_KEY=<key> \
    INFURA_API_KEY=<key> \
    npx hardhat transferOwnershipAndSetRemoteTokenBridge \
    --safe-address <address> \
    --remote-token-bridge-address <address> \
    --token-bridge-address <address> \
    --token-bridge-proxy-admin-address <address> \
    --remote-network sepolia \
    --network linea_sepolia
    *******************************************************************************************
*/

task(
  "transferOwnershipAndSetRemoteTokenBridge",
  "Transfers the ownership of TokenBridge and Proxy Admin to the Safe address and also sets the remoteTokenBridge address.",
)
  .addParam("safeAddress")
  .addOptionalParam("remoteTokenBridgeAddress")
  .addOptionalParam("tokenBridgeAddress")
  .addOptionalParam("tokenBridgeProxyAdminAddress")
  .addParam("remoteNetwork")
  .setAction(async (taskArgs, hre) => {
    const ethers = hre.ethers;

    let remoteTokenBridgeAddress = getTaskCliOrEnvValue(
      taskArgs,
      "remoteTokenBridgeAddress",
      "REMOTE_TOKEN_BRIDGE_ADDRESS",
    );
    let tokenBridgeAddress = getTaskCliOrEnvValue(taskArgs, "tokenBridgeAddress", "TOKEN_BRIDGE_ADDRESS");
    let tokenBridgeProxyAdmin = taskArgs.tokenBridgeProxyAdminAddress;

    if (tokenBridgeAddress === undefined) {
      tokenBridgeAddress = await getDeployedContractOnNetwork(hre.network.name, "TokenBridge");
      if (tokenBridgeAddress === undefined) {
        throw "tokenBridgeAddress is undefined";
      }
    }

    if (remoteTokenBridgeAddress === undefined) {
      remoteTokenBridgeAddress = await getDeployedContractOnNetwork(taskArgs.remoteNetwork, "TokenBridge");
      if (remoteTokenBridgeAddress === undefined) {
        throw "remoteTokenBridgeAddress is undefined";
      }
    }

    if (tokenBridgeProxyAdmin === undefined) {
      tokenBridgeProxyAdmin = await getDeployedContractOnNetwork(hre.network.name, "TokenBridgeProxyAdmin");
      if (tokenBridgeProxyAdmin === undefined) {
        throw "tokenBridgeProxyAdmin is undefined";
      }
    }

    const chainId = (await ethers.provider.getNetwork()).chainId;
    console.log(`Current network's chainId is ${chainId}`);

    const TokenBridge = await ethers.getContractFactory("TokenBridge");
    const tokenBridge = TokenBridge.attach(tokenBridgeAddress) as TokenBridge;
    const tx = await tokenBridge.setRemoteTokenBridge(remoteTokenBridgeAddress);

    await tx.wait();

    console.log(`RemoteTokenBridge set for the TokenBridge on: ${hre.network.name}`);

    const ProxyAdmin = await ethers.getContractFactory("ProxyAdminReplica");
    const proxyAdmin = ProxyAdmin.attach(tokenBridgeProxyAdmin) as ProxyAdminReplica;
    await proxyAdmin.transferOwnership(taskArgs.safeAddress);

    console.log(`TokenBridge ownership and proxy admin set to: ${taskArgs.safeAddress}`);
  });
