import { Chain, Token } from "@/types";

import { cctpAdapter } from "./cctp";
import { hyperlaneAdapter } from "./hyperlane";
import { nativeAdapter } from "./native";

import type { BridgeAdapter } from "./types";

export const allAdapters: readonly BridgeAdapter[] = [nativeAdapter, cctpAdapter, hyperlaneAdapter];

function isEnabled(adapter: BridgeAdapter): boolean {
  return adapter.isEnabled();
}

export function getAdapter(token: Token, fromChain: Chain, toChain: Chain): BridgeAdapter | undefined {
  return allAdapters.find((a) => isEnabled(a) && a.canHandle(token, fromChain, toChain));
}

export function getAdapterById(id: string): BridgeAdapter | undefined {
  // Adapter ids stored in transaction history must always resolve,
  // even when an adapter is feature-flagged off for new bridge actions.
  return allAdapters.find((a) => a.id === id);
}

export function getAllAdapters(): readonly BridgeAdapter[] {
  return allAdapters.filter(isEnabled);
}
