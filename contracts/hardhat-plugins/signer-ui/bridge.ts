/**
 * Browser-wallet signing bridge: a `network.onRequest` handler (registered by `withSignerUi`, `./session.ts`) sends
 * `eth_sendTransaction` to the local signer app (`contracts/signer-ui/`) through a loopback HTTP session, and checks
 * on chain that what the wallet broadcast is the transaction Hardhat asked for.
 */
import { randomBytes, randomUUID } from "node:crypto";
import { createServer, type IncomingMessage, type Server, type ServerResponse } from "node:http";
import { type Address, type Hash, getAddress, hexToNumber, isAddress, isHex } from "viem";

import { appendSignerUiBroadcast, getSignerUiBroadcastLogPath } from "./broadcast-log.js";
import {
  applySignerUiCors,
  buildSignerUiUrl,
  isAllowedSignerUiOrigin,
  isValidSessionToken,
  parseErrorPayload,
  parseRespondPayload,
  parseWalletPayload,
  readJsonBody,
  writeJson,
} from "./http.js";
import {
  LOCALHOST,
  getFreePort,
  killTcpListenersOnPort,
  openBrowser,
  startNextDev,
  stopNextDev,
  waitForHttpOk,
} from "./next-dev.js";
import {
  assertExclusiveSignerMode as assertExclusiveSignerModeFor,
  hasConfiguredDeployerPrivateKey,
} from "./signer-mode.js";
import {
  buildSerializedTransactionRequest,
  calldataMatchesPromptForSignerUiValidation,
  createTransactionLookup,
  getOnChainTransactionPromptMismatch,
  getTransactionWithRetry,
  logSubmittedTransactionMismatch,
  normalizeGasFeeFieldsForWallet,
  parseSendTransactionParams,
  sleep,
} from "./transactions.js";
import { getOptionalEnvVar } from "../../common/helpers/environment.js";
import signerUiChainCatalog from "../../signer-ui/shared/chainCatalog.json" with { type: "json" };

import type { SerializedTransactionRequest, SignerUiTransactionLookup } from "./transactions.js";
import type { WalletClient } from "@nomicfoundation/hardhat-viem/types";
import type { HookContext } from "hardhat/types/hooks";
import type { ChainType, NetworkConnection } from "hardhat/types/network";
import type { JsonRpcRequest, JsonRpcResponse } from "hardhat/types/providers";
import type { ChildProcess } from "node:child_process";

export type { SerializedTransactionRequest, SignerUiTransactionLookup } from "./transactions.js";
export { calldataMatchesPromptForSignerUiValidation } from "./transactions.js";

type ChainMetadata = {
  chainId: number;
  chainName: string;
  rpcUrls: string[];
  blockExplorerUrls: string[];
  nativeCurrency: { name: string; symbol: string; decimals: number };
};

/** Optional human-readable context shown in the signer UI (constructor / initializer args, etc.). */
export type UiTransactionDetails = {
  contractName?: string;
  constructorArgs?: unknown;
  initializerArgs?: unknown;
  /** Short summary of proxy options when relevant */
  proxyOptions?: string;
  notes?: string;
  /** Set for OpenZeppelin `deployProxy` flows (`transparent` | `uups` | `beacon`). */
  openZeppelinProxyKind?: "transparent" | "uups" | "beacon";
};

type TransactionPrompt = {
  id: string;
  label: string;
  description: string;
  createdAt: string;
  request: SerializedTransactionRequest;
  transactionDetails?: UiTransactionDetails | null;
};

/** What the browser reported and Hardhat validated on chain for one transaction. */
export type TransactionOutcome = {
  requestId: string;
  hash: Hash;
  from: Address;
  chainId: number;
  /** Nonce of the transaction as found on chain (the wallet may pick its own). */
  nonce: number;
};

export type SessionWalletState = { address: Address; chainId: number; connectedAt: string };

type TransactionProgressStage =
  | "awaiting_wallet_approval"
  | "submitted_waiting_for_rpc"
  | "validating_submitted_transaction"
  | "waiting_for_hardhat_confirmation"
  | "failed";

export type WorkflowStatusStage = "waiting_for_transaction_receipt" | "waiting_for_contract_verification";

type WorkflowStatus = { stage: WorkflowStatusStage; message: string; updatedAt: string };

/** `GET /api/session` payload (wire protocol of `signer-ui/app/page.tsx`). */
type SessionState = {
  sessionId: string;
  scriptContext: string;
  networkName: string;
  chain: ChainMetadata;
  expectedSignerAddress: string | null;
  wallet: SessionWalletState | null;
  pendingRequest: TransactionPrompt | null;
  transactionProgress: {
    requestId: string;
    stage: TransactionProgressStage;
    message: string;
    updatedAt: string;
  } | null;
  workflowStatus: WorkflowStatus | null;
  startedAt: string;
  /** 1-based index of signer-UI scopes (`withSignerUi`) that used this session. */
  scriptOrdinal: number;
  /** True while a signer-UI scope is open. */
  batchRunActive: boolean;
  /** Former hardhat-deploy `--tags` list; always `null`. */
  batchTagsSummary: null;
  /** Browser-signed transactions requested in this session. */
  transactionOrdinal: number;
  /** Set just before the bridge closes so the UI can stop polling; `null` while the run is in progress. */
  sessionOutcome: "complete" | "error" | null;
  outcomeMessage: string | null;
};

export type CloseSignerUiSessionOptions = {
  /** Stop the Next.js app after closing the bridge. Default: `HARDHAT_SIGNER_UI_SHUTDOWN_NEXT_DEV === "true"`. */
  shutdownNextDev?: boolean;
};

