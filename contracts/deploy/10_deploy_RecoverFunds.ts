import { network } from "hardhat";
import { DeployFunction } from "hardhat-deploy/types";

import { tryVerifyContract, requireAddressFromRegistryOrEnv, LogContractDeployment } from "../common/helpers";
import { withSignerUiSession } from "../scripts/hardhat/signer-ui-bridge";
import { deployUpgradableFromFactory } from "../scripts/hardhat/utils";

const func: DeployFunction = withSignerUiSession("10_deploy_RecoverFunds.ts", async function () {
  const contractName = "RecoverFunds";

  // RecoverFunds DEPLOYED AS UPGRADEABLE PROXY
  const RecoverFunds_securityCouncil = requireAddressFromRegistryOrEnv(
    network.name,
    "L1_SECURITY_COUNCIL",
    "L1_SECURITY_COUNCIL",
  );
  const RecoverFunds_executorAddress = requireAddressFromRegistryOrEnv(
    network.name,
    "RECOVERFUNDS_EXECUTOR_ADDRESS",
    "RECOVERFUNDS_EXECUTOR_ADDRESS",
  );

  const contract = await deployUpgradableFromFactory(
    "RecoverFunds",
    [RecoverFunds_securityCouncil, RecoverFunds_executorAddress],
    {
      initializer: "initialize(address, address)",
      unsafeAllow: ["constructor"],
    },
  );

  await LogContractDeployment(contractName, contract);
  const contractAddress = await contract.getAddress();

  await tryVerifyContract(contractAddress);
});

export default func;
func.tags = ["RecoverFunds"];
