import { upgrades } from "@openzeppelin/hardhat-upgrades/viem";
import hre, { network } from "hardhat";
import { getAddress } from "viem";

import { getRequiredEnvVar } from "../../common/helpers/environment.js";
import {
  getUiWalletClient,
  withSignerUi,
  warnIfUsingPrivateKeySigning,
} from "../../hardhat-plugins/signer-ui/session.js";

/*
    *******************************************************************************************
    If upgrading a to a timelock controller 
    be sure to deploy the timelock controller and
    use the timelock controller address for the PROXY_ADMIN_OWNER_ADDRESS
    *******************************************************************************************

    NB: Be sure of who owns the Timelock before transferring admin to the
    timelock controller. There is the potential to brick ownership

    NB: OpenZeppelin Upgrades 4.x only supports changeProxyAdmin for proxies built from OpenZeppelin
    Contracts 4.x (e.g. the pinned OZ 4.x TransparentUpgradeableProxy used in production).

    
    *******************************************************************************************
    NEW_PROXY_ADMIN_ADDRESS=0x.. PROXY_ADDRESS=0x.. CONTRACT_TYPE=TokenBridge pnpm exec hardhat run --network zkevm_dev scripts/operational/changeProxyAdmin.ts
    *******************************************************************************************
*/

async function main() {
  const connection = await network.getOrCreate();
  warnIfUsingPrivateKeySigning({
    networkName: connection.networkName,
    scriptContext: "scripts/operational/changeProxyAdmin.ts",
  });

  const newProxyAdmin = getAddress(getRequiredEnvVar("NEW_PROXY_ADMIN_ADDRESS"));
  const proxyAddress = getAddress(getRequiredEnvVar("PROXY_ADDRESS"));
  const contractType = getRequiredEnvVar("CONTRACT_TYPE");

  await withSignerUi(hre, "scripts/operational/changeProxyAdmin.ts", async () => {
    const oz = await upgrades(hre, connection);
    const wallet = await getUiWalletClient(connection);
    const publicClient = await connection.viem.getPublicClient();

    await oz.forceImport(proxyAddress, contractType, {
      kind: "transparent",
      client: { public: publicClient, wallet },
    });

    // // CHANGE OWNERSHIP OF PROXY ADMIN
    console.log(`Changing ProxyAdmin of ${contractType} at ${proxyAddress} to new ProxyAdmin: ${newProxyAdmin}`);
    await oz.admin.changeProxyAdmin(proxyAddress, newProxyAdmin, wallet);
    console.log("Done");
  });
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
