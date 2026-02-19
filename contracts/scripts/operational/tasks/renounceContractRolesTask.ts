import { task } from "hardhat/config";
import { getTaskCliOrEnvValue } from "../../../common/helpers/environmentHelper";

/*
    *******************************************************************************************
    1. Set the OLD_ADMIN_ADDRESS to the EOA address
    2. Set the NEW_ADMIN_ADDRESS that you have just granted the roles to
    3. Set the PROXY_ADDRESS for the contract 
    4. Set the CONTRACT_TYPE of the proxy - e.g. LineaRollup
    5. Set the CONTRACT_ROLES comma separated, e.g "0x356a809dfdea9198dd76fb76bf6d403ecf13ea675eb89e1eda2db2c4a4676a26,0x1185e52d62bfbbea270e57d3d09733d221b53ab7a18bae82bb3c6c74bab16d82,0x0000000000000000000000000000000000000000000000000000000000000000"
    *******************************************************************************************
    NB: Be sure to have use the roles initially set to the security council EOA before changing

    DO NOT CALL THIS UNTIL YOU HAVE GIVEN THE NEW ADDRESS ALLL THE ROLES YOU ARE REVOKING

    MAKE SURE THAT THE DEFAULT ADMIN ROLE IS LAST AS IT IS REVOKING/RENOUNCING FROM SELF
    *******************************************************************************************
    DEPLOYER_PRIVATE_KEY=<key> \
    INFURA_API_KEY=<key> \
    npx hardhat renounceContractRoles \
    --old-admin-address <address>  \
    --new-admin-address <address>  \
    --proxy-address <address> \
    --contract-type <string> \
    --contract-roles <bytes> \
    --network sepolia
    *******************************************************************************************
*/

task("renounceContractRoles", "Sets the rate limit on a Message Service contract")
  .addOptionalParam("oldAdminAddress")
  .addOptionalParam("newAdminAddress")
  .addOptionalParam("proxyAddress")
  .addOptionalParam("contractType")
  .addOptionalParam("contractRoles")
  .setAction(async (taskArgs, hre) => {
    const ethers = hre.ethers;

    const { deployments, getNamedAccounts } = hre;
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    const { deployer } = await getNamedAccounts();
    const { get } = deployments;

    const oldAdminAddress = getTaskCliOrEnvValue(taskArgs, "oldAdminAddress", "OLD_ADMIN_ADDRESS");
    const newAdminAddress = getTaskCliOrEnvValue(taskArgs, "newAdminAddress", "NEW_ADMIN_ADDRESS");
    let proxyAddress = getTaskCliOrEnvValue(taskArgs, "proxyAddress", "PROXY_ADDRESS");
    const contractType = getTaskCliOrEnvValue(taskArgs, "contractType", "CONTRACT_TYPE");
    const contractRoles = getTaskCliOrEnvValue(taskArgs, "contractRoles", "CONTRACT_ROLES");

    if (contractType === undefined) {
      throw "Please specify a Message Service name e.g: --proxy-address LineaRollup or PROXY_ADDRESS=LineaRollup";
    }

    if (proxyAddress === undefined) {
      proxyAddress = (await get(contractType)).address;
      if (proxyAddress === undefined) {
        throw "proxyAddress is undefined";
      }
    }

    if (contractRoles === undefined) {
      throw "Please specify a role e.g. --contract-roles 0x9a80e24e463f00a8763c4dcec6a92d07d33272fa5db895d8589be70dccb002df or CONTRACT_ROLES=0x9a80e24e463f00a8763c4dcec6a92d07d33272fa5db895d8589be70dccb002df";
    }

    if (oldAdminAddress === undefined) {
      throw "Please specify an Old admin address e.g. --old-admin-address 0x5B38Da6a701c568545dCfcB03FcB875f56beddC4 or OLD_ADMIN_ADDRESS=0x5B38Da6a701c568545dCfcB03FcB875f56beddC4";
    }

    if (newAdminAddress === undefined) {
      throw "Please specify a New admin address e.g. --new-admin-address 0x5B38Da6a701c568545dCfcB03FcB875f56beddC4 or NEW_ADMIN_ADDRESS=0x5B38Da6a701c568545dCfcB03FcB875f56beddC4";
    }

    const contract = await ethers.getContractAt(contractType, proxyAddress);

    const rolesArray = contractRoles?.split(",");
    for (let i = 0; i < rolesArray.length; i++) {
      console.log(
        `Checking the new admin ${newAdminAddress} has ${rolesArray[i]} before revoking the old admin ${oldAdminAddress}`,
      );
      const newAdminHasRole = await contract.hasRole(rolesArray[i], newAdminAddress);
      if (newAdminHasRole) {
        console.log(`Revoking ${rolesArray[i]} from ${oldAdminAddress}`);
        const tx = await contract.renounceRole(rolesArray[i], oldAdminAddress);

        console.log("Waiting for transaction to process");
        await tx.wait();
      } else {
        console.log(`New admin does not have ${rolesArray[i]}, skipping`);
      }
    }

    console.log("Done");
  });
