import { Address } from "viem";
import { Config } from "wagmi";

import { getAllAdapters } from "@/adapters";
import { type HistoryActionsForCompleteTxCaching } from "@/stores/historyStore";
import { BridgeTransaction, Chain, Token } from "@/types";

type TransactionHistoryParams = {
  historyStoreActions: HistoryActionsForCompleteTxCaching;
  fromChain: Chain;
  toChain: Chain;
  address: Address;
  tokens: Token[];
  wagmiConfig: Config;
};

export async function fetchTransactionsHistory({
  fromChain,
  toChain,
  address,
  tokens,
  historyStoreActions,
  wagmiConfig,
}: TransactionHistoryParams): Promise<BridgeTransaction[]> {
  const events = await Promise.all([
    fetchBridgeEvents(fromChain, toChain, address, tokens, historyStoreActions, wagmiConfig),
    fetchBridgeEvents(toChain, fromChain, address, tokens, historyStoreActions, wagmiConfig),
  ]);
  return events.flat().sort((a, b) => Number(b.timestamp.toString()) - Number(a.timestamp.toString()));
}

async function fetchBridgeEvents(
  fromChain: Chain,
  toChain: Chain,
  address: Address,
  tokens: Token[],
  historyStoreActions: HistoryActionsForCompleteTxCaching,
  wagmiConfig: Config,
): Promise<BridgeTransaction[]> {
  const adapters = getAllAdapters();
  const results = await Promise.all(
    adapters.map((adapter) =>
      adapter.fetchHistory({ historyStoreActions, address, fromChain, toChain, tokens, wagmiConfig }),
    ),
  );
  return results.flat();
}
