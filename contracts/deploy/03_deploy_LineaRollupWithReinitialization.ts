import { ethers, upgrades } from "hardhat";
import { DeployFunction } from "hardhat-deploy/types";
import { tryVerifyContract, getRequiredEnvVar } from "../common/helpers";
import { LineaRollup__factory } from "contracts/typechain-types";
import {
  PAUSE_STATE_DATA_SUBMISSION_ROLE,
  UNPAUSE_STATE_DATA_SUBMISSION_ROLE,
  STATE_DATA_SUBMISSION_PAUSE_TYPE,
  SECURITY_COUNCIL_ROLE,
} from "contracts/common/constants";

const func: DeployFunction = async function () {
  let upgradePauseTypeRoles = [];
  let upgradeUnpauseTypeRoles = [];
  let upgradeRoleAddresses = [];

  const securityCouncilAddress = getRequiredEnvVar("LINEA_ROLLUP_SECURITY_COUNCIL");
  const proxyAddress = getRequiredEnvVar("LINEA_ROLLUP_ADDRESS");

  upgradeRoleAddresses = [
    {
      addressWithRole: securityCouncilAddress,
      role: SECURITY_COUNCIL_ROLE,
    },
    {
      addressWithRole: securityCouncilAddress,
      role: PAUSE_STATE_DATA_SUBMISSION_ROLE,
    },
    {
      addressWithRole: securityCouncilAddress,
      role: UNPAUSE_STATE_DATA_SUBMISSION_ROLE,
    },
  ];

  upgradePauseTypeRoles = [{ pauseType: STATE_DATA_SUBMISSION_PAUSE_TYPE, role: PAUSE_STATE_DATA_SUBMISSION_ROLE }];
  upgradeUnpauseTypeRoles = [{ pauseType: STATE_DATA_SUBMISSION_PAUSE_TYPE, role: UNPAUSE_STATE_DATA_SUBMISSION_ROLE }];

  const contractName = "LineaRollup";

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
        LineaRollup__factory.createInterface().encodeFunctionData("reinitializeV8", [
          upgradeRoleAddresses,
          upgradePauseTypeRoles,
          upgradeUnpauseTypeRoles,
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
