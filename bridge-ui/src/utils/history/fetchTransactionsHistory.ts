import { Address } from "viem";
import { LineaSDK } from "@consensys/linea-sdk";
import { config } from "@/config";
import { BridgeTransaction, Chain, Token } from "@/types";
import { fetchETHBridgeEvents } from "./fetchETHBridgeEvents";
import { fetchERC20BridgeEvents } from "./fetchERC20BridgeEvents";
import { fetchCctpBridgeEvents } from "./fetchCctpBridgeEvents";
import { HistoryActionsForCompleteTxCaching } from "@/stores";

type TransactionHistoryParams = {
  lineaSDK: LineaSDK;
  historyStoreActions: HistoryActionsForCompleteTxCaching;
  fromChain: Chain;
  toChain: Chain;
  address: Address;
  tokens: Token[];
};

export async function fetchTransactionsHistory({
  lineaSDK,
  fromChain,
  toChain,
  address,
  tokens,
  historyStoreActions,
}: TransactionHistoryParams): Promise<BridgeTransaction[]> {
  const events = await Promise.all([
    fetchBridgeEvents(lineaSDK, fromChain, toChain, address, tokens, historyStoreActions),
    fetchBridgeEvents(lineaSDK, toChain, fromChain, address, tokens, historyStoreActions),
  ]);
  return events.flat().sort((a, b) => Number(b.timestamp.toString()) - Number(a.timestamp.toString()));
}

async function fetchBridgeEvents(
  lineaSDK: LineaSDK,
  fromChain: Chain,
  toChain: Chain,
  address: Address,
  tokens: Token[],
  historyStoreActions: HistoryActionsForCompleteTxCaching,
): Promise<BridgeTransaction[]> {
  const [ethEvents, erc20Events, cctpEvents] = await Promise.all([
    fetchETHBridgeEvents(historyStoreActions, lineaSDK, address, fromChain, toChain, tokens),
    fetchERC20BridgeEvents(historyStoreActions, lineaSDK, address, fromChain, toChain, tokens),
    // Feature toggle for CCTP, will filter out USDC transactions if isCctpEnabled == false
    config.isCctpEnabled ? fetchCctpBridgeEvents(historyStoreActions, address, fromChain, toChain, tokens) : [],
  ]);

  return [...ethEvents, ...erc20Events, ...cctpEvents];
}
