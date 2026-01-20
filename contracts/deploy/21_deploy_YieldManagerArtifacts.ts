// Usage:
// npx hardhat deploy --network <network> --tags YieldManagerArtifacts
//
// Required environment variables:
//   LINEA_ROLLUP_ADDRESS
//   LINEA_ROLLUP_SECURITY_COUNCIL
//   NATIVE_YIELD_AUTOMATION_SERVICE_ADDRESS
//   VAULT_HUB
//   VAULT_FACTORY
//   STETH
//   MINIMUM_WITHDRAWAL_RESERVE_PERCENTAGE_BPS
//   TARGET_WITHDRAWAL_RESERVE_PERCENTAGE_BPS
//   MINIMUM_WITHDRAWAL_RESERVE_AMOUNT
//   TARGET_WITHDRAWAL_RESERVE_AMOUNT
//
// Optional environment variables:
//   GI_FIRST_VALIDATOR
//   GI_PENDING_PARTIAL_WITHDRAWALS_ROOT
//   VALIDATOR_CONTAINER_PROOF_VERIFIER_ADMIN
//   YIELD_MANAGER_PAUSE_TYPES_ROLES
//   YIELD_MANAGER_UNPAUSE_TYPES_ROLES

import { DeployFunction } from "hardhat-deploy/types";
import { HardhatRuntimeEnvironment } from "hardhat/types";
import {
  deployContractFromArtifacts,
  deployProxyAdminAndProxy,
  generateRoleAssignments,
  getEnvVarOrDefault,
  getInitializerData,
  getRequiredEnvVar,
} from "../common/helpers";
import {
  YIELD_MANAGER_OPERATOR_ROLES,
  YIELD_MANAGER_PAUSE_TYPES_ROLES,
  YIELD_MANAGER_SECURITY_COUNCIL_ROLES,
  YIELD_MANAGER_UNPAUSE_TYPES_ROLES,
} from "../common/constants";
import { YieldManagerInitializationData } from "../test/hardhat/yield/helpers/types";
import { GI_FIRST_VALIDATOR, GI_PENDING_PARTIAL_WITHDRAWALS_ROOT } from "../test/hardhat/common/constants";
import {
  contractName as LidoStVaultYieldProviderFactoryContractName,
  abi as LidoStVaultYieldProviderFactoryAbi,
  bytecode as LidoStVaultYieldProviderFactoryBytecode,
} from "../deployments/bytecode/2026-01-14/LidoStVaultYieldProviderFactory.json";
import {
  contractName as ValidatorContainerProofVerifierContractName,
  abi as ValidatorContainerProofVerifierAbi,
  bytecode as ValidatorContainerProofVerifierBytecode,
} from "../deployments/bytecode/2026-01-14/ValidatorContainerProofVerifier.json";
import {
  contractName as YieldManagerContractName,
  abi as YieldManagerAbi,
  bytecode as YieldManagerBytecode,
} from "../deployments/bytecode/2026-01-14/YieldManager.json";

