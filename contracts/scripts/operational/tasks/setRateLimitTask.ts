import { task } from "hardhat/config";
import { getTaskCliOrEnvValue } from "../../../common/helpers/environmentHelper";

/*
    *******************************************************************************************
    1. Set the MESSAGE_SERVICE_ADDRESS 
    2. Set the MESSAGE_SERVICE_TYPE ( e.g. L2MessageService )
    3. Set the WITHDRAW_LIMIT_IN_WEI value
    *******************************************************************************************

    *******************************************************************************************
    DEPLOYER_PRIVATE_KEY=<key> \
    INFURA_API_KEY=<key> \
    npx hardhat setRateLimit \
    --message-service-address <address> \
    --message-service-type <string> \
    --withdraw-limit <uint256> \
    --network linea_sepolia
    *******************************************************************************************
*/

task("setRateLimit", "Sets the rate limit on a Message Service contract")
  .addOptionalParam("messageServiceAddress")
  .addOptionalParam("messageServiceType")
  .addOptionalParam("withdrawLimit")
  .setAction(async (taskArgs, hre) => {
    const ethers = hre.ethers;

    const { deployments, getNamedAccounts } = hre;
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    const { deployer } = await getNamedAccounts();
    const { get } = deployments;

    const messageServiceContractType = getTaskCliOrEnvValue(taskArgs, "messageServiceType", "MESSAGE_SERVICE_TYPE");
    let messageServiceAddress = getTaskCliOrEnvValue(taskArgs, "messageServiceAddress", "MESSAGE_SERVICE_ADDRESS");

    if (messageServiceContractType === undefined) {
      throw "Please specify a Message Service name e.g: --message-service-type LineaRollup or MESSAGE_SERVICE_TYPE=LineaRollup";
    }

    if (messageServiceAddress === undefined) {
      messageServiceAddress = (await get(messageServiceContractType)).address;
    }

    const withdrawLimitInWei = getTaskCliOrEnvValue(taskArgs, "withdrawlimit", "WITHDRAW_LIMIT_IN_WEI");

    const messageService = await ethers.getContractAt(messageServiceContractType, messageServiceAddress);

    // get existing limit
    const limitInWei = await messageService.limitInWei();
    console.log(
      `Starting with rate limit in wei of ${limitInWei} at ${messageServiceAddress} of type ${messageServiceContractType}`,
    );

    // set limit

    const updateTx = await messageService.resetRateLimitAmount(withdrawLimitInWei);
    await updateTx.wait();

    console.log(`Changed rate limit in wei to ${withdrawLimitInWei} at ${messageServiceAddress}`);

    // get new updated limited
    const newLimitInWei = await messageService.limitInWei();
    console.log(`Validated rate limit in wei of ${newLimitInWei} at ${messageServiceAddress}`);
  });
