import { task } from "hardhat/config";
import { getTaskCliOrEnvValue } from "../../../common/helpers/environmentHelper";

/*
  *******************************************************************************************
  Setup prerequisites for YieldManager::initiateOssification
  i.) Pause staking
  ii.) Grant Dashboard DEFAULT_ADMIN_ROLE to YieldManager
  
  This script generates calldata for two transactions:
  1) pauseStaking - Pause staking on the yield provider
  2) grantRole - Grant DEFAULT_ADMIN_ROLE to YieldManager on the Dashboard
  
  The calldata can be used to create Safe multisig transactions or execute via other methods.
  
  -------------------------------------------------------------------------------------------
  Example (Hoodi):
  -------------------------------------------------------------------------------------------
  DEPLOYER_PRIVATE_KEY=0000000000000000000000000000000000000000000000000000000000000002 \
  CUSTOM_RPC_URL=https://0xrpc.io/hoodi \
  npx hardhat prepareInitiateOssification \
    --yield-manager <address> \
    --yield-provider <address> \
    --dashboard <address> \
    --network custom
  -------------------------------------------------------------------------------------------
  
  Env var alternatives (used if CLI params omitted):
    YIELD_MANAGER_ADDRESS
    YIELD_PROVIDER_ADDRESS
    DASHBOARD
  *******************************************************************************************
*/
task("prepareInitiateOssification", "Generates calldata for prerequisites before initiating ossification")
  .addOptionalParam("yieldManager")
  .addOptionalParam("yieldProvider")
  .addOptionalParam("dashboard")
  .setAction(async (taskArgs, hre) => {
    const { ethers } = hre;

    // --- Resolve inputs from CLI or ENV ---
    const yieldManager = getTaskCliOrEnvValue(taskArgs, "yieldManager", "YIELD_MANAGER_ADDRESS");
    const yieldProvider = getTaskCliOrEnvValue(taskArgs, "yieldProvider", "YIELD_PROVIDER_ADDRESS");
    const dashboard = getTaskCliOrEnvValue(taskArgs, "dashboard", "DASHBOARD");

    // --- Basic required fields check ---
    const missing: string[] = [];
    if (!yieldManager) missing.push("yieldManager / YIELD_MANAGER_ADDRESS");
    if (!yieldProvider) missing.push("yieldProvider / YIELD_PROVIDER_ADDRESS");
    if (!dashboard) missing.push("dashboard / DASHBOARD");
    if (missing.length) {
      throw new Error(`Missing required params/envs: ${missing.join(", ")}`);
    }

    // --- Log params ---
    console.log("Params:");
    console.log("  yieldManager:", yieldManager);
    console.log("  yieldProvider:", yieldProvider);
    console.log("  dashboard:", dashboard);

    /********************************************************************
     *                Below here requires Security Council              *
     ********************************************************************/
    // Generate calldata for Safe tx

    // Step 1 - Prepare and log calldata for YieldManager::pauseStaking(address yieldProvider)
    // Minimal ABI for YieldManager.pauseStaking
    const YIELD_MANAGER_ABI = [
      {
        inputs: [{ internalType: "address", name: "_yieldProvider", type: "address" }],
        name: "pauseStaking",
        outputs: [],
        stateMutability: "nonpayable",
        type: "function",
      },
    ];

    const yieldManagerContract = await ethers.getContractAt(YIELD_MANAGER_ABI, yieldManager!);
    const pauseStakingCalldata = yieldManagerContract.interface.encodeFunctionData("pauseStaking", [yieldProvider!]);

    // Step 2 - Prepare and log calldata for Dashboard.grantRole(DEFAULT_ADMIN_ROLE, yieldManager)
    // DEFAULT_ADMIN_ROLE = 0x00 (bytes32 zero)
    // Minimal ABI for AccessControl.grantRole
    const DASHBOARD_ABI = [
      {
        inputs: [
          { internalType: "bytes32", name: "role", type: "bytes32" },
          { internalType: "address", name: "account", type: "address" },
        ],
        name: "grantRole",
        outputs: [],
        stateMutability: "nonpayable",
        type: "function",
      },
    ];

    const dashboardContract = await ethers.getContractAt(DASHBOARD_ABI, dashboard!);
    const grantRoleCalldata = dashboardContract.interface.encodeFunctionData("grantRole", [
      ethers.ZeroHash, // DEFAULT_ADMIN_ROLE = 0x00
      yieldManager!,
    ]);

    // --- Output calldata ---
    console.log("\n" + "=".repeat(80));
    console.log("Transaction Calldata for Safe Multisig:");
    console.log("=".repeat(80));
    console.log("\n1. pauseStaking");
    console.log("   Target Contract:", yieldManager);
    console.log("   Function: pauseStaking(address)");
    console.log("   Calldata:", pauseStakingCalldata);
    console.log("\n2. grantRole (Grant Dashboard DEFAULT_ADMIN_ROLE to YieldManager)");
    console.log("   Target Contract:", dashboard);
    console.log("   Function: grantRole(bytes32,address)");
    console.log("   Calldata:", grantRoleCalldata);
    console.log("\n" + "=".repeat(80));
  });
