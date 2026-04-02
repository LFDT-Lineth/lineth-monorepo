import {
  AbstractSigner,
  AddressLike,
  Provider,
  TypedDataDomain,
  TypedDataField,
  TransactionRequest,
  TransactionResponse,
  getAddress,
  getBytes,
  hexlify,
  isAddress,
  keccak256,
  toQuantity,
} from "ethers";
import { ChildProcess, execFileSync, spawn } from "node:child_process";
import { randomBytes, randomUUID, timingSafeEqual } from "node:crypto";
import { existsSync, readFileSync, unlinkSync } from "node:fs";
import { createServer, IncomingMessage, Server, ServerResponse } from "node:http";
import { createRequire } from "node:module";
import { join, resolve } from "node:path";

import {
  assertExclusiveSignerMode,
  hasConfiguredDeployerPrivateKey,
  isSignerUiEnabled as isSignerUiEnabledFromMode,
} from "./signer-mode";
import { getBlockchainNode, getL2BlockchainNode } from "../../common";
import { getOptionalEnvVar } from "../../common/helpers/environment";

import type { HardhatRuntimeEnvironment } from "hardhat/types";

export const isSignerUiEnabled = isSignerUiEnabledFromMode;

/**
 * Mirrors hardhat-deploy's `DeployFunction` without importing `hardhat-deploy/types`, which is not a
 * resolvable runtime module under Node's native ESM resolver (this file is loaded via `require` from `hardhat.config`).
 */
export type DeployFunction = ((hre: HardhatRuntimeEnvironment) => Promise<void | boolean>) & {
  skip?: (env: HardhatRuntimeEnvironment) => Promise<boolean>;
  tags?: string[];
  dependencies?: string[];
  runAtTheEnd?: boolean;
  id?: string;
};

type HexString = `0x${string}`;

type NativeCurrency = {
  name: string;
  symbol: string;
  decimals: number;
};

type ChainMetadata = {
  chainId: number;
  chainName: string;
  rpcUrls: string[];
  blockExplorerUrls: string[];
  nativeCurrency: NativeCurrency;
};

type SignerUiChainCatalogEntry = ChainMetadata;

