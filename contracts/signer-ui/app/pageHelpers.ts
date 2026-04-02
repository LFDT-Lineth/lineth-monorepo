export type TransactionProgressStage =
  | "awaiting_wallet_approval"
  | "submitted_waiting_for_rpc"
  | "validating_submitted_transaction"
  | "waiting_for_hardhat_confirmation"
  | "failed";

export type WorkflowStatusStage = "waiting_for_transaction_receipt" | "waiting_for_contract_verification";

export type WorkflowStatus = {
  stage: WorkflowStatusStage;
  message: string;
  updatedAt: string;
};

export type TransactionDetails = {
  contractName?: string;
  constructorArgs?: unknown;
  initializerArgs?: unknown;
  proxyOptions?: string;
  notes?: string;
  openZeppelinProxyKind?: "transparent" | "uups" | "beacon";
} | null;

export function progressStageLabel(stage: TransactionProgressStage): string {
  switch (stage) {
    case "awaiting_wallet_approval":
      return "Awaiting wallet approval";
    case "submitted_waiting_for_rpc":
      return "Waiting for RPC broadcast";
    case "validating_submitted_transaction":
      return "Validating submitted transaction";
    case "waiting_for_hardhat_confirmation":
      return "Waiting for Hardhat confirmation";
    case "failed":
      return "Transaction failed";
  }
}

export function progressTone(stage: TransactionProgressStage): "active" | "error" {
  return stage === "failed" ? "error" : "active";
}

export function workflowStageLabel(stage: WorkflowStatusStage): string {
  switch (stage) {
    case "waiting_for_transaction_receipt":
      return "Waiting for transaction receipt";
    case "waiting_for_contract_verification":
      return "Waiting for contract verification";
  }
}

export function nextWorkflowDisplayState(params: {
  incomingWorkflow: WorkflowStatus | null | undefined;
  currentWorkflow: WorkflowStatus | null;
  currentExpiresAtMs: number;
  nowMs: number;
  minimumVisibilityMs?: number;
}): { workflow: WorkflowStatus | null; expiresAtMs: number } {
  const minVisibilityMs = params.minimumVisibilityMs ?? 2000;
  if (params.incomingWorkflow) {
    return {
      workflow: params.incomingWorkflow,
      expiresAtMs: params.nowMs + minVisibilityMs,
    };
  }

  if (params.currentWorkflow && params.nowMs < params.currentExpiresAtMs) {
    return {
      workflow: params.currentWorkflow,
      expiresAtMs: params.currentExpiresAtMs,
    };
  }

  return {
    workflow: null,
    expiresAtMs: 0,
  };
}

export function signerUiHistoryStorageKey(apiBaseUrl: string, sessionId: string): string {
  return `signerUiTxHistory:${apiBaseUrl}:${sessionId}`;
}

export function signerUiLastSessionIdStorageKey(apiBaseUrl: string): string {
  return `signerUiLastSessionId:${apiBaseUrl}`;
}

export function signerUiSessionSecretStorageKey(apiBaseUrl: string): string {
  return `signerUiSessionSecret:${apiBaseUrl}`;
}

export function transactionExplorerUrl(blockExplorerUrls: string[], txHash: string): string | null {
  const base = blockExplorerUrls.find((u) => typeof u === "string" && u.length > 0);
  if (!base) {
    return null;
  }
  const trimmed = base.replace(/\/$/, "");
  return `${trimmed}/tx/${txHash}`;
}

/** Stable fragment for in-page links / `:target` (request ids are UUIDs). */
export function signerTxFragmentId(requestId: string): string {
  return `signer-tx-${requestId.replace(/[^a-zA-Z0-9_-]/g, "-")}`;
}

export function scrollToHistoryFragment(frag: string): void {
  window.history.replaceState(null, "", `${window.location.pathname}${window.location.search}#${frag}`);
  const run = () => document.getElementById(frag)?.scrollIntoView({ behavior: "smooth", block: "center" });
  run();
  requestAnimationFrame(run);
  window.setTimeout(run, 100);
}

export function transactionKindBadgeLabels(details: TransactionDetails | undefined): string[] {
  if (!details) {
    return [];
  }

  const labels: string[] = [];
  if (details.openZeppelinProxyKind === "uups") {
    labels.push("UUPS proxy (OpenZeppelin)");
  } else if (details.openZeppelinProxyKind === "transparent") {
    labels.push("Transparent proxy (OpenZeppelin)");
  } else if (details.openZeppelinProxyKind === "beacon") {
    labels.push("Beacon proxy (OpenZeppelin)");
  }

  const name = (details.contractName ?? "").replace(/\s+/g, "");
  if (/proxyadmin/i.test(name)) {
    labels.push("Proxy admin contract");
  }
  return labels;
}

export function firstWalletAddress(accounts: readonly string[] | null | undefined): string | null {
  if (!accounts || accounts.length === 0) {
    return null;
  }
  const [first] = accounts;
  return typeof first === "string" && first.length > 0 ? first : null;
}

export function signerAddressMismatch(
  expectedSignerAddress: string | null | undefined,
  connectedWalletAddress: string | null | undefined,
): boolean {
  if (!expectedSignerAddress || !connectedWalletAddress) {
    return false;
  }
  return expectedSignerAddress.toLowerCase() !== connectedWalletAddress.toLowerCase();
}

export function canEnableSignButton(params: {
  isSubmitting: boolean;
  walletUiReady: boolean;
  pendingRequestId: string | null | undefined;
  connectedWalletAddress: string | null | undefined;
  expectedSignerAddress: string | null | undefined;
}): boolean {
  if (params.isSubmitting || !params.walletUiReady || !params.pendingRequestId || !params.connectedWalletAddress) {
    return false;
  }
  return !signerAddressMismatch(params.expectedSignerAddress, params.connectedWalletAddress);
}
