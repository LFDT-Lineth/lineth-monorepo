import { task } from "hardhat/config";
import { getTaskCliOrEnvValue } from "../../../common/helpers/environmentHelper";
import { generateRoleAssignments, buildVendorInitializationData } from "../../../common/helpers";
import { LIDO_DASHBOARD_OPERATIONAL_ROLES } from "../../../common/constants";

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
  CUSTOM_PRIVATE_KEY=0000000000000000000000000000000000000000000000000000000000000002 \
  CUSTOM_BLOCKCHAIN_URL=https://0xrpc.io/hoodi \
  npx hardhat addLidoStVaultYieldProvider \
    --yield-manager <address> \
    --yield-provider <address> \
    --node-operator <address> \
    --security-council <address> \
    --node-operator-fee <uint256> \
    --confirm-expiry <uint256> \
    --network custom
  -------------------------------------------------------------------------------------------

  Env var alternatives (used if CLI params omitted):
    YIELD_MANAGER
    YIELD_PROVIDER
    NODE_OPERATOR
    SECURITY_COUNCIL
    NODE_OPERATOR_FEE
    CONFIRM_EXPIRY
  *******************************************************************************************
*/
task("addLidoStVaultYieldProvider", "Generates parameters for adding and configuring a new LidoStVaultYieldProvider")
  .addOptionalParam("yieldManager")
  .addOptionalParam("yieldProvider")
  .addOptionalParam("nodeOperator")
  .addOptionalParam("securityCouncil")
  .addOptionalParam("nodeOperatorFee")
  .addOptionalParam("confirmExpiry")
  .setAction(async (taskArgs, hre) => {
    const { deployments } = hre;
    const { get } = deployments;

    // --- Resolve inputs from CLI or ENV (with sensible fallbacks to deployments) ---
    let yieldManager = getTaskCliOrEnvValue(taskArgs, "yieldManager", "YIELD_MANAGER");
    const yieldProvider = getTaskCliOrEnvValue(taskArgs, "yieldProvider", "YIELD_PROVIDER");
    const nodeOperator = getTaskCliOrEnvValue(taskArgs, "nodeOperator", "NODE_OPERATOR");
    const securityCouncil = getTaskCliOrEnvValue(taskArgs, "securityCouncil", "SECURITY_COUNCIL");
    const nodeOperatorFeeRaw = getTaskCliOrEnvValue(taskArgs, "nodeOperatorFee", "NODE_OPERATOR_FEE");
    const confirmExpiryRaw = getTaskCliOrEnvValue(taskArgs, "confirmExpiry", "CONFIRM_EXPIRY");

    // --- Use address from artifacts ---
    if (yieldManager === undefined) {
      yieldManager = (await get("YieldManager")).address;
    }

    // --- Basic required fields check (adjust as needed) ---
    const missing: string[] = [];
    if (!yieldProvider) missing.push("yieldProvider / YIELD_PROVIDER");
    if (!nodeOperator) missing.push("nodeOperator / NODE_OPERATOR");
    if (!securityCouncil) missing.push("securityCouncil / SECURITY_COUNCIL");
    if (missing.length) {
      throw new Error(`Missing required params/envs: ${missing.join(", ")}`);
    }

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
      defaultAdmin: securityCouncil!,
      nodeOperator: nodeOperator!,
      nodeOperatorManager: securityCouncil!,
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
  });
