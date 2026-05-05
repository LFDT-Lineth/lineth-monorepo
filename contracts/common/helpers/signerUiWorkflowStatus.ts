type WorkflowStatusStage = "waiting_for_transaction_receipt" | "waiting_for_contract_verification";

type SignerUiBridgeWorkflowApi = {
  setUiWorkflowStatus: (stage: WorkflowStatusStage, message: string) => void;
  clearUiWorkflowStatus: () => void;
};

let signerUiBridgeWorkflowApiPromise: Promise<SignerUiBridgeWorkflowApi | null> | undefined;

async function loadSignerUiBridgeWorkflowApi(): Promise<SignerUiBridgeWorkflowApi | null> {
  if (!signerUiBridgeWorkflowApiPromise) {
    signerUiBridgeWorkflowApiPromise = import("../../scripts/hardhat/signer-ui-bridge.js")
      .then((mod) => ({
        setUiWorkflowStatus: mod.setUiWorkflowStatus,
        clearUiWorkflowStatus: mod.clearUiWorkflowStatus,
      }))
      .catch(() => null);
  }

  return signerUiBridgeWorkflowApiPromise;
}

export async function setSignerUiWorkflowStatus(stage: WorkflowStatusStage, message: string): Promise<void> {
  const api = await loadSignerUiBridgeWorkflowApi();
  api?.setUiWorkflowStatus(stage, message);
}

export async function clearSignerUiWorkflowStatus(): Promise<void> {
  const api = await loadSignerUiBridgeWorkflowApi();
  api?.clearUiWorkflowStatus();
}
