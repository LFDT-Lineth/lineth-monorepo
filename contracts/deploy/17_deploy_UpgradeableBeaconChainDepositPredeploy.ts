import { DeployFunction } from "hardhat-deploy/types";

import { EMPTY_INITIALIZE_SIGNATURE } from "../common/constants";
import { tryVerifyContract, LogContractDeployment } from "../common/helpers";
import { deployUpgradableFromFactory } from "../scripts/hardhat/utils";

const func: DeployFunction = async function () {
  const contractName = "UpgradeableBeaconChainDepositPredeploy";

  const contract = await deployUpgradableFromFactory("UpgradeableBeaconChainDepositPredeploy", [], {
    initializer: EMPTY_INITIALIZE_SIGNATURE,
    unsafeAllow: ["constructor"],
  });

  await LogContractDeployment(contractName, contract);
  const contractAddress = await contract.getAddress();

  await tryVerifyContract(
    contractAddress,
    "src/predeploy/UpgradeableBeaconChainDepositPredeploy.sol:UpgradeableBeaconChainDepositPredeploy",
  );
};

export default func;
func.tags = ["UpgradeableBeaconChainDepositPredeploy"];
