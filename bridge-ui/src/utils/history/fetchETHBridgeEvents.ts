import { getL1ToL2MessageStatus, getL2ToL1MessageStatus } from "@consensys/linea-sdk-viem";
import { getPublicClient } from "@wagmi/core";
import { Address, Client, Hex } from "viem";
import { Config } from "wagmi";

import { config } from "@/config";
import { type HistoryActionsForCompleteTxCaching } from "@/stores/historyStore";
import { defaultTokensConfig } from "@/stores/tokenStore";
import {
  BridgeTransaction,
  BridgeTransactionType,
  Chain,
  ChainLayer,
  MessageSentABIEvent,
  MessageSentLogEvent,
  Token,
} from "@/types";

import { formatOnChainMessageStatus } from "./formatOnChainMessageStatus";
import { isBlockTooOld } from "./isBlockTooOld";
import { restoreFromTransactionCache } from "./restoreFromTransactionCache";
import { saveToTransactionCache } from "./saveToTransactionCache";

export async function fetchETHBridgeEvents(
  historyStoreActions: HistoryActionsForCompleteTxCaching,
  address: Address,
  fromChain: Chain,
  toChain: Chain,
  tokens: Token[],
  wagmiConfig: Config,
): Promise<BridgeTransaction[]> {
  const transactionsMap = new Map<string, BridgeTransaction>();

  const originLayerClient = getPublicClient(wagmiConfig, {
    chainId: fromChain.id,
  });

  if (!originLayerClient) {
    throw new Error(`No public client for chain ${fromChain.name}`);
  }

  const destinationLayerClient = getPublicClient(wagmiConfig, {
    chainId: toChain.id,
  });

  const messageServiceAddress = fromChain.messageServiceAddress;
  const [ethLogsForSender, ethLogsForRecipient] = await Promise.all([
    originLayerClient.getLogs({
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
    originLayerClient.getLogs({
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
      if (
        restoreFromTransactionCache(historyStoreActions, fromChain.id, log.transactionHash, transactionsMap, uniqueKey)
      ) {
        return;
      }

      const messageHash = log.args._messageHash as Hex;

      const block = await originLayerClient.getBlock({ blockNumber: log.blockNumber, includeTransactions: false });
      if (isBlockTooOld(block)) return;

      const messageStatus =
        fromChain.layer === ChainLayer.L1
          ? await getL1ToL2MessageStatus(destinationLayerClient as Client, {
              messageHash,
              ...(config.e2eTestMode
                ? { l2MessageServiceAddress: config.chains[toChain.id].messageServiceAddress as Address }
                : {}),
            })
          : await getL2ToL1MessageStatus(destinationLayerClient as Client, {
              messageHash,
              l2Client: originLayerClient as Client,
              ...(config.e2eTestMode
                ? {
                    lineaRollupAddress: config.chains[toChain.id].messageServiceAddress as Address,
                    l2MessageServiceAddress: config.chains[fromChain.id].messageServiceAddress as Address,
                  }
                : {}),
            });

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

      saveToTransactionCache(historyStoreActions, tx);
      transactionsMap.set(uniqueKey, tx);
    }),
  );

  return Array.from(transactionsMap.values());
}
