import { loadConfig, sanitizeText } from "./config";
import { runDeployment } from "./runner";
import { createCheckpointStore } from "./store";

async function main(): Promise<void> {
  const config = loadConfig();
  const store = await createCheckpointStore(config);
  const checkpoint = await runDeployment(config, store);
  const publicAddresses = {
    linethRollup: checkpoint.deployments["l1-rollup.proxy"]?.address,
    l2MessageService: checkpoint.deployments["l2-message-service.proxy"]?.address,
    l1TokenBridge: checkpoint.deployments["l1-token-bridge.proxy"]?.address,
    l2TokenBridge: checkpoint.deployments["l2-token-bridge.proxy"]?.address,
  };
  console.log(`Contract deployment complete: ${JSON.stringify(publicAddresses)}`);
}

main().catch((error: unknown) => {
  const sensitiveValues: string[] = [];
  for (const name of ["L1_DEPLOYER_PRIVATE_KEY", "L2_DEPLOYER_PRIVATE_KEY", "L1_RPC_URL", "L2_RPC_URL"]) {
    if (process.env[name]) sensitiveValues.push(process.env[name]!);
  }
  const detail = error instanceof Error ? (error.stack ?? error.message) : String(error);
  console.error(sanitizeText(detail, sensitiveValues));
  process.exitCode = 1;
});