export type SignerUiSessionContext = {
  scriptContext: string;
  networkName: string;
  chain: ChainMetadata;
  expectedSignerAddress: Address | null;
  lookup: SignerUiTransactionLookup;
};

/** Session surface used by the network hook and the scope; `SignerUiSession` is the real implementation. */
export interface SignerUiSessionHandle {
  readonly chainId: number;
  start(): Promise<void>;
  close(options?: CloseSignerUiSessionOptions): Promise<void>;
  notifyScriptContext(scriptContext: string): void;
  waitForWallet(): Promise<SessionWalletState>;
  requestTransaction(
    request: SerializedTransactionRequest,
    label: string,
    description: string,
    details: UiTransactionDetails | undefined,
  ): Promise<TransactionOutcome>;
  clearTransactionProgress(requestId?: string): void;
  setWorkflowStatus(stage: WorkflowStatusStage, message: string): void;
  clearWorkflowStatus(): void;
  setTerminalSessionOutcome(outcome: "complete" | "error", message?: string): void;
}

export type SignerUiSessionFactory = (context: SignerUiSessionContext) => SignerUiSessionHandle;

/** Rejection raised when the browser reports a wallet error (user rejection, wallet/RPC failure). */
class SignerUiWalletError extends Error {}

const chainCatalog = new Map<number, ChainMetadata>(signerUiChainCatalog.map((entry) => [entry.chainId, entry]));
const SESSION_TIMEOUT_MS = 10 * 60 * 1000;
const SERVER_CLOSE_TIMEOUT_MS = 3000;
const ACCOUNT_METHODS = new Set(["eth_accounts", "eth_requestAccounts"]);
/** Signing methods the browser bridge does not support (it only signs and sends transactions). */
const UNSUPPORTED_SIGNING_METHODS = new Set([
  "eth_sign",
  "personal_sign",
  "eth_signTransaction",
  "eth_signTypedData",
  "eth_signTypedData_v3",
  "eth_signTypedData_v4",
]);
/** EIP-1193 / JSON-RPC error codes. */
const RPC_ERROR = {
  userRejected: 4001,
  unauthorized: 4100,
  unsupportedMethod: 4200,
  invalidParams: -32602,
  internal: -32603,
} as const;
const MAX_SESSION_OUTCOME_MESSAGE_CHARS = 500;

/** Non-negative integer env var (milliseconds), else `fallback`. */
function envMs(name: string, fallback: number): number {
  const value = Number.parseInt(process.env[name] ?? "", 10);
  return Number.isFinite(value) && value >= 0 ? value : fallback;
}

/** Some RPC / wallet paths (e.g. private submission) return txs slowly: 1–300 s, default 60 s. */
function txLookupTimeoutMs(): number {
  const value = Number.parseInt(getOptionalEnvVar("HARDHAT_SIGNER_UI_TX_LOOKUP_TIMEOUT_MS") ?? "", 10);
  return Number.isFinite(value) ? Math.min(Math.max(value, 1000), 300_000) : 60_000;
}

/** Depth of nested `runSignerUiScope` calls; browser signing is on only inside a scope. */
let signerUiScopeDepth = 0;
/** Label of the outermost scope (shown in the UI as the script context). */
let signerUiScopeLabel: string | null = null;
let activeSession: SignerUiSessionHandle | undefined;
let sessionCreation: Promise<SignerUiSessionHandle> | undefined;
let sessionFactory: SignerUiSessionFactory = (context) => new SignerUiSession(context);
let privateKeyUsageWarningShown = false;
/** Labels for the next browser-signed transaction (set before the session may exist; consumed by the hook). */
let nextTransactionDetails: UiTransactionDetails | undefined;
/** Serializes `eth_sendTransaction` prompts: the UI shows one pending request at a time. */
let sendQueue: Promise<void> = Promise.resolve();
const chainIdByConnection = new WeakMap<object, number>();
/** Test-only override of `<paths.cache>` for the broadcast log. */
let broadcastLogDirOverride: string | undefined;

/** True inside a `runSignerUiScope` (i.e. inside `withSignerUi` with `--signer-ui` / `HARDHAT_SIGNER_UI=true`). */
export function isSignerUiEnabled(): boolean {
  return signerUiScopeDepth > 0;
}

function assertExclusiveSignerMode(): void {
  assertExclusiveSignerModeFor({ hardhatSignerUiEnabled: isSignerUiEnabled() });
}

/**
 * The bridge server and Next.js child are unref'd so a finished Hardhat command can exit (Hardhat 3 does not call
 * `process.exit`). While the bridge waits for the browser, nothing else keeps the event loop alive, so waits hold it.
 */
let keepAliveHolds = 0;
let keepAliveTimer: NodeJS.Timeout | undefined;

async function holdingProcessOpen<T>(wait: Promise<T>): Promise<T> {
  keepAliveHolds += 1;
  keepAliveTimer ??= setInterval(() => undefined, 60_000);
  try {
    return await wait;
  } finally {
    keepAliveHolds -= 1;
    if (keepAliveHolds === 0) {
      clearInterval(keepAliveTimer);
      keepAliveTimer = undefined;
    }
  }
}

