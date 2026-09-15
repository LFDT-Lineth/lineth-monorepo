import { L2_DETERMINISTIC_PROXY_FACTORY_KEY } from "./address-plan";
import { loadConfig, sanitizeText, SENSITIVE_ENV_VAR_NAMES } from "./config";
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
    deterministicDeploymentProxy: checkpoint.deployments[L2_DETERMINISTIC_PROXY_FACTORY_KEY]?.address,
  };
  console.log(`Contract deployment complete: ${JSON.stringify(publicAddresses)}`);
}

main().catch((error: unknown) => {
  const sensitiveValues: string[] = [];
  for (const name of SENSITIVE_ENV_VAR_NAMES) {
    if (process.env[name]) sensitiveValues.push(process.env[name]!);
  }
  const detail = error instanceof Error ? (error.stack ?? error.message) : String(error);
  console.error(sanitizeText(detail, sensitiveValues));
  process.exitCode = 1;
});
