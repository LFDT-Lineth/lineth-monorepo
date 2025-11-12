import { ethers, upgrades } from "hardhat";
import { DeployFunction } from "hardhat-deploy/types";
import { tryVerifyContract, getRequiredEnvVar, generateRoleAssignments } from "../common/helpers";
import { TokenBridge__factory } from "contracts/typechain-types";
import {
  PAUSE_ALL_ROLE,
  PAUSE_COMPLETE_TOKEN_BRIDGING_ROLE,
  PAUSE_INITIATE_TOKEN_BRIDGING_ROLE,
  REMOVE_RESERVED_TOKEN_ROLE,
  SET_CUSTOM_CONTRACT_ROLE,
  SET_MESSAGE_SERVICE_ROLE,
  SET_REMOTE_TOKENBRIDGE_ROLE,
  SET_RESERVED_TOKEN_ROLE,
  TOKEN_BRIDGE_PAUSE_TYPES_ROLES,
  TOKEN_BRIDGE_UNPAUSE_TYPES_ROLES,
  UNPAUSE_ALL_ROLE,
  UNPAUSE_COMPLETE_TOKEN_BRIDGING_ROLE,
  UNPAUSE_INITIATE_TOKEN_BRIDGING_ROLE,
} from "contracts/common/constants";

const func: DeployFunction = async function () {
  let securityCouncilAddress;

  if (process.env.TOKEN_BRIDGE_L1 === "true") {
    securityCouncilAddress = getRequiredEnvVar("L1_TOKEN_BRIDGE_SECURITY_COUNCIL");
  } else {
    securityCouncilAddress = getRequiredEnvVar("L2_TOKEN_BRIDGE_SECURITY_COUNCIL");
  }

  const newRoles = [
    PAUSE_ALL_ROLE,
    UNPAUSE_ALL_ROLE,
    PAUSE_INITIATE_TOKEN_BRIDGING_ROLE,
    UNPAUSE_INITIATE_TOKEN_BRIDGING_ROLE,
    PAUSE_COMPLETE_TOKEN_BRIDGING_ROLE,
    UNPAUSE_COMPLETE_TOKEN_BRIDGING_ROLE,
    SET_CUSTOM_CONTRACT_ROLE,
    REMOVE_RESERVED_TOKEN_ROLE,
    SET_MESSAGE_SERVICE_ROLE,
    SET_REMOTE_TOKENBRIDGE_ROLE,
    SET_RESERVED_TOKEN_ROLE,
  ];

  const newRoleAddresses = generateRoleAssignments(newRoles, securityCouncilAddress, []);
  console.log("New role addresses", newRoleAddresses);

  const contractName = "TokenBridge";

  const proxyAddress = getRequiredEnvVar("TOKEN_BRIDGE_ADDRESS");

  const factory = await ethers.getContractFactory(contractName);

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
        TokenBridge__factory.createInterface().encodeFunctionData("reinitializePauseTypesAndPermissions", [
          securityCouncilAddress,
          newRoleAddresses,
          TOKEN_BRIDGE_PAUSE_TYPES_ROLES,
          TOKEN_BRIDGE_UNPAUSE_TYPES_ROLES,
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
func.tags = ["TokenBridgeWithReinitialization"];