export function warnIfUsingPrivateKeySigning(details: { networkName?: string; scriptContext?: string } = {}): void {
  assertExclusiveSignerMode();
  if (privateKeyUsageWarningShown || isSignerUiEnabled() || !hasConfiguredDeployerPrivateKey()) {
    return;
  }
  privateKeyUsageWarningShown = true;

  const suffix = [details.networkName ? `network=${details.networkName}` : "", details.scriptContext ?? ""]
    .filter((part) => part.length > 0)
    .join(" ");
  console.warn(
    [
      "",
      "=".repeat(100),
      "PRIVATE KEY SIGNING ACTIVE",
      `DEPLOYER_PRIVATE_KEY is being used for transaction signing${suffix ? ` (${suffix})` : ""}.`,
      "Keeping the deployer key in the local environment is not recommended when the browser signer UI can be used.",
      "Re-run the same command with --signer-ui (or HARDHAT_SIGNER_UI=true) to sign in the browser wallet instead.",
      "=".repeat(100),
      "",
    ].join("\n"),
  );
}

/**
 * Minimum time a workflow stage stays on screen (`HARDHAT_SIGNER_UI_WORKFLOW_STATUS_STAGE_MIN_MS`) before the next one
 * replaces it, so fast stages remain readable.
 */
function workflowStatusTransitionDelayMs(params: {
  currentStage: WorkflowStatusStage | null;
  nextStage: WorkflowStatusStage | null;
  currentStageChangedAtMs: number;
  nowMs: number;
  minimumStageMs: number;
}): number {
  const { currentStage, nextStage, currentStageChangedAtMs, nowMs, minimumStageMs } = params;
  if (currentStage === null || currentStage === nextStage || minimumStageMs <= 0) {
    return 0;
  }
  return Math.max(minimumStageMs - Math.max(nowMs - currentStageChangedAtMs, 0), 0);
}

type PendingTransaction = {
  prompt: TransactionPrompt;
  resolve: (value: TransactionOutcome) => void;
  reject: (reason: Error) => void;
};

type WalletWaiter = { resolve: (wallet: SessionWalletState) => void; reject: (error: Error) => void };

class SignerUiSession implements SignerUiSessionHandle {
  private readonly sessionId = randomUUID();
  /** Opaque secret; required as the session token header on bridge HTTP calls. */
  private readonly sessionSecret = randomBytes(32).toString("base64url");
  private readonly startedAt = new Date().toISOString();
  private activeScriptContext: string;
  private scriptOrdinal = 0;
  private transactionOrdinal = 0;
  private server: Server | undefined;
  private apiPort: number | undefined;
  private uiPort: number | undefined;
  private uiProcess: ChildProcess | undefined;
  private started = false;
  private closed = false;
  private sessionAuthenticated = false;
  private pendingRequest: PendingTransaction | undefined;
  private transactionProgress: SessionState["transactionProgress"] = null;
  private workflowStatus: WorkflowStatus | null = null;
  private workflowStatusChangedAtMs = 0;
  private workflowStatusTimer: NodeJS.Timeout | undefined;
  private walletState: SessionWalletState | null = null;
  private walletWaiters: WalletWaiter[] = [];
  private sessionOutcome: SessionState["sessionOutcome"] = null;
  private sessionOutcomeMessage: string | null = null;

  public constructor(private readonly context: SignerUiSessionContext) {
    this.activeScriptContext = context.scriptContext;
  }

  public get chainId(): number {
    return this.context.chain.chainId;
  }

  /** Reported to the browser (`/api/session`) before the bridge closes. */
  public setTerminalSessionOutcome(outcome: "complete" | "error", message?: string): void {
    this.sessionOutcome = outcome;
    this.sessionOutcomeMessage =
      message && message.length > MAX_SESSION_OUTCOME_MESSAGE_CHARS
        ? `${message.slice(0, MAX_SESSION_OUTCOME_MESSAGE_CHARS - 1)}…`
        : (message ?? null) || null;
  }

  public notifyScriptContext(scriptContext: string): void {
    this.scriptOrdinal += 1;
    this.activeScriptContext = scriptContext;
    const scopePart = signerUiScopeLabel && signerUiScopeLabel !== scriptContext ? `${signerUiScopeLabel} · ` : "";
    console.log(`HARDHAT_SIGNER_UI: step ${this.scriptOrdinal} (${scopePart}${scriptContext})`);
  }

  public async start(): Promise<void> {
    if (this.started) {
      return;
    }
    const apiPort = await getFreePort();
    const uiPort = await getFreePort();
    this.apiPort = apiPort;
    this.uiPort = uiPort;
    const server = createServer((request, response) => {
      void this.handleRequest(request, response, uiPort).catch((error: unknown) => {
        const message = error instanceof Error ? error.message : String(error);
        if (response.headersSent) {
          response.destroy();
          return;
        }
        applySignerUiCors(request, response, uiPort);
        writeJson(response, message.includes("exceeds") && message.includes("bytes") ? 413 : 500, { error: message });
      });
    });
    this.server = server;
    await new Promise<void>((resolveServer, rejectServer) => {
      server.on("error", rejectServer);
      server.listen(apiPort, LOCALHOST, () => resolveServer());
    });
    // Waits on the browser hold the process open (`holdingProcessOpen`); an idle bridge must not.
    server.unref();

    this.uiProcess = await startNextDev(uiPort, (code) => {
      if (!this.closed) {
        this.failPendingState(new Error(`Signer UI process exited unexpectedly with code ${code ?? "unknown"}`));
      }
    });

    const apiBaseUrl = `http://${LOCALHOST}:${apiPort}`;
    const uiUrl = buildSignerUiUrl(uiPort, apiBaseUrl);
    console.log(`HARDHAT_SIGNER_UI: starting Next dev on http://${LOCALHOST}:${uiPort} (bridge API on ${apiBaseUrl})…`);
    await waitForHttpOk(uiUrl);
    console.log(
      `HARDHAT_SIGNER_UI is enabled for ${this.activeScriptContext}. Waiting for browser wallet approval at ${uiUrl}`,
    );
    openBrowser(uiUrl);
    this.started = true;
  }