type SerializedTransactionRequest = {
  to?: string;
  data?: HexString;
  value?: HexString;
  gas?: HexString;
  gasPrice?: HexString;
  maxFeePerGas?: HexString;
  maxPriorityFeePerGas?: HexString;
  nonce?: HexString;
  type?: HexString;
  chainId?: HexString;
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

type TransactionOutcome = {
  requestId: string;
  hash: string;
  from: string;
  chainId: number;
};

type SessionWalletState = {
  address: string;
  chainId: number;
  connectedAt: string;
};

type TransactionProgressStage =
  | "awaiting_wallet_approval"
  | "submitted_waiting_for_rpc"
  | "validating_submitted_transaction"
  | "waiting_for_hardhat_confirmation"
  | "failed";

type TransactionProgress = {
  requestId: string;
  stage: TransactionProgressStage;
  message: string;
  updatedAt: string;
};

type WorkflowStatusStage = "waiting_for_transaction_receipt" | "waiting_for_contract_verification";

type WorkflowStatus = {
  stage: WorkflowStatusStage;
  message: string;
  updatedAt: string;
};

type SessionState = {
  sessionId: string;
  /** Currently executing script or task identifier (updates across a multi-script batch). */
  scriptContext: string;
  networkName: string;
  chain: ChainMetadata;
  /** Optional signer address guard from EXPECTED_SIGNER_ADDRESS. */
  expectedSignerAddress: string | null;
  wallet: SessionWalletState | null;
  pendingRequest: TransactionPrompt | null;
  transactionProgress: TransactionProgress | null;
  workflowStatus: WorkflowStatus | null;
  startedAt: string;
  /** 1-based index of scripts that entered `withSignerUiSession` in this session. */
  scriptOrdinal: number;
  /** True while `deploy:runDeploy` is running (multi-tag / multi-file batch). */
  batchRunActive: boolean;
  /** Comma-separated tags from `--tags`, if any. */
  batchTagsSummary: string | null;
  /**
   * Set just before the deploy batch closes the bridge so the UI can stop polling gracefully.
   * `null` while the run is in progress.
   */
  sessionOutcome: "complete" | "error" | null;
  /** Short Hardhat error message when `sessionOutcome === "error"`. */
  outcomeMessage: string | null;
};

/** Options for `SignerUiSession.close` / internal `closeActiveSignerUiSession`. */
type CloseSignerUiSessionOptions = {
  /**
   * When `true`, stop the Next.js dev child after closing the bridge.
   * When `false`, never stop Next from this close.
   * When omitted, use `HARDHAT_SIGNER_UI_SHUTDOWN_NEXT_DEV === "true"`.
   */
  shutdownNextDev?: boolean;
};

type PendingTransaction = {
  prompt: TransactionPrompt;
  resolve: (value: TransactionOutcome) => void;
  reject: (reason?: Error) => void;
};

type SignerUiContext = {
  scriptContext: string;
  networkName: string;
  chain: ChainMetadata;
  expectedSignerAddress: string | null;
  provider: Provider;
  batchTagsSummary: string | null;
};

const SIGNER_UI_DIR = resolve(__dirname, "../../signer-ui");
const requireFromSignerUiBridge = createRequire(__filename);
const SIGNER_UI_CHAIN_CATALOG = requireFromSignerUiBridge(
  "../../signer-ui/shared/chainCatalog.json",
) as readonly SignerUiChainCatalogEntry[];
const signerUiChainCatalogByChainId = new Map<number, SignerUiChainCatalogEntry>(
  SIGNER_UI_CHAIN_CATALOG.map((entry) => [entry.chainId, entry]),
);
const LOCALHOST = "127.0.0.1";
const SESSION_TIMEOUT_MS = 10 * 60 * 1000;
const UI_READY_TIMEOUT_MS = 60 * 1000;
const UI_POLL_INTERVAL_MS = 500;
const REQUEST_POLL_INTERVAL_MS = 1000;
const UI_PROCESS_KILL_TIMEOUT_MS = 5000;
const SERVER_CLOSE_TIMEOUT_MS = 3000;
/**
 * Authenticates requests to the loopback bridge (any local process could otherwise hit the API).
 * Must match the header the signer UI sends. Not branded: this is a generic Hardhat↔browser session secret.
 */
const HARDHAT_SIGNER_UI_SESSION_TOKEN_HEADER = "x-hardhat-signer-ui-session-token";
/** Reject huge JSON bodies on the local bridge (DoS / accidental OOM). */
const MAX_JSON_BODY_BYTES = 256 * 1024;
const TX_LOOKUP_TIMEOUT_MS_DEFAULT = 12_000;
const TX_LOOKUP_INTERVAL_MS = 200;

/**
 * Some RPC / wallet paths (e.g. private submission) return txs slowly; override with
 * HARDHAT_SIGNER_UI_TX_LOOKUP_TIMEOUT_MS (milliseconds, 1000–300000).
 */
function getSignerUiTxLookupTimeoutMs(): number {
  const raw = getOptionalEnvVar("HARDHAT_SIGNER_UI_TX_LOOKUP_TIMEOUT_MS");
  if (raw === undefined || raw === "") {
    return TX_LOOKUP_TIMEOUT_MS_DEFAULT;
  }
  const n = Number.parseInt(raw, 10);
  if (!Number.isFinite(n)) {
    return TX_LOOKUP_TIMEOUT_MS_DEFAULT;
  }
  return Math.min(Math.max(n, 1000), 300_000);
}
/** Time to allow the browser to poll terminal `sessionOutcome` before the bridge closes (deploy batch only). */
const DEFAULT_SHUTDOWN_DRAIN_MS = 1500;
/** After the bridge port closes, wait before SIGTERM on Next so the UI can observe connection loss first. */
const DEFAULT_SHUTDOWN_GRACE_MS = 2000;
const MAX_SESSION_OUTCOME_MESSAGE_CHARS = 500;
const DEFAULT_WORKFLOW_STATUS_STAGE_MIN_MS = 2000;

let activeSession: SignerUiSession | undefined;
let privateKeyUsageWarningShown = false;

/**
 * True while hardhat-deploy `deploy:runDeploy` is executing (entire tag batch).
 * Session close is deferred until the batch ends; individual `withSignerUiSession` wrappers skip close.
 */
let signerUiHardhatDeployBatchActive = false;
let signerUiHardhatDeployBatchTags: string | null = null;

async function sleep(ms: number): Promise<void> {
  await new Promise((resolveSleep) => setTimeout(resolveSleep, ms));
}

function loadHardhatRuntime(): typeof import("hardhat") {
  return requireFromSignerUiBridge("hardhat") as typeof import("hardhat");
}

type PrivateKeyWarningDetails = {
  networkName?: string;
  scriptContext?: string;
  uiSupported?: boolean;
};

export function warnIfUsingPrivateKeySigning(details: PrivateKeyWarningDetails = {}): void {
  assertExclusiveSignerMode();

  if (details.uiSupported === false && isSignerUiEnabled()) {
    throw new Error(
      `HARDHAT_SIGNER_UI=true is not supported for ${details.scriptContext ?? "this script"}. ` +
        "Disable HARDHAT_SIGNER_UI and use DEPLOYER_PRIVATE_KEY for this operation.",
    );
  }

  if (privateKeyUsageWarningShown || isSignerUiEnabled() || !hasConfiguredDeployerPrivateKey()) {
    return;
  }

  privateKeyUsageWarningShown = true;

  const suffixParts = [details.networkName ? `network=${details.networkName}` : undefined, details.scriptContext]
    .filter((value): value is string => value !== undefined && value.length > 0)
    .join(" ");

  const uiRecommendation =
    details.uiSupported === false
      ? "This script is not wired to HARDHAT_SIGNER_UI yet, so the private-key flow remains required here."
      : "Re-run the same command with HARDHAT_SIGNER_UI=true to sign in the browser wallet instead.";

  console.warn("");
  console.warn("=".repeat(100));
  console.warn("PRIVATE KEY SIGNING ACTIVE");
  console.warn(`DEPLOYER_PRIVATE_KEY is being used for transaction signing${suffixParts ? ` (${suffixParts})` : ""}.`);
  console.warn(
    "Keeping the deployer key in the local environment is not recommended when the browser signer UI can be used.",
  );
  console.warn(uiRecommendation);
  console.warn("=".repeat(100));
  console.warn("");
}

function parseNonNegativeIntEnv(name: string, defaultMs: number): number {
  const raw = process.env[name];
  if (raw === undefined || raw === "") {
    return defaultMs;
  }
  const n = Number.parseInt(raw, 10);
  if (!Number.isFinite(n) || n < 0) {
    return defaultMs;
  }
  return n;
}

function truncateSessionOutcomeMessage(message: string): string {
  if (message.length <= MAX_SESSION_OUTCOME_MESSAGE_CHARS) {
    return message;
  }
  return `${message.slice(0, MAX_SESSION_OUTCOME_MESSAGE_CHARS - 1)}…`;
}

function resolveShutdownNextDev(explicit?: boolean): boolean {
  if (explicit === true) {
    return true;
  }
  if (explicit === false) {
    return false;
  }
  return shutdownNextDevWithBridge();
}

async function closeActiveSignerUiSession(options?: CloseSignerUiSessionOptions): Promise<void> {
  if (!activeSession) {
    return;
  }

  try {
    await activeSession.close(options);
  } finally {
    activeSession = undefined;
  }
}

/**
 * SIGKILL any process listening on `port` (Next dev often survives pnpm SIGKILL).
 * Best-effort on Unix; no-op on Windows.
 */
function killTcpListenersOnPort(port: number): void {
  if (process.platform === "win32") {
    return;
  }

  const attempts: readonly [string, string[]][] = [
    ["lsof", ["-ti", `tcp:${port}`, "-sTCP:LISTEN"]],
    ["lsof", ["-ti", `:${port}`]],
  ];

  for (const [cmd, args] of attempts) {
    try {
      const out = execFileSync(cmd, args, {
        encoding: "utf8",
        stdio: ["ignore", "pipe", "ignore"],
        maxBuffer: 512 * 1024,
      }).trim();
      if (!out) {
        continue;
      }

      for (const line of out.split("\n")) {
        const pid = Number.parseInt(line.trim(), 10);
        if (Number.isFinite(pid) && pid > 0) {
          try {
            process.kill(pid, "SIGKILL");
          } catch {
            /* process may have exited */
          }
        }
      }
      return;
    } catch {
      /* try next lsof variant or no listeners */
    }
  }
}

function isPidListeningOnPort(pid: number, port: number): boolean {
  if (process.platform === "win32") {
    return false;
  }

  try {
    const out = execFileSync("lsof", ["-Pan", "-p", String(pid), "-iTCP", "-sTCP:LISTEN", "-Fn"], {
      encoding: "utf8",
      stdio: ["ignore", "pipe", "ignore"],
      maxBuffer: 512 * 1024,
    });
    return out.split("\n").some((line) => line.startsWith("n") && new RegExp(`:${port}(?:$|->)`).test(line.slice(1)));
  } catch {
    return false;
  }
}

/**
 * Next.js 16+ keeps a singleton dev server via `.next/dev/lock` (JSON with pid/port).
 * A stale lock or orphan PID makes `next dev` exit immediately — with stdio ignored the CLI only "hangs" until HTTP wait times out.
 */
function clearNextDevSingletonLock(signerUiDir: string): void {
  const lockPath = join(signerUiDir, ".next", "dev", "lock");
  if (!existsSync(lockPath)) {
    return;
  }

  try {
    const parsed = JSON.parse(readFileSync(lockPath, "utf8")) as { pid?: number; port?: number };
    const lockPort = parsed.port;
    const hasKnownPort = typeof lockPort === "number" && Number.isFinite(lockPort) && lockPort > 0;

    // Do not signal a lockfile PID unless we can confirm it's still listening on the lockfile port.
    // PIDs can be reused and blindly signaling could affect unrelated local processes.
    if (
      typeof parsed.pid === "number" &&
      parsed.pid > 0 &&
      hasKnownPort &&
      isPidListeningOnPort(parsed.pid, lockPort)
    ) {
      try {
        process.kill(parsed.pid, "SIGTERM");
      } catch {
        /* process does not exist or cannot be signaled */
      }
    }

    // Intentionally avoid blind kill-by-port here: stale lock metadata can point to a port
    // now used by an unrelated local service. Only the verified lockfile PID may be signaled above.
  } catch {
    /* invalid lock */
  }

  try {
    unlinkSync(lockPath);
  } catch {
    /* lock recreated or permission */
  }
}

function removeNextDevLockFile(signerUiDir: string): void {
  const lockPath = join(signerUiDir, ".next", "dev", "lock");
  try {
    if (existsSync(lockPath)) {
      unlinkSync(lockPath);
    }
  } catch {
    /* ignore */
  }
}

async function waitForChildEarlyExit(proc: ChildProcess, timeoutMs: number): Promise<number | null> {
  return await new Promise<number | null>((resolveEarly) => {
    const timer = setTimeout(() => {
      proc.off("exit", onExit);
      resolveEarly(null);
    }, timeoutMs);

    const onExit = (code: number | null) => {
      clearTimeout(timer);
      resolveEarly(code);
    };

    proc.once("exit", onExit);
  });
}

function createProcessOutputBuffer(maxChars: number): {
  append: (chunk: string | Buffer) => void;
  read: () => string;
} {
  let text = "";
  return {
    append: (chunk) => {
      text += chunk.toString();
      if (text.length > maxChars) {
        text = text.slice(text.length - maxChars);
      }
    },
    read: () => text.trim(),
  };
}

/** When false (default), the Next.js dev server is left running after the HTTP bridge closes so the tab stays usable. */
function shutdownNextDevWithBridge(): boolean {
  return process.env.HARDHAT_SIGNER_UI_SHUTDOWN_NEXT_DEV === "true";
}

function getSignerUiChainCatalogEntry(chainId: number): SignerUiChainCatalogEntry | undefined {
  return signerUiChainCatalogByChainId.get(chainId);
}

function getExplorerUrls(chainId: number): string[] {
  return getSignerUiChainCatalogEntry(chainId)?.blockExplorerUrls ?? [];
}

function getChainName(networkName: string, chainId: number): string {
  return getSignerUiChainCatalogEntry(chainId)?.chainName ?? networkName;
}

function getRpcUrl(hre: HardhatRuntimeEnvironment): string {
  const networkConfig = hre.network.config as { url?: string };

  if (typeof networkConfig.url === "string" && networkConfig.url.length > 0) {
    return networkConfig.url;
  }

  switch (hre.network.name) {
    case "zkevm_dev":
      return getBlockchainNode();
    case "l2":
      return getL2BlockchainNode() ?? "";
    case "hardhat":
      return "http://127.0.0.1:8545";
    default:
      return "";
  }
}

async function getChainMetadata(hre: HardhatRuntimeEnvironment): Promise<ChainMetadata> {
  const network = await hre.ethers.provider.getNetwork();
  const chainId = Number(network.chainId);

  return {
    chainId,
    chainName: getChainName(hre.network.name, chainId),
    rpcUrls: [getRpcUrl(hre)].filter(Boolean),
    blockExplorerUrls: getExplorerUrls(chainId),
    nativeCurrency: {
      name: "Ether",
      symbol: "ETH",
      decimals: 18,
    },
  };
}

function isSigner(value: unknown): value is AbstractSigner {
  return typeof value === "object" && value !== null && "provider" in value && "sendTransaction" in value;
}

function isProviderLike(value: unknown): value is Provider & { getSigner?: () => Promise<AbstractSigner> } {
  return typeof value === "object" && value !== null && "getNetwork" in value;
}

async function getFreePort(): Promise<number> {
  return await new Promise((resolvePort, reject) => {
    const server = createServer();

    server.listen(0, LOCALHOST, () => {
      const address = server.address();
      const port = typeof address === "object" && address ? address.port : undefined;
      server.close((error) => {
        if (error) {
          reject(error);
          return;
        }

        if (!port) {
          reject(new Error("Failed to allocate a local port for HARDHAT_SIGNER_UI"));
          return;
        }

        resolvePort(port);
      });
    });

    server.on("error", reject);
  });
}

async function readJsonBody(request: IncomingMessage, maxBytes: number): Promise<unknown> {
  const chunks: Buffer[] = [];
  let total = 0;

  for await (const chunk of request) {
    const buf = typeof chunk === "string" ? Buffer.from(chunk) : chunk;
    total += buf.length;
    if (total > maxBytes) {
      throw new Error(`Request body exceeds ${maxBytes} bytes`);
    }
    chunks.push(buf);
  }

  if (chunks.length === 0) {
    return undefined;
  }

  return JSON.parse(Buffer.concat(chunks).toString("utf8"));
}

function isValidTxHash(value: unknown): value is string {
  return typeof value === "string" && /^0x[0-9a-fA-F]{64}$/.test(value);
}

type ParsedSuccess<T> = { ok: true; value: T };
type ParsedFailure = { ok: false; error: string };
type ParsedResult<T> = ParsedSuccess<T> | ParsedFailure;

type WalletPayload = { address: string; chainId: number };
type RespondPayload = { requestId: string; hash: string; from: string; chainId: number };
type ErrorPayload = { requestId: string; message: string };

function parseWalletPayload(raw: unknown): ParsedResult<WalletPayload> {
  const payload = raw as { address?: unknown; chainId?: unknown };
  if (typeof payload?.address !== "string" || !isAddress(payload.address)) {
    return { ok: false, error: "Invalid wallet address." };
  }
  if (typeof payload?.chainId !== "number" || !Number.isInteger(payload.chainId)) {
    return { ok: false, error: "Invalid chainId." };
  }
  return { ok: true, value: { address: payload.address, chainId: payload.chainId } };
}

function parseRespondPayload(raw: unknown): ParsedResult<RespondPayload> {
  if (!raw || typeof raw !== "object") {
    return { ok: false, error: "Invalid JSON body." };
  }
  const payload = raw as Record<string, unknown>;

  if (typeof payload.requestId !== "string" || payload.requestId.length === 0) {
    return { ok: false, error: "Missing or invalid requestId." };
  }
  if (!isValidTxHash(payload.hash)) {
    return { ok: false, error: "Missing or invalid transaction hash." };
  }
  if (typeof payload.from !== "string" || !isAddress(payload.from)) {
    return { ok: false, error: "Missing or invalid from address." };
  }
  if (typeof payload.chainId !== "number" || !Number.isInteger(payload.chainId)) {
    return { ok: false, error: "Missing or invalid chainId." };
  }

  return {
    ok: true,
    value: {
      requestId: payload.requestId,
      hash: payload.hash,
      from: payload.from,
      chainId: payload.chainId,
    },
  };
}

function parseErrorPayload(raw: unknown): ParsedResult<ErrorPayload> {
  if (!raw || typeof raw !== "object") {
    return { ok: false, error: "Invalid JSON body." };
  }

  const payload = raw as Record<string, unknown>;
  if (typeof payload.requestId !== "string") {
    return { ok: false, error: "Missing requestId." };
  }

  return {
    ok: true,
    value: {
      requestId: payload.requestId,
      message: typeof payload.message === "string" ? payload.message.slice(0, 4000) : "Wallet or network error",
    },
  };
}

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

  const elapsed = Math.max(nowMs - currentStageChangedAtMs, 0);
  const remaining = minimumStageMs - elapsed;
  return remaining > 0 ? remaining : 0;
}

