import { DeployFunction } from "hardhat-deploy/types";

import {
  LINEA_ROLLUP_INITIALIZE_SIGNATURE,
  LINEA_ROLLUP_V8_PAUSE_TYPES_ROLES,
  LINEA_ROLLUP_V8_UNPAUSE_TYPES_ROLES,
  LINEA_ROLLUP_V8_ROLES,
  OPERATOR_ROLE,
  ADDRESS_ZERO,
} from "../common/constants";
import {
  generateRoleAssignments,
  getEnvVarOrDefault,
  getRequiredEnvVar,
  tryVerifyContract,
  LogContractDeployment,
} from "../common/helpers";
import { withSignerUiSession } from "../scripts/hardhat/signer-ui-bridge";
import { deployUpgradableFromFactory } from "../scripts/hardhat/utils";

const func: DeployFunction = withSignerUiSession("03_deploy_LineaRollup.ts", async function () {
  const contractName = "LineaRollup";

  // LineaRollup DEPLOYED AS UPGRADEABLE PROXY (OpenZeppelin transparent). Hardhat Upgrades may reuse an
  // implementation and/or ProxyAdmin from `.openzeppelin/` for this network, so you might sign fewer than three txs.
  const verifierAddress = getRequiredEnvVar("PLONKVERIFIER_ADDRESS");
  const lineaRollupInitialStateRootHash = getRequiredEnvVar("INITIAL_L2_STATE_ROOT_HASH");
  const lineaRollupInitialL2BlockNumber = getRequiredEnvVar("INITIAL_L2_BLOCK_NUMBER");
  const lineaRollupSecurityCouncil = getRequiredEnvVar("L1_SECURITY_COUNCIL");
  const lineaRollupOperators = getRequiredEnvVar("LINEA_ROLLUP_OPERATORS").split(",");
  const lineaRollupRateLimitPeriodInSeconds = getRequiredEnvVar("LINEA_ROLLUP_RATE_LIMIT_PERIOD");
  const lineaRollupRateLimitAmountInWei = getRequiredEnvVar("LINEA_ROLLUP_RATE_LIMIT_AMOUNT");
  const lineaRollupGenesisTimestamp = getRequiredEnvVar("L2_GENESIS_TIMESTAMP");
  const MultiCallAddress = "0xcA11bde05977b3631167028862bE2a173976CA11";

  const pauseTypeRoles = getEnvVarOrDefault("LINEA_ROLLUP_PAUSE_TYPES_ROLES", LINEA_ROLLUP_V8_PAUSE_TYPES_ROLES);
  const unpauseTypeRoles = getEnvVarOrDefault("LINEA_ROLLUP_UNPAUSE_TYPES_ROLES", LINEA_ROLLUP_V8_UNPAUSE_TYPES_ROLES);
  const defaultRoleAddresses = generateRoleAssignments(LINEA_ROLLUP_V8_ROLES, lineaRollupSecurityCouncil, [
    { role: OPERATOR_ROLE, addresses: lineaRollupOperators },
  ]);
  const roleAddresses = getEnvVarOrDefault("LINEA_ROLLUP_ROLE_ADDRESSES", defaultRoleAddresses);
  const yieldManagerAddress = getRequiredEnvVar("YIELD_MANAGER_ADDRESS");

  const addressFilter = getRequiredEnvVar("LINEA_ROLLUP_ADDRESS_FILTER");

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
        addressFilter,
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
});

export default func;
func.tags = ["LineaRollup"];
