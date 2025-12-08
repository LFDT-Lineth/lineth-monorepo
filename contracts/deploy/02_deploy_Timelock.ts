import { ethers } from "hardhat";
import { DeployFunction } from "hardhat-deploy/types";
import { deployFromFactory } from "../scripts/hardhat/utils";
import { get1559Fees } from "../scripts/utils";
import { LogContractDeployment, getRequiredEnvVar, tryVerifyContractWithConstructorArgs } from "../common/helpers";

const func: DeployFunction = async function () {
  const contractName = "TimeLock";

  const provider = ethers.provider;

  // This should be the safe
  const timeLockProposers = getRequiredEnvVar("TIMELOCK_PROPOSERS");

  // This should be the safe
  const timelockExecutors = getRequiredEnvVar("TIMELOCK_EXECUTORS");

  // This should be the safe
  const adminAddress = getRequiredEnvVar("TIMELOCK_ADMIN_ADDRESS");

  const minDelay = process.env.MIN_DELAY || 0;

  const contract = await deployFromFactory(
    contractName,
    provider,
    minDelay,
    timeLockProposers?.split(","),
    timelockExecutors?.split(","),
    adminAddress,
    await get1559Fees(provider),
  );

  await LogContractDeployment(contractName, contract);
  const contractAddress = await contract.getAddress();

  const args = [minDelay, timeLockProposers?.split(","), timelockExecutors?.split(","), adminAddress];

  await tryVerifyContractWithConstructorArgs(contractAddress, "src/governance/TimeLock.sol:TimeLock", args);
};
export default func;
func.tags = ["Timelock"];
