import {
  getL1ToL2MessageStatus,
  getL2ToL1MessageStatus,
  getMessagesByTransactionHash,
} from "@consensys/linea-sdk-viem";
import { getPublicClient } from "@wagmi/core";
import { Address, Client, decodeAbiParameters } from "viem";
import { Config } from "wagmi";

import { type HistoryActionsForCompleteTxCaching } from "@/stores/historyStore";
import { BridgeTransaction, Chain, ChainLayer, Token } from "@/types";
import { isBlockTooOld } from "@/utils/history/isBlockTooOld";
import { restoreFromTransactionCache } from "@/utils/history/restoreFromTransactionCache";
import { saveToTransactionCache } from "@/utils/history/saveToTransactionCache";
import { isUndefined, isUndefinedOrNull } from "@/utils/misc";

import { BridgingInitiatedV2ABIEvent, BridgingInitiatedV2LogEvent } from "../events";
import { formatOnChainMessageStatus } from "./formatOnChainMessageStatus";

export async function fetchERC20BridgeEvents(
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
    throw new Error(`No public client found for chain ID ${fromChain.id}`);
  }

  const destinationLayerClient = getPublicClient(wagmiConfig, {
    chainId: toChain.id,
  });

  const tokenBridgeAddress = fromChain.tokenBridgeAddress;
  const [erc20LogsForSender, erc20LogsForRecipient] = await Promise.all([
    originLayerClient.getLogs({
      event: BridgingInitiatedV2ABIEvent,
      fromBlock: "earliest",
      toBlock: "latest",
      address: tokenBridgeAddress,
      args: {
        sender: address,
      },
    }),
    originLayerClient.getLogs({
      event: BridgingInitiatedV2ABIEvent,
      fromBlock: "earliest",
      toBlock: "latest",
      address: tokenBridgeAddress,
      args: {
        recipient: address,
      },
    }),
  ]);

  const erc20Logs = [...erc20LogsForSender, ...erc20LogsForRecipient] as unknown as BridgingInitiatedV2LogEvent[];

  const uniqueLogsMap = new Map<string, (typeof erc20Logs)[0]>();
  for (const log of erc20Logs) {
    const transactionHash = log.transactionHash;
    if (!uniqueLogsMap.has(transactionHash)) {
      uniqueLogsMap.set(transactionHash, log);
    }
  }

  await Promise.all(
    Array.from(uniqueLogsMap.values()).map(async (log) => {
      try {
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

        const block = await originLayerClient.getBlock({ blockNumber: log.blockNumber, includeTransactions: false });
        if (isBlockTooOld(block)) return;

        const message = await getMessagesByTransactionHash(originLayerClient as Client, {
          transactionHash,
        });

        if (isUndefinedOrNull(message) || message.length === 0) {
          return;
        }

        const messageStatus =
          fromChain.layer === ChainLayer.L1
            ? await getL1ToL2MessageStatus(destinationLayerClient as Client, {
                messageHash: message[0].messageHash,
              })
            : await getL2ToL1MessageStatus(destinationLayerClient as Client, {
                messageHash: message[0].messageHash,
                l2Client: originLayerClient as Client,
              });

        const token = tokens.find(
          (token) =>
            token.L1?.toLowerCase() === log.args.token.toLowerCase() ||
            token.L2?.toLowerCase() === log.args.token.toLowerCase(),
        );

        if (isUndefined(token)) {
          return;
        }

        const [amount] = decodeAbiParameters([{ type: "uint256", name: "amount" }], log.data);
        const tx = {
          adapterId: "native",
          status: formatOnChainMessageStatus(messageStatus),
          token,
          fromChain,
          toChain,
          timestamp: block.timestamp,
          bridgingTx: log.transactionHash,
          message: {
            from: message[0].from as Address,
            to: message[0].to as Address,
            fee: message[0].fee,
            value: message[0].value,
            nonce: message[0].nonce,
            calldata: message[0].calldata,
            messageHash: message[0].messageHash,
            amountSent: amount,
          },
        };

        saveToTransactionCache(historyStoreActions, tx);
        transactionsMap.set(transactionHash, tx);
      } catch (error) {
        console.error(`Failed to process native ERC20 transaction ${log.transactionHash}:`, error);
      }
    }),
  );

  return Array.from(transactionsMap.values());
}
