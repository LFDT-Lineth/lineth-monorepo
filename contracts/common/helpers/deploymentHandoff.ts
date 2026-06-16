/**
 * In-process address handoff for chained deploy scripts.
 *
 * When a deploy script produces an address that a subsequent script in the same
 * `hardhat deploy` run needs (e.g. PlonkVerifier → LineaRollup, BridgedToken → TokenBridge),
 * the producer calls setHandoffAddress instead of writing to process.env. This prevents
 * requireAddressFromRegistryOrEnv from treating the freshly deployed address as an operator
 * override and conflicting with a stale registry entry for the same key on stable networks.
 *
 * Values are never persisted between process invocations.
 */

const store = new Map<string, string>();

export function setHandoffAddress(envVarName: string, address: string): void {
  store.set(envVarName, address);
}

export function getHandoffAddress(envVarName: string): string | undefined {
  return store.get(envVarName);
}

/** Call once per deploy run (e.g. at the start of TASK_DEPLOY_RUN_DEPLOY) so that stale values
 * from a previous run in the same process — possible in Hardhat test suites — do not leak. */
export function clearHandoffStore(): void {
  store.clear();
}
