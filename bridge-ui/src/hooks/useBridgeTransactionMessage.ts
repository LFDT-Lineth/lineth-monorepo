import {
  BridgeTransaction,
  BridgeTransactionType,
  CctpV2BridgeMessage,
  ChainLayer,
  NativeBridgeMessage,
  TransactionStatus,
} from "@/types";
import { isNativeBridgeMessage } from "@/utils/message";
import { useQuery } from "@tanstack/react-query";
import useLineaSDK from "./useLineaSDK";

const useBridgeTransactionMessage = (
  transaction: BridgeTransaction | undefined,
): { message: CctpV2BridgeMessage | NativeBridgeMessage | undefined; isLoading: boolean } => {
  const { lineaSDK } = useLineaSDK();

  // queryFn for useQuery cannot return undefined - https://tanstack.com/query/latest/docs/framework/react/reference/useQuery
  async function getBridgeTransactionMessage(
    transaction: BridgeTransaction | undefined,
  ): Promise<CctpV2BridgeMessage | NativeBridgeMessage> {
    const { status, type, fromChain, toChain, message } = transaction as BridgeTransaction;
    if (!status || !type || !fromChain || !toChain || !message) return message;
    // Cannot claim, so don't waste time getting claim parameters
    if (status !== TransactionStatus.READY_TO_CLAIM) return message;

    switch (type) {
      case BridgeTransactionType.ETH: {
        if (toChain.layer === ChainLayer.L2) return message;
        if (!isNativeBridgeMessage(message) || !message?.messageHash) return message;
        const { messageHash } = message;
        message.proof = await lineaSDK.getL1ClaimingService().getMessageProof(messageHash);
        return message;
      }
      case BridgeTransactionType.ERC20: {
        if (toChain.layer === ChainLayer.L2) return message;
        if (!isNativeBridgeMessage(message) || !message?.messageHash) return message;
        const { messageHash } = message;
        message.proof = await lineaSDK.getL1ClaimingService().getMessageProof(messageHash);
        return message;
      }
      case BridgeTransactionType.USDC: {
        // If message is READY_TO_CLAIM, then we will have already gotten the required params in TransactionList component.
        return message;
      }
      default: {
        return message;
      }
    }
  }

  const { data, isLoading } = useQuery({
    // TODO - Do we need to account for undefined props here? Otherwise caching behaviour is not as expected?
    queryKey: ["useBridgeTransactionMessage", transaction?.bridgingTx, transaction?.toChain?.id],
    queryFn: async () => getBridgeTransactionMessage(transaction),
  });

  return { message: data, isLoading };
};

export default useBridgeTransactionMessage;
