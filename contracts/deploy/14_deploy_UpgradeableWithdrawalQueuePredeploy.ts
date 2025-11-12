import { DeployFunction } from "hardhat-deploy/types";
import { deployUpgradableFromFactory } from "../scripts/hardhat/utils";
import { tryVerifyContract, LogContractDeployment } from "../common/helpers";
import { EMPTY_INITIALIZE_SIGNATURE } from "../common/constants";

const func: DeployFunction = async function () {
  const contractName = "UpgradeableWithdrawalQueuePredeploy";

  const contract = await deployUpgradableFromFactory("UpgradeableWithdrawalQueuePredeploy", [], {
    initializer: EMPTY_INITIALIZE_SIGNATURE,
    unsafeAllow: ["constructor"],
  });

  await LogContractDeployment(contractName, contract);
  const contractAddress = await contract.getAddress();

  await tryVerifyContract(
    contractAddress,
    "src/predeploy/UpgradeableWithdrawalQueuePredeploy.sol:UpgradeableWithdrawalQueuePredeploy",
  );
};

export default func;
func.tags = ["UpgradeableWithdrawalQueuePredeploy"];
