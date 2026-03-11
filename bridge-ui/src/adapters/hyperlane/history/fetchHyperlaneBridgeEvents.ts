import { getPublicClient, readContract } from "@wagmi/core";
import { Address } from "viem";
import { Config } from "wagmi";

import { type HistoryActionsForCompleteTxCaching } from "@/stores/historyStore";
import { BridgeProvider, BridgeTransaction, Chain, Token, TransactionStatus } from "@/types";
import { isBlockTooOld } from "@/utils/history/isBlockTooOld";
import { restoreFromTransactionCache } from "@/utils/history/restoreFromTransactionCache";
import { saveToTransactionCache } from "@/utils/history/saveToTransactionCache";
import { isUndefined } from "@/utils/misc";

import { MAILBOX_ABI } from "../abis";
import { MTokenSentAbiEvent, MTokenSentLogEvent } from "../events";

async function getHyperlaneTransactionStatus(
  messageId: `0x${string}`,
  toChain: Chain,
  wagmiConfig: Config,
): Promise<TransactionStatus> {
  if (isUndefined(toChain.hyperlaneMailboxAddress)) {
    return TransactionStatus.PENDING;
  }

  const delivered = await readContract(wagmiConfig, {
    abi: MAILBOX_ABI,
    address: toChain.hyperlaneMailboxAddress,
    functionName: "delivered",
    chainId: toChain.id,
    args: [messageId],
  });

  return delivered ? TransactionStatus.COMPLETED : TransactionStatus.PENDING;
}

export async function fetchHyperlaneBridgeEvents(
  historyStoreActions: HistoryActionsForCompleteTxCaching,
  address: Address,
  fromChain: Chain,
  toChain: Chain,
  tokens: Token[],
  wagmiConfig: Config,
): Promise<BridgeTransaction[]> {
  const transactionsMap = new Map<string, BridgeTransaction>();
  const fromChainClient = getPublicClient(wagmiConfig, { chainId: fromChain.id });

  if (!fromChainClient) {
    throw new Error(`No public client found for chain ID ${fromChain.id}`);
  }

  if (isUndefined(fromChain.hyperlanePortalLiteAddress)) {
    return [];
  }

  const token = tokens.find((t) => t.bridgeProvider === BridgeProvider.HYPERLANE);
  if (isUndefined(token)) return [];

  const [senderLogs, recipientLogs] = await Promise.all([
    fromChainClient.getLogs({
      event: MTokenSentAbiEvent,
      fromBlock: "earliest",
      toBlock: "latest",
      address: fromChain.hyperlanePortalLiteAddress,
      args: { sender: address },
    }),
    fromChainClient.getLogs({
      event: MTokenSentAbiEvent,
      fromBlock: "earliest",
      toBlock: "latest",
      address: fromChain.hyperlanePortalLiteAddress,
      args: { recipient: address },
    }),
  ]);

  const seenHashes = new Set<string>();
  const allLogs = [...senderLogs, ...recipientLogs].filter((log) => {
    if (seenHashes.has(log.transactionHash)) return false;
    seenHashes.add(log.transactionHash);
    return true;
  }) as unknown as MTokenSentLogEvent[];

  await Promise.all(
    allLogs.map(async (log) => {
      const transactionHash = log.transactionHash;

      if (
        restoreFromTransactionCache(
          historyStoreActions,
          fromChain.id,
          transactionHash,
          transactionsMap,
          transactionHash,
        )
      ) {
        return;
      }

      const fromBlock = await fromChainClient.getBlock({ blockNumber: log.blockNumber, includeTransactions: false });
      if (isBlockTooOld(fromBlock)) return;

      const messageId = log.args.messageId;
      const status = await getHyperlaneTransactionStatus(messageId, toChain, wagmiConfig);

      const tx: BridgeTransaction = {
        adapterId: "hyperlane",
        status,
        token,
        fromChain,
        toChain,
        timestamp: fromBlock.timestamp,
        bridgingTx: transactionHash,
        message: {
          messageId,
          amountSent: log.args.amount,
          transferIndex: log.args.index,
          sender: log.args.sender,
          recipient: log.args.recipient,
        },
      };

      saveToTransactionCache(historyStoreActions, tx);
      transactionsMap.set(transactionHash, tx);
    }),
  );

  return Array.from(transactionsMap.values());
}
