import { ethers, upgrades } from "hardhat";
import { DeployFunction } from "hardhat-deploy/types";
import { tryVerifyContract, getRequiredEnvVar, generateRoleAssignments } from "../common/helpers";
import { LineaRollup__factory } from "contracts/typechain-types";
import {
  LINEA_ROLLUP_PAUSE_TYPES_ROLES,
  LINEA_ROLLUP_UNPAUSE_TYPES_ROLES,
  PAUSE_ALL_ROLE,
  PAUSE_BLOB_SUBMISSION_ROLE,
  PAUSE_FINALIZATION_ROLE,
  PAUSE_L1_L2_ROLE,
  PAUSE_L2_L1_ROLE,
  UNPAUSE_ALL_ROLE,
  UNPAUSE_BLOB_SUBMISSION_ROLE,
  UNPAUSE_FINALIZATION_ROLE,
  UNPAUSE_L1_L2_ROLE,
  UNPAUSE_L2_L1_ROLE,
  USED_RATE_LIMIT_RESETTER_ROLE,
  VERIFIER_UNSETTER_ROLE,
} from "contracts/common/constants";

const func: DeployFunction = async function () {
  const fallbackOperatorAddress = getRequiredEnvVar("LINEA_ROLLUP_FALLBACK_OPERATOR");
  const securityCouncilAddress = getRequiredEnvVar("LINEA_ROLLUP_SECURITY_COUNCIL");

  const newRoles = [
    PAUSE_ALL_ROLE,
    PAUSE_L1_L2_ROLE,
    PAUSE_L2_L1_ROLE,
    UNPAUSE_ALL_ROLE,
    UNPAUSE_L1_L2_ROLE,
    UNPAUSE_L2_L1_ROLE,
    PAUSE_BLOB_SUBMISSION_ROLE,
    UNPAUSE_BLOB_SUBMISSION_ROLE,
    PAUSE_FINALIZATION_ROLE,
    UNPAUSE_FINALIZATION_ROLE,
    USED_RATE_LIMIT_RESETTER_ROLE,
    VERIFIER_UNSETTER_ROLE,
  ];

  const newRoleAddresses = generateRoleAssignments(newRoles, securityCouncilAddress, []);
  console.log("New role addresses", newRoleAddresses);

  const contractName = "LineaRollup";

  const proxyAddress = getRequiredEnvVar("LINEA_ROLLUP_ADDRESS");

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
        LineaRollup__factory.createInterface().encodeFunctionData("reinitializeLineaRollupV6", [
          newRoleAddresses,
          LINEA_ROLLUP_PAUSE_TYPES_ROLES,
          LINEA_ROLLUP_UNPAUSE_TYPES_ROLES,
          fallbackOperatorAddress,
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
func.tags = ["LineaRollupWithReinitialization"];
