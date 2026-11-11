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
    HARDHAT_SIGNER_UI: the OpenZeppelin admin helper signs with getUiWalletClient, so the
    transaction goes to the browser wallet when HARDHAT_SIGNER_UI=true.

    The ownership of the ProxyAdmin that administers PROXY_ADDRESS is transferred
    (OpenZeppelin Upgrades 4.x resolves the ProxyAdmin from the proxy's ERC-1967 admin slot).

    If upgrading a to a timelock controller 
    be sure to deploy the timelock controller and
    use the timelock controller address for the PROXY_ADMIN_OWNER_ADDRESS
    *******************************************************************************************

    NB: Be sure of who owns the Timelock before transferring admin to the
    timelock controller. There is the potential to brick ownership

    
    *******************************************************************************************
    PROXY_ADMIN_OWNER_ADDRESS=0x.. PROXY_ADDRESS=0x.. CONTRACT_TYPE=TokenBridge pnpm exec hardhat run --network zkevm_dev scripts/operational/transferProxyAdminOwnership.ts
    *******************************************************************************************
*/

async function main() {
  const connection = await network.getOrCreate();
  warnIfUsingPrivateKeySigning({
    networkName: connection.networkName,
    scriptContext: "scripts/operational/transferProxyAdminOwnership.ts",
  });

  const proxyAdminOwnerAddress = getAddress(getRequiredEnvVar("PROXY_ADMIN_OWNER_ADDRESS"));
  const proxyAddress = getAddress(getRequiredEnvVar("PROXY_ADDRESS"));
  const contractType = getRequiredEnvVar("CONTRACT_TYPE");

  await withSignerUi(hre, "scripts/operational/transferProxyAdminOwnership.ts", async () => {
    const oz = await upgrades(hre, connection);
    const wallet = await getUiWalletClient(connection);
    const publicClient = await connection.viem.getPublicClient();

    await oz.forceImport(proxyAddress, contractType, {
      kind: "transparent",
      client: { public: publicClient, wallet },
    });

    // CHANGE OWNERSHIP OF PROXY ADMIN
    console.log(
      `Changing ProxyAdmin owner of ${contractType} at ${proxyAddress} to new owner: ${proxyAdminOwnerAddress}`,
    );
    await oz.admin.transferProxyAdminOwnership(proxyAddress, proxyAdminOwnerAddress, wallet);
    console.log("Done");
  });
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
