import { BridgeTransaction, BridgeTransactionType, TransactionStatus, CctpMessageReceivedAbiEvent } from "@/types";
import { getPublicClient } from "@wagmi/core";
import { config as wagmiConfig } from "@/lib/wagmi";
import { isNativeBridgeMessage, isCctpV2BridgeMessage } from "@/utils/message";
import { useQuery } from "@tanstack/react-query";
import { getNativeBridgeMessageClaimedTxHash } from "@/utils";
import { isUndefinedOrEmptyString, isUndefined } from "@/utils";

const useClaimingTx = (transaction: BridgeTransaction | undefined): string | undefined => {
  // queryFn for useQuery cannot return undefined - https://tanstack.com/query/latest/docs/framework/react/reference/useQuery
  const { data } = useQuery({
    queryKey: ["useClaimingTx", transaction?.bridgingTx, transaction?.toChain?.id, transaction?.status],
    queryFn: async () => getClaimTx(transaction),
  });

  if (isUndefinedOrEmptyString(data)) return;
  return data;
};

export default useClaimingTx;

async function getClaimTx(transaction: BridgeTransaction | undefined): Promise<string> {
  if (isUndefined(transaction)) return "";
  if (transaction?.claimingTx) return "";
  const { status, type, toChain, message } = transaction;
  if (isUndefined(status) || isUndefined(type) || isUndefined(toChain) || isUndefined(message)) return "";
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
      if (!isCctpV2BridgeMessage(message) || isUndefinedOrEmptyString(message.nonce)) return "";
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
