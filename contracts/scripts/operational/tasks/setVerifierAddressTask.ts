import { getAddress } from "viem";

import { getTaskCliOrEnvValue } from "../../../common/helpers/environmentHelper.js";
import { getUiWalletClient, withSignerUi } from "../../../hardhat-plugins/signer-ui/session.js";
import { requireRegistryAddress } from "../registryAddress.js";

import type { HardhatRuntimeEnvironment } from "hardhat/types/hre";
import type { TaskArguments } from "hardhat/types/tasks";

/*
    *******************************************************************************************
    1. Deploy the verifier and get the address
    2. Run this script matching the correct PROOF_TYPE
    *******************************************************************************************

    *******************************************************************************************
    DEPLOYER_PRIVATE_KEY=<key> \
    INFURA_API_KEY=<key> \
    pnpm exec hardhat setVerifierAddress \
    --verifier-proof-type <uint256> \
    --proxy-address <address> \
    --verifier-address <address> \
    --verifier-name <string> \
    --network sepolia
    *******************************************************************************************
*/

export default async function setVerifierAddress(taskArgs: TaskArguments, hre: HardhatRuntimeEnvironment) {
  const connection = await hre.network.getOrCreate();
  return withSignerUi(hre, "task:setVerifierAddress", async () => {
    const { viem, networkName } = connection;

    const proofType = getTaskCliOrEnvValue(taskArgs, "verifierProofType", "VERIFIER_PROOF_TYPE");
    let LinethRollupAddress = getTaskCliOrEnvValue(taskArgs, "proxyAddress", "LINETH_ROLLUP_ADDRESS");
    const verifierName = getTaskCliOrEnvValue(taskArgs, "verifierName", "VERIFIER_CONTRACT_NAME");

    if (LinethRollupAddress === undefined) {
      LinethRollupAddress = requireRegistryAddress(networkName, "LinethRollup");
    }

    let verifierAddress = getTaskCliOrEnvValue(taskArgs, "verifierAddress", "VERIFIER_ADDRESS");
    if (verifierAddress === undefined) {
      if (verifierName === undefined) {
        throw "Please specify a verifier name e.g. --verifier-name PlonkVerifierDev";
      }
      verifierAddress = requireRegistryAddress(networkName, verifierName);
    }

    if (!proofType) {
      throw "Please specify a verifierProofType";
    }

    const wallet = await getUiWalletClient(connection);
    const publicClient = await viem.getPublicClient();
    const LinethRollup = await viem.getContractAt("LinethRollup", getAddress(LinethRollupAddress), {
      client: { public: publicClient, wallet },
    });

    console.log(`Setting verifier address ${verifierAddress} of type ${proofType}`);
    const hash = await LinethRollup.write.setVerifierAddress([getAddress(verifierAddress), BigInt(proofType)]);

    console.log("Waiting for transaction to process");
    await publicClient.waitForTransactionReceipt({ hash });

    const checkVerifierIsSet = await LinethRollup.read.verifiers([BigInt(proofType)]);
    console.log(`Lineth Rollup implementation added ${checkVerifierIsSet} as new verifier`);
  });
}
