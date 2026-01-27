import { getMessageProof } from "@consensys/linea-sdk-viem";
import { useQuery } from "@tanstack/react-query";
import { getPublicClient } from "@wagmi/core";
import { Address, Client, Hex } from "viem";
import { useConfig } from "wagmi";

import { config } from "@/config";
import {
  BridgeTransaction,
  BridgeTransactionType,
  CctpV2BridgeMessage,
  ChainLayer,
  NativeBridgeMessage,
  TransactionStatus,
} from "@/types";
import { isUndefined, isUndefinedOrEmptyString } from "@/utils";
import { isNativeBridgeMessage } from "@/utils/message";

const useBridgeTransactionMessage = (
  transaction: BridgeTransaction | undefined,
): { message: CctpV2BridgeMessage | NativeBridgeMessage | undefined; isLoading: boolean } => {
  const wagmiConfig = useConfig();
  // queryFn for useQuery cannot return undefined - https://tanstack.com/query/latest/docs/framework/react/reference/useQuery
  async function getBridgeTransactionMessage(
    transaction: BridgeTransaction | undefined,
  ): Promise<CctpV2BridgeMessage | NativeBridgeMessage> {
    const { status, type, fromChain, toChain, message } = transaction as BridgeTransaction;
    if (
      isUndefined(status) ||
      isUndefined(type) ||
      isUndefined(fromChain) ||
      isUndefined(toChain) ||
      isUndefined(message)
    )
      return message;
    // Cannot claim, so don't waste time getting claim parameters
    if (status !== TransactionStatus.READY_TO_CLAIM) return message;

    switch (type) {
      case BridgeTransactionType.ETH: {
        if (toChain.layer === ChainLayer.L2) return message;
        if (!isNativeBridgeMessage(message) || isUndefinedOrEmptyString(message?.messageHash)) return message;
        const { messageHash } = message;

        const originLayerClient = getPublicClient(wagmiConfig, { chainId: fromChain.id });
        const destinationLayerClient = getPublicClient(wagmiConfig, { chainId: toChain.id });

        message.proof = await getMessageProof(destinationLayerClient as Client, {
          messageHash: messageHash as Hex,
          l2Client: originLayerClient as Client,
          ...(config.e2eTestMode
            ? {
                lineaRollupAddress: config.chains[toChain.id].messageServiceAddress as Address,
                l2MessageServiceAddress: config.chains[fromChain.id].messageServiceAddress as Address,
              }
            : {}),
        });

        return message;
      }
      case BridgeTransactionType.ERC20: {
        if (toChain.layer === ChainLayer.L2) return message;
        if (!isNativeBridgeMessage(message) || isUndefinedOrEmptyString(message?.messageHash)) return message;
        const { messageHash } = message;

        const originLayerClient = getPublicClient(wagmiConfig, { chainId: fromChain.id });
        const destinationLayerClient = getPublicClient(wagmiConfig, { chainId: toChain.id });

        message.proof = await getMessageProof(destinationLayerClient as Client, {
          messageHash: messageHash as Hex,
          l2Client: originLayerClient as Client,
          ...(config.e2eTestMode
            ? {
                lineaRollupAddress: config.chains[toChain.id].messageServiceAddress as Address,
                l2MessageServiceAddress: config.chains[fromChain.id].messageServiceAddress as Address,
              }
            : {}),
        });
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
    // transaction.status must be a cache key. PENDING -> Don't have correct params, READY_TO_CLAIM -> Have correct params
    queryKey: ["useBridgeTransactionMessage", transaction?.bridgingTx, transaction?.toChain?.id, transaction?.status],
    queryFn: async () => getBridgeTransactionMessage(transaction),
  });

  return { message: data, isLoading };
};

export default useBridgeTransactionMessage;
