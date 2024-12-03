import { ethers } from "hardhat";
import { DeployFunction } from "hardhat-deploy/types";
import { HardhatRuntimeEnvironment } from "hardhat/types";
import { deployFromFactory } from "../scripts/hardhat/utils";
import { get1559Fees } from "../scripts/utils";
import {
  getDeployedContractAddress,
  getRequiredEnvVar,
  tryStoreAddress,
  tryVerifyContractWithConstructorArgs,
} from "../common/helpers";

const func: DeployFunction = async function (hre: HardhatRuntimeEnvironment) {
  const { deployments } = hre;

  const contractName = "TimeLock";
  const existingContractAddress = await getDeployedContractAddress(contractName, deployments);

  const provider = ethers.provider;

  // This should be the safe
  const timeLockProposers = getRequiredEnvVar("TIMELOCK_PROPOSERS");

  // This should be the safe
  const timelockExecutors = getRequiredEnvVar("TIMELOCK_EXECUTORS");

  // This should be the safe
  const adminAddress = getRequiredEnvVar("TIMELOCK_ADMIN_ADDRESS");

  const minDelay = process.env.MIN_DELAY || 0;

  if (existingContractAddress === undefined) {
    console.log(`Deploying initial version, NB: the address will be saved if env SAVE_ADDRESS=true.`);
  } else {
    console.log(`Deploying new version, NB: ${existingContractAddress} will be overwritten if env SAVE_ADDRESS=true.`);
  }
  const contract = await deployFromFactory(
    contractName,
    provider,
    minDelay,
    timeLockProposers?.split(","),
    timelockExecutors?.split(","),
    adminAddress,
    await get1559Fees(provider),
  );
  const contractAddress = await contract.getAddress();

  console.log(`${contractName} deployed at ${contractAddress}`);

  const deployTx = contract.deploymentTransaction();
  if (!deployTx) {
    throw "Deployment transaction not found.";
  }

  await tryStoreAddress(hre.network.name, contractName, contractAddress, deployTx.hash);
  const args = [minDelay, timeLockProposers?.split(","), timelockExecutors?.split(","), adminAddress];

  await tryVerifyContractWithConstructorArgs(
    contractAddress,
    "contracts/messageService/lib/TimeLock.sol:TimeLock",
    args,
  );
};
export default func;
func.tags = ["Timelock"];