  public async close(options?: CloseSignerUiSessionOptions): Promise<void> {
    if (this.closed) {
      return;
    }
    this.closed = true;
    this.failPendingState(new Error(`HARDHAT_SIGNER_UI session closed during ${this.activeScriptContext}`));
    clearTimeout(this.workflowStatusTimer);

    const server = this.server;
    this.server = undefined;
    if (server) {
      server.closeAllConnections();
      await Promise.race([
        new Promise<void>((resolveServer) => server.close(() => resolveServer())),
        sleep(SERVER_CLOSE_TIMEOUT_MS),
      ]);
      server.closeAllConnections();
      if (this.apiPort !== undefined) {
        killTcpListenersOnPort(this.apiPort);
      }
    }

    if (options?.shutdownNextDev ?? process.env.HARDHAT_SIGNER_UI_SHUTDOWN_NEXT_DEV === "true") {
      await sleep(envMs("HARDHAT_SIGNER_UI_SHUTDOWN_GRACE_MS", 2000));
      await stopNextDev(this.uiProcess, this.uiPort);
      this.uiProcess = undefined;
      console.log("HARDHAT_SIGNER_UI: Next.js signer UI dev server stopped.");
    } else if (this.uiPort !== undefined) {
      console.log(
        `HARDHAT_SIGNER_UI: HTTP bridge closed. Signer UI still at http://${LOCALHOST}:${this.uiPort} — tab stays open. Stop Next with HARDHAT_SIGNER_UI_SHUTDOWN_NEXT_DEV=true (non-deploy), or after deploy set HARDHAT_SIGNER_UI_LEAVE_NEXT_DEV_AFTER_DEPLOY=true to keep Next running.`,
      );
    } else {
      console.log(
        "HARDHAT_SIGNER_UI: HTTP bridge closed (Next.js UI port unknown). Set HARDHAT_SIGNER_UI_SHUTDOWN_NEXT_DEV=true to force child teardown when used.",
      );
    }
  }

  private getState(): SessionState {
    return {
      sessionId: this.sessionId,
      scriptContext: this.activeScriptContext,
      networkName: this.context.networkName,
      chain: this.context.chain,
      expectedSignerAddress: this.context.expectedSignerAddress,
      wallet: this.walletState,
      pendingRequest: this.pendingRequest?.prompt ?? null,
      transactionProgress: this.transactionProgress,
      workflowStatus: this.workflowStatus,
      startedAt: this.startedAt,
      scriptOrdinal: this.scriptOrdinal,
      batchRunActive: isSignerUiEnabled(),
      batchTagsSummary: null,
      transactionOrdinal: this.transactionOrdinal,
      sessionOutcome: this.sessionOutcome,
      outcomeMessage: this.sessionOutcomeMessage,
    };
  }

  private setTransactionProgress(requestId: string, stage: TransactionProgressStage, message: string): void {
    this.transactionProgress = { requestId, stage, message, updatedAt: new Date().toISOString() };
  }

  public clearTransactionProgress(requestId?: string): void {
    if (this.transactionProgress && (!requestId || this.transactionProgress.requestId === requestId)) {
      this.transactionProgress = null;
    }
  }

  public setWorkflowStatus(stage: WorkflowStatusStage, message: string): void {
    this.transitionWorkflowStatus({ stage, message });
  }

  public clearWorkflowStatus(): void {
    this.transitionWorkflowStatus(null);
  }

  /** Applies `next` now, or once the current stage has been visible for the minimum time (latest update wins). */
  private transitionWorkflowStatus(next: { stage: WorkflowStatusStage; message: string } | null): void {
    clearTimeout(this.workflowStatusTimer);
    const apply = () => {
      this.workflowStatus = next && { ...next, updatedAt: new Date().toISOString() };
      this.workflowStatusChangedAtMs = next ? Date.now() : 0;
    };
    const delayMs = workflowStatusTransitionDelayMs({
      currentStage: this.workflowStatus?.stage ?? null,
      nextStage: next?.stage ?? null,
      currentStageChangedAtMs: this.workflowStatusChangedAtMs,
      nowMs: Date.now(),
      minimumStageMs: envMs("HARDHAT_SIGNER_UI_WORKFLOW_STATUS_STAGE_MIN_MS", 2000),
    });
    if (delayMs <= 0) {
      apply();
      return;
    }
    this.workflowStatusTimer = setTimeout(apply, delayMs);
    // A deferred status update must not keep a finished command alive.
    this.workflowStatusTimer.unref();
  }

  public async waitForWallet(): Promise<SessionWalletState> {
    await this.start();
    if (this.walletState) {
      return this.walletState;
    }
    return holdingProcessOpen(
      new Promise<SessionWalletState>((resolve, reject) => {
        const waiter: WalletWaiter = {
          resolve: (wallet) => {
            clearTimeout(timeout);
            resolve(wallet);
          },
          reject: (error) => {
            clearTimeout(timeout);
            reject(error);
          },
        };
        const timeout = setTimeout(() => {
          this.walletWaiters = this.walletWaiters.filter((other) => other !== waiter);
          reject(new Error(`Timed out while waiting for a wallet connection for ${this.activeScriptContext}`));
        }, SESSION_TIMEOUT_MS);
        this.walletWaiters.push(waiter);
      }),
    );
  }

