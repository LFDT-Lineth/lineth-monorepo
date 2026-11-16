import { parseChecksummedAddress } from "../../../common/helpers/addressRegistry.js";
import { getTaskCliOrEnvValue } from "../../../common/helpers/environmentHelper.js";
import { waitForSuccessfulReceipt } from "../../../common/helpers/receipt.js";
import { getUiWalletClient, withSignerUi } from "../../../hardhat-plugins/signer-ui/session.js";
import { requireRegistryAddress } from "../registryAddress.js";

import type { HardhatRuntimeEnvironment } from "hardhat/types/hre";
import type { TaskArguments } from "hardhat/types/tasks";

/*
    *******************************************************************************************
    1. Deploy the TokenBridge + MessageService contracts on both networks and get the addresses
    2. Run this script on both addresses with the correct variables set.
    *******************************************************************************************
    DEPLOYER_PRIVATE_KEY=<key> \
    INFURA_API_KEY=<key> \
    pnpm exec hardhat setMessageServiceOnTokenBridge \
    --message-service-address <address> \
    --token-bridge-address <address> \
    --network sepolia
    *******************************************************************************************
*/

export default async function setMessageServiceOnTokenBridge(taskArgs: TaskArguments, hre: HardhatRuntimeEnvironment) {
  const connection = await hre.network.getOrCreate();
  return withSignerUi(hre, "task:setMessageServiceOnTokenBridge", async () => {
    const { viem, networkName } = connection;

    const messageServiceAddressInput = getTaskCliOrEnvValue(
      taskArgs,
      "messageServiceAddress",
      "MESSAGE_SERVICE_ADDRESS",
    );
    if (!messageServiceAddressInput) {
      throw "messageServiceAddress is undefined";
    }
    const messageServiceAddress = parseChecksummedAddress(
      messageServiceAddressInput,
      "--message-service-address / MESSAGE_SERVICE_ADDRESS",
    );

    const tokenBridgeAddressInput = getTaskCliOrEnvValue(taskArgs, "tokenBridgeAddress", "TOKEN_BRIDGE_ADDRESS");
    const tokenBridgeAddress = tokenBridgeAddressInput
      ? parseChecksummedAddress(tokenBridgeAddressInput, "--token-bridge-address / TOKEN_BRIDGE_ADDRESS")
      : requireRegistryAddress(networkName, "TokenBridge");

    const publicClient = await viem.getPublicClient();
    const chainId = await publicClient.getChainId();
    console.log(`Current network's chainId is ${chainId}`);

    const wallet = await getUiWalletClient(connection);
    const tokenBridge = await viem.getContractAt("TokenBridge", tokenBridgeAddress, {
      client: { public: publicClient, wallet },
    });
    const hash = await tokenBridge.write.setMessageService([messageServiceAddress]);

    await waitForSuccessfulReceipt(publicClient, hash);

    console.log(`MessageService set for the TokenBridge on: ${networkName}`);
  });
}
