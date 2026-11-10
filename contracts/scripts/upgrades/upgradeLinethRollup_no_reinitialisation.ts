import { upgrades } from "@openzeppelin/hardhat-upgrades/viem";
import hre, { network } from "hardhat";
import { getAddress } from "viem";

import { getRequiredEnvVar } from "../../common/helpers/environment.js";
import {
  getUiWalletClient,
  withSignerUi,
  warnIfUsingPrivateKeySigning,
} from "../../hardhat-plugins/signer-ui/session.js";

// NB: REMEMBER TO RENAME THE EXISTING CONTRACT TO SOMETHING ELSE TO RETAIN
// THE SAME NAME FOR THE CONTRACT GOING FORWARD
// THE TWO CONTRACTS MUST BE NAMED DIFFERENTLY

async function main() {
  const connection = await network.getOrCreate();
  warnIfUsingPrivateKeySigning({
    networkName: connection.networkName,
    scriptContext: "scripts/upgrades/upgradeLinethRollup_no_reinitialisation.ts",
  });

  const newContractName = getRequiredEnvVar("NEW_CONTRACT_NAME");
  const oldContractName = getRequiredEnvVar("OLD_CONTRACT_NAME");
  const proxyAddress = getAddress(getRequiredEnvVar("PROXY_ADDRESS"));

  if (!newContractName || !proxyAddress) {
    throw new Error(`PROXY_ADDRESS and CONTRACT_NAME env variables are undefined.`);
  }
  console.log(`Upgrading contract at ${proxyAddress}`);

  try {
    await withSignerUi(hre, "scripts/upgrades/upgradeLinethRollup_no_reinitialisation.ts", async () => {
      const oz = await upgrades(hre, connection);
      const wallet = await getUiWalletClient(connection);
      const publicClient = await connection.viem.getPublicClient();

      console.log("Importing contract");
      await oz.forceImport(proxyAddress, oldContractName, {
        kind: "transparent",
        client: { public: publicClient, wallet },
      });

      await oz.validateUpgrade(proxyAddress, newContractName, {
        kind: "transparent",
      });

      await oz.upgradeProxy(proxyAddress, newContractName, {
        kind: "transparent",
        client: { public: publicClient, wallet },
      });
    });

    console.log(`Upgraded contract at ${proxyAddress} with a new version of ${newContractName}`);
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
