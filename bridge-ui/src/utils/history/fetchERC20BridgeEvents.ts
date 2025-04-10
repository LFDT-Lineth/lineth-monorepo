import { Address, decodeAbiParameters } from "viem";
import { getPublicClient } from "@wagmi/core";
import { LineaSDK } from "@consensys/linea-sdk";
import { config as wagmiConfig } from "@/lib/wagmi";
import {
  BridgeTransaction,
  BridgeTransactionType,
  Chain,
  ChainLayer,
  Token,
  BridgingInitiatedV2LogEvent,
  BridgingInitiatedV2ABIEvent,
} from "@/types";
import { formatOnChainMessageStatus } from "./formatOnChainMessageStatus";
import { HistoryActionsForCompleteTxCaching } from "@/stores";
import { getCompleteTxStoreKey } from "./getCompleteTxStoreKey";
import { isBlockTooOld } from "./isBlockTooOld";
import { isUndefined, isUndefinedOrNull } from "@/utils";

export async function fetchERC20BridgeEvents(
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

  const [originContract, destinationContract] =
    fromChain.layer === ChainLayer.L1
      ? [lineaSDK.getL1Contract(), lineaSDK.getL2Contract()]
      : [lineaSDK.getL2Contract(), lineaSDK.getL1Contract()];

  const tokenBridgeAddress = fromChain.tokenBridgeAddress;
  const [erc20LogsForSender, erc20LogsForRecipient] = await Promise.all([
    client.getLogs({
      event: BridgingInitiatedV2ABIEvent,
      fromBlock: "earliest",
      toBlock: "latest",
      address: tokenBridgeAddress,
      args: {
        sender: address,
      },
    }),
    client.getLogs({
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
      const transactionHash = log.transactionHash;

      // Search cache for completed tx for this txHash, if cache-hit can skip remaining logic
      const cacheKey = getCompleteTxStoreKey(fromChain.id, transactionHash);
      const cachedCompletedTx = historyStoreActions.getCompleteTx(cacheKey);
      if (cachedCompletedTx) {
        transactionsMap.set(transactionHash, cachedCompletedTx);
        return;
      }

      const block = await client.getBlock({ blockNumber: log.blockNumber, includeTransactions: false });
      if (isBlockTooOld(block)) return;

      const message = await originContract.getMessagesByTransactionHash(transactionHash);
      if (isUndefinedOrNull(message) || message.length === 0) {
        return;
      }

      const messageStatus = await destinationContract.getMessageStatus(message[0].messageHash);

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
        type: BridgeTransactionType.ERC20,
        status: formatOnChainMessageStatus(messageStatus),
        token,
        fromChain,
        toChain,
        timestamp: block.timestamp,
        bridgingTx: log.transactionHash,
        message: {
          from: message[0].messageSender as Address,
          to: message[0].destination as Address,
          fee: message[0].fee,
          value: message[0].value,
          nonce: message[0].messageNonce,
          calldata: message[0].calldata,
          messageHash: message[0].messageHash,
          amountSent: amount,
        },
      };

      // Store COMPLETE tx in cache
      historyStoreActions.setCompleteTx(tx);
      transactionsMap.set(transactionHash, tx);
    }),
  );

  return Array.from(transactionsMap.values());
}