  public async requestTransaction(
    request: SerializedTransactionRequest,
    label: string,
    description: string,
    details: UiTransactionDetails | undefined,
  ): Promise<TransactionOutcome> {
    await this.start();
    if (this.pendingRequest) {
      throw new Error(
        `HARDHAT_SIGNER_UI only supports one in-flight transaction at a time (${this.activeScriptContext})`,
      );
    }

    this.transactionOrdinal += 1;
    const multiStep = this.transactionOrdinal > 1;
    const stepLabel = multiStep ? `${label} (step ${this.transactionOrdinal})` : label;
    console.log(
      `HARDHAT_SIGNER_UI: waiting for wallet signature — ${stepLabel} (${this.activeScriptContext}). ` +
        "Deployments may require multiple signatures.",
    );
    const hasDetails = Object.values(details ?? {}).some((v) => v !== undefined && v !== null && v !== "");
    const prompt: TransactionPrompt = {
      id: randomUUID(),
      label: stepLabel,
      description: multiStep
        ? `${description} Proxy deploys often need several sequential wallet approvals.`
        : description,
      createdAt: new Date().toISOString(),
      request,
      transactionDetails: hasDetails && details ? details : null,
    };

    return holdingProcessOpen(
      new Promise<TransactionOutcome>((resolve, reject) => {
        this.pendingRequest = { prompt, resolve, reject };
        this.setTransactionProgress(
          prompt.id,
          "awaiting_wallet_approval",
          `Waiting for wallet approval for ${stepLabel}.`,
        );
      }),
    );
  }

  private async handleRequest(request: IncomingMessage, response: ServerResponse, uiPort: number): Promise<void> {
    applySignerUiCors(request, response, uiPort);
    if (request.method === "OPTIONS") {
      response.statusCode = 204;
      response.setHeader("Connection", "close");
      response.end();
      return;
    }

    const route = `${request.method} ${new URL(request.url ?? "/", `http://${LOCALHOST}`).pathname}`;
    if (route === "POST /api/bootstrap") {
      if (!isAllowedSignerUiOrigin(request, uiPort)) {
        writeJson(response, 403, { error: "Signer UI bootstrap must originate from the local signer UI." });
      } else if (this.sessionAuthenticated) {
        writeJson(response, 409, {
          error: "Signer UI session has already been bootstrapped. Reopen the original tab for this Hardhat run.",
        });
      } else {
        writeJson(response, 200, { sessionToken: this.sessionSecret });
      }
      return;
    }

    if (!isValidSessionToken(this.sessionSecret, request)) {
      writeJson(response, 401, { error: "Missing or invalid signer UI session token." });
      return;
    }
    this.sessionAuthenticated = true;

    switch (route) {
      case "GET /health":
        return writeJson(response, 200, { ok: true, sessionId: this.sessionId });
      case "GET /api/session":
        return writeJson(response, 200, this.getState());
      case "POST /api/wallet":
        return this.handleWallet(await readJsonBody(request), response);
      case "POST /api/respond":
        return this.handleRespond(await readJsonBody(request), response);
      case "POST /api/error":
        return this.handleError(await readJsonBody(request), response);
      case "POST /api/terminate":
        return this.handleTerminate(response);
      default:
        return writeJson(response, 404, { error: "Not found" });
    }
  }

  private handleWallet(body: unknown, response: ServerResponse): void {
    const parsed = parseWalletPayload(body);
    if (!parsed.ok) {
      writeJson(response, 400, { error: parsed.error });
      return;
    }
    // The wallet may connect on another chain first: the chain is enforced before sending (UI) and in /api/respond.
    const wallet = {
      address: getAddress(parsed.value.address),
      chainId: parsed.value.chainId,
      connectedAt: new Date().toISOString(),
    };
    this.walletState = wallet;
    const waiters = this.walletWaiters.splice(0);
    writeJson(response, 200, { ok: true }, () => {
      for (const waiter of waiters) {
        queueMicrotask(() => waiter.resolve(wallet));
      }
    });
  }

  /** Ends the pending request with `error` once the HTTP response is sent. */
  private rejectPending(response: ServerResponse, status: number, pending: PendingTransaction, error: Error): void {
    this.pendingRequest = undefined;
    writeJson(response, status, status === 200 ? { ok: true } : { error: error.message }, () => {
      queueMicrotask(() => pending.reject(error));
    });
  }