// Deploys YieldManager, ValidatorContainerProofVerifier and LidoStVaultYieldProviderFactory
// Must verify contracts from git tag "contract-audit-2026-01-14" or commit 25e323d055dec40ef167a190c71c30aa9bf92c23
const func: DeployFunction = async function (hre: HardhatRuntimeEnvironment) {
  const { ethers, getNamedAccounts } = hre;
  const { deployer } = await getNamedAccounts();
  const signer = await ethers.getSigner(deployer);

  // YieldManager DEPLOYED AS UPGRADEABLE PROXY
  const lineaRollupAddress = getRequiredEnvVar("LINEA_ROLLUP_ADDRESS");
  const lineaRollupSecurityCouncil = getRequiredEnvVar("LINEA_ROLLUP_SECURITY_COUNCIL");
  const nativeYieldAutomationServiceAddress = getRequiredEnvVar("NATIVE_YIELD_AUTOMATION_SERVICE_ADDRESS");
  const vaultHub = getRequiredEnvVar("VAULT_HUB");
  const vaultFactory = getRequiredEnvVar("VAULT_FACTORY");
  const steth = getRequiredEnvVar("STETH");
  const initialMinimumWithdrawalReservePercentageBps = parseInt(
    getRequiredEnvVar("MINIMUM_WITHDRAWAL_RESERVE_PERCENTAGE_BPS"),
  );
  const initialTargetWithdrawalReservePercentageBps = parseInt(
    getRequiredEnvVar("TARGET_WITHDRAWAL_RESERVE_PERCENTAGE_BPS"),
  );
  const initialMinimumWithdrawalReserveAmount = BigInt(getRequiredEnvVar("MINIMUM_WITHDRAWAL_RESERVE_AMOUNT"));
  const initialTargetWithdrawalReserveAmount = BigInt(getRequiredEnvVar("TARGET_WITHDRAWAL_RESERVE_AMOUNT"));
  const gIFirstValidator = getEnvVarOrDefault("GI_FIRST_VALIDATOR", GI_FIRST_VALIDATOR);
  const gIPendingPartialWithdrawalsRoot = getEnvVarOrDefault(
    "GI_PENDING_PARTIAL_WITHDRAWALS_ROOT",
    GI_PENDING_PARTIAL_WITHDRAWALS_ROOT,
  );
  const verifierAdmin = getEnvVarOrDefault("VALIDATOR_CONTAINER_PROOF_VERIFIER_ADMIN", lineaRollupSecurityCouncil);

  const securityCouncilRoles = generateRoleAssignments(
    YIELD_MANAGER_SECURITY_COUNCIL_ROLES,
    lineaRollupSecurityCouncil,
    [],
  );
  const automationServiceRoles = generateRoleAssignments(
    YIELD_MANAGER_OPERATOR_ROLES,
    nativeYieldAutomationServiceAddress,
    [],
  );
  const roleAddresses = [...securityCouncilRoles, ...automationServiceRoles];

  const pauseTypeRoles = getEnvVarOrDefault("YIELD_MANAGER_PAUSE_TYPES_ROLES", YIELD_MANAGER_PAUSE_TYPES_ROLES);
  const unpauseTypeRoles = getEnvVarOrDefault("YIELD_MANAGER_UNPAUSE_TYPES_ROLES", YIELD_MANAGER_UNPAUSE_TYPES_ROLES);

  /********************************************************************
   *                          YieldManager                            *
   ********************************************************************/
  const yieldManagerImpl = await deployContractFromArtifacts(
    YieldManagerContractName,
    YieldManagerAbi,
    YieldManagerBytecode,
    signer,
    lineaRollupAddress,
  );

  const yieldManagerInitData: YieldManagerInitializationData = {
    pauseTypeRoles: pauseTypeRoles,
    unpauseTypeRoles: unpauseTypeRoles,
    roleAddresses: roleAddresses,
    initialL2YieldRecipients: [],
    defaultAdmin: lineaRollupSecurityCouncil,
    initialMinimumWithdrawalReservePercentageBps: initialMinimumWithdrawalReservePercentageBps,
    initialTargetWithdrawalReservePercentageBps: initialTargetWithdrawalReservePercentageBps,
    initialMinimumWithdrawalReserveAmount: initialMinimumWithdrawalReserveAmount,
    initialTargetWithdrawalReserveAmount: initialTargetWithdrawalReserveAmount,
  };

  const yieldManagerInitializer = getInitializerData(YieldManagerAbi, "initialize", [yieldManagerInitData]);

  const { proxyAddress: yieldManagerAddress } = await deployProxyAdminAndProxy(
    await yieldManagerImpl.getAddress(),
    signer,
    yieldManagerInitializer,
  );

  /********************************************************************
   *                ValidatorContainerProofVerifier                   *
   ********************************************************************/
  const verifier = await deployContractFromArtifacts(
    ValidatorContainerProofVerifierContractName,
    ValidatorContainerProofVerifierAbi,
    ValidatorContainerProofVerifierBytecode,
    signer,
    verifierAdmin,
    gIFirstValidator,
    gIPendingPartialWithdrawalsRoot,
  );

  const verifierAddress = await verifier.getAddress();

  /********************************************************************
   *                LidoStVaultYieldProviderFactory                   *
   ********************************************************************/

  const factory = await deployContractFromArtifacts(
    LidoStVaultYieldProviderFactoryContractName,
    LidoStVaultYieldProviderFactoryAbi,
    LidoStVaultYieldProviderFactoryBytecode,
    signer,
    lineaRollupAddress,
    yieldManagerAddress,
    vaultHub,
    vaultFactory,
    steth,
    verifierAddress,
  );
  const factoryAddress = await factory.getAddress();

  /********************************************************************
   *                    LidoStVaultYieldProvider                      *
   ********************************************************************/

  const factoryContract = await ethers.getContractAt("LidoStVaultYieldProviderFactory", factoryAddress, signer);
  const yieldProvider = await factoryContract.createLidoStVaultYieldProvider.staticCall();
  const createYieldProviderTx = await factoryContract.createLidoStVaultYieldProvider();
  await createYieldProviderTx.wait(5);
  console.log("Created LidoStVaultYieldProvider at ", yieldProvider);
};

export default func;
func.tags = ["YieldManagerArtifacts"];