function normalizeCalldataHex(data: string | null | undefined): string {
  if (data === undefined || data === null || data === "" || data === "0x") {
    return "0x";
  }
  const d = data.toLowerCase();
  return d.startsWith("0x") ? d : `0x${d}`;
}

function isAsciiHexDigitByte(b: number): boolean {
  return (b >= 0x30 && b <= 0x39) || (b >= 0x41 && b <= 0x46) || (b >= 0x61 && b <= 0x66);
}

/**
 * Some wallets / RPCs return `input` where each byte is an ASCII hex character of the real calldata
 * (double-encoded: UTF-8 bytes spell "60806040…" instead of raw 0x60,0x80,0x60,0x40…).
 * Length is typically 2× the real bytecode. Sepolia often returns normal hex; mainnet + MetaMask
 * private flows have been observed to return this shape.
 */
function tryDecodeHexAsciiEncodedCalldata(normalizedLowerHex: string): string | null {
  if (normalizedLowerHex === "0x" || normalizedLowerHex.length < 6) {
    return null;
  }
  if ((normalizedLowerHex.length - 2) % 2 !== 0) {
    return null;
  }
  let bytes: Uint8Array;
  try {
    bytes = getBytes(normalizedLowerHex);
  } catch {
    return null;
  }
  if (bytes.length < 4 || bytes.length % 2 !== 0) {
    return null;
  }
  for (let i = 0; i < bytes.length; i++) {
    if (!isAsciiHexDigitByte(bytes[i]!)) {
      return null;
    }
  }
  const ascii = Buffer.from(bytes).toString("latin1").toLowerCase();
  try {
    const inner = getBytes(`0x${ascii}`);
    if (inner.length === 0) {
      return null;
    }
    return normalizeCalldataHex(hexlify(inner));
  } catch {
    return null;
  }
}

/** Normalize RPC `tx.data` for comparison to Hardhat’s requested calldata. */
function normalizeTxDataForSignerUiValidation(data: string | null | undefined): string {
  const base = normalizeCalldataHex(data);
  return tryDecodeHexAsciiEncodedCalldata(base) ?? base;
}

/**
 * Calldata is considered matching if:
 * 1) raw normalized tx data equals expected, or
 * 2) tx data decodes from ASCII-hex wrapper and then equals expected.
 *
 * This ordering avoids false mismatches for valid raw calldata that happens to look like ASCII bytes
 * (e.g. `0x31323334`), while still supporting RPCs that return ASCII-wrapped payloads.
 */
export function calldataMatchesPromptForSignerUiValidation(
  expectedData: string | null | undefined,
  actualData: string | null | undefined,
): boolean {
  const expected = normalizeCalldataHex(expectedData);
  const actualRaw = normalizeCalldataHex(actualData);
  if (actualRaw === expected) {
    return true;
  }
  const decoded = tryDecodeHexAsciiEncodedCalldata(actualRaw);
  return decoded !== null && decoded === expected;
}

function hexQuantityToBigInt(hex: HexString | undefined): bigint {
  if (hex === undefined || hex === "0x") {
    return 0n;
  }
  return BigInt(hex);
}

async function getTransactionWithRetry(
  provider: Provider,
  hash: string,
  timeoutMs: number,
  intervalMs: number,
): Promise<TransactionResponse | null> {
  const startedAt = Date.now();
  while (Date.now() - startedAt < timeoutMs) {
    const tx = await provider.getTransaction(hash);
    if (tx) {
      return tx;
    }
    await new Promise((resolveSleep) => setTimeout(resolveSleep, intervalMs));
  }
  return null;
}

function formatMismatchValue(value: string | bigint | null | undefined): string {
  if (typeof value === "bigint") {
    return value.toString();
  }
  if (value === undefined) {
    return "undefined";
  }
  if (value === null) {
    return "null";
  }
  return value;
}

/**
 * Ensures the hash the browser reported is the same tx Hardhat asked for (same sender, to, data, value).
 * Without this, any local process could POST /api/respond with a valid unrelated tx hash.
 */
function getOnChainTransactionPromptMismatch(
  tx: TransactionResponse,
  promptRequest: SerializedTransactionRequest,
  expectedFrom: string,
): string | null {
  let normalizedFrom: string;
  try {
    normalizedFrom = getAddress(expectedFrom);
  } catch {
    return "Expected sender address is invalid.";
  }

  try {
    if (!tx.from || getAddress(tx.from) !== normalizedFrom) {
      return `Sender mismatch: expected ${normalizedFrom}, got ${formatMismatchValue(tx.from)}.`;
    }
  } catch {
    return `Sender mismatch: expected ${normalizedFrom}, got ${formatMismatchValue(tx.from)}.`;
  }

  const expectedTo = promptRequest.to;
  if (expectedTo === undefined || expectedTo === null || expectedTo === "") {
    if (tx.to !== null && tx.to !== undefined) {
      return `Recipient mismatch: expected contract creation (null), got ${formatMismatchValue(tx.to)}.`;
    }
  } else {
    try {
      if (!tx.to || getAddress(tx.to) !== getAddress(expectedTo)) {
        return `Recipient mismatch: expected ${expectedTo}, got ${formatMismatchValue(tx.to)}.`;
      }
    } catch {
      return `Recipient mismatch: expected ${expectedTo}, got ${formatMismatchValue(tx.to)}.`;
    }
  }

  const normalizedExpectedData = normalizeCalldataHex(promptRequest.data);
  if (!calldataMatchesPromptForSignerUiValidation(promptRequest.data, tx.data)) {
    return `Calldata mismatch: expected ${normalizedExpectedData}, got ${normalizeCalldataHex(tx.data)}.`;
  }

  const expectedValue = hexQuantityToBigInt(promptRequest.value);
  const actualValue = tx.value ?? 0n;
  if (actualValue !== expectedValue) {
    return `Value mismatch: expected ${expectedValue.toString()}, got ${actualValue.toString()}.`;
  }

  return null;
}

function hexPayloadByteLength(normalizedHex: string): number {
  if (normalizedHex === "0x" || normalizedHex.length <= 2) {
    return 0;
  }
  return (normalizedHex.length - 2) / 2;
}

function safeKeccak256HexPayload(normalizedHex: string): string {
  try {
    if (normalizedHex === "0x" || normalizedHex.length <= 2) {
      return keccak256("0x");
    }
    return keccak256(normalizedHex);
  } catch {
    return "(keccak256 failed)";
  }
}

