import { ethers, upgrades } from "hardhat";
import { DeployFunction } from "hardhat-deploy/types";
import { HardhatRuntimeEnvironment } from "hardhat/types";
import { BridgedToken } from "../typechain-types";
import { tryVerifyContract, getDeployedContractAddress, tryStoreAddress } from "../common/helpers";

const func: DeployFunction = async function (hre: HardhatRuntimeEnvironment) {
  const { deployments } = hre;

  const contractName = "BridgedToken";
  const existingContractAddress = await getDeployedContractAddress(contractName, deployments);

  const chainId = (await ethers.provider.getNetwork()).chainId;
  console.log(`Current network's chainId is ${chainId}`);

  if (existingContractAddress === undefined) {
    console.log(`Deploying initial version, NB: the address will be saved if env SAVE_ADDRESS=true.`);
  } else {
    console.log(`Deploying new version, NB: ${existingContractAddress} will be overwritten if env SAVE_ADDRESS=true.`);
  }

  // Deploy beacon for bridged token
  const BridgedToken = await ethers.getContractFactory(contractName);

  const bridgedToken = (await upgrades.deployBeacon(BridgedToken)) as unknown as BridgedToken;

  await bridgedToken.waitForDeployment();

  const bridgedTokenAddress = await bridgedToken.getAddress();
  process.env.BRIDGED_TOKEN_ADDRESS = bridgedTokenAddress;

  // @ts-expect-error - deployTransaction is not a standard property but exists in this plugin's return type
  const deployTx = bridgedToken.deployTransaction;
  if (!deployTx) {
    throw "Contract deployment transaction receipt not found.";
  }

  await tryStoreAddress(hre.network.name, contractName, bridgedTokenAddress, deployTx.hash);

  if (process.env.TOKEN_BRIDGE_L1 === "true") {
    console.log(`L1 BridgedToken beacon deployed on ${hre.network.name}, at address:`, bridgedTokenAddress);
  } else {
    console.log(`L2 BridgedToken beacon deployed on ${hre.network.name}, at address:`, bridgedTokenAddress);
  }

  await tryVerifyContract(bridgedTokenAddress);
};
export default func;
func.tags = ["BridgedToken"];
