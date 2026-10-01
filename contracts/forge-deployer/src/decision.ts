import { DeploymentStep } from "./address-plan";
import { DeploymentCheckpoint } from "./checkpoint";

/**
 * "adopt" is distinct from "recover": "recover" marks a step complete when
 * every deployment already has a checkpoint record (only the completion
 * marker is missing). "adopt" is for steps whose deployments can legitimately
 * exist on-chain with *no* checkpoint record at all (e.g. a well-known,
 * non-nonce-derived address a chain preinstalls in genesis) — the caller must
 * synthesize checkpoint records for each deployment before marking it complete.
 */
export type StepAction = { action: "deploy" | "recover" | "skip" | "adopt" };

/**
 * Whether on-chain code at a step's well-known (non-nonce-derived) address is
 * the expected contract. Only relevant for steps that can be legitimately
 * pre-deployed outside of this tool's own deployment flow.
 */
export type WellKnownCodeStatus = "match" | "mismatch";

interface DecideStepActionInput {
  step: DeploymentStep;
  checkpoint: DeploymentCheckpoint;
  codeByKey: Record<string, boolean>;
  currentNonce: number;
  /**
   * Present only for steps whose deployments live at a well-known address
   * rather than a nonce-derived one. Called only when code exists without a
   * checkpoint record, to distinguish "the expected contract is already
   * there" from "something unexpected occupies this address".
   */
  verifyWellKnownCode?: () => Promise<WellKnownCodeStatus>;
}

export async function decideStepAction(input: DecideStepActionInput): Promise<StepAction> {
  const { step, checkpoint, codeByKey, currentNonce, verifyWellKnownCode } = input;
  const records = step.deployments.map((deployment) => checkpoint.deployments[deployment.key]);

  for (const [index, record] of records.entries()) {
    if (!record) continue;
    const expected = step.deployments[index]!;
    if (record.address.toLowerCase() !== expected.expectedAddress.toLowerCase()) {
      throw new Error(
        `checkpointed deployment ${expected.key} has address ${record.address}, expected ${expected.expectedAddress}`,
      );
    }
    if (!codeByKey[expected.key]) {
      throw new Error(`checkpointed deployment ${expected.key} has no bytecode at ${record.address}`);
    }
  }

  const codeStates = step.deployments.map((deployment) => Boolean(codeByKey[deployment.key]));
  const hasAnyCode = codeStates.some(Boolean);
  const hasAllCode = codeStates.every(Boolean);
  const hasAllRecords = records.every((record) => record !== undefined);

  if (hasAllCode) {
    if (!hasAllRecords) {
      if (verifyWellKnownCode) {
        const status = await verifyWellKnownCode();
        if (status === "mismatch") {
          throw new Error(`refusing to recover ${step.id}; unexpected bytecode at its well-known address`);
        }
        return { action: "adopt" };
      }
      throw new Error(`uncheckpointed code detected for ${step.id}; refusing to infer contract identity`);
    }
    return checkpoint.completedSteps.includes(step.id) ? { action: "skip" } : { action: "recover" };
  }
  if (hasAnyCode) {
    throw new Error(`partial deployment detected for ${step.id}; refusing to create a second contract stack`);
  }
  if (checkpoint.completedSteps.includes(step.id)) {
    throw new Error(`checkpoint marks ${step.id} complete but its bytecode is missing`);
  }
  if (currentNonce !== step.startingNonce) {
    throw new Error(
      `nonce drift for ${step.id}: expected ${step.startingNonce}, found ${currentNonce}; refusing deployment`,
    );
  }

  return { action: "deploy" };
}
