import { DeployFunction } from "hardhat-deploy/types";
import { deployUpgradableFromFactory } from "../scripts/hardhat/utils";
import {
  generateRoleAssignments,
  getEnvVarOrDefault,
  getRequiredEnvVar,
  tryVerifyContract,
  LogContractDeployment,
} from "../common/helpers";
import {
  L1_L2_MESSAGE_SETTER_ROLE,
  L2_MESSAGE_SERVICE_INITIALIZE_SIGNATURE,
  L2_MESSAGE_SERVICE_PAUSE_TYPES_ROLES,
  L2_MESSAGE_SERVICE_ROLES,
  L2_MESSAGE_SERVICE_UNPAUSE_TYPES_ROLES,
} from "../common/constants";

const func: DeployFunction = async function () {
  const contractName = "L2MessageService";

  const l2MessageServiceSecurityCouncil = getRequiredEnvVar("L2_SECURITY_COUNCIL");
  const l2MessageServiceL1L2MessageSetter = getRequiredEnvVar("L2_MESSAGE_SERVICE_L1L2_MESSAGE_SETTER");
  const l2MessageServiceRateLimitPeriod = getRequiredEnvVar("L2_MESSAGE_SERVICE_RATE_LIMIT_PERIOD");
  const l2MessageServiceRateLimitAmount = getRequiredEnvVar("L2_MESSAGE_SERVICE_RATE_LIMIT_AMOUNT");

  const pauseTypeRoles = getEnvVarOrDefault(
    "L2_MESSAGE_SERVICE_PAUSE_TYPES_ROLES",
    L2_MESSAGE_SERVICE_PAUSE_TYPES_ROLES,
  );
  const unpauseTypeRoles = getEnvVarOrDefault(
    "L2_MESSAGE_SERVICE_UNPAUSE_TYPES_ROLES",
    L2_MESSAGE_SERVICE_UNPAUSE_TYPES_ROLES,
  );
  const defaultRoleAddresses = generateRoleAssignments(L2_MESSAGE_SERVICE_ROLES, l2MessageServiceSecurityCouncil, [
    { role: L1_L2_MESSAGE_SETTER_ROLE, addresses: [l2MessageServiceL1L2MessageSetter] },
  ]);
  const roleAddresses = getEnvVarOrDefault("L2_MESSAGE_SERVICE_ROLE_ADDRESSES", defaultRoleAddresses);

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

  await LogContractDeployment(contractName, contract);
  const contractAddress = await contract.getAddress();

  await tryVerifyContract(contractAddress);
};
export default func;
func.tags = ["L2MessageService"];
