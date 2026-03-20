import { getPublicClient } from "@wagmi/core";
import { Address } from "viem";
import { Config } from "wagmi";

import { type HistoryActionsForCompleteTxCaching } from "@/stores/historyStore";
import { BridgeTransaction, Chain, Token, TransactionStatus } from "@/types";
import { isBlockTooOld } from "@/utils/history/isBlockTooOld";
import { restoreFromTransactionCache } from "@/utils/history/restoreFromTransactionCache";
import { saveToTransactionCache } from "@/utils/history/saveToTransactionCache";
import { isUndefined } from "@/utils/misc";
import { isCctp } from "@/utils/tokens";

import { CctpDepositForBurnAbiEvent, DepositForBurnLogEvent } from "../events";
import { getCctpMessageByTxHash, getCctpModeFromFinalityThreshold, getCctpTransactionStatus } from "../utils";

export async function fetchCctpBridgeEvents(
  historyStoreActions: HistoryActionsForCompleteTxCaching,
  address: Address,
  fromChain: Chain,
  toChain: Chain,
  tokens: Token[],
  wagmiConfig: Config,
): Promise<BridgeTransaction[]> {
  const transactionsMap = new Map<string, BridgeTransaction>();
  const fromChainClient = getPublicClient(wagmiConfig, {
    chainId: fromChain.id,
  });

  if (!fromChainClient) {
    throw new Error(`No public client found for chain ID ${fromChain.id}`);
  }

  const usdcLogs = (await fromChainClient.getLogs({
    event: CctpDepositForBurnAbiEvent,
    fromBlock: "earliest",
    toBlock: "latest",
    address: fromChain.cctpTokenMessengerV2Address,
    args: {
      depositor: address,
    },
  })) as unknown as DepositForBurnLogEvent[];

  const filteredUSDCLogs = usdcLogs.filter((log) => log.args.destinationDomain === toChain.cctpDomain);

  await Promise.all(
    filteredUSDCLogs.map(async (log) => {
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

        const fromBlock = await fromChainClient.getBlock({
          blockNumber: log.blockNumber,
          includeTransactions: false,
        });
        if (isBlockTooOld(fromBlock)) return;

        const token = tokens.find((token) => isCctp(token));
        if (isUndefined(token)) return;

        const cctpMessage = await getCctpMessageByTxHash(
          transactionHash,
          fromChain.cctpDomain,
          fromChain.testnet,
        ).catch((error) => {
          console.warn(`Failed to fetch CCTP message for ${transactionHash}:`, error);
          return undefined;
        });

        if (isUndefined(cctpMessage)) {
          // Incomplete payload: do not cache (avoids persisting bad data; each refresh retries the API).
          transactionsMap.set(transactionHash, {
            adapterId: "cctp",
            status: TransactionStatus.PENDING,
            token,
            fromChain,
            toChain,
            timestamp: fromBlock.timestamp,
            bridgingTx: log.transactionHash,
            message: {
              amountSent: BigInt(log.args.amount),
              // Placeholder nonce — claim is prevented by missing attestation/message fields (see CCTP adapter claim checks).
              nonce: "0x" as `0x${string}`,
            },
            mode: getCctpModeFromFinalityThreshold(log.args.minFinalityThreshold),
          });
          return;
        }

        const nonce = cctpMessage.eventNonce;
        const status = await getCctpTransactionStatus(toChain, cctpMessage, nonce, wagmiConfig).catch((error) => {
          console.warn(`Failed to resolve CCTP transaction status for ${transactionHash}:`, error);
          return TransactionStatus.PENDING;
        });

        const tx: BridgeTransaction = {
          adapterId: "cctp",
          status,
          token,
          fromChain,
          toChain,
          timestamp: fromBlock.timestamp,
          bridgingTx: log.transactionHash,
          message: {
            amountSent: BigInt(log.args.amount),
            nonce: nonce,
            attestation: cctpMessage.attestation,
            message: cctpMessage.message,
          },
          mode: getCctpModeFromFinalityThreshold(log.args.minFinalityThreshold),
        };

        saveToTransactionCache(historyStoreActions, tx);
        transactionsMap.set(transactionHash, tx);
      } catch (error) {
        console.error(`Failed to process CCTP transaction ${log.transactionHash}:`, error);
      }
    }),
  );

  return Array.from(transactionsMap.values());
}