/** First byte index (0-based) where hex payloads differ, or null if identical. */
function firstDifferingDataByteIndex(expected: string, got: string): number | null {
  let i = 2;
  while (i < expected.length && i < got.length && expected[i] === got[i]) {
    i++;
  }
  if (i >= expected.length && i >= got.length) {
    return null;
  }
  return Math.floor((i - 2) / 2);
}

/**
 * Logs compact fingerprints when `/api/respond` validation fails (RPC tx vs Hardhat prompt).
 * Full mismatch strings can be megabytes for contract creation; keep stderr usable.
 */
function logSignerUiSubmittedTxMismatchDiagnostics(params: {
  txHash: string;
  mismatch: string;
  tx: TransactionResponse;
  promptRequest: SerializedTransactionRequest;
}): void {
  const { txHash, mismatch, tx, promptRequest } = params;
  const expData = normalizeCalldataHex(promptRequest.data);
  const gotDataRaw = normalizeCalldataHex(tx.data);
  const gotDataNormalized = normalizeTxDataForSignerUiValidation(tx.data);
  const rpcCalldataWasHexAsciiEncoded = tryDecodeHexAsciiEncodedCalldata(gotDataRaw) !== null;
  const headChars = 2 + 128; // 0x + 64 bytes hex
  const tailChars = 128;

  const summaryLine =
    mismatch.startsWith("Calldata mismatch:") && mismatch.length > 220
      ? "Calldata mismatch (expected Hardhat `data` !== RPC `tx.data`; details below)."
      : mismatch.length > 400
        ? `${mismatch.slice(0, 400)}…`
        : mismatch;

  console.warn("HARDHAT_SIGNER_UI: submitted transaction did not match the pending request.");
  console.warn(`HARDHAT_SIGNER_UI: ${summaryLine}`);
  console.warn(
    "HARDHAT_SIGNER_UI: mismatch diagnostics:",
    JSON.stringify(
      {
        txHash,
        txChainId: tx.chainId !== null && tx.chainId !== undefined ? Number(tx.chainId) : null,
        promptChainId:
          promptRequest.chainId !== undefined && promptRequest.chainId !== null ? promptRequest.chainId : null,
        txType: tx.type,
        txNonce: tx.nonce,
        txFrom: tx.from ?? null,
        toExpected:
          promptRequest.to === undefined || promptRequest.to === null || promptRequest.to === ""
            ? null
            : promptRequest.to,
        toFromRpc: tx.to ?? null,
        valueExpected: promptRequest.value ?? null,
        valueFromRpc: tx.value !== null && tx.value !== undefined ? tx.value.toString() : null,
        rpcCalldataWasHexAsciiEncoded,
        dataByteLengthExpected: hexPayloadByteLength(expData),
        dataByteLengthFromRpcRaw: hexPayloadByteLength(gotDataRaw),
        dataByteLengthFromRpcNormalized: hexPayloadByteLength(gotDataNormalized),
        dataKeccakExpected: safeKeccak256HexPayload(expData),
        dataKeccakFromRpcNormalized: safeKeccak256HexPayload(gotDataNormalized),
        firstDifferingDataByteIndex: firstDifferingDataByteIndex(expData, gotDataNormalized),
        expectedDataHead: expData.slice(0, Math.min(headChars, expData.length)),
        rpcDataHeadNormalized: gotDataNormalized.slice(0, Math.min(headChars, gotDataNormalized.length)),
        expectedDataTail: expData.length > headChars ? expData.slice(-Math.min(tailChars, expData.length - 2)) : null,
        rpcDataTailNormalized:
          gotDataNormalized.length > headChars
            ? gotDataNormalized.slice(-Math.min(tailChars, gotDataNormalized.length - 2))
            : null,
      },
      null,
      2,
    ),
  );
}

/**
 * Sends JSON and optionally runs `onFullySent` after the response is flushed.
 * Deferring deploy continuation until `finish` avoids closing the HTTP server while the
 * browser is still reading the `/api/respond` body (which surfaces as "Failed to fetch").
 */
function writeJson(response: ServerResponse, statusCode: number, payload: unknown, onFullySent?: () => void) {
  response.statusCode = statusCode;
  response.setHeader("Content-Type", "application/json");
  response.setHeader("Connection", "close");
  if (onFullySent) {
    response.once("finish", () => {
      queueMicrotask(onFullySent);
    });
  }
  response.end(JSON.stringify(payload));
}

/**
 * Only allow the local Next dev origin. Reflecting arbitrary Origin would let any website trigger
 * credentialess cross-origin requests to the bridge from the user's browser (mitigated by Private
 * Network Access in some browsers, but we still avoid wildcard / reflection).
 */
function applySignerUiCors(request: IncomingMessage, response: ServerResponse, uiPort: number): void {
  const allowedOrigin = `http://${LOCALHOST}:${uiPort}`;
  const origin = request.headers.origin;
  if (origin === allowedOrigin) {
    response.setHeader("Access-Control-Allow-Origin", origin);
    response.setHeader("Vary", "Origin");
  }
  response.setHeader("Access-Control-Allow-Methods", "GET, POST, OPTIONS");
  response.setHeader("Access-Control-Allow-Headers", `Content-Type, ${HARDHAT_SIGNER_UI_SESSION_TOKEN_HEADER}`);
}

function isValidSignerUiSessionToken(expected: string, request: IncomingMessage): boolean {
  const received = request.headers[HARDHAT_SIGNER_UI_SESSION_TOKEN_HEADER];
  if (typeof received !== "string" || received.length === 0) {
    return false;
  }
  try {
    const a = Buffer.from(expected, "utf8");
    const b = Buffer.from(received, "utf8");
    if (a.length !== b.length) {
      return false;
    }
    return timingSafeEqual(a, b);
  } catch {
    return false;
  }
}

function toHexQuantityString(value: bigint | number | string | undefined): HexString | undefined {
  if (value === undefined) {
    return undefined;
  }

  return toQuantity(value) as HexString;
}

async function normalizeAddress(value: AddressLike | null | undefined): Promise<string | undefined> {
  if (value === null || value === undefined) {
    return undefined;
  }

  const address = typeof value === "string" ? value : await Promise.resolve(value.toString());
  return isAddress(address) ? getAddress(address) : address;
}

function transactionTypeNumber(transactionRequest: TransactionRequest): number | undefined {
  const raw = transactionRequest.type;
  if (raw === undefined || raw === null) {
    return undefined;
  }
  if (typeof raw === "bigint") {
    return Number(raw);
  }
  if (typeof raw === "number") {
    return raw;
  }
  if (typeof raw === "string") {
    const parsed = Number.parseInt(raw, raw.startsWith("0x") ? 16 : 10);
    return Number.isNaN(parsed) ? undefined : parsed;
  }
  return undefined;
}

/**
 * Wallets reject `eth_sendTransaction` if both legacy `gasPrice` and EIP-1559 fee fields are set.
 * Hardhat networks often define `gasPrice` while ethers also fills `maxFeePerGas` from the RPC.
 */
function normalizeGasFeeFieldsForWallet(params: {
  gasPrice: HexString | undefined;
  maxFeePerGas: HexString | undefined;
  maxPriorityFeePerGas: HexString | undefined;
  type: number | undefined;
}): Pick<SerializedTransactionRequest, "gasPrice" | "maxFeePerGas" | "maxPriorityFeePerGas"> {
  const { gasPrice, maxFeePerGas, maxPriorityFeePerGas, type } = params;
  const hasEip1559Suggestion = maxFeePerGas !== undefined || maxPriorityFeePerGas !== undefined;

  if (type === 0 || type === 1) {
    return {
      ...(gasPrice !== undefined ? { gasPrice } : {}),
    };
  }
  if (type === 2) {
    return {
      ...(maxFeePerGas !== undefined ? { maxFeePerGas } : {}),
      ...(maxPriorityFeePerGas !== undefined ? { maxPriorityFeePerGas } : {}),
    };
  }
  if (hasEip1559Suggestion) {
    return {
      ...(maxFeePerGas !== undefined ? { maxFeePerGas } : {}),
      ...(maxPriorityFeePerGas !== undefined ? { maxPriorityFeePerGas } : {}),
    };
  }
  return {
    ...(gasPrice !== undefined ? { gasPrice } : {}),
  };
}

/**
 * Under `exactOptionalPropertyTypes`, optional keys must be omitted—not set to `undefined`.
 * Input may include explicit `undefined` per field (e.g. from `toHexQuantityString`); only defined strings are copied.
 */
function buildSerializedTransactionRequest(
  values: Partial<Record<keyof SerializedTransactionRequest, string | undefined>>,
): SerializedTransactionRequest {
  const out: SerializedTransactionRequest = {};
  const keys = [
    "to",
    "data",
    "value",
    "gas",
    "gasPrice",
    "maxFeePerGas",
    "maxPriorityFeePerGas",
    "nonce",
    "type",
    "chainId",
  ] as const satisfies readonly (keyof SerializedTransactionRequest)[];
  for (const key of keys) {
    const v = values[key];
    if (v !== undefined) {
      (out as Record<string, string>)[key] = v;
    }
  }
  return out;
}

