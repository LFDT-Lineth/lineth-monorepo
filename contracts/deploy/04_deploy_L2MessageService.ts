import { DeployFunction } from "hardhat-deploy/types";
import { HardhatRuntimeEnvironment } from "hardhat/types";
import { deployUpgradableFromFactory } from "../scripts/hardhat/utils";
import {
  generateRoleAssignments,
  getEnvVarOrDefault,
  getRequiredEnvVar,
  tryVerifyContract,
  getDeployedContractAddress,
  tryStoreAddress,
} from "../common/helpers";
import {
  L1_L2_MESSAGE_SETTER_ROLE,
  L2_MESSAGE_SERVICE_INITIALIZE_SIGNATURE,
  L2_MESSAGE_SERVICE_PAUSE_TYPES_ROLES,
  L2_MESSAGE_SERVICE_ROLES,
  L2_MESSAGE_SERVICE_UNPAUSE_TYPES_ROLES,
} from "../common/constants";

const func: DeployFunction = async function (hre: HardhatRuntimeEnvironment) {
  const { deployments } = hre;

  const contractName = "L2MessageService";
  const existingContractAddress = await getDeployedContractAddress(contractName, deployments);

  const l2MessageServiceSecurityCouncil = getRequiredEnvVar("L2MSGSERVICE_SECURITY_COUNCIL");
  const l2MessageServiceL1L2MessageSetter = getRequiredEnvVar("L2MSGSERVICE_L1L2_MESSAGE_SETTER");
  const l2MessageServiceRateLimitPeriod = getRequiredEnvVar("L2MSGSERVICE_RATE_LIMIT_PERIOD");
  const l2MessageServiceRateLimitAmount = getRequiredEnvVar("L2MSGSERVICE_RATE_LIMIT_AMOUNT");

  const pauseTypeRoles = getEnvVarOrDefault("L2MSGSERVICE_PAUSE_TYPE_ROLES", L2_MESSAGE_SERVICE_PAUSE_TYPES_ROLES);
  const unpauseTypeRoles = getEnvVarOrDefault(
    "L2MSGSERVICE_UNPAUSE_TYPE_ROLES",
    L2_MESSAGE_SERVICE_UNPAUSE_TYPES_ROLES,
  );
  const defaultRoleAddresses = generateRoleAssignments(L2_MESSAGE_SERVICE_ROLES, l2MessageServiceSecurityCouncil, [
    { role: L1_L2_MESSAGE_SETTER_ROLE, addresses: [l2MessageServiceL1L2MessageSetter] },
  ]);
  const roleAddresses = getEnvVarOrDefault("L2MSGSERVICE_ROLE_ADDRESSES", defaultRoleAddresses);

  if (!existingContractAddress) {
    console.log(`Deploying initial version, NB: the address will be saved if env SAVE_ADDRESS=true.`);
  } else {
    console.log(`Deploying new version, NB: ${existingContractAddress} will be overwritten if env SAVE_ADDRESS=true.`);
  }

  const contract = await deployUpgradableFromFactory(
    "L2MessageService",
    [
      l2MessageServiceRateLimitPeriod,
      l2MessageServiceRateLimitAmount,
      l2MessageServiceSecurityCouncil,
      roleAddresses,
      pauseTypeRoles,
      unpauseTypeRoles,
    ],
    {
      initializer: L2_MESSAGE_SERVICE_INITIALIZE_SIGNATURE,
      unsafeAllow: ["constructor"],
    },
  );
  const contractAddress = await contract.getAddress();
  const txReceipt = await contract.deploymentTransaction()?.wait();
  if (!txReceipt) {
    throw "Contract deployment transaction receipt not found.";
  }
  console.log(`${contractName} deployed: address=${contractAddress} blockNumber=${txReceipt.blockNumber}`);

  await tryStoreAddress(hre.network.name, contractName, contractAddress, txReceipt.hash);

  await tryVerifyContract(contractAddress);
};
export default func;
func.tags = ["L2MessageService"];
