"use client";

import { Suspense, useEffect, useMemo, useRef, useState } from "react";

import { useSearchParams } from "next/navigation";
import { flushSync } from "react-dom";

import { HARDHAT_SIGNER_UI_SESSION_TOKEN_HEADER, postJson } from "./bridgeApiClient";
import { LineaWordmark } from "./components/LineaWordmark";
import {
  canEnableSignButton,
  firstWalletAddress,
  nextWorkflowDisplayState,
  progressStageLabel,
  progressTone,
  scrollToHistoryFragment,
  signerAddressMismatch,
  signerTxFragmentId,
  signerUiHistoryStorageKey,
  signerUiLastSessionIdStorageKey,
  signerUiSessionSecretStorageKey,
  transactionExplorerUrl,
  transactionKindBadgeLabels,
  type TransactionDetails,
  type TransactionProgressStage,
  type WorkflowStatus,
  workflowStageLabel,
} from "./pageHelpers";

declare global {
  interface Window {
    ethereum?: {
      on?: (event: string, handler: (...args: unknown[]) => void) => void;
      removeListener?: (event: string, handler: (...args: unknown[]) => void) => void;
      request: <T>(request: { method: string; params?: unknown[] }) => Promise<T>;
    };
  }
}

type ChainMetadata = {
  chainId: number;
  chainName: string;
  rpcUrls: string[];
  blockExplorerUrls: string[];
  nativeCurrency: {
    name: string;
    symbol: string;
    decimals: number;
  };
};

type TransactionPrompt = {
  id: string;
  label: string;
  description: string;
  createdAt: string;
  request: {
    to?: string;
    data?: string;
    value?: string;
    gas?: string;
    gasPrice?: string;
    maxFeePerGas?: string;
    maxPriorityFeePerGas?: string;
    nonce?: string;
    type?: string;
    chainId?: string;
  };
  transactionDetails?: TransactionDetails;
};

type SessionState = {
  sessionId: string;
  scriptContext: string;
  networkName: string;
  chain: ChainMetadata;
  expectedSignerAddress?: string | null;
  wallet: {
    address: string;
    chainId: number;
    connectedAt: string;
  } | null;
  pendingRequest: TransactionPrompt | null;
  transactionProgress?: {
    requestId: string;
    stage: TransactionProgressStage;
    message: string;
    updatedAt: string;
  } | null;
  workflowStatus?: WorkflowStatus | null;
  startedAt: string;
  scriptOrdinal?: number;
  batchRunActive?: boolean;
  batchTagsSummary?: string | null;
  /** Set by the bridge just before a deploy batch closes so the UI can stop polling. */
  sessionOutcome?: "complete" | "error" | null;
  outcomeMessage?: string | null;
};

/** One wallet-submitted tx for this browser session; persisted so the UI stays useful after the bridge closes. */
type CompletedDeploymentTx = {
  requestId: string;
  completedAt: string;
  label: string;
  description: string;
  txHash: string;
  from: string;
  chainId: number;
  chainName: string;
  blockExplorerUrls: string[];
  request: TransactionPrompt["request"];
  transactionDetails: TransactionPrompt["transactionDetails"];
};

function PendingTransactionProgress({ progress }: { progress: NonNullable<SessionState["transactionProgress"]> }) {
  const tone = progressTone(progress.stage);
  return (
    <div className={`deploy-progress deploy-progress--${tone}`} aria-live="polite">
      <div className="deploy-progress__row">
        {tone === "active" ? <span className="deploy-spinner" aria-hidden="true" /> : null}
        <div className="deploy-progress__copy">
          <p className="deploy-progress__stage">{progressStageLabel(progress.stage)}</p>
          <p className="deploy-progress__message">{progress.message}</p>
        </div>
      </div>
    </div>
  );
}

function WorkflowStatusPanel({ workflow }: { workflow: NonNullable<SessionState["workflowStatus"]> }) {
  return (
    <div className="deploy-progress deploy-progress--active" aria-live="polite">
      <div className="deploy-progress__row">
        <span className="deploy-spinner" aria-hidden="true" />
        <div className="deploy-progress__copy">
          <p className="deploy-progress__stage">{workflowStageLabel(workflow.stage)}</p>
          <p className="deploy-progress__message">{workflow.message}</p>
        </div>
      </div>
    </div>
  );
}

function loadDeployHistoryFromStorage(key: string): CompletedDeploymentTx[] {
  try {
    const raw = sessionStorage.getItem(key);
    if (!raw) {
      return [];
    }
    const parsed = JSON.parse(raw) as unknown;
    return Array.isArray(parsed) ? (parsed as CompletedDeploymentTx[]) : [];
  } catch {
    return [];
  }
}

function persistDeployHistory(key: string, items: CompletedDeploymentTx[]): void {
  sessionStorage.setItem(key, JSON.stringify(items));
}