async function serializeTransactionRequest(
  transactionRequest: TransactionRequest,
): Promise<SerializedTransactionRequest> {
  const gasPrice = toHexQuantityString(transactionRequest.gasPrice as bigint | number | string | undefined);
  const maxFeePerGas = toHexQuantityString(transactionRequest.maxFeePerGas as bigint | number | string | undefined);
  const maxPriorityFeePerGas = toHexQuantityString(
    transactionRequest.maxPriorityFeePerGas as bigint | number | string | undefined,
  );
  const feeFields = normalizeGasFeeFieldsForWallet({
    gasPrice,
    maxFeePerGas,
    maxPriorityFeePerGas,
    type: transactionTypeNumber(transactionRequest),
  });

  return buildSerializedTransactionRequest({
    to: await normalizeAddress(transactionRequest.to),
    data: transactionRequest.data ? (transactionRequest.data.toString() as HexString) : undefined,
    value: toHexQuantityString(transactionRequest.value as bigint | number | string | undefined),
    gas: toHexQuantityString(transactionRequest.gasLimit as bigint | number | string | undefined),
    ...feeFields,
    nonce: toHexQuantityString(transactionRequest.nonce as number | undefined),
    type: toHexQuantityString(transactionRequest.type as number | undefined),
    chainId: toHexQuantityString(transactionRequest.chainId as bigint | number | string | undefined),
  });
}

async function waitForHttpOk(url: string, timeoutMs: number) {
  const startedAt = Date.now();

  while (Date.now() - startedAt < timeoutMs) {
    try {
      const response = await fetch(url);
      if (response.ok) {
        return;
      }
    } catch {
      // Keep polling until the timeout expires.
    }

    await new Promise((resolveSleep) => setTimeout(resolveSleep, UI_POLL_INTERVAL_MS));
  }

  throw new Error(`Timed out while waiting for the signer UI to start at ${url}`);
}

async function openBrowser(url: string) {
  if (process.env.HARDHAT_SIGNER_UI_OPEN_BROWSER === "false") {
    return;
  }

  const command = process.platform === "darwin" ? "open" : "xdg-open";
  const child = spawn(command, [url], {
    stdio: "ignore",
    detached: true,
  });
  child.on("error", (error: Error) => {
    console.warn(
      `HARDHAT_SIGNER_UI: failed to open browser automatically (${error.message}). Open this URL manually: ${url}`,
    );
  });
  child.unref();
}

class SignerUiSession {
  private readonly sessionId = randomUUID();
  /** Opaque secret; required as `x-hardhat-signer-ui-session-token` on bridge HTTP calls. */
  private readonly sessionSecret = randomBytes(32).toString("base64url");
  private readonly startedAt = new Date().toISOString();
  private readonly context: SignerUiContext;
  private readonly batchTagsSummary: string | null;
  private activeScriptContext: string;
  private scriptOrdinal = 0;
  private server?: Server;
  private serverPort?: number;
  private uiPort?: number;
  private uiProcess?: ChildProcess;
  private started = false;
  private closed = false;
  private pendingRequest: PendingTransaction | undefined;
  private transactionProgress: TransactionProgress | null = null;
  private workflowStatus: WorkflowStatus | null = null;
  private workflowStatusChangedAtMs = 0;
  private pendingWorkflowStatus: { stage: WorkflowStatusStage; message: string } | null | undefined;
  private workflowStatusTransitionTimer: NodeJS.Timeout | undefined;
  private nextTransactionDetails?: UiTransactionDetails;
  private walletState: SessionWalletState | null = null;
  private walletResolvers: Array<(wallet: SessionWalletState) => void> = [];
  private walletRejectors: Array<(error: Error) => void> = [];
  private signer?: SignerUiSigner;
  private sessionOutcome: "complete" | "error" | null = null;
  private sessionOutcomeMessage: string | null = null;

  public constructor(context: SignerUiContext) {
    this.context = context;
    this.batchTagsSummary = context.batchTagsSummary;
    this.activeScriptContext = context.scriptContext;
  }

  /**
   * Called by the deploy subtask after success or failure, before a drain delay and `close()`,
   * so `/api/session` can report a terminal outcome to the browser.
   */
  public setTerminalSessionOutcome(outcome: "complete" | "error", message?: string): void {
    this.sessionOutcome = outcome;
    this.sessionOutcomeMessage = message ? truncateSessionOutcomeMessage(message) : null;
  }

  /** Called when each wrapped script starts (same session across a Hardhat deploy batch). */
  public notifyScriptContext(scriptContext: string): void {
    this.scriptOrdinal += 1;
    this.activeScriptContext = scriptContext;
    if (signerUiHardhatDeployBatchActive) {
      const tagPart =
        this.batchTagsSummary && this.batchTagsSummary.length > 0 ? `tags ${this.batchTagsSummary} · ` : "";
      console.log(`HARDHAT_SIGNER_UI: script ${this.scriptOrdinal} in this deploy run (${tagPart}${scriptContext})`);
    } else {
      console.log(`HARDHAT_SIGNER_UI: ${scriptContext}`);
    }
  }

  public async start(): Promise<void> {
    if (this.started) {
      return;
    }

    this.serverPort = await getFreePort();
    this.uiPort = await getFreePort();
    this.server = createServer((request, response) => {
      void this.handleRequest(request, response).catch((error: unknown) => {
        const message = error instanceof Error ? error.message : String(error);
        const status =
          typeof message === "string" && message.includes("exceeds") && message.includes("bytes") ? 413 : 500;
        if (!response.headersSent) {
          try {
            if (this.uiPort !== undefined) {
              applySignerUiCors(request, response, this.uiPort);
            }
            writeJson(response, status, { error: message });
          } catch {
            response.destroy();
          }
        } else {
          response.destroy();
        }
      });
    });
    await new Promise<void>((resolveServer, rejectServer) => {
      this.server?.listen(this.serverPort, LOCALHOST, () => resolveServer());
      this.server?.on("error", rejectServer);
    });

    clearNextDevSingletonLock(SIGNER_UI_DIR);
    await new Promise((resolveDelay) => setTimeout(resolveDelay, 400));

    // Launch the local signer-ui app by directory. It remains a private workspace app for deps/tooling,
    // but runtime does not depend on a workspace package name.
    // Never use "pipe" without draining: Next/Turbopack logs fill buffers and the child blocks before listening.
    const captureStartupLogs = process.env.HARDHAT_SIGNER_UI_DEBUG !== "true";
    const startupOutput = createProcessOutputBuffer(8000);
    this.uiProcess = spawn(
      "pnpm",
      ["--dir", SIGNER_UI_DIR, "exec", "next", "dev", "--hostname", LOCALHOST, "--port", String(this.uiPort)],
      {
        cwd: SIGNER_UI_DIR,
        env: { ...process.env },
        stdio: captureStartupLogs ? "pipe" : "inherit",
      },
    );

    if (captureStartupLogs) {
      this.uiProcess.stdout?.on("data", (chunk: string | Buffer) => {
        startupOutput.append(chunk);
      });
      this.uiProcess.stderr?.on("data", (chunk: string | Buffer) => {
        startupOutput.append(chunk);
      });
    }

    this.uiProcess.on("exit", (code) => {
      if (!this.closed && code !== 0) {
        this.failPendingState(new Error(`Signer UI process exited unexpectedly with code ${code ?? "unknown"}`));
      }
    });

    const earlyExitCode = await waitForChildEarlyExit(this.uiProcess, 3500);
    if (earlyExitCode !== null) {
      const startupOutputText = startupOutput.read();
      const startupDetails =
        startupOutputText.length > 0 ? ` Early output:\n${startupOutputText}` : " No startup output was captured.";
      throw new Error(
        `Signer UI (next dev) exited early with code ${earlyExitCode}. ` +
          `This can be caused by a stale .next/dev/lock, another running \`next dev\` for contracts/signer-ui, or missing signer-ui dependencies after a clean/install change.` +
          startupDetails,
      );
    }

    const uiUrl = this.getUiUrl();
    console.log(
      `HARDHAT_SIGNER_UI: starting Next dev on http://${LOCALHOST}:${this.uiPort} (bridge API on ${this.getApiBaseUrl()})…`,
    );
    await waitForHttpOk(uiUrl, UI_READY_TIMEOUT_MS);
    console.log(
      `HARDHAT_SIGNER_UI is enabled for ${this.activeScriptContext}. Waiting for browser wallet approval at ${uiUrl}`,
    );
    await openBrowser(uiUrl);
    this.started = true;
  }

  public async close(options?: CloseSignerUiSessionOptions): Promise<void> {
    if (this.closed) {
      return;
    }

    this.closed = true;
    this.failPendingState(new Error(`HARDHAT_SIGNER_UI session closed during ${this.activeScriptContext}`));
    if (this.workflowStatusTransitionTimer) {
      clearTimeout(this.workflowStatusTransitionTimer);
      this.workflowStatusTransitionTimer = undefined;
    }
    this.pendingWorkflowStatus = undefined;

    const apiPort = this.serverPort;
    const nextDevPort = this.uiPort;
    const shouldStopNext = resolveShutdownNextDev(options?.shutdownNextDev);

    if (this.server) {
      const server = this.server;
      this.server = undefined;
      server.closeAllConnections?.();
      await Promise.race([
        new Promise<void>((resolveServer) => {
          server.close(() => resolveServer());
        }),
        new Promise<void>((resolveTimeout) => {
          setTimeout(resolveTimeout, SERVER_CLOSE_TIMEOUT_MS);
        }),
      ]);
      server.closeAllConnections?.();
      if (apiPort !== undefined) {
        killTcpListenersOnPort(apiPort);
      }
    }

    if (shouldStopNext) {
      await sleep(parseNonNegativeIntEnv("HARDHAT_SIGNER_UI_SHUTDOWN_GRACE_MS", DEFAULT_SHUTDOWN_GRACE_MS));
      await this.stopUiProcess();
      if (nextDevPort !== undefined) {
        killTcpListenersOnPort(nextDevPort);
      }
      removeNextDevLockFile(SIGNER_UI_DIR);
      console.log("HARDHAT_SIGNER_UI: Next.js signer UI dev server stopped.");
    } else {
      if (nextDevPort !== undefined) {
        console.log(
          `HARDHAT_SIGNER_UI: HTTP bridge closed. Signer UI still at http://${LOCALHOST}:${nextDevPort} — tab stays open. Stop Next with HARDHAT_SIGNER_UI_SHUTDOWN_NEXT_DEV=true (non-deploy), or after deploy set HARDHAT_SIGNER_UI_LEAVE_NEXT_DEV_AFTER_DEPLOY=true to keep Next running.`,
        );
      } else {
        console.log(
          "HARDHAT_SIGNER_UI: HTTP bridge closed (Next.js UI port unknown). Set HARDHAT_SIGNER_UI_SHUTDOWN_NEXT_DEV=true to force child teardown when used.",
        );
      }
    }
  }

