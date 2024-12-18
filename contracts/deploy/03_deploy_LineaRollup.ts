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
  LogContractDeployment,
} from "../common/helpers";
import {
  LINEA_ROLLUP_INITIALIZE_SIGNATURE,
  LINEA_ROLLUP_PAUSE_TYPES_ROLES,
  LINEA_ROLLUP_ROLES,
  LINEA_ROLLUP_UNPAUSE_TYPES_ROLES,
  OPERATOR_ROLE,
} from "../common/constants";

const func: DeployFunction = async function (hre: HardhatRuntimeEnvironment) {
  const { deployments } = hre;

  const contractName = "LineaRollup";
  const verifierName = "PlonkVerifier";
  const existingContractAddress = await getDeployedContractAddress(contractName, deployments);
  let verifierAddress = await getDeployedContractAddress(verifierName, deployments);
  if (!verifierAddress) {
    verifierAddress = getRequiredEnvVar("PLONKVERIFIER_ADDRESS");
  } else {
    console.log(`Using deployed variable for PlonkVerifier , ${verifierAddress}`);
  }

  // LineaRollup DEPLOYED AS UPGRADEABLE PROXY
  const lineaRollupInitialStateRootHash = getRequiredEnvVar("LINEA_ROLLUP_INITIAL_STATE_ROOT_HASH");
  const lineaRollupInitialL2BlockNumber = getRequiredEnvVar("LINEA_ROLLUP_INITIAL_L2_BLOCK_NUMBER");
  const lineaRollupSecurityCouncil = getRequiredEnvVar("LINEA_ROLLUP_SECURITY_COUNCIL");
  const lineaRollupOperators = getRequiredEnvVar("LINEA_ROLLUP_OPERATORS").split(",");
  const lineaRollupRateLimitPeriodInSeconds = getRequiredEnvVar("LINEA_ROLLUP_RATE_LIMIT_PERIOD");
  const lineaRollupRateLimitAmountInWei = getRequiredEnvVar("LINEA_ROLLUP_RATE_LIMIT_AMOUNT");
  const lineaRollupGenesisTimestamp = getRequiredEnvVar("LINEA_ROLLUP_GENESIS_TIMESTAMP");
  const MultiCallAddress = "0xcA11bde05977b3631167028862bE2a173976CA11";

  const pauseTypeRoles = getEnvVarOrDefault("LINEA_ROLLUP_PAUSE_TYPE_ROLES", LINEA_ROLLUP_PAUSE_TYPES_ROLES);
  const unpauseTypeRoles = getEnvVarOrDefault("LINEA_ROLLUP_UNPAUSE_TYPE_ROLES", LINEA_ROLLUP_UNPAUSE_TYPES_ROLES);
  const defaultRoleAddresses = generateRoleAssignments(LINEA_ROLLUP_ROLES, lineaRollupSecurityCouncil, [
    { role: OPERATOR_ROLE, addresses: lineaRollupOperators },
  ]);
  const roleAddresses = getEnvVarOrDefault("LINEA_ROLLUP_ROLE_ADDRESSES", defaultRoleAddresses);

  if (!existingContractAddress) {
    console.log(`Deploying initial version, NB: the address will be saved if env SAVE_ADDRESS=true.`);
  } else {
    console.log(`Deploying new version, NB: ${existingContractAddress} will be overwritten if env SAVE_ADDRESS=true.`);
  }

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
        fallbackOperator: MultiCallAddress,
        defaultAdmin: lineaRollupSecurityCouncil,
      },
    ],
    {
      initializer: LINEA_ROLLUP_INITIALIZE_SIGNATURE,
      unsafeAllow: ["constructor"],
    },
  );

  await LogContractDeployment(contractName, contract);
  const contractAddress = await contract.getAddress();

  await tryStoreAddress(hre.network.name, contractName, contractAddress, contract.deploymentTransaction()!.hash);

  await tryVerifyContract(contractAddress);
};

export default func;
func.tags = ["LineaRollup"];
