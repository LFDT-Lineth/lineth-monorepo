import { BridgeTransaction, BridgeTransactionType, TransactionStatus, CctpMessageReceivedAbiEvent } from "@/types";
import { getPublicClient } from "@wagmi/core";
import { config as wagmiConfig } from "@/lib/wagmi";
import { isNativeBridgeMessage, isCctpV2BridgeMessage } from "@/utils/message";
import { useQuery } from "@tanstack/react-query";
import { getNativeBridgeMessageClaimedTxHash } from "@/utils";

const useClaimingTx = (transaction: BridgeTransaction | undefined): string | undefined => {
  // queryFn for useQuery cannot return undefined - https://tanstack.com/query/latest/docs/framework/react/reference/useQuery
  const { data } = useQuery({
    // TODO - Do we need to account for undefined props here? Otherwise caching behaviour is not as expected?
    queryKey: ["useClaimingTx", transaction?.bridgingTx, transaction?.toChain?.id],
    queryFn: async () => getClaimTx(transaction),
  });

  if (!data || data === "") return;
  return data;
};

export default useClaimingTx;

async function getClaimTx(transaction: BridgeTransaction | undefined): Promise<string> {
  if (!transaction) return "";
  if (transaction?.claimingTx) return "";
  const { status, type, toChain, message } = transaction;
  if (!status || !type || !toChain || !message) return "";
  // Not completed -> no existing claim tx
  if (status !== TransactionStatus.COMPLETED) return "";

  const toChainClient = getPublicClient(wagmiConfig, {
    chainId: toChain.id,
  });

  switch (type) {
    case BridgeTransactionType.ETH: {
      if (!isNativeBridgeMessage(message)) return "";
      return await getNativeBridgeMessageClaimedTxHash(
        toChainClient,
        toChain.messageServiceAddress,
        message?.messageHash as `0x${string}`,
      );
    }
    case BridgeTransactionType.ERC20: {
      if (!isNativeBridgeMessage(message)) return "";
      return await getNativeBridgeMessageClaimedTxHash(
        toChainClient,
        toChain.messageServiceAddress,
        message?.messageHash as `0x${string}`,
      );
    }
    case BridgeTransactionType.USDC: {
      if (!isCctpV2BridgeMessage(message) || !message.nonce) return "";
      const messageReceivedEvents = await toChainClient.getLogs({
        event: CctpMessageReceivedAbiEvent,
        fromBlock: "earliest",
        toBlock: "latest",
        address: toChain.cctpMessageTransmitterV2Address,
        args: {
          nonce: message?.nonce,
        },
      });
      if (messageReceivedEvents.length === 0) return "";
      return messageReceivedEvents[0].transactionHash;
    }
    default: {
      return "";
    }
  }
}
