// Usage:
// npx hardhat deploy --network <network> --tags LineaRollupV7WithReinitialization
//
// Required environment variables:
//   LINEA_ROLLUP_SECURITY_COUNCIL
//   LINEA_ROLLUP_ADDRESS
//   YIELD_MANAGER_ADDRESS
//   NATIVE_YIELD_AUTOMATION_SERVICE_ADDRESS
//
// This script deploys LineaRollupV7 implementation from artifacts signed off by auditors
// and generates encoded calldata for upgrading an existing LineaRollup proxy with reinitialization.
// The encoded calldata should be used through the Safe for the actual upgrade.

import { DeployFunction } from "hardhat-deploy/types";
import {
  getRequiredEnvVar,
  deployContractFromArtifacts,
  getInitializerData,
  generateRoleAssignments,
} from "../common/helpers";
import {
  UNPAUSE_NATIVE_YIELD_STAKING_ROLE,
  PAUSE_NATIVE_YIELD_STAKING_ROLE,
  YIELD_PROVIDER_STAKING_ROLE,
  SET_YIELD_MANAGER_ROLE,
  NATIVE_YIELD_STAKING_PAUSE_TYPE,
} from "contracts/common/constants";
import {
  contractName as LineaRollupV7ContractName,
  abi as LineaRollupV7Abi,
  bytecode as LineaRollupV7Bytecode,
} from "../deployments/bytecode/2026-01-14/LineaRollup.json";
import { HardhatRuntimeEnvironment } from "hardhat/types";

const func: DeployFunction = async function (hre: HardhatRuntimeEnvironment) {
  const { ethers, getNamedAccounts } = hre;
  const { deployer } = await getNamedAccounts();
  const signer = await ethers.getSigner(deployer);

  const securityCouncilAddress = getRequiredEnvVar("LINEA_ROLLUP_SECURITY_COUNCIL");
  const automationServiceAddress = getRequiredEnvVar("NATIVE_YIELD_AUTOMATION_SERVICE_ADDRESS");
  const proxyAddress = getRequiredEnvVar("LINEA_ROLLUP_ADDRESS");
  const yieldManagerAddress = getRequiredEnvVar("YIELD_MANAGER_ADDRESS");

  const newRoles = [
    SET_YIELD_MANAGER_ROLE,
    YIELD_PROVIDER_STAKING_ROLE,
    PAUSE_NATIVE_YIELD_STAKING_ROLE,
    UNPAUSE_NATIVE_YIELD_STAKING_ROLE,
  ];

  const newRoleAddresses = [
    ...generateRoleAssignments(newRoles, securityCouncilAddress, []),
    {
      role: YIELD_PROVIDER_STAKING_ROLE,
      addressWithRole: automationServiceAddress,
    },
  ];
  console.log("New role addresses", newRoleAddresses);

  const newPauseRoles = [{ pauseType: NATIVE_YIELD_STAKING_PAUSE_TYPE, role: PAUSE_NATIVE_YIELD_STAKING_ROLE }];
  const newUnPauseRoles = [{ pauseType: NATIVE_YIELD_STAKING_PAUSE_TYPE, role: UNPAUSE_NATIVE_YIELD_STAKING_ROLE }];

  const lineaRollupV7Impl = await deployContractFromArtifacts(
    LineaRollupV7ContractName,
    LineaRollupV7Abi,
    LineaRollupV7Bytecode,
    signer,
  );

  const lineaRollupV7Reinitializer = getInitializerData(LineaRollupV7Abi, "reinitializeLineaRollupV7", [
    newRoleAddresses,
    newPauseRoles,
    newUnPauseRoles,
    yieldManagerAddress,
  ]);

  // The encoding should be used through the safe.
  // THIS IS JUST A SAMPLE AND WILL BE ADJUSTED WHEN NEEDED FOR GENERATING THE CALLDATA FOR THE UPGRADE CALL
  // https://www.4byte.directory/signatures/?bytes4_signature=0x9623609d
  const upgradeCallWithReinitializationUsingSecurityCouncil = ethers.concat([
    "0x9623609d",
    ethers.AbiCoder.defaultAbiCoder().encode(
      ["address", "address", "bytes"],
      [proxyAddress, await lineaRollupV7Impl.getAddress(), lineaRollupV7Reinitializer],
    ),
  ]);

  console.log(
    "Encoded Tx Upgrade with Reinitialization from Security Council:",
    "\n",
    upgradeCallWithReinitializationUsingSecurityCouncil,
  );
  console.log("\n");
};

export default func;
func.tags = ["LineaRollupV7WithReinitialization"];
