import { ethers, upgrades } from "hardhat";

import { warnIfUsingPrivateKeySigning } from "../hardhat/signer-ui-bridge";
import { requireEnv } from "../hardhat/utils";

/*
    *******************************************************************************************
    If upgrading a to a timelock controller 
    be sure to deploy the timelock controller and
    use the timelock controller address for the PROXY_ADMIN_OWNER_ADDRESS
    *******************************************************************************************

    NB: Be sure of who owns the Timelock before transferring admin to the
    timelock controller. There is the potential to brick ownership

    
    *******************************************************************************************
    NEW_PROXY_ADMIN_ADDRESS=0x.. PROXY_ADDRESS=0x.. CONTRACT_TYPE=TokenBridge pnpm exec hardhat run --network zkevm_dev scripts/operational/changeProxyAdmin.ts
    *******************************************************************************************
*/

async function main() {
  warnIfUsingPrivateKeySigning({
    scriptContext: "scripts/operational/changeProxyAdmin.ts",
    uiSupported: false,
  });

  const newProxyAdmin = requireEnv("NEW_PROXY_ADMIN_ADDRESS");
  const proxyAddress = requireEnv("PROXY_ADDRESS");
  const contractType = requireEnv("CONTRACT_TYPE");

  const proxyContract = await ethers.getContractFactory(contractType);
  await upgrades.forceImport(proxyAddress, proxyContract, {
    kind: "transparent",
  });

  // // CHANGE OWNERSHIP OF PROXY ADMIN
  console.log(`Changing ProxyAdmin of ${contractType} at ${proxyAddress} to new ProxyAdmin: ${newProxyAdmin}`);
  await upgrades.admin.changeProxyAdmin(proxyAddress, newProxyAdmin);
  console.log("Done");
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