function TransactionKindBadges({ details }: { details: TransactionPrompt["transactionDetails"] }) {
  const labels = transactionKindBadgeLabels(details);
  if (labels.length === 0) {
    return null;
  }
  return (
    <div className="deploy-kind-badges" aria-label="Transaction context">
      {labels.map((label, index) => (
        <span key={`${label}-${index}`} className="deploy-kind-badge">
          {label}
        </span>
      ))}
    </div>
  );
}

const WORKFLOW_STATUS_MIN_VISIBILITY_MS = 2000;

function ContractsDeployUiPage() {
  const searchParams = useSearchParams();
  const apiBaseUrl = searchParams.get("apiBaseUrl");
  const sessionSecretFromUrl = searchParams.get("sessionToken");
  const signerUiDebug = searchParams.get("debugSignerUi") === "1";
  const [sessionSecret, setSessionSecret] = useState<string | null>(null);
  const [sessionAuthReady, setSessionAuthReady] = useState(false);
  const [session, setSession] = useState<SessionState | null>(null);
  const [deployHistory, setDeployHistory] = useState<CompletedDeploymentTx[]>([]);
  const [statusMessage, setStatusMessage] = useState<string>("Waiting for Hardhat signer session...");
  const [actionError, setActionError] = useState<string | null>(null);
  const [isConnectingWallet, setIsConnectingWallet] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isTerminating, setIsTerminating] = useState(false);
  const [workflowDisplayState, setWorkflowDisplayState] = useState<{
    workflow: WorkflowStatus | null;
    expiresAtMs: number;
  }>({
    workflow: null,
    expiresAtMs: 0,
  });
  /** Wagmi reconnect differs SSR vs first client paint — gate wallet UI until mount. */
  const [walletUiReady, setWalletUiReady] = useState(false);
  const [providerWalletAddress, setProviderWalletAddress] = useState<string | null>(null);
  const [sessionEnded, setSessionEnded] = useState(false);
  const hadSuccessfulBridgeFetch = useRef(false);
  /** After terminal `sessionOutcome`, ignore late poll results (e.g. connection errors). */
  const pollingTerminatedRef = useRef(false);
  /** Prevents double submission before React re-renders (wallet popup / network switch). */
  const approveTransactionFlowRef = useRef(false);

  useEffect(() => {
    setWalletUiReady(true);
  }, []);

  useEffect(() => {
    if (typeof window === "undefined" || !window.ethereum) {
      setProviderWalletAddress(null);
      return;
    }

    let cancelled = false;
    const syncProviderWalletAddress = async () => {
      try {
        const accounts = await window.ethereum?.request<string[]>({ method: "eth_accounts" });
        if (!cancelled) {
          setProviderWalletAddress(firstWalletAddress(accounts));
        }
      } catch {
        if (!cancelled) {
          setProviderWalletAddress(null);
        }
      }
    };

    void syncProviderWalletAddress();

    const accountsChanged = (accounts: unknown) => {
      setProviderWalletAddress(firstWalletAddress(Array.isArray(accounts) ? (accounts as string[]) : undefined));
    };

    window.ethereum.on?.("accountsChanged", accountsChanged);

    return () => {
      cancelled = true;
      window.ethereum?.removeListener?.("accountsChanged", accountsChanged);
    };
  }, []);

  useEffect(() => {
    if (!signerUiDebug) {
      return;
    }
    console.info("[signer-ui] wallet state", {
      providerWalletAddress,
      sessionWalletAddress: session?.wallet?.address ?? null,
      resolvedWalletAddress: providerWalletAddress,
      expectedSignerAddress: session?.expectedSignerAddress ?? null,
      pendingRequestId: session?.pendingRequest?.id ?? null,
      sessionEnded,
      walletUiReady,
      actionError,
    });
  }, [
    actionError,
    providerWalletAddress,
    session?.expectedSignerAddress,
    session?.pendingRequest?.id,
    session?.wallet?.address,
    sessionEnded,
    signerUiDebug,
    walletUiReady,
  ]);

  useEffect(() => {
    if (!apiBaseUrl || !session?.sessionId) {
      return;
    }
    try {
      sessionStorage.setItem(signerUiLastSessionIdStorageKey(apiBaseUrl), session.sessionId);
    } catch {
      /* storage full or disabled */
    }
    const key = signerUiHistoryStorageKey(apiBaseUrl, session.sessionId);
    setDeployHistory(loadDeployHistoryFromStorage(key));
  }, [apiBaseUrl, session?.sessionId]);

  /** Rehydrate tx history after Fast Refresh before `/api/session` returns (sessionId in sessionStorage). */
  useEffect(() => {
    if (typeof window === "undefined" || !apiBaseUrl || !sessionAuthReady) {
      return;
    }
    if (session?.sessionId) {
      return;
    }
    const lastId = sessionStorage.getItem(signerUiLastSessionIdStorageKey(apiBaseUrl));
    if (!lastId) {
      return;
    }
    setDeployHistory(loadDeployHistoryFromStorage(signerUiHistoryStorageKey(apiBaseUrl, lastId)));
  }, [apiBaseUrl, sessionAuthReady, session?.sessionId]);

  useEffect(() => {
    if (!apiBaseUrl) {
      setSessionSecret(null);
      setSessionAuthReady(true);
      return;
    }

    if (sessionSecretFromUrl) {
      setSessionSecret(sessionSecretFromUrl);
      const nextUrl = new URL(window.location.href);
      nextUrl.searchParams.delete("sessionToken");
      window.history.replaceState({}, "", `${nextUrl.pathname}${nextUrl.search}${nextUrl.hash}`);
      setSessionAuthReady(true);
      return;
    }

    const storageKey = signerUiSessionSecretStorageKey(apiBaseUrl);
    setSessionSecret((previousSessionSecret) => {
      return previousSessionSecret ?? sessionStorage.getItem(storageKey);
    });
    setSessionAuthReady(true);
  }, [apiBaseUrl, sessionSecretFromUrl]);

  const sessionUrl = useMemo(() => {
    if (!apiBaseUrl) {
      return undefined;
    }

    return `${apiBaseUrl}/api/session`;
  }, [apiBaseUrl]);

  const deployRunLooksComplete = sessionEnded && deployHistory.length > 0 && session?.pendingRequest === null;

  const heroSubtitle = useMemo(() => {
    if (!sessionEnded) {
      return statusMessage;
    }
    if (session?.sessionOutcome === "error") {
      if (deployHistory.length > 0) {
        return "Run failed. Submitted transactions are preserved below; polling has stopped.";
      }
      return "Run failed. Polling has stopped; check the terminal for details.";
    }
    if (session?.sessionOutcome === "complete") {
      if (deployHistory.length > 0 && session.pendingRequest === null) {
        return "Run complete. The Hardhat bridge has closed; submitted transactions are saved below and polling has stopped.";
      }
      if (deployHistory.length > 0) {
        return "Run complete. Submitted transactions are preserved below; polling has stopped.";
      }
      return "Run complete. The Hardhat bridge has closed; polling has stopped.";
    }
    if (deployHistory.length > 0 && session?.pendingRequest === null) {
      return "Run complete. The Hardhat bridge has closed; submitted transactions are saved below and polling has stopped.";
    }
    if (deployHistory.length > 0) {
      return "Session ended. Submitted transactions are preserved below; polling has stopped.";
    }
    return "Signer session ended (bridge closed). Polling has stopped.";
  }, [sessionEnded, deployHistory.length, session?.pendingRequest, session?.sessionOutcome, statusMessage]);

  useEffect(() => {
    if (!sessionAuthReady) {
      return;
    }

    if (!apiBaseUrl) {
      setActionError("Missing apiBaseUrl query parameter.");
      return;
    }

    if (!sessionSecret) {
      setActionError(
        "Missing session token. Open this UI using the full URL from Hardhat (HARDHAT_SIGNER_UI), not a bookmark without sessionToken.",
      );
      return;
    }

    if (!sessionUrl) {
      return;
    }

    if (sessionEnded) {
      return;
    }

    let cancelled = false;
    pollingTerminatedRef.current = false;

    const load = async () => {
      if (pollingTerminatedRef.current) {
        return;
      }
      try {
        const response = await fetch(sessionUrl, {
          headers: { [HARDHAT_SIGNER_UI_SESSION_TOKEN_HEADER]: sessionSecret },
        });
        if (!response.ok) {
          if (!cancelled && !pollingTerminatedRef.current && hadSuccessfulBridgeFetch.current) {
            setSessionEnded(true);
          } else if (!cancelled && !pollingTerminatedRef.current && !hadSuccessfulBridgeFetch.current) {
            setActionError(`Session request failed (${response.status}).`);
          }
          return;
        }
        const data = (await response.json()) as SessionState;
        if (cancelled || pollingTerminatedRef.current) {
          return;
        }
        setWorkflowDisplayState((prev) =>
          nextWorkflowDisplayState({
            incomingWorkflow: data.workflowStatus ?? null,
            currentWorkflow: prev.workflow,
            currentExpiresAtMs: prev.expiresAtMs,
            nowMs: Date.now(),
            minimumVisibilityMs: WORKFLOW_STATUS_MIN_VISIBILITY_MS,
          }),
        );

        if (data.sessionOutcome === "complete" || data.sessionOutcome === "error") {
          pollingTerminatedRef.current = true;
          hadSuccessfulBridgeFetch.current = true;
          setSession(data);
          setSessionEnded(true);
          if (data.sessionOutcome === "error") {
            const detail = data.outcomeMessage?.trim();
            setStatusMessage(detail ? `Run failed: ${detail}` : "Run failed. Check the terminal for details.");
          } else {
            setStatusMessage(
              "Run complete. The Hardhat bridge is closing; submitted transactions stay in the list below.",
            );
          }
          setActionError(null);
          return;
        }

        hadSuccessfulBridgeFetch.current = true;
        setSession(data);
        const hasExpectedSignerMismatch =
          !!data.expectedSignerAddress &&
          !!data.wallet?.address &&
          data.expectedSignerAddress.toLowerCase() !== data.wallet.address.toLowerCase();
        setStatusMessage(
          hasExpectedSignerMismatch
            ? `Expected signer is ${data.expectedSignerAddress}. Disconnect and connect the correct wallet.`
            : `Connected to ${data.scriptContext} on ${data.networkName}`,
        );
        setActionError(null);
      } catch {
        if (!cancelled && !pollingTerminatedRef.current) {
          if (hadSuccessfulBridgeFetch.current) {
            setSessionEnded(true);
          } else {
            setActionError("Failed to reach the signer bridge. Check apiBaseUrl and that Hardhat started the session.");
          }
        }
      }
    };

    void load();
    const interval = window.setInterval(() => {
      void load();
    }, 1500);

    return () => {
      cancelled = true;
      window.clearInterval(interval);
    };
  }, [sessionUrl, sessionEnded, sessionAuthReady, sessionSecret, apiBaseUrl]);

  useEffect(() => {
    if (deployHistory.length === 0 || typeof window === "undefined") {
      return;
    }
    const raw = window.location.hash.slice(1);
    if (!raw.startsWith("signer-tx-")) {
      return;
    }
    window.setTimeout(() => scrollToHistoryFragment(raw), 0);
  }, [deployHistory.length]);

  useEffect(() => {
    const activeWalletAddress = providerWalletAddress;
    if (!apiBaseUrl || !sessionSecret || !activeWalletAddress || !window.ethereum || sessionEnded) {
      return;
    }

    const syncWalletState = async () => {
      try {
        const currentChainIdHex = await window.ethereum?.request<string>({ method: "eth_chainId" });
        if (!currentChainIdHex) {
          return;
        }

        await postJson(
          `${apiBaseUrl}/api/wallet`,
          {
            address: activeWalletAddress,
            chainId: parseInt(currentChainIdHex, 16),
          },
          sessionSecret,
        );
        setActionError(null);
      } catch (error) {
        setActionError((error as Error).message ?? "Failed to register wallet with the signer bridge.");
      }
    };

    void syncWalletState();

    const accountsChanged = () => {
      void syncWalletState();
    };
    const chainChanged = () => {
      void syncWalletState();
    };

    window.ethereum.on?.("accountsChanged", accountsChanged);
    window.ethereum.on?.("chainChanged", chainChanged);

    return () => {
      window.ethereum?.removeListener?.("accountsChanged", accountsChanged);
      window.ethereum?.removeListener?.("chainChanged", chainChanged);
    };
  }, [apiBaseUrl, providerWalletAddress, sessionSecret, sessionEnded]);

  useEffect(() => {
    if (typeof window === "undefined" || !workflowDisplayState.workflow) {
      return;
    }

    const clearIfExpired = () => {
      setWorkflowDisplayState((prev) =>
        nextWorkflowDisplayState({
          incomingWorkflow: null,
          currentWorkflow: prev.workflow,
          currentExpiresAtMs: prev.expiresAtMs,
          nowMs: Date.now(),
          minimumVisibilityMs: WORKFLOW_STATUS_MIN_VISIBILITY_MS,
        }),
      );
    };

    const remainingMs = workflowDisplayState.expiresAtMs - Date.now();
    if (remainingMs <= 0) {
      clearIfExpired();
      return;
    }

    const timeout = window.setTimeout(clearIfExpired, remainingMs);
    return () => {
      window.clearTimeout(timeout);
    };
  }, [workflowDisplayState.workflow, workflowDisplayState.expiresAtMs]);

  const ensureTargetChain = async (chain: ChainMetadata) => {
    if (!window.ethereum) {
      throw new Error("No injected wallet was found in this browser.");
    }

    const targetChainIdHex = `0x${chain.chainId.toString(16)}`;
    const currentChainId = await window.ethereum.request<string>({ method: "eth_chainId" });

    if (currentChainId === targetChainIdHex) {
      return;
    }

    try {
      await window.ethereum.request({
        method: "wallet_switchEthereumChain",
        params: [{ chainId: targetChainIdHex }],
      });
      return;
    } catch (error) {
      const switchError = error as { code?: number; message?: string };
      if (switchError.code !== 4902) {
        throw new Error(switchError.message ?? "Failed to switch to the target chain.");
      }
    }

    await window.ethereum.request({
      method: "wallet_addEthereumChain",
      params: [
        {
          chainId: targetChainIdHex,
          chainName: chain.chainName,
          rpcUrls: chain.rpcUrls,
          blockExplorerUrls: chain.blockExplorerUrls,
          nativeCurrency: chain.nativeCurrency,
        },
      ],
    });
    await window.ethereum.request({
      method: "wallet_switchEthereumChain",
      params: [{ chainId: targetChainIdHex }],
    });
  };

  const terminateSession = async () => {
    if (!apiBaseUrl || !sessionSecret || sessionEnded || isTerminating) {
      return;
    }
    setIsTerminating(true);
    setActionError(null);
    try {
      await postJson(`${apiBaseUrl}/api/terminate`, {}, sessionSecret);
      setStatusMessage("Session terminated. Waiting for Hardhat to stop.");
      setSessionEnded(true);
    } catch (error) {
      setActionError((error as Error).message ?? "Failed to terminate session.");
    } finally {
      setIsTerminating(false);
    }
  };

  const requestWalletAccount = async (): Promise<string | null> => {
    if (!window.ethereum) {
      throw new Error("No injected wallet was found in this browser.");
    }

    try {
      await window.ethereum.request({
        method: "wallet_requestPermissions",
        params: [{ eth_accounts: {} }],
      });
    } catch {
      // Some wallets do not support wallet_requestPermissions; fall back to eth_requestAccounts.
    }

    const requestedAccounts = await window.ethereum.request<string[]>({ method: "eth_requestAccounts" });
    return firstWalletAddress(requestedAccounts);
  };

  const connectWallet = async () => {
    if (!window.ethereum) {
      setActionError("No injected wallet was found in this browser.");
      return;
    }

    setActionError(null);
    setIsConnectingWallet(true);
    try {
      const nextAddress = await requestWalletAccount();
      if (signerUiDebug) {
        console.info("[signer-ui] eth_requestAccounts result", {
          nextAddress,
        });
      }
      if (!nextAddress) {
        throw new Error("Wallet did not expose an account after connection.");
      }
      setProviderWalletAddress(nextAddress);
    } catch (error) {
      setActionError((error as Error).message ?? "Failed to connect wallet.");
    } finally {
      setIsConnectingWallet(false);
    }
  };

  const disconnectWallet = () => {
    setProviderWalletAddress(null);
    setActionError(null);
  };

  const approvePendingRequest = async () => {
    let submissionAddress = providerWalletAddress ?? session?.wallet?.address ?? null;
    if (!apiBaseUrl || !sessionSecret || !session?.pendingRequest || !window.ethereum) {
      return;
    }

    if (!submissionAddress) {
      submissionAddress = await requestWalletAccount();
      if (!submissionAddress) {
        return;
      }
      setProviderWalletAddress(submissionAddress);
    }

    if (approveTransactionFlowRef.current || isSubmitting) {
      return;
    }
    approveTransactionFlowRef.current = true;
    setIsSubmitting(true);
    setActionError(null);

    try {
      const activeSubmissionAddress = submissionAddress;
      if (signerUiDebug) {
        console.info("[signer-ui] approvePendingRequest", {
          activeSubmissionAddress,
          expectedSignerAddress: session.expectedSignerAddress ?? null,
          pendingRequestId: session.pendingRequest.id,
          chainId: session.chain.chainId,
        });
      }
      if (signerAddressMismatch(session.expectedSignerAddress ?? null, activeSubmissionAddress)) {
        throw new Error(
          `Expected signer is ${session.expectedSignerAddress}. Disconnect and connect the correct wallet, then try again.`,
        );
      }

      await ensureTargetChain(session.chain);
      const currentChainIdHex = await window.ethereum.request<string>({ method: "eth_chainId" });
      const currentChainId = Number.parseInt(currentChainIdHex, 16);
      if (!Number.isInteger(currentChainId)) {
        throw new Error("Wallet did not return a valid chain ID.");
      }
      await postJson(
        `${apiBaseUrl}/api/wallet`,
        {
          address: activeSubmissionAddress,
          chainId: currentChainId,
        },
        sessionSecret,
      );

      const hash = await window.ethereum.request<string>({
        method: "eth_sendTransaction",
        params: [
          {
            ...session.pendingRequest.request,
            from: activeSubmissionAddress,
          },
        ],
      });

      await postJson(
        `${apiBaseUrl}/api/respond`,
        {
          requestId: session.pendingRequest.id,
          hash,
          from: activeSubmissionAddress,
          chainId: currentChainId,
        },
        sessionSecret,
      );

      const historyKey = signerUiHistoryStorageKey(apiBaseUrl, session.sessionId);
      const entry: CompletedDeploymentTx = {
        requestId: session.pendingRequest.id,
        completedAt: new Date().toISOString(),
        label: session.pendingRequest.label,
        description: session.pendingRequest.description,
        txHash: hash,
        from: activeSubmissionAddress,
        chainId: session.chain.chainId,
        chainName: session.chain.chainName,
        blockExplorerUrls: session.chain.blockExplorerUrls,
        request: { ...session.pendingRequest.request },
        transactionDetails: session.pendingRequest.transactionDetails ?? null,
      };
      flushSync(() => {
        setDeployHistory((prev) => {
          const next = [...prev.filter((x) => x.requestId !== entry.requestId), entry];
          persistDeployHistory(historyKey, next);
          return next;
        });
      });

      const frag = signerTxFragmentId(entry.requestId);
      scrollToHistoryFragment(frag);

      setStatusMessage(`Submitted ${session.pendingRequest.label} — jumped to #${frag} in the list below.`);
    } catch (error) {
      const message = (error as Error).message;
      setActionError(message);
      const isExpectedSignerMismatch = message.toLowerCase().includes("expected signer");
      if (isExpectedSignerMismatch) {
        return;
      }

      if (session.pendingRequest) {
        await postJson(
          `${apiBaseUrl}/api/error`,
          {
            requestId: session.pendingRequest.id,
            message,
          },
          sessionSecret,
        );
      }
    } finally {
      approveTransactionFlowRef.current = false;
      setIsSubmitting(false);
    }
  };

  const pending = session?.pendingRequest;
  const progress = session?.transactionProgress;
  const workflow = workflowDisplayState.workflow;
  const isPostSubmitProgress =
    progress !== undefined &&
    progress !== null &&
    progress.stage !== "awaiting_wallet_approval" &&
    progress.stage !== "failed" &&
    pending?.id === progress.requestId;
  const hasWorkflowStatus = workflow !== null;
  const expectedSignerAddress = session?.expectedSignerAddress ?? null;
  const resolvedWalletAddress = providerWalletAddress;
  const walletConnected = !!resolvedWalletAddress;
  const hasExpectedSignerMismatch = signerAddressMismatch(expectedSignerAddress, resolvedWalletAddress);
  const signPendingDisabled = !canEnableSignButton({
    isSubmitting,
    walletUiReady,
    pendingRequestId: pending?.id,
    connectedWalletAddress: resolvedWalletAddress,
    expectedSignerAddress,
  });

  return (
    <div className="deploy-shell">
      <header className="deploy-header">
        <div className="deploy-header__brand">
          <LineaWordmark className="deploy-header__logo" />
          <span className="deploy-header__badge">Contracts · local</span>
        </div>
      </header>

      <main className="deploy-main">
        <section className="deploy-card deploy-card--hero">
          <h1 className="deploy-card__title">Sign with your wallet</h1>
          <p className="deploy-card__subtitle">{heroSubtitle}</p>
          {hasWorkflowStatus ? <WorkflowStatusPanel workflow={workflow} /> : null}
        </section>

        {pending && !sessionEnded ? (
          <section className="deploy-card deploy-quick-action" aria-label="Quick sign current transaction">
            <div className="deploy-quick-action__row">
              <div className="deploy-quick-action__text">
                <h2 className="deploy-quick-action__title">
                  {isPostSubmitProgress ? "Current transaction in progress" : "Sign current transaction"}
                </h2>
                <p className="deploy-quick-action__label">{pending.label}</p>
                <TransactionKindBadges details={pending.transactionDetails} />
                {progress && pending.id === progress.requestId ? (
                  <PendingTransactionProgress progress={progress} />
                ) : null}
              </div>
              {isPostSubmitProgress ? (
                <div className="deploy-quick-action__waiting" aria-hidden="true">
                  <span className="deploy-spinner deploy-spinner--large" />
                </div>
              ) : (
                <button
                  type="button"
                  className="deploy-btn deploy-btn--primary deploy-quick-action__btn"
                  disabled={signPendingDisabled}
                  onClick={() => void approvePendingRequest()}
                >
                  {isSubmitting ? "Wallet or network in progress…" : "Switch chain, sign, and send"}
                </button>
              )}
            </div>
          </section>
        ) : null}

        {sessionEnded ? (
          <section className={`deploy-card ${deployRunLooksComplete ? "deploy-card--complete" : "deploy-card--muted"}`}>
            <h2>{deployRunLooksComplete ? "Run complete" : "Session disconnected"}</h2>
            <p>
              {deployRunLooksComplete
                ? "Hardhat finished and closed the local HTTP bridge. The Next.js UI usually keeps running so this tab stays loaded. Details below are kept in session storage; use #signer-tx-… links to jump to each signed transaction."
                : deployHistory.length > 0
                  ? "The bridge stopped before the last poll showed an idle session—check Hardhat for errors. Any transactions you already signed are listed below."
                  : "The Hardhat signer bridge has stopped. Polling is disabled. If you are not running another script, you can close this browser tab."}
            </p>
          </section>
        ) : null}

        <section className="deploy-card">
          <h2>Wallet</h2>
          <p>
            {!walletUiReady
              ? "Preparing wallet connection…"
              : walletConnected
                ? `Connected as ${resolvedWalletAddress}`
                : "Connect the wallet that should sign for this Hardhat script."}
          </p>
          {expectedSignerAddress ? (
            <p className={hasExpectedSignerMismatch ? "deploy-connect-error" : "deploy-expected-signer"}>
              {hasExpectedSignerMismatch
                ? `Connected wallet ${resolvedWalletAddress} does not match expected signer ${expectedSignerAddress}. Choose the correct wallet before signing.`
                : `Expected signer: ${expectedSignerAddress}`}
            </p>
          ) : null}
          <div className="deploy-actions">
            {!walletUiReady ? null : !walletConnected ? (
              <button
                type="button"
                className="deploy-btn deploy-btn--primary"
                disabled={isConnectingWallet}
                onClick={() => void connectWallet()}
              >
                {isConnectingWallet ? "Connecting..." : "Connect wallet"}
              </button>
            ) : null}
            {!walletUiReady ? null : walletConnected ? (
              <button type="button" className="deploy-btn deploy-btn--secondary" onClick={() => disconnectWallet()}>
                Disconnect
              </button>
            ) : null}
            {!walletUiReady ? null : (
              <button
                type="button"
                className="deploy-btn deploy-btn--secondary"
                disabled={isConnectingWallet}
                onClick={() => void connectWallet()}
              >
                {isConnectingWallet
                  ? "Opening wallet..."
                  : walletConnected
                    ? "Choose different wallet"
                    : "Choose wallet"}
              </button>
            )}
            {sessionEnded ? null : (
              <button
                type="button"
                className="deploy-btn deploy-btn--secondary"
                disabled={isTerminating}
                onClick={() => void terminateSession()}
              >
                {isTerminating ? "Terminating…" : "Terminate session"}
              </button>
            )}
          </div>
        </section>

        <section className="deploy-card">
          <h2>Session target</h2>
          {session?.batchRunActive ? (
            <p className="deploy-batch-note">
              <strong>Batch run:</strong>{" "}
              {session.batchTagsSummary
                ? `Hardhat is running multiple deploy scripts for --tags ${session.batchTagsSummary}. This tab stays open until the full run finishes.`
                : "Hardhat is running multiple deploy scripts in one run. This tab stays open until every script completes."}
            </p>
          ) : null}
          <p className="deploy-meta-row">
            <strong>Script / operation:</strong> {session?.scriptContext ?? "Loading…"}
            {session && (session.scriptOrdinal ?? 0) > 0 ? (
              <span className="deploy-meta-row--subtle">
                {" "}
                (step {session.scriptOrdinal}
                {session.batchRunActive ? " in this batch" : ""})
              </span>
            ) : null}
          </p>
          <p className="deploy-meta-row">
            <strong>Network:</strong> {session?.networkName ?? "Loading…"}
          </p>
          <p className="deploy-meta-row">
            <strong>Chain:</strong> {session?.chain.chainName ?? "Loading…"}
          </p>
          <p className="deploy-meta-row">
            <strong>Chain ID:</strong> {session?.chain.chainId ?? "Loading…"}
          </p>
          <p className="deploy-proxy-tip">
            <strong>Upgradeable (OpenZeppelin) deploys:</strong> Hardhat Upgrades stores prior deployments under{" "}
            <code>.openzeppelin/</code>. If an implementation or ProxyAdmin for this chain is already recorded, only the
            missing contracts are deployed — so you may see <em>one</em> wallet prompt for LineaRollup instead of three
            (implementation + ProxyAdmin + proxy). The rollup you use is still the <strong>proxy</strong> address from
            Hardhat logs; fewer prompts does not mean steps were skipped incorrectly.
          </p>
        </section>

        <section className="deploy-card">
          <h2>Pending request</h2>
          {session?.pendingRequest ? (
            <>
              <p className="deploy-pending-label">{session.pendingRequest.label}</p>
              <TransactionKindBadges details={session.pendingRequest.transactionDetails} />
              <p>{session.pendingRequest.description}</p>
              {progress && session.pendingRequest.id === progress.requestId ? (
                <>
                  <h3>Progress</h3>
                  <PendingTransactionProgress progress={progress} />
                </>
              ) : null}
              {session.pendingRequest.transactionDetails ? (
                <>
                  <h3>Transaction context (Hardhat)</h3>
                  <pre className="deploy-code">
                    {JSON.stringify(session.pendingRequest.transactionDetails, null, 2)}
                  </pre>
                </>
              ) : null}
              <h3>Raw transaction request</h3>
              <pre className="deploy-code">{JSON.stringify(session.pendingRequest.request, null, 2)}</pre>
              <div className="deploy-actions">
                <button
                  type="button"
                  className="deploy-btn deploy-btn--primary"
                  disabled={signPendingDisabled || isPostSubmitProgress}
                  onClick={() => void approvePendingRequest()}
                >
                  {isSubmitting ? "Wallet or network in progress…" : "Switch chain, sign, and send"}
                </button>
              </div>
            </>
          ) : (
            <>
              {hasWorkflowStatus ? <WorkflowStatusPanel workflow={workflow} /> : null}
              <p>
                {hasWorkflowStatus
                  ? "No wallet approval is pending right now."
                  : "No transaction is waiting for approval yet."}
              </p>
            </>
          )}
        </section>

        {deployHistory.length > 0 ? (
          <section className="deploy-card">
            <h2>Submitted transactions ({deployHistory.length})</h2>
            <p>
              {sessionEnded
                ? "Saved for this signer session in your browser (session storage). Starting a new Hardhat run opens a new session."
                : "Each approval is appended here. After Hardhat exits, this list stays visible and polling stops."}
            </p>
            <nav className="deploy-history-toc" aria-label="Jump to signed transaction">
              <span className="deploy-history-toc__title">Jump to</span>
              <ul>
                {[...deployHistory].reverse().map((item) => {
                  const frag = signerTxFragmentId(item.requestId);
                  return (
                    <li key={`jump-${item.requestId}`}>
                      <a
                        className="deploy-history-toc__link"
                        href={`#${frag}`}
                        onClick={(e) => {
                          e.preventDefault();
                          scrollToHistoryFragment(frag);
                        }}
                      >
                        {item.label}
                      </a>
                    </li>
                  );
                })}
              </ul>
            </nav>
            <ul className="deploy-history-list">
              {[...deployHistory].reverse().map((item) => {
                const explorerHref = transactionExplorerUrl(item.blockExplorerUrls, item.txHash);
                const frag = signerTxFragmentId(item.requestId);
                return (
                  <li key={item.requestId} id={frag} className="deploy-history-item">
                    <details>
                      <summary>
                        <div className="deploy-history-summary-box">
                          <span className="deploy-history-summary-text">{item.label}</span>
                          <TransactionKindBadges details={item.transactionDetails} />
                        </div>
                      </summary>
                      <div className="deploy-history-meta">
                        <div>
                          <strong>Time:</strong> {new Date(item.completedAt).toLocaleString()}
                        </div>
                        <div>
                          <strong>Tx hash:</strong>{" "}
                          {explorerHref ? (
                            <a className="deploy-hash-link" href={explorerHref} target="_blank" rel="noreferrer">
                              {item.txHash}
                            </a>
                          ) : (
                            <code>{item.txHash}</code>
                          )}
                        </div>
                        <div>
                          <strong>From:</strong> <code>{item.from}</code>
                        </div>
                        {item.request.to ? (
                          <div>
                            <strong>To (contract):</strong> <code>{item.request.to}</code>
                          </div>
                        ) : (
                          <div>
                            <strong>Contract create</strong> — use the explorer or receipt for the deployed address.
                          </div>
                        )}
                        <div>
                          <strong>Chain:</strong> {item.chainName} ({item.chainId})
                        </div>
                        {item.description ? (
                          <div>
                            <strong>Note:</strong> {item.description}
                          </div>
                        ) : null}
                        <div className="deploy-bookmark-row">
                          <span>
                            <strong>Bookmark:</strong>{" "}
                            <a
                              className="deploy-hash-link"
                              href={`#${frag}`}
                              onClick={(e) => {
                                e.preventDefault();
                                scrollToHistoryFragment(frag);
                              }}
                            >
                              #{frag}
                            </a>
                          </span>
                          <button
                            type="button"
                            className="deploy-btn deploy-btn--secondary deploy-btn--small"
                            onClick={() => {
                              const url = new URL(window.location.href);
                              url.hash = frag;
                              void navigator.clipboard.writeText(url.toString());
                            }}
                          >
                            Copy page link
                          </button>
                        </div>
                      </div>
                      {item.transactionDetails ? (
                        <div className="deploy-history-details">
                          <h3>Transaction context</h3>
                          <pre className="deploy-code">{JSON.stringify(item.transactionDetails, null, 2)}</pre>
                        </div>
                      ) : null}
                      <div className="deploy-history-details">
                        <h3>Raw transaction request</h3>
                        <pre className="deploy-code">{JSON.stringify(item.request, null, 2)}</pre>
                      </div>
                    </details>
                  </li>
                );
              })}
            </ul>
          </section>
        ) : null}

        {actionError ? (
          <section className="deploy-card deploy-card--error">
            <h2>Error</h2>
            <p>{actionError}</p>
          </section>
        ) : null}
      </main>
    </div>
  );
}

export default function Page() {
  return (
    <Suspense fallback={null}>
      <ContractsDeployUiPage />
    </Suspense>
  );
}
