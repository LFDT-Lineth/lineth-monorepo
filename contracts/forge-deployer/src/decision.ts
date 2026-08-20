import { DeploymentStep } from "./address-plan";
import { DeploymentCheckpoint } from "./checkpoint";

export type StepAction = { action: "deploy" | "recover" | "skip" };

interface DecideStepActionInput {
  step: DeploymentStep;
  checkpoint: DeploymentCheckpoint;
  codeByKey: Record<string, boolean>;
  currentNonce: number;
}

export function decideStepAction(input: DecideStepActionInput): StepAction {
  const { step, checkpoint, codeByKey, currentNonce } = input;
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
