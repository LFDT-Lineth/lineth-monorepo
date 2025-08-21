import { ethers } from "hardhat";
import { DeployFunction } from "hardhat-deploy/types";
import { HardhatRuntimeEnvironment } from "hardhat/types";
import { deployFromFactory } from "../scripts/hardhat/utils";
import {
  tryVerifyContractWithConstructorArgs,
  getDeployedContractAddress,
  tryStoreAddress,
  getRequiredEnvVar,
  LogContractDeployment,
} from "../common/helpers";

const func: DeployFunction = async function (hre: HardhatRuntimeEnvironment) {
  const { deployments } = hre;

  const contractName = "LineaSequencerUptimeFeed";
  const existingContractAddress = await getDeployedContractAddress(contractName, deployments);
  const provider = ethers.provider;

  const initialStatus = getRequiredEnvVar("LINEA_SEQUENCER_UPTIME_FEED_INITIAL_STATUS");
  const adminAddress = getRequiredEnvVar("LINEA_SEQUENCER_UPTIME_FEED_ADMIN");
  const feedUpdaterAddress = getRequiredEnvVar("LINEA_SEQUENCER_UPTIME_FEED_UPDATER");

  if (existingContractAddress === undefined) {
    console.log(`Deploying initial version, NB: the address will be saved if env SAVE_ADDRESS=true.`);
  } else {
    console.log(`Deploying new version, NB: ${existingContractAddress} will be overwritten if env SAVE_ADDRESS=true.`);
  }

  const args = [initialStatus, adminAddress, feedUpdaterAddress];

  const contract = await deployFromFactory(contractName, provider, ...args);

  await LogContractDeployment(contractName, contract);
  const contractAddress = await contract.getAddress();

  await tryStoreAddress(hre.network.name, contractName, contractAddress, contract.deploymentTransaction()!.hash);

  await tryVerifyContractWithConstructorArgs(
    contractAddress,
    "src/operational/LineaSequencerUptimeFeed.sol:LineaSequencerUptimeFeed",
    args,
  );
};
export default func;
func.tags = ["LineaSequencerUptimeFeed"];
