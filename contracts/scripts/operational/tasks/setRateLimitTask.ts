import { getAddress } from "viem";

import { getTaskCliOrEnvValue } from "../../../common/helpers/environmentHelper.js";
import { getUiWalletClient, withSignerUi } from "../../../hardhat-plugins/signer-ui/session.js";
import { requireRegistryAddress } from "../registryAddress.js";

import type { HardhatRuntimeEnvironment } from "hardhat/types/hre";
import type { TaskArguments } from "hardhat/types/tasks";

/*
    *******************************************************************************************
    1. Set the MESSAGE_SERVICE_ADDRESS 
    2. Set the MESSAGE_SERVICE_TYPE ( e.g. L2MessageService )
    3. Set the WITHDRAW_LIMIT_IN_WEI value
    *******************************************************************************************

    *******************************************************************************************
    DEPLOYER_PRIVATE_KEY=<key> \
    INFURA_API_KEY=<key> \
    pnpm exec hardhat setRateLimit \
    --message-service-address <address> \
    --message-service-type <string> \
    --withdraw-limit <uint256> \
    --network linea_sepolia
    *******************************************************************************************
*/

export default async function setRateLimit(taskArgs: TaskArguments, hre: HardhatRuntimeEnvironment) {
  const connection = await hre.network.getOrCreate();
  return withSignerUi(hre, "task:setRateLimit", async () => {
    const { viem, networkName } = connection;

    const messageServiceContractType = getTaskCliOrEnvValue(taskArgs, "messageServiceType", "MESSAGE_SERVICE_TYPE");
    let messageServiceAddress = getTaskCliOrEnvValue(taskArgs, "messageServiceAddress", "MESSAGE_SERVICE_ADDRESS");

    if (messageServiceContractType === undefined) {
      throw "Please specify a Message Service name e.g: --message-service-type LinethRollup or MESSAGE_SERVICE_TYPE=LinethRollup";
    }

    if (messageServiceAddress === undefined) {
      messageServiceAddress = requireRegistryAddress(networkName, messageServiceContractType);
    }

    const withdrawLimitRaw = getTaskCliOrEnvValue(taskArgs, "withdrawLimit", "WITHDRAW_LIMIT_IN_WEI");
    if (withdrawLimitRaw === undefined || String(withdrawLimitRaw).trim() === "") {
      throw new Error(
        "Missing withdraw limit. Pass --withdraw-limit <wei> or set WITHDRAW_LIMIT_IN_WEI (Hardhat exposes this as withdrawLimit on the task).",
      );
    }
    const withdrawLimitInWei = BigInt(String(withdrawLimitRaw).trim());

    const wallet = await getUiWalletClient(connection);
    const publicClient = await viem.getPublicClient();
    const messageService = await viem.getContractAt(messageServiceContractType, getAddress(messageServiceAddress), {
      client: { public: publicClient, wallet },
    });

    // get existing limit
    const limitInWei = await messageService.read.limitInWei();
    console.log(
      `Starting with rate limit in wei of ${limitInWei} at ${messageServiceAddress} of type ${messageServiceContractType}`,
    );

    // set limit

    const hash = await messageService.write.resetRateLimitAmount([withdrawLimitInWei]);
    await publicClient.waitForTransactionReceipt({ hash });

    console.log(`Changed rate limit in wei to ${withdrawLimitInWei} at ${messageServiceAddress}`);

    // get new updated limited
    const newLimitInWei = await messageService.read.limitInWei();
    console.log(`Validated rate limit in wei of ${newLimitInWei} at ${messageServiceAddress}`);
  });
}
