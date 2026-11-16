import { LIDO_DASHBOARD_OPERATIONAL_ROLES } from "../../../common/constants/index.js";
import { parseChecksummedAddress } from "../../../common/helpers/addressRegistry.js";
import { buildVendorInitializationData } from "../../../common/helpers/buildVendorInitializationData.js";
import { getTaskCliOrEnvValue } from "../../../common/helpers/environmentHelper.js";
import { generateRoleAssignments } from "../../../common/helpers/roles.js";
import { requireRegistryAddress } from "../registryAddress.js";

import type { HardhatRuntimeEnvironment } from "hardhat/types/hre";
import type { TaskArguments } from "hardhat/types/tasks";

/*
  *******************************************************************************************
  Generates parameters for adding and configuring a new LidoStVaultYieldProvider.

  This script generates parameters for addYieldProvider function call:
  - yieldProvider address
  - yieldProviderInitData (encoded initialization bytes)

  1) YieldManager + LidoStVaultYieldProvider deployment
  2) Run this task with the right params or env vars.
  3) No transactions are executed - only parameters are logged

  -------------------------------------------------------------------------------------------
  Example (Hoodi):
  -------------------------------------------------------------------------------------------
  DEPLOYER_PRIVATE_KEY=0000000000000000000000000000000000000000000000000000000000000002 \
  CUSTOM_RPC_URL=https://0xrpc.io/hoodi \
  pnpm exec hardhat addLidoStVaultYieldProvider \
    --yield-manager <address> \
    --yield-provider <address> \
    --node-operator <address> \
    --security-council <address> \
    --node-operator-fee <uint256> \
    --confirm-expiry <uint256> \
    --network custom
  -------------------------------------------------------------------------------------------

  Env var alternatives (used if CLI params omitted):
    YIELD_MANAGER_ADDRESS
    YIELD_PROVIDER_ADDRESS
    NODE_OPERATOR
    L1_SECURITY_COUNCIL
    NODE_OPERATOR_FEE
    CONFIRM_EXPIRY
  *******************************************************************************************
*/
export default async function addLidoStVaultYieldProvider(taskArgs: TaskArguments, hre: HardhatRuntimeEnvironment) {
  const { networkName } = await hre.network.getOrCreate();

  // --- Resolve inputs from CLI or ENV (with sensible fallbacks to the address registry) ---
  const yieldManagerInput = getTaskCliOrEnvValue(taskArgs, "yieldManager", "YIELD_MANAGER_ADDRESS");
  const yieldProviderInput = getTaskCliOrEnvValue(taskArgs, "yieldProvider", "YIELD_PROVIDER_ADDRESS");
  const nodeOperatorInput = getTaskCliOrEnvValue(taskArgs, "nodeOperator", "NODE_OPERATOR");
  const securityCouncilInput = getTaskCliOrEnvValue(taskArgs, "securityCouncil", "L1_SECURITY_COUNCIL");
  const nodeOperatorFeeRaw = getTaskCliOrEnvValue(taskArgs, "nodeOperatorFee", "NODE_OPERATOR_FEE");
  const confirmExpiryRaw = getTaskCliOrEnvValue(taskArgs, "confirmExpiry", "CONFIRM_EXPIRY");

  // --- Use address from the address registry ---
  const yieldManager =
    yieldManagerInput === undefined
      ? requireRegistryAddress(networkName, "YieldManager")
      : parseChecksummedAddress(yieldManagerInput, "--yield-manager / YIELD_MANAGER_ADDRESS");

  // --- Basic required fields check (adjust as needed) ---
  const missing: string[] = [];
  if (!yieldProviderInput) missing.push("yieldProvider / YIELD_PROVIDER_ADDRESS");
  if (!nodeOperatorInput) missing.push("nodeOperator / NODE_OPERATOR");
  if (!securityCouncilInput) missing.push("securityCouncil / L1_SECURITY_COUNCIL");
  if (!yieldProviderInput || !nodeOperatorInput || !securityCouncilInput) {
    throw new Error(`Missing required params/envs: ${missing.join(", ")}`);
  }
  const yieldProvider = parseChecksummedAddress(yieldProviderInput, "--yield-provider / YIELD_PROVIDER_ADDRESS");
  const nodeOperator = parseChecksummedAddress(nodeOperatorInput, "--node-operator / NODE_OPERATOR");
  const securityCouncil = parseChecksummedAddress(securityCouncilInput, "--security-council / L1_SECURITY_COUNCIL");

  // --- Parse numeric params ---
  const nodeOperatorFee = nodeOperatorFeeRaw ? BigInt(nodeOperatorFeeRaw) : 0n;
  const confirmExpiry = confirmExpiryRaw ? BigInt(confirmExpiryRaw) : 0n;

  // --- Log params ---
  console.log("Params:");
  console.log("  yieldManager:", yieldManager);
  console.log("  yieldProvider:", yieldProvider);
  console.log("  nodeOperator:", nodeOperator);
  console.log("  securityCouncil:", securityCouncil);
  console.log("  nodeOperatorFee:", nodeOperatorFee.toString());
  console.log("  confirmExpiry:", confirmExpiry.toString());

  /********************************************************************
   *                Below here requires Security Council              *
   ********************************************************************/

  // --- Build YieldProvider initialization data ---
  const yieldProviderInitData = buildVendorInitializationData({
    defaultAdmin: securityCouncil,
    nodeOperator,
    nodeOperatorManager: securityCouncil,
    nodeOperatorFeeBP: nodeOperatorFee,
    confirmExpiry,
    roleAssignments: generateRoleAssignments(LIDO_DASHBOARD_OPERATIONAL_ROLES, yieldManager, []),
  });

  // --- Output parameters for addYieldProvider ---
  console.log("\n" + "=".repeat(80));
  console.log("Parameters for YieldManager.addYieldProvider:");
  console.log("=".repeat(80));
  console.log("\nFunction: addYieldProvider(address _yieldProvider, bytes _initializationData)");
  console.log("\nParameters:");
  console.log("  _yieldProvider:", yieldProvider);
  console.log("  _initializationData:", yieldProviderInitData);
  console.log("\n" + "=".repeat(80));
}
