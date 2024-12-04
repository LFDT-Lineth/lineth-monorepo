import { ethers, upgrades } from "hardhat";
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
    PROXY_ADMIN_OWNER_ADDRESS=0x.. PROXY_ADDRESS=0x.. CONTRACT_TYPE=TokenBridge npx hardhat run --network zkevm_dev scripts/operational/transferProxyAdminOwnership.ts
    *******************************************************************************************
*/

async function main() {
  const proxyAdminOwnerAddress = requireEnv("PROXY_ADMIN_OWNER_ADDRESS");
  const proxyAddress = requireEnv("PROXY_ADDRESS");
  const contractType = requireEnv("CONTRACT_TYPE");

  const proxyContract = await ethers.getContractFactory(contractType);
  await upgrades.forceImport(proxyAddress, proxyContract, {
    kind: "transparent",
  });

  // // CHANGE OWNERSHIP OF PROXY ADMIN
  console.log(`Changing proxy admin of ${contractType} at ${proxyAddress} to owner: ${proxyAdminOwnerAddress}`);
  await upgrades.admin.transferProxyAdminOwnership(proxyAdminOwnerAddress);
  console.log("Done");
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
