import { task } from "hardhat/config";
import { getTaskCliOrEnvValue } from "../../common/helpers/environmentHelper";

/*
    *******************************************************************************************
    1. Set the CONTRACT_TYPE of the proxy - e.g. LineaRollup
    2. Set the PROXY_ADDRESS for the contract 
    *******************************************************************************************

    *******************************************************************************************
    SEPOLIA_PRIVATE_KEY=<key> \
    INFURA_API_KEY=<key> \
    npx hardhat getCurrentFinalizedBlockNumber \
    --contract-type <string> \
    --proxy-address <address> \
    --network sepolia
    *******************************************************************************************
*/

task("getCurrentFinalizedBlockNumber", "Gets the finalized block number")
  .addOptionalParam("contractType")
  .addOptionalParam("proxyAddress")
  .setAction(async (taskArgs, hre) => {
    const ethers = hre.ethers;

    const contractType = getTaskCliOrEnvValue(taskArgs, "contractType", "CONTRACT_TYPE");
    const proxyAddress = getTaskCliOrEnvValue(taskArgs, "proxyAddress", "PROXY_ADDRESS");

    if (!contractType || !proxyAddress) {
      throw new Error(`PROXY_ADDRESS and CONTRACT_NAME env variables are undefined.`);
    }

    const LineaRollupContract = await ethers.getContractAt(contractType, proxyAddress);
    const blockNum = await LineaRollupContract.currentL2BlockNumber();

    console.log("Current finalized L2 block number", blockNum);
  });
