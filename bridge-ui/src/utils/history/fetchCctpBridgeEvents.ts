import { Address } from "viem";
import { getPublicClient } from "@wagmi/core";
import { BridgeTransaction, BridgeTransactionType, CctpDepositForBurnAbiEvent, Chain, Token } from "@/types";
import {
  getCctpMessageByTxHash,
  getCctpModeFromFinalityThreshold,
  getCctpTransactionStatus,
  isCctp,
  isUndefined,
} from "@/utils";
import { DepositForBurnLogEvent } from "@/types/events";
import { HistoryActionsForCompleteTxCaching } from "@/stores";
import { isBlockTooOld } from "./isBlockTooOld";
import { restoreFromTransactionCache } from "./restoreFromTransactionCache";
import { saveToTransactionCache } from "./saveToTransactionCache";
import { Config } from "wagmi";

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
      const transactionHash = log.transactionHash;

      // Search cache for completed tx for this txHash, if cache-hit can skip remaining logic
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

      const token = tokens.find((token) => isCctp(token));
      if (isUndefined(token)) return;

      const cctpMessage = await getCctpMessageByTxHash(transactionHash, fromChain.cctpDomain, fromChain.testnet);
      if (isUndefined(cctpMessage)) return;
      const nonce = cctpMessage.eventNonce;
      const status = await getCctpTransactionStatus(toChain, cctpMessage, nonce, wagmiConfig);

      const tx: BridgeTransaction = {
        type: BridgeTransactionType.USDC,
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
        cctpMode: getCctpModeFromFinalityThreshold(log.args.minFinalityThreshold),
      };

      saveToTransactionCache(historyStoreActions, tx);
      transactionsMap.set(transactionHash, tx);
    }),
  );

  return Array.from(transactionsMap.values());
}
