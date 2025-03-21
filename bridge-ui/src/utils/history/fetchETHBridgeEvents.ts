import { Address } from "viem";
import { getPublicClient } from "@wagmi/core";
import { LineaSDK } from "@consensys/linea-sdk";
import { config as wagmiConfig } from "@/lib/wagmi";
import { defaultTokensConfig, HistoryActionsForCompleteTxCaching } from "@/stores";
import {
  BridgeTransaction,
  BridgeTransactionType,
  Chain,
  ChainLayer,
  Token,
  MessageSentABIEvent,
  MessageSentLogEvent,
} from "@/types";
import { formatOnChainMessageStatus } from "./formatOnChainMessageStatus";
import { getCompleteTxStoreKey } from "./getCompleteTxStoreKey";
import { isBlockTooOld } from "./isBlockTooOld";

export async function fetchETHBridgeEvents(
  historyStoreActions: HistoryActionsForCompleteTxCaching,
  lineaSDK: LineaSDK,
  address: Address,
  fromChain: Chain,
  toChain: Chain,
  tokens: Token[],
): Promise<BridgeTransaction[]> {
  const transactionsMap = new Map<string, BridgeTransaction>();

  const client = getPublicClient(wagmiConfig, {
    chainId: fromChain.id,
  });

  const contract = fromChain.layer === ChainLayer.L1 ? lineaSDK.getL2Contract() : lineaSDK.getL1Contract();

  const messageServiceAddress = fromChain.messageServiceAddress;
  const [ethLogsForSender, ethLogsForRecipient] = await Promise.all([
    client.getLogs({
      event: MessageSentABIEvent,
      // No need to find more 'optimal' value than earliest.
      // Empirical testing showed no practical difference when using hardcoded block number (that was 90 days old).
      fromBlock: "earliest",
      toBlock: "latest",
      address: messageServiceAddress,
      args: {
        _from: address,
      },
    }),
    client.getLogs({
      event: MessageSentABIEvent,
      fromBlock: "earliest",
      toBlock: "latest",
      address: messageServiceAddress,
      args: {
        _to: address,
      },
    }),
  ]);

  const messageSentLogs = [...ethLogsForSender, ...ethLogsForRecipient] as unknown as MessageSentLogEvent[];

  const uniqueLogsMap = new Map<string, (typeof messageSentLogs)[0]>();
  for (const log of messageSentLogs) {
    const uniqueKey = `${log.args._from}-${log.args._to}-${log.transactionHash}`;
    if (!uniqueLogsMap.has(uniqueKey)) {
      uniqueLogsMap.set(uniqueKey, log);
    }
  }

  await Promise.all(
    Array.from(uniqueLogsMap.values()).map(async (log) => {
      const uniqueKey = `${log.args._from}-${log.args._to}-${log.transactionHash}`;

      // Search cache for completed tx for this txHash, if cache-hit can skip remaining logic
      const cacheKey = getCompleteTxStoreKey(fromChain.id, log.transactionHash);
      const cachedCompletedTx = historyStoreActions.getCompleteTx(cacheKey);
      if (cachedCompletedTx) {
        transactionsMap.set(uniqueKey, cachedCompletedTx);
        return;
      }

      const messageHash = log.args._messageHash;

      const block = await client.getBlock({ blockNumber: log.blockNumber, includeTransactions: false });
      if (isBlockTooOld(block)) return;

      const messageStatus = await contract.getMessageStatus(messageHash);

      const token = tokens.find((token) => token.type.includes("eth"));

      const tx = {
        type: BridgeTransactionType.ETH,
        status: formatOnChainMessageStatus(messageStatus),
        token: token || defaultTokensConfig.MAINNET[0],
        fromChain,
        toChain,
        timestamp: block.timestamp,
        bridgingTx: log.transactionHash,
        message: {
          from: log.args._from,
          to: log.args._to,
          fee: log.args._fee,
          value: log.args._value,
          nonce: log.args._nonce,
          calldata: log.args._calldata,
          messageHash: log.args._messageHash,
          amountSent: log.args._value,
        },
      };

      // Store COMPLETE tx in cache
      historyStoreActions.setCompleteTx(tx);
      transactionsMap.set(uniqueKey, tx);
    }),
  );

  return Array.from(transactionsMap.values());
}
