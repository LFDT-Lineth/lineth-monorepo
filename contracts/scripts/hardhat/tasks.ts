import { task } from "hardhat/config";
import { ArgumentType } from "hardhat/types/arguments";

/**
 * Operational tasks. Every option is an optional string (`--kebab-case` on the CLI) whose fallback is an
 * environment variable resolved inside the action via `getTaskCliOrEnvValue`, as in Hardhat 2.
 * Actions are loaded lazily, so a task module is only imported when that task runs.
 */
function optionalString<NameT extends string>(name: NameT) {
  return { name, type: ArgumentType.STRING_WITHOUT_DEFAULT, defaultValue: undefined } as const;
}

export const operationalTasks = [
  task("getCurrentFinalizedBlockNumber", "Gets the finalized block number")
    .addOption(optionalString("contractType"))
    .addOption(optionalString("proxyAddress"))
    .setAction(() => import("../operational/tasks/getCurrentFinalizedBlockNumberTask.js"))
    .build(),
  task("grantContractRoles", "Grants roles to specific accounts")
    .addOption(optionalString("adminAddress"))
    .addOption(optionalString("proxyAddress"))
    .addOption(optionalString("contractType"))
    .addOption(optionalString("contractRoles"))
    .setAction(() => import("../operational/tasks/grantContractRolesTask.js"))
    .build(),
  task("renounceContractRoles", "Sets the rate limit on a Message Service contract")
    .addOption(optionalString("oldAdminAddress"))
    .addOption(optionalString("newAdminAddress"))
    .addOption(optionalString("proxyAddress"))
    .addOption(optionalString("contractType"))
    .addOption(optionalString("contractRoles"))
    .setAction(() => import("../operational/tasks/renounceContractRolesTask.js"))
    .build(),
  task("setMessageServiceOnTokenBridge", "Sets The Message Service On A TokenBridge")
    .addOption(optionalString("messageServiceAddress"))
    .addOption(optionalString("tokenBridgeAddress"))
    .setAction(() => import("../operational/tasks/setMessageServiceOnTokenBridgeTask.js"))
    .build(),
  task("setRateLimit", "Sets the rate limit on a Message Service contract")
    .addOption(optionalString("messageServiceAddress"))
    .addOption(optionalString("messageServiceType"))
    .addOption(optionalString("withdrawLimit"))
    .setAction(() => import("../operational/tasks/setRateLimitTask.js"))
    .build(),
  task("setVerifierAddress", "Sets the verifier address on a Message Service contract")
    .addOption(optionalString("verifierProofType"))
    .addOption(optionalString("proxyAddress"))
    .addOption(optionalString("verifierAddress"))
    .addOption(optionalString("verifierName"))
    .setAction(() => import("../operational/tasks/setVerifierAddressTask.js"))
    .build(),
  task("addLidoStVaultYieldProvider", "Generates parameters for adding and configuring a new LidoStVaultYieldProvider")
    .addOption(optionalString("yieldManager"))
    .addOption(optionalString("yieldProvider"))
    .addOption(optionalString("nodeOperator"))
    .addOption(optionalString("securityCouncil"))
    .addOption(optionalString("nodeOperatorFee"))
    .addOption(optionalString("confirmExpiry"))
    .setAction(() => import("../operational/yieldBoost/addLidoStVaultYieldProvider.js"))
    .build(),
  task("prepareInitiateOssification", "Generates calldata for prerequisites before initiating ossification")
    .addOption(optionalString("yieldManager"))
    .addOption(optionalString("yieldProvider"))
    .addOption(optionalString("dashboard"))
    .setAction(() => import("../operational/yieldBoost/prepareInitiateOssification.js"))
    .build(),
  task(
    "addAndClaimMessage",
    "Setup and execute TestLinethRollup.claimMessageWithProof by adding L2->L1 message merkle tree root",
  )
    .addOption(optionalString("linethRollupAddress"))
    .addOption(optionalString("from"))
    .addOption(optionalString("to"))
    .addOption(optionalString("value"))
    .addOption(optionalString("data"))
    .setAction(() => import("../operational/yieldBoost/testing/addAndClaimMessage.js"))
    .build(),
  task(
    "addAndClaimMessageForLST",
    "Setup and execute TestLinethRollup.claimMessageWithProofAndWithdrawLST by adding L2->L1 message merkle tree root",
  )
    .addOption(optionalString("linethRollupAddress"))
    .addOption(optionalString("from"))
    .addOption(optionalString("to"))
    .addOption(optionalString("value"))
    .addOption(optionalString("data"))
    .addOption(optionalString("yieldProvider"))
    .setAction(() => import("../operational/yieldBoost/testing/addAndClaimMessageForLST.js"))
    .build(),
  task("unstakePermissionless", "Performs YieldManager::unstakePermissionless (current contract version)")
    .addOption(optionalString("yieldManager"))
    .addOption(optionalString("yieldProvider"))
    .addOption(optionalString("validatorIndex"))
    .addOption(optionalString("slot"))
    .addOption(optionalString("beaconRpcUrl"))
    .setAction(() => import("../operational/yieldBoost/testing/unstakePermissionless.js"))
    .build(),
];

/**
 * Ignition deployment tasks (replace hardhat-deploy `deploy --tags`). Actions are imported lazily so a module's env-driven
 * shape (e.g. `VERIFIER_CONTRACT_NAME`) is only read when that task runs. See docs/deployment/README.md.
 */
/** Presigned system-contract deployments (Nick's method); everything else deploys with `hardhat ignition deploy`. */
export const deployTasks = [
  task("deploy:eip2935-system-contract", "Broadcasts the presigned EIP-2935 system contract deployment")
    .setInlineAction(async (args, hre) =>
      (await import("./systemContractTasks.js")).eip2935SystemContractAction(args, hre),
    )
    .build(),
  task("deploy:eip4788-system-contract", "Broadcasts the presigned EIP-4788 system contract deployment")
    .setInlineAction(async (args, hre) =>
      (await import("./systemContractTasks.js")).eip4788SystemContractAction(args, hre),
    )
    .build(),
];
