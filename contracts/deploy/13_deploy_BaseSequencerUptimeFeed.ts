import { HardhatRuntimeEnvironment } from "hardhat/types";
import { DeployFunction } from "hardhat-deploy/types";

import { tryVerifyContractWithConstructorArgs, getRequiredEnvVar, LogContractDeployment } from "../common/helpers";
import { getUiSigner, withSignerUiSession } from "../scripts/hardhat/signer-ui-bridge";
import { deployFromFactory } from "../scripts/hardhat/utils";

const func: DeployFunction = withSignerUiSession(
  "13_deploy_BaseSequencerUptimeFeed.ts",
  async function (hre: HardhatRuntimeEnvironment) {
    const contractName = "LineaSequencerUptimeFeed";
    const signer = await getUiSigner(hre);

    const initialStatus = getRequiredEnvVar("LINEA_SEQUENCER_UPTIME_FEED_INITIAL_STATUS");
    const adminAddress = getRequiredEnvVar("LINEA_SEQUENCER_UPTIME_FEED_ADMIN");
    const feedUpdaterAddress = getRequiredEnvVar("LINEA_SEQUENCER_UPTIME_FEED_UPDATER");

    const args = [initialStatus, adminAddress, feedUpdaterAddress];

    const contract = await deployFromFactory(contractName, signer, ...args);

    await LogContractDeployment(contractName, contract);
    const contractAddress = await contract.getAddress();

    await tryVerifyContractWithConstructorArgs(
      contractAddress,
      "src/operational/LineaSequencerUptimeFeed.sol:LineaSequencerUptimeFeed",
      args,
    );
  },
);
export default func;
func.tags = ["LineaSequencerUptimeFeed"];