  private async handleRespond(body: unknown, response: ServerResponse): Promise<void> {
    const parsed = parseRespondPayload(body);
    if (!parsed.ok) {
      writeJson(response, 400, { error: parsed.error });
      return;
    }
    const { requestId, hash, chainId } = parsed.value;
    if (chainId !== this.context.chain.chainId) {
      writeJson(response, 400, { error: "chainId does not match this Hardhat signer session." });
      return;
    }
    if (!this.walletState) {
      writeJson(response, 400, { error: "Register the wallet with the session before submitting a transaction." });
      return;
    }
    const from = getAddress(parsed.value.from);
    if (this.context.expectedSignerAddress && from !== this.context.expectedSignerAddress) {
      const message = `Expected signer is ${this.context.expectedSignerAddress}. Disconnect and connect the correct wallet, then try again.`;
      this.setTransactionProgress(requestId, "awaiting_wallet_approval", message);
      writeJson(response, 409, { error: message });
      return;
    }
    if (from !== this.walletState.address) {
      writeJson(response, 400, { error: "from does not match the wallet registered for this session." });
      return;
    }
    const pending = this.pendingRequest;
    if (!pending || pending.prompt.id !== requestId) {
      writeJson(response, 400, { error: "No matching pending transaction request was found." });
      return;
    }

    this.setTransactionProgress(
      requestId,
      "submitted_waiting_for_rpc",
      "Transaction submitted in the wallet. Waiting for the RPC to return the transaction.",
    );
    const timeoutMs = txLookupTimeoutMs();
    const onChain = await getTransactionWithRetry(this.context.lookup, hash, timeoutMs);
    if (this.pendingRequest !== pending) {
      writeJson(response, 409, { error: "Pending transaction request was cancelled before validation completed." });
      return;
    }
    if (!onChain) {
      const message =
        `Transaction ${hash} was returned by the browser wallet, but Hardhat could not find it on ` +
        `${this.context.networkName} after ${timeoutMs}ms. ` +
        "Check the wallet activity against this exact RPC/chain, or increase HARDHAT_SIGNER_UI_TX_LOOKUP_TIMEOUT_MS.";
      this.setTransactionProgress(requestId, "failed", message);
      console.warn(`HARDHAT_SIGNER_UI: ${message}`);
      this.rejectPending(response, 400, pending, new Error(message));
      return;
    }

    this.setTransactionProgress(
      requestId,
      "validating_submitted_transaction",
      "Validating that the submitted transaction matches the pending Hardhat request.",
    );
    const mismatch = getOnChainTransactionPromptMismatch(onChain, pending.prompt.request, this.walletState.address);
    if (mismatch) {
      logSubmittedTransactionMismatch(hash, mismatch, onChain, pending.prompt.request);
      const message = `Submitted transaction did not match the pending Hardhat request. ${mismatch}`;
      this.setTransactionProgress(requestId, "failed", message);
      this.rejectPending(response, 400, pending, new Error(message));
      return;
    }

    this.pendingRequest = undefined;
    this.setTransactionProgress(
      requestId,
      "waiting_for_hardhat_confirmation",
      `Validation passed. Waiting for Hardhat/provider confirmation for transaction ${hash}.`,
    );
    const outcome: TransactionOutcome = { requestId, hash, from, chainId, nonce: onChain.nonce };
    writeJson(response, 200, { ok: true }, () => queueMicrotask(() => pending.resolve(outcome)));
  }

  private handleError(body: unknown, response: ServerResponse): void {
    const parsed = parseErrorPayload(body);
    if (!parsed.ok) {
      writeJson(response, 400, { error: parsed.error });
      return;
    }
    const { requestId, message } = parsed.value;
    const pending = this.pendingRequest;
    if (pending?.prompt.id !== requestId) {
      writeJson(response, 200, { ok: true });
      return;
    }
    this.setTransactionProgress(requestId, "failed", message);
    this.rejectPending(response, 200, pending, new SignerUiWalletError(message));
  }

  private handleTerminate(response: ServerResponse): void {
    const message = "Session terminated by user from signer UI.";
    this.setTerminalSessionOutcome("error", message);
    writeJson(response, 200, { ok: true }, () => {
      this.failPendingState(new Error(message));
      void closeActiveSignerUiSession().catch(() => undefined);
    });
  }

  private failPendingState(error: Error): void {
    if (this.pendingRequest) {
      this.setTransactionProgress(this.pendingRequest.prompt.id, "failed", error.message);
      this.pendingRequest.reject(error);
      this.pendingRequest = undefined;
    }
    for (const waiter of this.walletWaiters.splice(0)) {
      waiter.reject(error);
    }
  }
}

async function closeActiveSignerUiSession(options?: CloseSignerUiSessionOptions): Promise<void> {
  const session = activeSession;
  activeSession = undefined;
  await session?.close(options);
}

async function getConnectionChainId<ChainTypeT extends ChainType | string>(
  connection: NetworkConnection<ChainTypeT>,
): Promise<number> {
  const cached = chainIdByConnection.get(connection);
  if (cached !== undefined) {
    return cached;
  }
  const chainIdHex = await connection.provider.request({ method: "eth_chainId" });
  if (typeof chainIdHex !== "string" || !isHex(chainIdHex)) {
    throw new Error(`HARDHAT_SIGNER_UI: eth_chainId returned an invalid value on ${connection.networkName}.`);
  }
  const chainId = hexToNumber(chainIdHex);
  chainIdByConnection.set(connection, chainId);
  return chainId;
}

async function getChainMetadata<ChainTypeT extends ChainType | string>(
  connection: NetworkConnection<ChainTypeT>,
): Promise<ChainMetadata> {
  const chainId = await getConnectionChainId(connection);
  const { networkConfig } = connection;
  // In-process network: only reachable from a browser wallet through `hardhat node`.
  const rpcUrl =
    networkConfig.type === "http" ? await networkConfig.url.get().catch(() => "") : "http://127.0.0.1:8545";
  const known = chainCatalog.get(chainId);
  return {
    chainId,
    chainName: known?.chainName ?? connection.networkName,
    rpcUrls: rpcUrl ? [rpcUrl] : [],
    blockExplorerUrls: known?.blockExplorerUrls ?? [],
    nativeCurrency: { name: "Ether", symbol: "ETH", decimals: 18 },
  };
}

