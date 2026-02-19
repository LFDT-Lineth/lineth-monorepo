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
  VALIDIUM_INITIALIZE_SIGNATURE,
  VALIDIUM_PAUSE_TYPES_ROLES,
  VALIDIUM_ROLES,
  VALIDIUM_UNPAUSE_TYPES_ROLES,
  OPERATOR_ROLE,
  ADDRESS_ZERO,
} from "../common/constants";

const func: DeployFunction = async function () {
  const contractName = "Validium";

  // Validium DEPLOYED AS UPGRADEABLE PROXY
  const verifierAddress = getRequiredEnvVar("PLONKVERIFIER_ADDRESS");
  const validiumInitialStateRootHash = getRequiredEnvVar("INITIAL_L2_STATE_ROOT_HASH");
  const validiumInitialL2BlockNumber = getRequiredEnvVar("INITIAL_L2_BLOCK_NUMBER");
  const validiumSecurityCouncil = getRequiredEnvVar("L1_SECURITY_COUNCIL");
  const validiumOperators = getRequiredEnvVar("VALIDIUM_OPERATORS").split(",");
  const validiumRateLimitPeriodInSeconds = getRequiredEnvVar("VALIDIUM_RATE_LIMIT_PERIOD");
  const validiumRateLimitAmountInWei = getRequiredEnvVar("VALIDIUM_RATE_LIMIT_AMOUNT");
  const validiumGenesisTimestamp = getRequiredEnvVar("L2_GENESIS_TIMESTAMP");

  const pauseTypeRoles = getEnvVarOrDefault("VALIDIUM_PAUSE_TYPES_ROLES", VALIDIUM_PAUSE_TYPES_ROLES);
  const unpauseTypeRoles = getEnvVarOrDefault("VALIDIUM_UNPAUSE_TYPES_ROLES", VALIDIUM_UNPAUSE_TYPES_ROLES);
  const defaultRoleAddresses = generateRoleAssignments(VALIDIUM_ROLES, validiumSecurityCouncil, [
    { role: OPERATOR_ROLE, addresses: validiumOperators },
  ]);
  const roleAddresses = getEnvVarOrDefault("VALIDIUM_ROLE_ADDRESSES", defaultRoleAddresses);

  const contract = await deployUpgradableFromFactory(
    "Validium",
    [
      {
        initialStateRootHash: validiumInitialStateRootHash,
        initialL2BlockNumber: validiumInitialL2BlockNumber,
        genesisTimestamp: validiumGenesisTimestamp,
        defaultVerifier: verifierAddress,
        rateLimitPeriodInSeconds: validiumRateLimitPeriodInSeconds,
        rateLimitAmountInWei: validiumRateLimitAmountInWei,
        roleAddresses,
        pauseTypeRoles,
        unpauseTypeRoles,
        defaultAdmin: validiumSecurityCouncil,
        shnarfProvider: ADDRESS_ZERO,
      },
    ],
    {
      initializer: VALIDIUM_INITIALIZE_SIGNATURE,
      unsafeAllow: ["constructor"],
    },
  );

  await LogContractDeployment(contractName, contract);
  const contractAddress = await contract.getAddress();

  await tryVerifyContract(contractAddress);
};

export default func;
func.tags = ["Validium"];
