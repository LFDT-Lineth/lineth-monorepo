import { DeployFunction } from "hardhat-deploy/types";

import { EMPTY_INITIALIZE_SIGNATURE } from "../common/constants";
import { tryVerifyContract, LogContractDeployment } from "../common/helpers";
import { withSignerUiSession } from "../scripts/hardhat/signer-ui-bridge";
import { deployUpgradableFromFactory } from "../scripts/hardhat/utils";

const func: DeployFunction = withSignerUiSession("14_deploy_UpgradeableWithdrawalQueuePredeploy.ts", async function () {
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
});

export default func;
func.tags = ["UpgradeableWithdrawalQueuePredeploy"];
