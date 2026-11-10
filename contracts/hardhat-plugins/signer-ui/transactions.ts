/**
 * `eth_sendTransaction` → wallet request conversion, and validation that the transaction the browser reports is the
 * one Hardhat asked for.
 */
import {
  type Address,
  type Hash,
  type Transaction,
  bytesToHex,
  createPublicClient,
  custom,
  getAddress,
  hexToBytes,
  hexToNumber,
  isAddress,
  isHex,
  keccak256,
} from "viem";

import type { ChainType, NetworkConnection } from "hardhat/types/network";

export type HexString = `0x${string}`;

export type ParsedResult<T> = { ok: true; value: T } | { ok: false; error: string };

export type SerializedTransactionRequest = {
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

/** RPC reads the bridge needs to validate what the browser broadcast. */
export type SignerUiTransactionLookup = {
  getTransaction(hash: Hash): Promise<Transaction | null>;
  hasTransactionReceipt(hash: Hash): Promise<boolean>;
};

const REQUEST_FIELDS = [
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
const QUANTITY_FIELDS = [
  "value",
  "gas",
  "gasPrice",
  "maxFeePerGas",
  "maxPriorityFeePerGas",
  "nonce",
  "type",
  "chainId",
] as const;
/** Transaction fields the browser bridge cannot forward to a wallet. */
const UNSUPPORTED_TRANSACTION_FIELDS = ["blobs", "blobVersionedHashes", "authorizationList"] as const;
const TX_LOOKUP_INTERVAL_MS = 200;
const TX_LOOKUP_RPC_ATTEMPT_TIMEOUT_MS = 5_000;

export async function sleep(ms: number): Promise<void> {
  await new Promise((resolveSleep) => setTimeout(resolveSleep, ms));
}

/**
 * Wallets reject `eth_sendTransaction` if both legacy `gasPrice` and EIP-1559 fee fields are set.
 * Hardhat networks often define `gasPrice` while clients also fill `maxFeePerGas` from the RPC.
 */
export function normalizeGasFeeFieldsForWallet(params: {
  gasPrice: HexString | undefined;
  maxFeePerGas: HexString | undefined;
  maxPriorityFeePerGas: HexString | undefined;
  type: number | undefined;
}): Pick<SerializedTransactionRequest, "gasPrice" | "maxFeePerGas" | "maxPriorityFeePerGas"> {
  const { gasPrice, maxFeePerGas, maxPriorityFeePerGas, type } = params;
  const useEip1559 =
    type === 2 || (type !== 0 && type !== 1 && (maxFeePerGas !== undefined || maxPriorityFeePerGas !== undefined));
  return useEip1559
    ? buildSerializedTransactionRequest({ maxFeePerGas, maxPriorityFeePerGas })
    : buildSerializedTransactionRequest({ gasPrice });
}

/** Copies only the defined fields (`exactOptionalPropertyTypes`: optional keys are omitted, never `undefined`). */
export function buildSerializedTransactionRequest(
  values: Partial<Record<keyof SerializedTransactionRequest, string | undefined>>,
): SerializedTransactionRequest {
  const out: Record<string, string> = {};
  for (const key of REQUEST_FIELDS) {
    const value = values[key];
    if (value !== undefined) {
      out[key] = value;
    }
  }
  return out;
}

/** Validates the JSON-RPC `eth_sendTransaction` params (hex quantities, as viem and Ignition send them). */
export function parseSendTransactionParams(
  params: unknown,
): ParsedResult<{ from: Address | undefined; request: SerializedTransactionRequest }> {
  const [raw] = Array.isArray(params) ? params : [];
  if (typeof raw !== "object" || raw === null) {
    return { ok: false, error: "eth_sendTransaction expects a transaction object." };
  }
  const tx: Record<string, unknown> = { ...raw };

  for (const field of UNSUPPORTED_TRANSACTION_FIELDS) {
    if (tx[field] !== undefined && tx[field] !== null) {
      return { ok: false, error: `HARDHAT_SIGNER_UI cannot forward transactions with \`${field}\` to the wallet.` };
    }
  }

  const { from, to } = tx;
  if (from !== undefined && (typeof from !== "string" || !isAddress(from, { strict: false }))) {
    return { ok: false, error: "Invalid `from` address." };
  }
  if (to !== undefined && to !== null && (typeof to !== "string" || !isAddress(to, { strict: false }))) {
    return { ok: false, error: "Invalid `to` address." };
  }
  const data = tx.data ?? tx.input;
  if (data !== undefined && data !== null && (typeof data !== "string" || !isHex(data))) {
    return { ok: false, error: "Invalid `data`: expected hex." };
  }

  const quantities: Partial<Record<(typeof QUANTITY_FIELDS)[number], HexString>> = {};
  for (const field of QUANTITY_FIELDS) {
    const value = tx[field];
    if (value === undefined || value === null) {
      continue;
    }
    if (typeof value !== "string" || !isHex(value)) {
      return { ok: false, error: `Invalid \`${field}\`: expected a hex quantity.` };
    }
    quantities[field] = value;
  }

  return {
    ok: true,
    value: {
      from: from === undefined ? undefined : getAddress(from),
      request: buildSerializedTransactionRequest({
        to: typeof to === "string" ? getAddress(to) : undefined,
        data: typeof data === "string" ? data : undefined,
        value: quantities.value,
        gas: quantities.gas,
        ...normalizeGasFeeFieldsForWallet({
          gasPrice: quantities.gasPrice,
          maxFeePerGas: quantities.maxFeePerGas,
          maxPriorityFeePerGas: quantities.maxPriorityFeePerGas,
          type: quantities.type === undefined ? undefined : hexToNumber(quantities.type),
        }),
        nonce: quantities.nonce,
        type: quantities.type,
        chainId: quantities.chainId,
      }),
    },
  };
}

function normalizeCalldataHex(data: string | null | undefined): string {
  if (data === undefined || data === null || data === "" || data === "0x") {
    return "0x";
  }
  const lower = data.toLowerCase();
  return lower.startsWith("0x") ? lower : `0x${lower}`;
}

/** Strict even-length hex → bytes, or `null` (viem's `hexToBytes` accepts odd lengths). */
function strictHexToBytes(value: string): Uint8Array | null {
  if (!isHex(value, { strict: true }) || (value.length - 2) % 2 !== 0) {
    return null;
  }
  try {
    return hexToBytes(value);
  } catch {
    return null;
  }
}

const isAsciiHexDigitByte = (b: number) =>
  (b >= 0x30 && b <= 0x39) || (b >= 0x41 && b <= 0x46) || (b >= 0x61 && b <= 0x66);

/**
 * Some wallets / RPCs return `input` where each byte is an ASCII hex character of the real calldata
 * (double-encoded: UTF-8 bytes spell "60806040…" instead of raw 0x60,0x80,0x60,0x40…).
 * Length is typically 2× the real bytecode. Sepolia often returns normal hex; mainnet + MetaMask
 * private flows have been observed to return this shape.
 */
function tryDecodeHexAsciiEncodedCalldata(normalizedLowerHex: string): string | null {
  if (normalizedLowerHex.length < 6) {
    return null;
  }
  const bytes = strictHexToBytes(normalizedLowerHex);
  if (bytes === null || bytes.length < 4 || bytes.length % 2 !== 0 || !bytes.every(isAsciiHexDigitByte)) {
    return null;
  }
  const inner = strictHexToBytes(`0x${Buffer.from(bytes).toString("latin1").toLowerCase()}`);
  return inner === null || inner.length === 0 ? null : normalizeCalldataHex(bytesToHex(inner));
}

/**
 * Calldata matches if the raw normalized tx data equals the expected data, or if it decodes from an ASCII-hex wrapper
 * to the expected data. Raw first, so valid calldata that happens to look like ASCII bytes (e.g. `0x31323334`) is
 * never misread.
 */
export function calldataMatchesPromptForSignerUiValidation(
  expectedData: string | null | undefined,
  actualData: string | null | undefined,
): boolean {
  const expected = normalizeCalldataHex(expectedData);
  const actualRaw = normalizeCalldataHex(actualData);
  return actualRaw === expected || tryDecodeHexAsciiEncodedCalldata(actualRaw) === expected;
}

export function createTransactionLookup<ChainTypeT extends ChainType | string>(
  connection: NetworkConnection<ChainTypeT>,
): SignerUiTransactionLookup {
  const client = createPublicClient({ transport: custom(connection.provider, { retryCount: 0 }) });
  return {
    getTransaction: (hash) => client.getTransaction({ hash }).catch(() => null),
    hasTransactionReceipt: (hash) =>
      client.getTransactionReceipt({ hash }).then(
        () => true,
        () => false,
      ),
  };
}

async function withTimeout<T>(promise: Promise<T>, timeoutMs: number, onTimeout: T): Promise<T> {
  let timeoutHandle: ReturnType<typeof setTimeout> | undefined;
  try {
    return await Promise.race([
      promise,
      new Promise<T>((resolveTimeout) => {
        timeoutHandle = setTimeout(() => resolveTimeout(onTimeout), timeoutMs);
      }),
    ]);
  } finally {
    clearTimeout(timeoutHandle);
  }
}

/** Polls `eth_getTransactionByHash` until `timeoutMs`; falls back to the receipt (some RPCs index it first). */
export async function getTransactionWithRetry(
  lookup: SignerUiTransactionLookup,
  hash: Hash,
  timeoutMs: number,
): Promise<Transaction | null> {
  const deadline = Date.now() + timeoutMs;
  for (let remainingMs = timeoutMs; remainingMs > 0; remainingMs = deadline - Date.now()) {
    const tx = await withTimeout(
      lookup.getTransaction(hash),
      Math.min(TX_LOOKUP_RPC_ATTEMPT_TIMEOUT_MS, remainingMs),
      null,
    );
    if (tx) {
      return tx;
    }
    await sleep(Math.min(TX_LOOKUP_INTERVAL_MS, Math.max(deadline - Date.now(), 0)));
  }

  const hasReceipt = await withTimeout(
    lookup.hasTransactionReceipt(hash),
    Math.min(TX_LOOKUP_RPC_ATTEMPT_TIMEOUT_MS, timeoutMs),
    false,
  );
  return hasReceipt ? await withTimeout(lookup.getTransaction(hash), TX_LOOKUP_RPC_ATTEMPT_TIMEOUT_MS, null) : null;
}

function sameAddress(a: string, b: string): boolean {
  return isAddress(a, { strict: false }) && isAddress(b, { strict: false }) && getAddress(a) === getAddress(b);
}

/**
 * Ensures the hash the browser reported is the same tx Hardhat asked for (same sender, to, data, value).
 * Without this, any local process could POST /api/respond with a valid unrelated tx hash.
 */
export function getOnChainTransactionPromptMismatch(
  tx: Transaction,
  promptRequest: SerializedTransactionRequest,
  expectedFrom: string,
): string | null {
  if (!isAddress(expectedFrom, { strict: false })) {
    return "Expected sender address is invalid.";
  }
  if (!sameAddress(tx.from, expectedFrom)) {
    return `Sender mismatch: expected ${getAddress(expectedFrom)}, got ${tx.from}.`;
  }

  const expectedTo = promptRequest.to;
  if (expectedTo === undefined || expectedTo === "") {
    if (tx.to !== null && tx.to !== undefined) {
      return `Recipient mismatch: expected contract creation (null), got ${tx.to}.`;
    }
  } else if (!tx.to || !sameAddress(tx.to, expectedTo)) {
    return `Recipient mismatch: expected ${expectedTo}, got ${String(tx.to)}.`;
  }

  if (!calldataMatchesPromptForSignerUiValidation(promptRequest.data, tx.input)) {
    return `Calldata mismatch: expected ${normalizeCalldataHex(promptRequest.data)}, got ${normalizeCalldataHex(tx.input)}.`;
  }

  const expectedValue =
    promptRequest.value === undefined || promptRequest.value === "0x" ? 0n : BigInt(promptRequest.value);
  if ((tx.value ?? 0n) !== expectedValue) {
    return `Value mismatch: expected ${expectedValue.toString()}, got ${(tx.value ?? 0n).toString()}.`;
  }
  return null;
}

/**
 * Logs compact fingerprints when `/api/respond` validation fails (RPC tx vs Hardhat prompt).
 * Full mismatch strings can be megabytes for contract creation; keep stderr usable.
 */
export function logSubmittedTransactionMismatch(
  txHash: string,
  mismatch: string,
  tx: Transaction,
  promptRequest: SerializedTransactionRequest,
): void {
  const expected = normalizeCalldataHex(promptRequest.data);
  const raw = normalizeCalldataHex(tx.input);
  const decoded = tryDecodeHexAsciiEncodedCalldata(raw);
  const actual = decoded ?? raw;
  const byteLength = (hex: string) => Math.max(hex.length - 2, 0) / 2;
  const keccak = (hex: string) => {
    try {
      return isHex(hex) ? keccak256(hex) : "(keccak256 failed)";
    } catch {
      return "(keccak256 failed)";
    }
  };
  let firstDifference = 2;
  while (firstDifference < expected.length && expected[firstDifference] === actual[firstDifference]) {
    firstDifference++;
  }
  const head = 130; // 0x + 64 bytes
  const tail = (hex: string) => (hex.length > head ? hex.slice(-Math.min(128, hex.length - 2)) : null);

  const summary =
    mismatch.startsWith("Calldata mismatch:") && mismatch.length > 220
      ? "Calldata mismatch (expected Hardhat `data` !== RPC `tx.input`; details below)."
      : mismatch.length > 400
        ? `${mismatch.slice(0, 400)}…`
        : mismatch;
  console.warn("HARDHAT_SIGNER_UI: submitted transaction did not match the pending request.");
  console.warn(`HARDHAT_SIGNER_UI: ${summary}`);
  console.warn(
    "HARDHAT_SIGNER_UI: mismatch diagnostics:",
    JSON.stringify(
      {
        txHash,
        txChainId: tx.chainId ?? null,
        promptChainId: promptRequest.chainId ?? null,
        txType: tx.type,
        txNonce: tx.nonce,
        txFrom: tx.from,
        toExpected: promptRequest.to === undefined || promptRequest.to === "" ? null : promptRequest.to,
        toFromRpc: tx.to ?? null,
        valueExpected: promptRequest.value ?? null,
        valueFromRpc: tx.value.toString(),
        rpcCalldataWasHexAsciiEncoded: decoded !== null,
        dataByteLengthExpected: byteLength(expected),
        dataByteLengthFromRpcRaw: byteLength(raw),
        dataByteLengthFromRpcNormalized: byteLength(actual),
        dataKeccakExpected: keccak(expected),
        dataKeccakFromRpcNormalized: keccak(actual),
        firstDifferingDataByteIndex:
          firstDifference >= expected.length && firstDifference >= actual.length
            ? null
            : Math.floor((firstDifference - 2) / 2),
        expectedDataHead: expected.slice(0, head),
        rpcDataHeadNormalized: actual.slice(0, head),
        expectedDataTail: tail(expected),
        rpcDataTailNormalized: tail(actual),
      },
      null,
      2,
    ),
  );
}
