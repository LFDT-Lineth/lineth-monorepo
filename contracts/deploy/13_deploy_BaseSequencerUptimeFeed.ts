import { ethers } from "hardhat";
import { DeployFunction } from "hardhat-deploy/types";
import { deployFromFactory } from "../scripts/hardhat/utils";
import { tryVerifyContractWithConstructorArgs, getRequiredEnvVar, LogContractDeployment } from "../common/helpers";

const func: DeployFunction = async function () {
  const contractName = "LineaSequencerUptimeFeed";
  const provider = ethers.provider;

  const initialStatus = getRequiredEnvVar("LINEA_SEQUENCER_UPTIME_FEED_INITIAL_STATUS");
  const adminAddress = getRequiredEnvVar("LINEA_SEQUENCER_UPTIME_FEED_ADMIN");
  const feedUpdaterAddress = getRequiredEnvVar("LINEA_SEQUENCER_UPTIME_FEED_UPDATER");

  const args = [initialStatus, adminAddress, feedUpdaterAddress];

  const contract = await deployFromFactory(contractName, provider, ...args);

  await LogContractDeployment(contractName, contract);
  const contractAddress = await contract.getAddress();

  await tryVerifyContractWithConstructorArgs(
    contractAddress,
    "src/operational/LineaSequencerUptimeFeed.sol:LineaSequencerUptimeFeed",
    args,
  );
};
export default func;
func.tags = ["LineaSequencerUptimeFeed"];
