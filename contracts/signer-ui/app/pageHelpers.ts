export type TransactionProgressStage =
  | "awaiting_wallet_approval"
  | "submitted_waiting_for_rpc"
  | "validating_submitted_transaction"
  | "waiting_for_hardhat_confirmation"
  | "failed";

export type TransactionDetails = {
  contractName?: string;
  notes?: string;
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

export function signerUiHistoryStorageKey(apiBaseUrl: string, sessionId: string): string {
  return `signerUiTxHistory:${apiBaseUrl}:${sessionId}`;
}

export function signerUiLastSessionIdStorageKey(apiBaseUrl: string): string {
  return `signerUiLastSessionId:${apiBaseUrl}`;
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
