import { parseChecksummedAddress } from "../../../common/helpers/addressRegistry.js";
import { getTaskCliOrEnvValue } from "../../../common/helpers/environmentHelper.js";
import { requireAddressFromRegistryOrEnv } from "../../../common/helpers/readAddress.js";
import { waitForSuccessfulReceipt } from "../../../common/helpers/receipt.js";
import { getUiWalletClient, withSignerUi } from "../../../hardhat-plugins/signer-ui/session.js";
import { requireRegistryAddress } from "../registryAddress.js";

import type { HardhatRuntimeEnvironment } from "hardhat/types/hre";
import type { TaskArguments } from "hardhat/types/tasks";

/*
    *******************************************************************************************
    1. Deploy the verifier and get the address
    2. Run this script matching the correct PROOF_TYPE
       (without --verifier-address, VERIFIER_ADDRESS from the address registry or env is used)
    *******************************************************************************************

    *******************************************************************************************
    DEPLOYER_PRIVATE_KEY=<key> \
    INFURA_API_KEY=<key> \
    pnpm exec hardhat setVerifierAddress \
    --verifier-proof-type <uint256> \
    --proxy-address <address> \
    --verifier-address <address> \
    --network sepolia
    *******************************************************************************************
*/

export default async function setVerifierAddress(taskArgs: TaskArguments, hre: HardhatRuntimeEnvironment) {
  const connection = await hre.network.getOrCreate();
  return withSignerUi(hre, "task:setVerifierAddress", async () => {
    const { viem, networkName } = connection;

    const proofType = getTaskCliOrEnvValue(taskArgs, "verifierProofType", "VERIFIER_PROOF_TYPE");
    const linethRollupAddressInput = getTaskCliOrEnvValue(taskArgs, "proxyAddress", "LINETH_ROLLUP_ADDRESS");
    const linethRollupAddress =
      linethRollupAddressInput === undefined
        ? requireRegistryAddress(networkName, "LinethRollup")
        : parseChecksummedAddress(linethRollupAddressInput, "--proxy-address / LINETH_ROLLUP_ADDRESS");

    // The VERIFIER_ADDRESS env var is resolved (and checked against the registry) by requireAddressFromRegistryOrEnv.
    const verifierAddressOption: string | undefined = taskArgs.verifierAddress;
    const verifierAddress =
      verifierAddressOption === undefined
        ? requireAddressFromRegistryOrEnv(networkName, "VERIFIER_ADDRESS", "VERIFIER_ADDRESS")
        : parseChecksummedAddress(verifierAddressOption, "--verifier-address");

    if (!proofType) {
      throw "Please specify a verifierProofType";
    }

    const wallet = await getUiWalletClient(connection);
    const publicClient = await viem.getPublicClient();
    const linethRollup = await viem.getContractAt("LinethRollup", linethRollupAddress, {
      client: { public: publicClient, wallet },
    });

    console.log(`Setting verifier address ${verifierAddress} of type ${proofType}`);
    const hash = await linethRollup.write.setVerifierAddress([verifierAddress, BigInt(proofType)]);

    console.log("Waiting for transaction to process");
    await waitForSuccessfulReceipt(publicClient, hash);

    const checkVerifierIsSet = await linethRollup.read.verifiers([BigInt(proofType)]);
    console.log(`Lineth Rollup implementation added ${checkVerifierIsSet} as new verifier`);
  });
}
