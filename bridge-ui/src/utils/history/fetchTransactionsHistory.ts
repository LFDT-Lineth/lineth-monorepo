import { Address } from "viem";
import { config } from "@/config";
import { BridgeTransaction, Chain, Token } from "@/types";
import { fetchETHBridgeEvents } from "./fetchETHBridgeEvents";
import { fetchERC20BridgeEvents } from "./fetchERC20BridgeEvents";
import { fetchCctpBridgeEvents } from "./fetchCctpBridgeEvents";
import { HistoryActionsForCompleteTxCaching } from "@/stores";
import { Config } from "wagmi";

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
  const [ethEvents, erc20Events, cctpEvents] = await Promise.all([
    fetchETHBridgeEvents(historyStoreActions, address, fromChain, toChain, tokens, wagmiConfig),
    fetchERC20BridgeEvents(historyStoreActions, address, fromChain, toChain, tokens, wagmiConfig),
    // Feature toggle for CCTP, will filter out USDC transactions if isCctpEnabled == false
    config.isCctpEnabled
      ? fetchCctpBridgeEvents(historyStoreActions, address, fromChain, toChain, tokens, wagmiConfig)
      : [],
  ]);

  return [...ethEvents, ...erc20Events, ...cctpEvents];
}
