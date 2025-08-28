import { DeployFunction } from "hardhat-deploy/types";
import { HardhatRuntimeEnvironment } from "hardhat/types";
import { deployUpgradableFromFactory } from "../scripts/hardhat/utils";
import {
  tryVerifyContract,
  getDeployedContractAddress,
  tryStoreAddress,
  LogContractDeployment,
} from "../common/helpers";
import { EMPTY_INITIALIZE_SIGNATURE } from "../common/constants";

const func: DeployFunction = async function (hre: HardhatRuntimeEnvironment) {
  const { deployments } = hre;

  const contractName = "UpgradeableBeaconChainDepositPredeploy";
  const existingContractAddress = await getDeployedContractAddress(contractName, deployments);

  // UpgradeableBeaconChainDepositPredeploy DEPLOYED AS UPGRADEABLE PROXY
  if (!existingContractAddress) {
    console.log(`Deploying initial version, NB: the address will be saved if env SAVE_ADDRESS=true.`);
  } else {
    console.log(`Deploying new version, NB: ${existingContractAddress} will be overwritten if env SAVE_ADDRESS=true.`);
  }

  const contract = await deployUpgradableFromFactory("UpgradeableBeaconChainDepositPredeploy", [], {
    initializer: EMPTY_INITIALIZE_SIGNATURE,
    unsafeAllow: ["constructor"],
  });

  await LogContractDeployment(contractName, contract);
  const contractAddress = await contract.getAddress();

  await tryStoreAddress(hre.network.name, contractName, contractAddress, contract.deploymentTransaction()!.hash);

  await tryVerifyContract(
    contractAddress,
    "src/predeploy/UpgradeableBeaconChainDepositPredeploy.sol:UpgradeableBeaconChainDepositPredeploy",
  );
};

export default func;
func.tags = ["UpgradeableBeaconChainDepositPredeploy"];
