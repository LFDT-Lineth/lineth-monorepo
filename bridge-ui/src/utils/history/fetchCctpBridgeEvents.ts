import { Address } from "viem";
import { getPublicClient } from "@wagmi/core";
import { config as wagmiConfig } from "@/lib/wagmi";
import { BridgeTransaction, BridgeTransactionType, Chain, Token, CctpDepositForBurnAbiEvent } from "@/types";
import { isCctp, getCctpMessageByTxHash, getCctpTransactionStatus, isUndefined } from "@/utils";
import { DepositForBurnLogEvent } from "@/types/events";
import { HistoryActionsForCompleteTxCaching } from "@/stores";
import { getCompleteTxStoreKey } from "./getCompleteTxStoreKey";
import { isBlockTooOld } from "./isBlockTooOld";

export async function fetchCctpBridgeEvents(
  historyStoreActions: HistoryActionsForCompleteTxCaching,
  address: Address,
  fromChain: Chain,
  toChain: Chain,
  tokens: Token[],
): Promise<BridgeTransaction[]> {
  const transactionsMap = new Map<string, BridgeTransaction>();
  const fromChainClient = getPublicClient(wagmiConfig, {
    chainId: fromChain.id,
  });

  const usdcLogs = (await fromChainClient.getLogs({
    event: CctpDepositForBurnAbiEvent,
    fromBlock: "earliest",
    toBlock: "latest",
    address: fromChain.cctpTokenMessengerV2Address,
    args: {
      depositor: address,
    },
  })) as unknown as DepositForBurnLogEvent[];

  await Promise.all(
    usdcLogs.map(async (log) => {
      const transactionHash = log.transactionHash;

      // Search cache for completed tx for this txHash, if cache-hit can skip remaining logic
      const cacheKey = getCompleteTxStoreKey(fromChain.id, transactionHash);
      const cachedCompletedTx = historyStoreActions.getCompleteTx(cacheKey);
      if (cachedCompletedTx) {
        transactionsMap.set(transactionHash, cachedCompletedTx);
        return;
      }

      const fromBlock = await fromChainClient.getBlock({ blockNumber: log.blockNumber, includeTransactions: false });
      if (isBlockTooOld(fromBlock)) return;

      const token = tokens.find((token) => isCctp(token));
      if (isUndefined(token)) return;

      const cctpMessage = await getCctpMessageByTxHash(transactionHash, fromChain.cctpDomain, fromChain.testnet);
      if (isUndefined(cctpMessage)) return;
      const nonce = cctpMessage.eventNonce;
      const status = await getCctpTransactionStatus(toChain, cctpMessage, nonce);

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
      };

      // Store COMPLETE tx in cache
      historyStoreActions.setCompleteTx(tx);
      transactionsMap.set(transactionHash, tx);
    }),
  );

  return Array.from(transactionsMap.values());
}