  private async stopUiProcess(): Promise<void> {
    const proc = this.uiProcess;
    if (!proc) {
      return;
    }

    this.uiProcess = undefined;

    await new Promise<void>((resolveStop) => {
      let settled = false;
      const finish = () => {
        if (settled) {
          return;
        }
        settled = true;
        clearTimeout(timer);
        resolveStop();
      };

      const timer = setTimeout(() => {
        try {
          proc.kill("SIGKILL");
        } catch {
          /* process may already be gone */
        }
        finish();
      }, UI_PROCESS_KILL_TIMEOUT_MS);

      proc.once("exit", finish);
      try {
        proc.kill("SIGTERM");
      } catch {
        finish();
      }
    });
  }

  public getState(): SessionState {
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
      batchRunActive: signerUiHardhatDeployBatchActive,
      batchTagsSummary: this.batchTagsSummary,
      sessionOutcome: this.sessionOutcome,
      outcomeMessage: this.sessionOutcomeMessage,
    };
  }

  public getSigner(provider?: Provider): SignerUiSigner {
    if (!this.signer || (provider && this.signer.provider !== provider)) {
      this.signer = new SignerUiSigner(provider ?? this.context.provider, this);
    }

    return this.signer;
  }

  private setTransactionProgress(requestId: string, stage: TransactionProgressStage, message: string): void {
    this.transactionProgress = {
      requestId,
      stage,
      message,
      updatedAt: new Date().toISOString(),
    };
  }

  public clearTransactionProgress(requestId?: string): void {
    if (!this.transactionProgress) {
      return;
    }
    if (requestId && this.transactionProgress.requestId !== requestId) {
      return;
    }
    this.transactionProgress = null;
  }

  public setWorkflowStatus(stage: WorkflowStatusStage, message: string): void {
    this.transitionWorkflowStatus({ stage, message });
  }

  public clearWorkflowStatus(): void {
    this.transitionWorkflowStatus(null);
  }

  private applyWorkflowStatus(status: { stage: WorkflowStatusStage; message: string } | null): void {
    if (this.workflowStatusTransitionTimer) {
      clearTimeout(this.workflowStatusTransitionTimer);
      this.workflowStatusTransitionTimer = undefined;
    }
    this.pendingWorkflowStatus = undefined;

    if (status === null) {
      this.workflowStatus = null;
      this.workflowStatusChangedAtMs = 0;
      return;
    }

    this.workflowStatus = {
      stage: status.stage,
      message: status.message,
      updatedAt: new Date().toISOString(),
    };
    this.workflowStatusChangedAtMs = Date.now();
  }

  private transitionWorkflowStatus(next: { stage: WorkflowStatusStage; message: string } | null): void {
    const delayMs = workflowStatusTransitionDelayMs({
      currentStage: this.workflowStatus?.stage ?? null,
      nextStage: next?.stage ?? null,
      currentStageChangedAtMs: this.workflowStatusChangedAtMs,
      nowMs: Date.now(),
      minimumStageMs: parseNonNegativeIntEnv(
        "HARDHAT_SIGNER_UI_WORKFLOW_STATUS_STAGE_MIN_MS",
        DEFAULT_WORKFLOW_STATUS_STAGE_MIN_MS,
      ),
    });

    if (delayMs <= 0) {
      this.applyWorkflowStatus(next);
      return;
    }

    this.pendingWorkflowStatus = next;
    if (this.workflowStatusTransitionTimer) {
      clearTimeout(this.workflowStatusTransitionTimer);
    }
    this.workflowStatusTransitionTimer = setTimeout(() => {
      this.workflowStatusTransitionTimer = undefined;
      const queued = this.pendingWorkflowStatus;
      this.pendingWorkflowStatus = undefined;
      this.applyWorkflowStatus(queued ?? null);
    }, delayMs);
  }

  public async waitForWallet(): Promise<SessionWalletState> {
    await this.start();

    if (this.walletState) {
      return this.walletState;
    }

    return await new Promise<SessionWalletState>((resolveWallet, rejectWallet) => {
      const timeout = setTimeout(() => {
        this.walletResolvers = this.walletResolvers.filter((resolver) => resolver !== onWallet);
        this.walletRejectors = this.walletRejectors.filter((rejector) => rejector !== onReject);
        rejectWallet(new Error(`Timed out while waiting for a wallet connection for ${this.activeScriptContext}`));
      }, SESSION_TIMEOUT_MS);

      const onWallet = (wallet: SessionWalletState) => {
        clearTimeout(timeout);
        resolveWallet(wallet);
      };

      const onReject = (error: Error) => {
        clearTimeout(timeout);
        rejectWallet(error);
      };

      this.walletResolvers.push(onWallet);
      this.walletRejectors.push(onReject);
    });
  }

  public async requestTransaction(
    transactionRequest: TransactionRequest,
    label: string,
    description: string,
  ): Promise<TransactionOutcome> {
    await this.start();

    if (this.pendingRequest) {
      throw new Error(
        `HARDHAT_SIGNER_UI only supports one in-flight transaction at a time (${this.activeScriptContext})`,
      );
    }

    const extra = this.takeNextTransactionDetails();
    const prompt: TransactionPrompt = {
      id: randomUUID(),
      label,
      description,
      createdAt: new Date().toISOString(),
      request: await serializeTransactionRequest(transactionRequest),
      transactionDetails: extra,
    };

    return await new Promise<TransactionOutcome>((resolvePrompt, rejectPrompt) => {
      this.pendingRequest = {
        prompt,
        resolve: resolvePrompt,
        reject: rejectPrompt,
      };
      this.setTransactionProgress(prompt.id, "awaiting_wallet_approval", `Waiting for wallet approval for ${label}.`);
    });
  }

  public async waitForTransaction(hash: string): Promise<TransactionResponse> {
    const provider = this.context.provider;
    const startedAt = Date.now();
    const requestId = this.transactionProgress?.requestId;

    if (requestId) {
      this.setTransactionProgress(
        requestId,
        "waiting_for_hardhat_confirmation",
        `Validation passed. Waiting for Hardhat/provider confirmation for transaction ${hash}.`,
      );
    }

    while (Date.now() - startedAt < SESSION_TIMEOUT_MS) {
      const transaction = await provider.getTransaction(hash);
      if (transaction) {
        return transaction;
      }

      await new Promise((resolveSleep) => setTimeout(resolveSleep, REQUEST_POLL_INTERVAL_MS));
    }

    if (requestId) {
      this.setTransactionProgress(
        requestId,
        "failed",
        `Timed out while waiting for transaction ${hash} to become available on ${this.context.networkName}.`,
      );
    }

    throw new Error(
      `Timed out while waiting for transaction ${hash} to become available on ${this.context.networkName}`,
    );
  }

  private async handleRequest(request: IncomingMessage, response: ServerResponse) {
    if (this.uiPort === undefined) {
      writeJson(response, 503, { error: "Signer UI session is not ready." });
      return;
    }

    applySignerUiCors(request, response, this.uiPort);

    if (request.method === "OPTIONS") {
      response.statusCode = 204;
      response.setHeader("Connection", "close");
      response.end();
      return;
    }

    if (!isValidSignerUiSessionToken(this.sessionSecret, request)) {
      writeJson(response, 401, { error: "Missing or invalid signer UI session token." });
      return;
    }

    const url = new URL(request.url ?? "/", `http://${LOCALHOST}`);

    if (request.method === "GET" && url.pathname === "/health") {
      writeJson(response, 200, { ok: true, sessionId: this.sessionId });
      return;
    }

    if (request.method === "GET" && url.pathname === "/api/session") {
      writeJson(response, 200, this.getState());
      return;
    }

    if (request.method === "POST" && url.pathname === "/api/wallet") {
      await this.handleWalletRoute(request, response);
      return;
    }

    if (request.method === "POST" && url.pathname === "/api/respond") {
      await this.handleRespondRoute(request, response);
      return;
    }

    if (request.method === "POST" && url.pathname === "/api/error") {
      await this.handleErrorRoute(request, response);
      return;
    }

    if (request.method === "POST" && url.pathname === "/api/terminate") {
      this.handleTerminateRoute(response);
      return;
    }

    writeJson(response, 404, { error: "Not found" });
  }

  private async handleWalletRoute(request: IncomingMessage, response: ServerResponse): Promise<void> {
    const parsedPayload = parseWalletPayload(await readJsonBody(request, MAX_JSON_BODY_BYTES));
    if (!parsedPayload.ok) {
      writeJson(response, 400, { error: parsedPayload.error });
      return;
    }

    const payload = parsedPayload.value;

    /* Do not require the wallet to already be on the target chain: MetaMask often connects on
     * the wrong network first, and rejecting here left waitForWallet() pending forever. Chain is
     * enforced before send in the UI (ensureTargetChain) and again in POST /api/respond. */

    let normalizedAddress: string;
    try {
      normalizedAddress = getAddress(payload.address);
    } catch {
      writeJson(response, 400, { error: "Invalid wallet address." });
      return;
    }

    this.walletState = {
      address: normalizedAddress,
      chainId: payload.chainId,
      connectedAt: new Date().toISOString(),
    };

    const resolvers = this.walletResolvers.splice(0);
    this.walletRejectors = [];
    const walletSnapshot = this.walletState;
    writeJson(response, 200, { ok: true }, () => {
      for (const resolver of resolvers) {
        queueMicrotask(() => resolver(walletSnapshot));
      }
    });
  }

  private async handleRespondRoute(request: IncomingMessage, response: ServerResponse): Promise<void> {
    const parsedPayload = parseRespondPayload(await readJsonBody(request, MAX_JSON_BODY_BYTES));
    if (!parsedPayload.ok) {
      writeJson(response, 400, { error: parsedPayload.error });
      return;
    }

    const payload = parsedPayload.value;
    if (payload.chainId !== this.context.chain.chainId) {
      writeJson(response, 400, { error: "chainId does not match this Hardhat signer session." });
      return;
    }

    if (!this.walletState) {
      writeJson(response, 400, { error: "Register the wallet with the session before submitting a transaction." });
      return;
    }

    let normalizedFrom: string;
    try {
      normalizedFrom = getAddress(payload.from);
    } catch {
      writeJson(response, 400, { error: "Invalid from address." });
      return;
    }
    if (this.context.expectedSignerAddress && normalizedFrom !== this.context.expectedSignerAddress) {
      const message = `Expected signer is ${this.context.expectedSignerAddress}. Disconnect and connect the correct wallet, then try again.`;
      this.setTransactionProgress(payload.requestId, "awaiting_wallet_approval", message);
      writeJson(response, 409, { error: message });
      return;
    }

    if (normalizedFrom !== this.walletState.address) {
      writeJson(response, 400, { error: "from does not match the wallet registered for this session." });
      return;
    }

    if (!this.pendingRequest || this.pendingRequest.prompt.id !== payload.requestId) {
      writeJson(response, 400, { error: "No matching pending transaction request was found." });
      return;
    }

    this.setTransactionProgress(
      payload.requestId,
      "submitted_waiting_for_rpc",
      "Transaction submitted in the wallet. Waiting for the RPC to return the transaction.",
    );

    const onChain = await getTransactionWithRetry(
      this.context.provider,
      payload.hash,
      getSignerUiTxLookupTimeoutMs(),
      TX_LOOKUP_INTERVAL_MS,
    );

    if (!onChain) {
      this.setTransactionProgress(
        payload.requestId,
        "failed",
        "Transaction not found on the RPC yet. Wait for the wallet to broadcast, then try again from the UI if needed.",
      );
      writeJson(response, 400, {
        error:
          "Transaction not found on the RPC yet. Wait for the wallet to broadcast, then try again from the UI if needed.",
      });
      return;
    }

    this.setTransactionProgress(
      payload.requestId,
      "validating_submitted_transaction",
      "Validating that the submitted transaction matches the pending Hardhat request.",
    );

    const mismatch = getOnChainTransactionPromptMismatch(
      onChain,
      this.pendingRequest.prompt.request,
      this.walletState.address,
    );
    if (mismatch) {
      logSignerUiSubmittedTxMismatchDiagnostics({
        txHash: payload.hash,
        mismatch,
        tx: onChain,
        promptRequest: this.pendingRequest.prompt.request,
      });
      this.setTransactionProgress(
        payload.requestId,
        "failed",
        `Submitted transaction did not match the pending Hardhat request. ${mismatch}`,
      );
      writeJson(response, 400, {
        error: `On-chain transaction does not match the pending signer UI request. ${mismatch}`,
      });
      return;
    }

    const outcome: TransactionOutcome = {
      requestId: payload.requestId,
      hash: payload.hash,
      from: normalizedFrom,
      chainId: payload.chainId,
    };

    const { resolve: resolveOutcome } = this.pendingRequest;
    this.pendingRequest = undefined;
    this.setTransactionProgress(
      payload.requestId,
      "waiting_for_hardhat_confirmation",
      `Validation passed. Waiting for Hardhat/provider confirmation for transaction ${payload.hash}.`,
    );
    writeJson(response, 200, { ok: true }, () => {
      queueMicrotask(() => resolveOutcome(outcome));
    });
  }

  private async handleErrorRoute(request: IncomingMessage, response: ServerResponse): Promise<void> {
    const parsedPayload = parseErrorPayload(await readJsonBody(request, MAX_JSON_BODY_BYTES));
    if (!parsedPayload.ok) {
      writeJson(response, 400, { error: parsedPayload.error });
      return;
    }

    const payload = parsedPayload.value;
    let rejectOutcome: ((reason: Error) => void) | undefined;
    if (this.pendingRequest && this.pendingRequest.prompt.id === payload.requestId) {
      this.setTransactionProgress(payload.requestId, "failed", payload.message);
      rejectOutcome = this.pendingRequest.reject;
      this.pendingRequest = undefined;
    }

    writeJson(response, 200, { ok: true }, () => {
      if (rejectOutcome) {
        queueMicrotask(() => rejectOutcome!(new Error(payload.message)));
      }
    });
  }

  private handleTerminateRoute(response: ServerResponse): void {
    const message = "Session terminated by user from signer UI.";
    this.setTerminalSessionOutcome("error", message);
    writeJson(response, 200, { ok: true }, () => {
      queueMicrotask(() => {
        this.failPendingState(new Error(message));
        void closeActiveSignerUiSession().catch(() => {
          /* session may already be closing */
        });
      });
    });
  }

  private failPendingState(error: Error) {
    if (this.pendingRequest) {
      this.setTransactionProgress(this.pendingRequest.prompt.id, "failed", error.message);
      this.pendingRequest.reject(error);
      this.pendingRequest = undefined;
    }

    for (const rejector of this.walletRejectors.splice(0)) {
      rejector(error);
    }

    this.walletResolvers = [];
  }

  private getUiUrl(): string {
    const base = `http://${LOCALHOST}:${this.uiPort}?apiBaseUrl=${encodeURIComponent(this.getApiBaseUrl())}`;
    return `${base}&sessionToken=${encodeURIComponent(this.sessionSecret)}`;
  }

  private getApiBaseUrl(): string {
    return `http://${LOCALHOST}:${this.serverPort}`;
  }

  public setNextTransactionDetails(details: UiTransactionDetails): void {
    this.nextTransactionDetails = details;
  }

  private takeNextTransactionDetails(): UiTransactionDetails | undefined {
    const details = this.nextTransactionDetails;
    this.nextTransactionDetails = undefined;
    if (!details) {
      return undefined;
    }
    const hasAny = Object.values(details).some((v) => v !== undefined && v !== null && v !== "");
    return hasAny ? details : undefined;
  }
}

