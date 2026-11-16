import { upgrades } from "@openzeppelin/hardhat-upgrades/viem";
import hre, { network } from "hardhat";

import { parseChecksummedAddress } from "../../common/helpers/addressRegistry.js";
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
    scriptContext: "scripts/upgrades/upgradeL2MessageService.ts",
  });

  const newContractName = getRequiredEnvVar("NEW_CONTRACT_NAME");
  const proxyAddress = parseChecksummedAddress(getRequiredEnvVar("PROXY_ADDRESS"), "PROXY_ADDRESS");
  console.log(`Upgrading contract at ${proxyAddress}`);

  await withSignerUi(hre, "scripts/upgrades/upgradeL2MessageService.ts", async () => {
    const oz = await upgrades(hre, connection);
    const wallet = await getUiWalletClient(connection);
    const publicClient = await connection.viem.getPublicClient();

    await oz.validateUpgrade(proxyAddress, newContractName, {
      kind: "transparent",
    });

    await oz.upgradeProxy(proxyAddress, newContractName, {
      kind: "transparent",
      client: { public: publicClient, wallet },
    });
  });

  console.log(`Upgraded contract at ${proxyAddress} with a new version of ${newContractName}`);
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
