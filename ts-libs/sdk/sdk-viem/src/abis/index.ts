/**
 * Minimal, hand-maintained ABI fragments for the protocol contracts the SDK interacts with,
 * grouped by contract.
 *
 * Each constant is intentionally scoped to a single function or event (rather than a full contract
 * ABI) so call sites stay tree-shakeable and the encoded selectors are easy to audit. They are kept
 * internal to `sdk-viem` (not re-exported from `src/index.ts`) and are therefore not part of the
 * public package API.
 */

export * from "./message-service";
export * from "./l2-message-service";
export * from "./lineth-rollup";
export * from "./token-bridge";
