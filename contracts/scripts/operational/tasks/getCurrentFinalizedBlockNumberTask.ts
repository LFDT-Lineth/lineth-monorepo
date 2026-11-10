import { getAddress } from "viem";

import { getTaskCliOrEnvValue } from "../../../common/helpers/environmentHelper.js";

import type { HardhatRuntimeEnvironment } from "hardhat/types/hre";
import type { TaskArguments } from "hardhat/types/tasks";

/*
    *******************************************************************************************
    1. Set the CONTRACT_TYPE of the proxy - e.g. LinethRollup
    2. Set the PROXY_ADDRESS for the contract 
    *******************************************************************************************

    *******************************************************************************************
    DEPLOYER_PRIVATE_KEY=<key> \
    INFURA_API_KEY=<key> \
    pnpm exec hardhat getCurrentFinalizedBlockNumber \
    --contract-type <string> \
    --proxy-address <address> \
    --network sepolia
    *******************************************************************************************
*/

export default async function getCurrentFinalizedBlockNumber(taskArgs: TaskArguments, hre: HardhatRuntimeEnvironment) {
  const { viem } = await hre.network.getOrCreate();

  const contractType = getTaskCliOrEnvValue(taskArgs, "contractType", "CONTRACT_TYPE");
  const proxyAddress = getTaskCliOrEnvValue(taskArgs, "proxyAddress", "PROXY_ADDRESS");

  if (!contractType || !proxyAddress) {
    throw new Error(`PROXY_ADDRESS and CONTRACT_NAME env variables are undefined.`);
  }

  const LinethRollupContract = await viem.getContractAt(contractType, getAddress(proxyAddress));
  const blockNum = await LinethRollupContract.read.currentL2BlockNumber();

  console.log("Current finalized L2 block number", blockNum);
}
