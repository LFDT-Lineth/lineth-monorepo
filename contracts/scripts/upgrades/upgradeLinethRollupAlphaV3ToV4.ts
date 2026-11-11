import { upgrades } from "@openzeppelin/hardhat-upgrades/viem";
import fs from "fs";
import hre, { network } from "hardhat";
import { getAddress } from "viem";

import { getRequiredEnvVar } from "../../common/helpers/environment.js";
import {
  getUiWalletClient,
  withSignerUi,
  warnIfUsingPrivateKeySigning,
} from "../../hardhat-plugins/signer-ui/session.js";

const OPENZEPPELIN_DIRECTORY = `${import.meta.dirname}/../../.openzeppelin`;

async function main() {
  const connection = await network.getOrCreate();
  warnIfUsingPrivateKeySigning({
    networkName: connection.networkName,
    scriptContext: "scripts/upgrades/upgradeLinethRollupAlphaV3ToV4.ts",
  });

  const newContractName = getRequiredEnvVar("NEW_CONTRACT_NAME");
  const oldContractName = getRequiredEnvVar("OLD_CONTRACT_NAME");
  const proxyAddress = getAddress(getRequiredEnvVar("PROXY_ADDRESS"));
  const initialShnarfs = process.env.INITIAL_SHNARFS;
  const initialShnarfsBlockNumbers = process.env.INITIAL_SHNARFS_BLOCK_NUMBERS;

  if (fs.existsSync(OPENZEPPELIN_DIRECTORY)) {
    fs.rmSync(OPENZEPPELIN_DIRECTORY, { recursive: true, force: true });
  }

  const initialShnarfsParam = initialShnarfs ? initialShnarfs.split(",") : [];
  const initialShnarfsBlockNumbersParam = initialShnarfsBlockNumbers ? initialShnarfsBlockNumbers.split(",") : [];

  console.log(`Upgrading contract at ${proxyAddress} from ${oldContractName} to ${newContractName}`);

  try {
    await withSignerUi(hre, "scripts/upgrades/upgradeLinethRollupAlphaV3ToV4.ts", async () => {
      const oz = await upgrades(hre, connection);
      const wallet = await getUiWalletClient(connection);
      const publicClient = await connection.viem.getPublicClient();

      console.log("Importing contract");
      await oz.forceImport(proxyAddress, oldContractName, {
        kind: "transparent",
        client: { public: publicClient, wallet },
      });

      await oz.upgradeProxy(proxyAddress, newContractName, {
        call: {
          fn: "initializeParentShnarfsAndFinalizedState",
          args: [initialShnarfsParam, initialShnarfsBlockNumbersParam],
        },
        kind: "transparent",
        client: { public: publicClient, wallet },
        // NOTE: THIS IS ONLY FOR TESTING.
        // TODO: this is because of the transient storage layout change.
        //  Openzeppelin upgradeProxy validation does not allow variable removal even if the storage layout didn't change.
        unsafeSkipStorageCheck: true,
      });
    });

    console.log(`Upgraded contract at ${proxyAddress} from ${oldContractName} to ${newContractName}`);
  } catch (error: unknown) {
    console.error("Failed to upgrade the proxy contract", error instanceof Error ? error.message : error);
    throw error;
  }
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