function getExpectedSignerAddress(): Address | undefined {
  const expected = getOptionalEnvVar("EXPECTED_SIGNER_ADDRESS");
  if (!expected) {
    return undefined;
  }
  if (!isAddress(expected, { strict: false })) {
    throw new Error(
      `Invalid EXPECTED_SIGNER_ADDRESS "${expected}". Provide a valid checksummed or lowercase hex address.`,
    );
  }
  return getAddress(expected);
}

/** Label of the session the network hook opens: the scope label, else the Hardhat command line. */
function defaultScriptContext(): string {
  if (signerUiScopeLabel !== null) {
    return signerUiScopeLabel;
  }
  const command = process.argv.slice(2).join(" ");
  return command.length > 0 ? `hardhat ${command.slice(0, 200)}` : "hardhat";
}

/** Returns the active session, or creates and starts one for `connection` (once, even when requests race). */
async function ensureSession<ChainTypeT extends ChainType | string>(
  connection: NetworkConnection<ChainTypeT>,
): Promise<SignerUiSessionHandle> {
  if (activeSession) {
    return activeSession;
  }
  const scriptContext = defaultScriptContext();
  sessionCreation ??= (async () => {
    const session = sessionFactory({
      scriptContext,
      networkName: connection.networkName,
      chain: await getChainMetadata(connection),
      expectedSignerAddress: getExpectedSignerAddress() ?? null,
      lookup: createTransactionLookup(connection),
    });
    try {
      await session.start();
    } catch (error) {
      // Also stop `next dev`: a start that timed out waiting for it leaves it running otherwise.
      await session.close({ shutdownNextDev: true }).catch(() => undefined);
      throw error;
    }
    activeSession = session;
    session.notifyScriptContext(scriptContext);
    return session;
  })().finally(() => {
    sessionCreation = undefined;
  });
  return sessionCreation;
}

const rpcResult = (request: JsonRpcRequest, result: unknown): JsonRpcResponse => ({
  jsonrpc: "2.0",
  id: request.id,
  result,
});

const rpcError = (request: JsonRpcRequest, code: number, message: string): JsonRpcResponse => ({
  jsonrpc: "2.0",
  id: request.id,
  error: { code, message: `HARDHAT_SIGNER_UI: ${message}` },
});

async function sendThroughSignerUi<ChainTypeT extends ChainType | string>(
  context: HookContext,
  connection: NetworkConnection<ChainTypeT>,
  request: JsonRpcRequest,
): Promise<JsonRpcResponse> {
  const parsed = parseSendTransactionParams(request.params);
  if (!parsed.ok) {
    return rpcError(request, RPC_ERROR.invalidParams, parsed.error);
  }
  const session = await ensureSession(connection);
  const wallet = await session.waitForWallet();
  if (parsed.value.from !== undefined && parsed.value.from !== wallet.address) {
    return rpcError(
      request,
      RPC_ERROR.unauthorized,
      `transaction is from ${parsed.value.from}, but the connected browser wallet is ${wallet.address}.`,
    );
  }
  const chainId = await getConnectionChainId(connection);
  if (chainId !== session.chainId) {
    return rpcError(
      request,
      RPC_ERROR.invalidParams,
      `the signer UI session targets chain ${session.chainId}, but ${connection.networkName} is chain ${chainId}.`,
    );
  }

  const txRequest = parsed.value.request;
  const serialized: SerializedTransactionRequest =
    txRequest.chainId === undefined ? { ...txRequest, chainId: `0x${chainId.toString(16)}` } : txRequest;
  const label = serialized.to ? "Contract transaction" : "Contract creation";
  const description = serialized.to ? `Send transaction to ${serialized.to}` : "Send contract creation transaction";
  const details = nextTransactionDetails;
  nextTransactionDetails = undefined;

  let outcome: TransactionOutcome;
  try {
    const send = sendQueue.then(() => session.requestTransaction(serialized, label, description, details));
    sendQueue = send.then(
      () => undefined,
      () => undefined,
    );
    outcome = await send;
  } catch (error) {
    const code = error instanceof SignerUiWalletError ? RPC_ERROR.userRejected : RPC_ERROR.internal;
    return rpcError(request, code, error instanceof Error ? error.message : String(error));
  }

  appendSignerUiBroadcast(
    getSignerUiBroadcastLogPath(broadcastLogDirOverride ?? context.config.paths.cache, outcome.chainId),
    { sender: outcome.from, nonce: outcome.nonce, hash: outcome.hash, at: new Date().toISOString() },
  );
  session.clearTransactionProgress(outcome.requestId);
  return rpcResult(request, outcome.hash);
}

/**
 * `network.onRequest` handler that `withSignerUi` (`./session.ts`) registers for its scope. Inside a scope it answers
 * `eth_accounts` / `eth_requestAccounts` with the browser wallet and routes `eth_sendTransaction` to the browser,
 * returning the hash; everything else goes to `next`. The first intercepted request opens a session when none is active.
 */
export async function handleSignerUiNetworkRequest<ChainTypeT extends ChainType | string>(
  context: HookContext,
  connection: NetworkConnection<ChainTypeT>,
  request: JsonRpcRequest,
  next: (
    nextContext: HookContext,
    nextConnection: NetworkConnection<ChainTypeT>,
    nextRequest: JsonRpcRequest,
  ) => Promise<JsonRpcResponse>,
): Promise<JsonRpcResponse> {
  if (!isSignerUiEnabled()) {
    return next(context, connection, request);
  }
  if (ACCOUNT_METHODS.has(request.method)) {
    assertExclusiveSignerMode();
    const wallet = await (await ensureSession(connection)).waitForWallet();
    return rpcResult(request, [wallet.address.toLowerCase()]);
  }
  if (request.method === "eth_sendTransaction") {
    assertExclusiveSignerMode();
    return sendThroughSignerUi(context, connection, request);
  }
  if (UNSUPPORTED_SIGNING_METHODS.has(request.method)) {
    return rpcError(
      request,
      RPC_ERROR.unsupportedMethod,
      `${request.method} is not supported; the browser bridge only signs and sends transactions.`,
    );
  }
  return next(context, connection, request);
}

