import { getAddress, isAddress, keccak256, toUtf8Bytes } from "ethers";

import { ChainName } from "./address-plan";

// Custom bootstrap items let operators of new networks (e.g. a consortium)
// deploy their own contracts/transactions on top of the 5 core steps. They are
// declared in a manifest file supplied at runtime so each network opts into its
// own set. Three kinds are supported:
//   - sign:      the deployer signs & submits (value send or contract create);
//                consumes a deployer nonce supplied by the runner for continuity.
//   - presigned: a keyless/external raw transaction broadcast as-is; consumes no
//                deployer nonce.
//   - script:    an operator-supplied child script that may create/sign/submit.
export type BootstrapItemKind = "sign" | "presigned" | "script";

interface BootstrapItemBase {
  id: string;
  chain: ChainName;
}

export interface SignBootstrapItem extends BootstrapItemBase {
  kind: "sign";
  to?: string;
  data?: string;
  valueWei: string;
  gasLimit?: string;
  expectAddress?: string;
}

export interface PresignedBootstrapItem extends BootstrapItemBase {
  kind: "presigned";
  rawTx: string;
  expectAddress?: string;
}

export interface ScriptBootstrapItem extends BootstrapItemBase {
  kind: "script";
  script: string;
}

export type BootstrapItem = SignBootstrapItem | PresignedBootstrapItem | ScriptBootstrapItem;

export interface BootstrapManifest {
  version: number;
  items: BootstrapItem[];
}

const ID_PATTERN = /^[a-z0-9][a-z0-9-]*$/;
const HEX_DATA_PATTERN = /^0x([0-9a-fA-F]{2})*$/;
const RAW_TX_PATTERN = /^0x[0-9a-fA-F]+$/;
const UINT_PATTERN = /^[0-9]+$/;

function fail(message: string): never {
  throw new Error(`bootstrap manifest: ${message}`);
}

function requireId(value: unknown): string {
  if (typeof value !== "string" || !ID_PATTERN.test(value)) {
    fail(`item id must be lowercase kebab-case (a-z0-9-), got: ${JSON.stringify(value)}`);
  }
  return value;
}

function requireChain(value: unknown, id: string): ChainName {
  if (value !== "l1" && value !== "l2") fail(`item ${id} chain must be "l1" or "l2", got: ${JSON.stringify(value)}`);
  return value;
}

function optionalAddress(value: unknown, field: string, id: string): string | undefined {
  if (value === undefined) return undefined;
  if (typeof value !== "string" || !isAddress(value)) {
    fail(`item ${id} ${field} must be an Ethereum address, got: ${JSON.stringify(value)}`);
  }
  return getAddress(value);
}

function optionalHex(value: unknown, field: string, id: string): string | undefined {
  if (value === undefined) return undefined;
  if (typeof value !== "string" || !HEX_DATA_PATTERN.test(value)) {
    fail(`item ${id} ${field} must be 0x-prefixed even-length hex, got: ${JSON.stringify(value)}`);
  }
  return value;
}

function requireUint(value: unknown, field: string, id: string): string {
  if (typeof value !== "string" || !UINT_PATTERN.test(value)) {
    fail(`item ${id} ${field} must be a non-negative integer string, got: ${JSON.stringify(value)}`);
  }
  return value;
}

function optionalUint(value: unknown, field: string, id: string): string | undefined {
  if (value === undefined) return undefined;
  return requireUint(value, field, id);
}

function parseItem(raw: unknown, index: number): BootstrapItem {
  if (typeof raw !== "object" || raw === null || Array.isArray(raw)) {
    fail(`item at index ${index} must be an object`);
  }
  const candidate = raw as Record<string, unknown>;
  const id = requireId(candidate.id);
  const chain = requireChain(candidate.chain, id);
  const kind = candidate.kind;

  if (kind === "sign") {
    const to = optionalAddress(candidate.to, "to", id);
    const data = optionalHex(candidate.data, "data", id);
    if (to === undefined && data === undefined) {
      fail(`item ${id} (sign) must set "to" for a transfer/call or "data" for a bare create`);
    }
    const gasLimit = optionalUint(candidate.gasLimit, "gasLimit", id);
    const expectAddress = optionalAddress(candidate.expectAddress, "expectAddress", id);
    return {
      id,
      chain,
      kind,
      valueWei: requireUint(candidate.valueWei ?? "0", "valueWei", id),
      ...(to !== undefined ? { to } : {}),
      ...(data !== undefined ? { data } : {}),
      ...(gasLimit !== undefined ? { gasLimit } : {}),
      ...(expectAddress !== undefined ? { expectAddress } : {}),
    };
  }

  if (kind === "presigned") {
    if (typeof candidate.rawTx !== "string" || !RAW_TX_PATTERN.test(candidate.rawTx)) {
      fail(`item ${id} (presigned) rawTx must be a 0x-prefixed raw transaction`);
    }
    const expectAddress = optionalAddress(candidate.expectAddress, "expectAddress", id);
    return {
      id,
      chain,
      kind,
      rawTx: candidate.rawTx,
      ...(expectAddress !== undefined ? { expectAddress } : {}),
    };
  }

  if (kind === "script") {
    if (typeof candidate.script !== "string" || candidate.script.length === 0) {
      fail(`item ${id} (script) must set a non-empty "script" path`);
    }
    if (candidate.script.includes("..")) {
      fail(`item ${id} (script) path must not contain ".."`);
    }
    return { id, chain, kind, script: candidate.script };
  }

  fail(`item ${id} kind must be "sign", "presigned", or "script", got: ${JSON.stringify(kind)}`);
}

export function parseBootstrapManifest(raw: string): BootstrapManifest {
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch (error) {
    fail(`invalid JSON: ${error instanceof Error ? error.message : String(error)}`);
  }
  if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
    fail("root must be an object");
  }
  const candidate = parsed as Record<string, unknown>;
  if (candidate.version !== 1) fail(`version must be 1, got: ${JSON.stringify(candidate.version)}`);
  if (!Array.isArray(candidate.items)) fail("items must be an array");

  const items = candidate.items.map((item, index) => parseItem(item, index));
  const seen = new Set<string>();
  for (const item of items) {
    if (seen.has(item.id)) fail(`duplicate item id ${item.id}`);
    seen.add(item.id);
  }
  return { version: 1, items };
}

// Normalized, deterministic serialization for the identity hash. Field order is
// fixed by the explicit per-kind mapping in parseItem, so JSON.stringify over the
// parsed items is stable for identical manifests.
export function bootstrapManifestHash(manifest: BootstrapManifest): string {
  return keccak256(toUtf8Bytes(JSON.stringify(manifest)));
}
