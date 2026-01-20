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
  LINEA_ROLLUP_INITIALIZE_SIGNATURE,
  LINEA_ROLLUP_V8_PAUSE_TYPES_ROLES,
  LINEA_ROLLUP_V8_UNPAUSE_TYPES_ROLES,
  LINEA_ROLLUP_V8_ROLES,
  OPERATOR_ROLE,
  ADDRESS_ZERO,
} from "../common/constants";

const func: DeployFunction = async function () {
  const contractName = "LineaRollup";

  // LineaRollup DEPLOYED AS UPGRADEABLE PROXY
  const verifierAddress = getRequiredEnvVar("PLONKVERIFIER_ADDRESS");
  const lineaRollupInitialStateRootHash = getRequiredEnvVar("LINEA_ROLLUP_INITIAL_STATE_ROOT_HASH");
  const lineaRollupInitialL2BlockNumber = getRequiredEnvVar("LINEA_ROLLUP_INITIAL_L2_BLOCK_NUMBER");
  const lineaRollupSecurityCouncil = getRequiredEnvVar("LINEA_ROLLUP_SECURITY_COUNCIL");
  const lineaRollupOperators = getRequiredEnvVar("LINEA_ROLLUP_OPERATORS").split(",");
  const lineaRollupRateLimitPeriodInSeconds = getRequiredEnvVar("LINEA_ROLLUP_RATE_LIMIT_PERIOD");
  const lineaRollupRateLimitAmountInWei = getRequiredEnvVar("LINEA_ROLLUP_RATE_LIMIT_AMOUNT");
  const lineaRollupGenesisTimestamp = getRequiredEnvVar("LINEA_ROLLUP_GENESIS_TIMESTAMP");
  const MultiCallAddress = "0xcA11bde05977b3631167028862bE2a173976CA11";

  const pauseTypeRoles = getEnvVarOrDefault("LINEA_ROLLUP_PAUSE_TYPE_ROLES", LINEA_ROLLUP_V8_PAUSE_TYPES_ROLES);
  const unpauseTypeRoles = getEnvVarOrDefault("LINEA_ROLLUP_UNPAUSE_TYPE_ROLES", LINEA_ROLLUP_V8_UNPAUSE_TYPES_ROLES);
  const defaultRoleAddresses = generateRoleAssignments(LINEA_ROLLUP_V8_ROLES, lineaRollupSecurityCouncil, [
    { role: OPERATOR_ROLE, addresses: lineaRollupOperators },
  ]);
  const roleAddresses = getEnvVarOrDefault("LINEA_ROLLUP_ROLE_ADDRESSES", defaultRoleAddresses);
  const yieldManagerAddress = getRequiredEnvVar("YIELD_MANAGER_ADDRESS");

  const contract = await deployUpgradableFromFactory(
    "LineaRollup",
    [
      {
        initialStateRootHash: lineaRollupInitialStateRootHash,
        initialL2BlockNumber: lineaRollupInitialL2BlockNumber,
        genesisTimestamp: lineaRollupGenesisTimestamp,
        defaultVerifier: verifierAddress,
        rateLimitPeriodInSeconds: lineaRollupRateLimitPeriodInSeconds,
        rateLimitAmountInWei: lineaRollupRateLimitAmountInWei,
        roleAddresses,
        pauseTypeRoles,
        unpauseTypeRoles,
        defaultAdmin: lineaRollupSecurityCouncil,
        shnarfProvider: ADDRESS_ZERO,
      },
      MultiCallAddress,
      yieldManagerAddress,
    ],
    {
      initializer: LINEA_ROLLUP_INITIALIZE_SIGNATURE,
      unsafeAllow: ["constructor"],
    },
  );

  await LogContractDeployment(contractName, contract);
  const contractAddress = await contract.getAddress();

  await tryVerifyContract(contractAddress);
};

export default func;
func.tags = ["LineaRollup"];