/**
 * Wallet client to sign with. UI mode: the connected browser wallet (opens a session if needed; signing goes through
 * the network hook). Key mode: the first configured account, checked against `EXPECTED_SIGNER_ADDRESS`, with the
 * private-key warning.
 */
export async function getUiWalletClient<ChainTypeT extends ChainType | string>(
  connection: NetworkConnection<ChainTypeT>,
): Promise<WalletClient> {
  assertExclusiveSignerMode();
  if (isSignerUiEnabled()) {
    const accounts = await connection.provider.request({ method: "eth_accounts" });
    const [account] = Array.isArray(accounts) ? accounts : [];
    if (typeof account !== "string" || !isAddress(account, { strict: false })) {
      throw new Error("HARDHAT_SIGNER_UI: the browser wallet did not expose an account.");
    }
    return connection.viem.getWalletClient(getAddress(account));
  }

  const [wallet] = await connection.viem.getWalletClients();
  if (wallet === undefined) {
    throw new Error(
      "No JSON-RPC account is configured for this network. Set DEPLOYER_PRIVATE_KEY (or your network accounts in hardhat.config), or pass --signer-ui (HARDHAT_SIGNER_UI=true) to sign in the browser.",
    );
  }
  warnIfUsingPrivateKeySigning({ networkName: connection.networkName });
  const expected = getExpectedSignerAddress();
  const actual = getAddress(wallet.account.address);
  if (expected && actual !== expected) {
    throw new Error(
      `Signer mismatch. EXPECTED_SIGNER_ADDRESS=${expected}, resolved signer address=${actual}. Update EXPECTED_SIGNER_ADDRESS or connect/configure the intended signer.`,
    );
  }
  return wallet;
}

/** Labels the next browser-signed transaction only (kept until the next `eth_sendTransaction`). No-op outside a scope. */
export function setUiTransactionContext(details: UiTransactionDetails): void {
  if (isSignerUiEnabled()) {
    nextTransactionDetails = details;
  }
}

export function setUiWorkflowStatus(stage: WorkflowStatusStage, message: string): void {
  if (isSignerUiEnabled()) {
    activeSession?.setWorkflowStatus(stage, message);
  }
}

export function clearUiWorkflowStatus(): void {
  if (isSignerUiEnabled()) {
    activeSession?.clearWorkflowStatus();
  }
}

export type SignerUiScopeOptions = CloseSignerUiSessionOptions;

/**
 * Enables browser signing for the duration of `fn`. The session opens on the first `eth_accounts` /
 * `eth_sendTransaction` routed through the network hook and is closed when the outermost scope ends, after the UI got
 * the terminal outcome. Nested scopes share the outer session. Callers use `withSignerUi` (`./session.ts`), which also
 * registers the network hook for the scope.
 */
export async function runSignerUiScope<T>(
  label: string,
  fn: () => Promise<T>,
  options: SignerUiScopeOptions = {},
): Promise<T> {
  assertExclusiveSignerModeFor({ hardhatSignerUiEnabled: true });
  if (signerUiScopeDepth > 0) {
    activeSession?.notifyScriptContext(label);
    signerUiScopeDepth += 1;
    try {
      return await fn();
    } finally {
      signerUiScopeDepth -= 1;
    }
  }

  signerUiScopeDepth = 1;
  signerUiScopeLabel = label;
  console.log(`HARDHAT_SIGNER_UI: signing in the browser wallet for ${label}.`);
  let failureMessage: string | undefined;
  try {
    return await fn();
  } catch (error: unknown) {
    failureMessage = error instanceof Error ? error.message : String(error);
    throw error;
  } finally {
    if (activeSession) {
      activeSession.setTerminalSessionOutcome(failureMessage === undefined ? "complete" : "error", failureMessage);
      await sleep(envMs("HARDHAT_SIGNER_UI_SHUTDOWN_DRAIN_MS", 1500));
    }
    try {
      await closeActiveSignerUiSession(options);
    } finally {
      signerUiScopeDepth = 0;
      signerUiScopeLabel = null;
    }
  }
}

export const __testOnlySignerUiBridge = {
  buildSignerUiUrl,
  buildSerializedTransactionRequest,
  calldataMatchesPromptForSignerUiValidation,
  normalizeGasFeeFieldsForWallet,
  parseErrorPayload,
  parseRespondPayload,
  parseSendTransactionParams,
  parseWalletPayload,
  workflowStatusTransitionDelayMs,
  /** Replace the session implementation (e.g. a stub that signs locally); `undefined` restores the real one. */
  setSessionFactory(factory: SignerUiSessionFactory | undefined): void {
    sessionFactory = factory ?? ((context) => new SignerUiSession(context));
  },
  /** Write the broadcast log under `dir` instead of `<paths.cache>`; `undefined` restores the default. */
  setBroadcastLogDir(dir: string | undefined): void {
    broadcastLogDirOverride = dir;
  },
  getActiveSession(): SignerUiSessionHandle | undefined {
    return activeSession;
  },
  closeActiveSession: closeActiveSignerUiSession,
  SignerUiWalletError,
};
