import { ethers, upgrades } from "hardhat";
import { DeployFunction } from "hardhat-deploy/types";
import { HardhatRuntimeEnvironment } from "hardhat/types";
import {
  tryVerifyContract,
  getDeployedContractAddress,
  getRequiredEnvVar,
  generateRoleAssignments,
} from "../common/helpers";
import { L2MessageService__factory } from "contracts/typechain-types";
import {
  L2_MESSAGE_SERVICE_PAUSE_TYPES_ROLES,
  L2_MESSAGE_SERVICE_UNPAUSE_TYPES_ROLES,
  PAUSE_ALL_ROLE,
  PAUSE_L1_L2_ROLE,
  PAUSE_L2_L1_ROLE,
  UNPAUSE_ALL_ROLE,
  UNPAUSE_L1_L2_ROLE,
  UNPAUSE_L2_L1_ROLE,
  USED_RATE_LIMIT_RESETTER_ROLE,
} from "contracts/common/constants";

const func: DeployFunction = async function (hre: HardhatRuntimeEnvironment) {
  const securityCouncilAddress = getRequiredEnvVar("L2MSGSERVICE_SECURITY_COUNCIL");

  const newRoles = [
    USED_RATE_LIMIT_RESETTER_ROLE,
    PAUSE_ALL_ROLE,
    PAUSE_L1_L2_ROLE,
    PAUSE_L2_L1_ROLE,
    UNPAUSE_ALL_ROLE,
    UNPAUSE_L1_L2_ROLE,
    UNPAUSE_L2_L1_ROLE,
  ];

  const newRoleAddresses = generateRoleAssignments(newRoles, securityCouncilAddress, []);
  console.log("New role addresses", newRoleAddresses);

  const { deployments } = hre;
  const contractName = "L2MessageService";
  const existingContractAddress = await getDeployedContractAddress(contractName, deployments);

  const proxyAddress = getRequiredEnvVar("L2_MESSAGE_SERVICE_ADDRESS");

  const factory = await ethers.getContractFactory(contractName);

  if (existingContractAddress === undefined) {
    console.log(`Deploying initial version, NB: the address will be saved if env SAVE_ADDRESS=true.`);
  } else {
    console.log(`Deploying new version, NB: ${existingContractAddress} will be overwritten if env SAVE_ADDRESS=true.`);
  }

  console.log("Deploying Contract...");
  const newContract = await upgrades.deployImplementation(factory, {
    kind: "transparent",
  });

  const contract = newContract.toString();

  console.log(`Contract deployed at ${contract}`);

  // The encoding should be used through the safe.
  // THIS IS JUST A SAMPLE AND WILL BE ADJUSTED WHEN NEEDED FOR GENERATING THE CALLDATA FOR THE UPGRADE CALL
  // https://www.4byte.directory/signatures/?bytes4_signature=0x9623609d
  const upgradeCallWithReinitializationUsingSecurityCouncil = ethers.concat([
    "0x9623609d",
    ethers.AbiCoder.defaultAbiCoder().encode(
      ["address", "address", "bytes"],
      [
        proxyAddress,
        newContract,
        L2MessageService__factory.createInterface().encodeFunctionData("reinitializePauseTypesAndPermissions", [
          newRoleAddresses,
          L2_MESSAGE_SERVICE_PAUSE_TYPES_ROLES,
          L2_MESSAGE_SERVICE_UNPAUSE_TYPES_ROLES,
        ]),
      ],
    ),
  ]);

  console.log(
    "Encoded Tx Upgrade with Reinitialization from Security Council:",
    "\n",
    upgradeCallWithReinitializationUsingSecurityCouncil,
  );
  console.log("\n");

  await tryVerifyContract(contract);
};

export default func;
func.tags = ["L2MessageServiceWithReinitialization"];