class SignerUiSigner extends AbstractSigner {
  private readonly session: SignerUiSession;

  public constructor(provider: Provider, session: SignerUiSession) {
    super(provider);
    this.session = session;
  }

  public override connect(provider: Provider | null): AbstractSigner {
    if (!provider) {
      throw new Error("HARDHAT_SIGNER_UI requires a provider-backed signer");
    }

    return new SignerUiSigner(provider, this.session);
  }

  public override async getAddress(): Promise<string> {
    const wallet = await this.session.waitForWallet();
    return wallet.address;
  }

  public override async signTransaction(transaction: TransactionRequest): Promise<string> {
    void transaction;
    throw new Error("HARDHAT_SIGNER_UI signs and sends through the browser wallet only.");
  }

  public override async signMessage(message: string | Uint8Array): Promise<string> {
    void message;
    throw new Error("HARDHAT_SIGNER_UI does not support signMessage.");
  }

  public override async signTypedData(
    domain: TypedDataDomain,
    types: Record<string, Array<TypedDataField>>,
    value: Record<string, unknown>,
  ): Promise<string> {
    void domain;
    void types;
    void value;
    throw new Error("HARDHAT_SIGNER_UI does not support signTypedData.");
  }

  public override async sendTransaction(transactionRequest: TransactionRequest): Promise<TransactionResponse> {
    const label = transactionRequest.to ? "Contract transaction" : "Contract creation";
    const description = transactionRequest.to
      ? `Send transaction to ${String(transactionRequest.to)}`
      : "Send contract creation transaction";
    const outcome = await this.session.requestTransaction(transactionRequest, label, description);
    try {
      return await this.session.waitForTransaction(outcome.hash);
    } finally {
      this.session.clearTransactionProgress(outcome.requestId);
    }
  }
}

