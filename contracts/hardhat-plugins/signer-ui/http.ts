/** Loopback HTTP API between the bridge and the signer app: CORS, session token, JSON bodies and their parsers. */
import { timingSafeEqual } from "node:crypto";
import { type Address, type Hash, isAddress } from "viem";

import { LOCALHOST } from "./next-dev.js";

import type { ParsedResult } from "./transactions.js";
import type { IncomingMessage, ServerResponse } from "node:http";

/**
 * Authenticates requests to the loopback bridge (any local process could otherwise hit the API). Must match the header
 * the signer UI sends (`signer-ui/app/bridgeApiClient.ts`).
 */
const SESSION_TOKEN_HEADER = "x-hardhat-signer-ui-session-token";
/** Reject huge JSON bodies on the local bridge (DoS / accidental OOM). */
const MAX_JSON_BODY_BYTES = 256 * 1024;

export function buildSignerUiUrl(uiPort: number, apiBaseUrl: string): string {
  return `http://${LOCALHOST}:${uiPort}?apiBaseUrl=${encodeURIComponent(apiBaseUrl)}`;
}

export function isAllowedSignerUiOrigin(request: IncomingMessage, uiPort: number): boolean {
  return request.headers.origin === `http://${LOCALHOST}:${uiPort}`;
}

/**
 * Only allow the local Next dev origin. Reflecting arbitrary Origin would let any website trigger credentialess
 * cross-origin requests to the bridge from the user's browser.
 */
export function applySignerUiCors(request: IncomingMessage, response: ServerResponse, uiPort: number): void {
  if (isAllowedSignerUiOrigin(request, uiPort) && typeof request.headers.origin === "string") {
    response.setHeader("Access-Control-Allow-Origin", request.headers.origin);
    response.setHeader("Vary", "Origin");
  }
  response.setHeader("Access-Control-Allow-Methods", "GET, POST, OPTIONS");
  response.setHeader("Access-Control-Allow-Headers", `Content-Type, ${SESSION_TOKEN_HEADER}`);
}

export function isValidSessionToken(expected: string, request: IncomingMessage): boolean {
  const received = request.headers[SESSION_TOKEN_HEADER];
  if (typeof received !== "string" || received.length === 0) {
    return false;
  }
  const a = Buffer.from(expected, "utf8");
  const b = Buffer.from(received, "utf8");
  return a.length === b.length && timingSafeEqual(a, b);
}

/**
 * Sends JSON and optionally runs `onFullySent` after the response is flushed. Deferring deploy continuation until
 * `finish` avoids closing the HTTP server while the browser is still reading the body ("Failed to fetch").
 */
export function writeJson(response: ServerResponse, statusCode: number, payload: unknown, onFullySent?: () => void) {
  response.statusCode = statusCode;
  response.setHeader("Content-Type", "application/json");
  response.setHeader("Connection", "close");
  if (onFullySent) {
    response.once("finish", () => queueMicrotask(onFullySent));
  }
  response.end(JSON.stringify(payload));
}

export async function readJsonBody(request: IncomingMessage): Promise<unknown> {
  const chunks: Buffer[] = [];
  let total = 0;
  for await (const chunk of request) {
    const buffer = typeof chunk === "string" ? Buffer.from(chunk) : chunk;
    total += buffer.length;
    if (total > MAX_JSON_BODY_BYTES) {
      throw new Error(`Request body exceeds ${MAX_JSON_BODY_BYTES} bytes`);
    }
    chunks.push(buffer);
  }
  return chunks.length === 0 ? undefined : JSON.parse(Buffer.concat(chunks).toString("utf8"));
}

function asRecord(raw: unknown): Record<string, unknown> | undefined {
  return typeof raw === "object" && raw !== null ? { ...raw } : undefined;
}

const isChainId = (value: unknown): value is number => typeof value === "number" && Number.isInteger(value);

export function parseWalletPayload(raw: unknown): ParsedResult<{ address: Address; chainId: number }> {
  const payload = asRecord(raw) ?? {};
  if (typeof payload.address !== "string" || !isAddress(payload.address)) {
    return { ok: false, error: "Invalid wallet address." };
  }
  if (!isChainId(payload.chainId)) {
    return { ok: false, error: "Invalid chainId." };
  }
  return { ok: true, value: { address: payload.address, chainId: payload.chainId } };
}

export function parseRespondPayload(
  raw: unknown,
): ParsedResult<{ requestId: string; hash: Hash; from: Address; chainId: number }> {
  const payload = asRecord(raw);
  if (payload === undefined) {
    return { ok: false, error: "Invalid JSON body." };
  }
  const { requestId, hash, from, chainId } = payload;
  if (typeof requestId !== "string" || requestId.length === 0) {
    return { ok: false, error: "Missing or invalid requestId." };
  }
  if (typeof hash !== "string" || !/^0x[0-9a-fA-F]{64}$/.test(hash)) {
    return { ok: false, error: "Missing or invalid transaction hash." };
  }
  if (typeof from !== "string" || !isAddress(from)) {
    return { ok: false, error: "Missing or invalid from address." };
  }
  if (!isChainId(chainId)) {
    return { ok: false, error: "Missing or invalid chainId." };
  }
  return { ok: true, value: { requestId, hash: `0x${hash.slice(2)}`, from, chainId } };
}

export function parseErrorPayload(raw: unknown): ParsedResult<{ requestId: string; message: string }> {
  const payload = asRecord(raw);
  if (payload === undefined) {
    return { ok: false, error: "Invalid JSON body." };
  }
  if (typeof payload.requestId !== "string") {
    return { ok: false, error: "Missing requestId." };
  }
  const message = typeof payload.message === "string" ? payload.message.slice(0, 4000) : "Wallet or network error";
  return { ok: true, value: { requestId: payload.requestId, message } };
}
