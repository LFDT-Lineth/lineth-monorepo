import { network } from "hardhat";
import { DeployFunction } from "hardhat-deploy/types";

import {
  VALIDIUM_INITIALIZE_SIGNATURE,
  VALIDIUM_PAUSE_TYPES_ROLES,
  VALIDIUM_ROLES,
  VALIDIUM_UNPAUSE_TYPES_ROLES,
  OPERATOR_ROLE,
  ADDRESS_ZERO,
} from "../common/constants";
import {
  generateRoleAssignments,
  getEnvVarOrDefault,
  getRequiredEnvVar,
  requireAddressFromRegistryOrEnv,
  requireAddressesFromRegistryOrEnv,
  tryVerifyContract,
  LogContractDeployment,
} from "../common/helpers";
import { withSignerUiSession } from "../scripts/hardhat/signer-ui-bridge";
import { deployUpgradableFromFactory } from "../scripts/hardhat/utils";

const func: DeployFunction = withSignerUiSession("03_deploy_Validium.ts", async function () {
  const contractName = "Validium";

  // Validium DEPLOYED AS UPGRADEABLE PROXY
  const verifierAddress = requireAddressFromRegistryOrEnv(network.name, "PlonkVerifier", "VERIFIER_ADDRESS");
  const validiumInitialStateRootHash = getRequiredEnvVar("INITIAL_L2_STATE_ROOT_HASH");
  const validiumInitialL2BlockNumber = getRequiredEnvVar("INITIAL_L2_BLOCK_NUMBER");
  const validiumSecurityCouncil = requireAddressFromRegistryOrEnv(
    network.name,
    "L1_SECURITY_COUNCIL",
    "L1_SECURITY_COUNCIL",
  );
  const validiumOperators = requireAddressesFromRegistryOrEnv(network.name, "VALIDIUM_OPERATORS", "VALIDIUM_OPERATORS");
  const validiumRateLimitPeriodInSeconds = getRequiredEnvVar("VALIDIUM_RATE_LIMIT_PERIOD");
  const validiumRateLimitAmountInWei = getRequiredEnvVar("VALIDIUM_RATE_LIMIT_AMOUNT");
  const validiumGenesisTimestamp = getRequiredEnvVar("L2_GENESIS_TIMESTAMP");

  const pauseTypeRoles = getEnvVarOrDefault("VALIDIUM_PAUSE_TYPES_ROLES", VALIDIUM_PAUSE_TYPES_ROLES);
  const unpauseTypeRoles = getEnvVarOrDefault("VALIDIUM_UNPAUSE_TYPES_ROLES", VALIDIUM_UNPAUSE_TYPES_ROLES);
  const defaultRoleAddresses = generateRoleAssignments(VALIDIUM_ROLES, validiumSecurityCouncil, [
    { role: OPERATOR_ROLE, addresses: validiumOperators },
  ]);
  const roleAddresses = getEnvVarOrDefault("VALIDIUM_ROLE_ADDRESSES", defaultRoleAddresses);
  const addressFilter = requireAddressFromRegistryOrEnv(network.name, "AddressFilter", "LINEA_ROLLUP_ADDRESS_FILTER");

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
        addressFilter,
      },
    ],
    {
      initializer: VALIDIUM_INITIALIZE_SIGNATURE,
      unsafeAllow: ["constructor", "incorrect-initializer-order"],
    },
  );

  await LogContractDeployment(contractName, contract);
  const contractAddress = await contract.getAddress();

  await tryVerifyContract(contractAddress);
});

export default func;
func.tags = ["Validium"];