async function getOrCreateSession(
  hre: HardhatRuntimeEnvironment,
  scriptContext: string,
): Promise<SignerUiSession | undefined> {
  if (!isSignerUiEnabled()) {
    return undefined;
  }

  if (!activeSession) {
    activeSession = new SignerUiSession({
      scriptContext,
      networkName: hre.network.name,
      chain: await getChainMetadata(hre),
      expectedSignerAddress: getExpectedSignerAddressIfConfigured() ?? null,
      provider: hre.ethers.provider,
      batchTagsSummary: signerUiHardhatDeployBatchTags,
    });
  }

  await activeSession.start();
  activeSession.notifyScriptContext(scriptContext);
  return activeSession;
}

async function resolveNamedDeployerAddress(
  getNamedAccounts: (() => Promise<{ deployer?: string }>) | undefined,
): Promise<string | undefined> {
  if (typeof getNamedAccounts !== "function") {
    return undefined;
  }

  try {
    const { deployer } = await getNamedAccounts();
    if (typeof deployer === "string" && deployer.length > 0) {
      return deployer;
    }
  } catch {
    /* fall back to provider default signer below */
  }

  return undefined;
}

function requireActiveSession(): SignerUiSession {
  if (!activeSession) {
    throw new Error("HARDHAT_SIGNER_UI is enabled but no active signer UI session exists.");
  }

  return activeSession;
}

function getExpectedSignerAddressIfConfigured(): string | undefined {
  const expectedSignerAddress = getOptionalEnvVar("EXPECTED_SIGNER_ADDRESS");
  if (!expectedSignerAddress) {
    return undefined;
  }

  try {
    return getAddress(expectedSignerAddress);
  } catch {
    throw new Error(
      `Invalid EXPECTED_SIGNER_ADDRESS "${expectedSignerAddress}". Provide a valid checksummed or lowercase hex address.`,
    );
  }
}

async function assertExpectedSignerAddressIfConfigured(signer: AbstractSigner): Promise<void> {
  const expected = getExpectedSignerAddressIfConfigured();
  if (!expected) {
    return;
  }

  const signerAddress = await signer.getAddress();
  const actual = getAddress(signerAddress);
  if (actual !== expected) {
    throw new Error(
      `Signer mismatch. EXPECTED_SIGNER_ADDRESS=${expected}, resolved signer address=${actual}. Update EXPECTED_SIGNER_ADDRESS or connect/configure the intended signer.`,
    );
  }
}

export async function getUiSigner(hre: HardhatRuntimeEnvironment): Promise<AbstractSigner> {
  assertExclusiveSignerMode();

  const uiMode = isSignerUiEnabled();
  let signer: AbstractSigner;
  if (uiMode) {
    signer = requireActiveSession().getSigner(hre.ethers.provider);
  } else {
    const signers = await hre.ethers.getSigners();
    if (signers.length === 0) {
      throw new Error(
        "No JSON-RPC account is configured for this network. Set DEPLOYER_PRIVATE_KEY (or your network accounts in hardhat.config), or set HARDHAT_SIGNER_UI=true to sign in the browser.",
      );
    }

    warnIfUsingPrivateKeySigning({
      networkName: hre.network.name,
    });

    const deployer = await resolveNamedDeployerAddress(hre.getNamedAccounts);
    signer = deployer ? await hre.ethers.getSigner(deployer) : signers[0]!;
  }

  if (!uiMode) {
    await assertExpectedSignerAddressIfConfigured(signer);
  }
  return signer;
}

export async function resolveUiRunner(runnerOrProvider?: AbstractSigner | Provider | null): Promise<AbstractSigner> {
  assertExclusiveSignerMode();

  if (isSigner(runnerOrProvider)) {
    return runnerOrProvider;
  }

  if (!runnerOrProvider) {
    // Lazy load: operational tasks import this module while Hardhat loads config; a top-level
    // `import "hardhat"` would trigger HH9 ("Hardhat can't be initialized while its config is being defined").
    const hh = loadHardhatRuntime() as unknown as HardhatRuntimeEnvironment;
    return await getUiSigner(hh);
  }

  if (isSignerUiEnabled()) {
    const provider = isProviderLike(runnerOrProvider) ? runnerOrProvider : undefined;
    return requireActiveSession().getSigner(provider);
  }

  if (isProviderLike(runnerOrProvider) && typeof runnerOrProvider.getSigner === "function") {
    warnIfUsingPrivateKeySigning();
    const hh = loadHardhatRuntime() as typeof import("hardhat") & {
      getNamedAccounts?: () => Promise<{ deployer?: string }>;
    };
    const deployer = await resolveNamedDeployerAddress(hh.getNamedAccounts);
    if (deployer) {
      return await runnerOrProvider.getSigner(deployer);
    }
    return await runnerOrProvider.getSigner();
  }

  const hh = loadHardhatRuntime() as unknown as HardhatRuntimeEnvironment;
  return await getUiSigner(hh);
}

/**
 * Attach constructor/initializer context for the next browser-signed transaction only.
 * No-op when HARDHAT_SIGNER_UI is not enabled or no session is active. Deploy helpers call this automatically.
 */
export function setUiTransactionContext(details: UiTransactionDetails): void {
  if (!isSignerUiEnabled() || !activeSession) {
    return;
  }

  activeSession.setNextTransactionDetails(details);
}

export function setUiWorkflowStatus(
  stage: "waiting_for_transaction_receipt" | "waiting_for_contract_verification",
  message: string,
): void {
  if (!isSignerUiEnabled() || !activeSession) {
    return;
  }

  activeSession.setWorkflowStatus(stage, message);
}

export function clearUiWorkflowStatus(): void {
  if (!isSignerUiEnabled() || !activeSession) {
    return;
  }

  activeSession.clearWorkflowStatus();
}

export function withSignerUiSession(scriptContext: string, deployFunction: DeployFunction): DeployFunction {
  const wrapped: DeployFunction = async function (hre: HardhatRuntimeEnvironment) {
    await getOrCreateSession(hre, scriptContext);

    try {
      return await deployFunction(hre);
    } finally {
      if (!signerUiHardhatDeployBatchActive) {
        await closeActiveSignerUiSession();
      }
    }
  };

  return wrapped;
}

/**
 * Runs `fn` with an active signer UI session when `HARDHAT_SIGNER_UI=true` (e.g. Hardhat tasks / operational scripts).
 * Closes the session when finished unless a `hardhat deploy` batch is in progress.
 */
export async function runWithSignerUiSession<T>(
  hre: HardhatRuntimeEnvironment,
  scriptContext: string,
  fn: () => Promise<T>,
): Promise<T> {
  if (!isSignerUiEnabled()) {
    return await fn();
  }

  await getOrCreateSession(hre, scriptContext);

  try {
    return await fn();
  } finally {
    if (!signerUiHardhatDeployBatchActive) {
      await closeActiveSignerUiSession();
    }
  }
}

/**
 * Wraps hardhat-deploy `deploy:runDeploy` so HARDHAT_SIGNER_UI keeps one browser session for the whole
 * tag batch. Registered from `hardhat.config.ts`.
 */
export async function signerUiHardhatDeployRunSubtaskAction(
  args: { tags?: string },
  hre: HardhatRuntimeEnvironment,
  runSuper: (taskArgs: unknown, env: HardhatRuntimeEnvironment) => Promise<unknown>,
): Promise<unknown> {
  void hre;
  if (!isSignerUiEnabled()) {
    return runSuper(args, hre);
  }

  const tagLabel = typeof args.tags === "string" ? args.tags.trim() : "";

  signerUiHardhatDeployBatchActive = true;
  signerUiHardhatDeployBatchTags = tagLabel.length > 0 ? tagLabel : null;

  console.log(
    `HARDHAT_SIGNER_UI: one browser session for this entire deploy run${
      tagLabel ? ` (--tags ${tagLabel})` : ""
    }. Scripts run sequentially; the bridge reports outcome then closes. Next.js stops after deploy unless HARDHAT_SIGNER_UI_LEAVE_NEXT_DEV_AFTER_DEPLOY=true.`,
  );

  let deployThrew = false;
  let deployFailureMessage: string | undefined;

  try {
    return await runSuper(args, hre);
  } catch (error: unknown) {
    deployThrew = true;
    deployFailureMessage = error instanceof Error ? error.message : String(error);
    throw error;
  } finally {
    signerUiHardhatDeployBatchActive = false;
    signerUiHardhatDeployBatchTags = null;

    const session = activeSession;
    if (session) {
      session.setTerminalSessionOutcome(deployThrew ? "error" : "complete", deployFailureMessage);
      await sleep(parseNonNegativeIntEnv("HARDHAT_SIGNER_UI_SHUTDOWN_DRAIN_MS", DEFAULT_SHUTDOWN_DRAIN_MS));
    }

    const shutdownNextDev = process.env.HARDHAT_SIGNER_UI_LEAVE_NEXT_DEV_AFTER_DEPLOY !== "true";
    await closeActiveSignerUiSession({ shutdownNextDev });

    if (deployThrew) {
      console.log("HARDHAT_SIGNER_UI: Hardhat deploy run failed — signer UI session closed.");
    } else {
      console.log("HARDHAT_SIGNER_UI: Hardhat deploy run complete.");
    }
  }
}

export const __testOnlySignerUiBridge = {
  buildSerializedTransactionRequest,
  calldataMatchesPromptForSignerUiValidation,
  normalizeGasFeeFieldsForWallet,
  parseErrorPayload,
  parseRespondPayload,
  parseWalletPayload,
  workflowStatusTransitionDelayMs,
};
