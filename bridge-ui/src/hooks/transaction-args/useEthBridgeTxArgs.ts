import { useMemo } from "react";
import { encodeFunctionData } from "viem";
import { useFormStore, useChainStore } from "@/stores";
import MessageService from "@/abis/MessageService.json";
import { isEth, isUndefinedOrNull, isZero } from "@/utils";
import { BridgeProvider, ChainLayer } from "@/types";
import { DEFAULT_ADDRESS_FOR_NON_CONNECTED_USER } from "@/constants";

type UseEthBridgeTxArgsProps = {
  isConnected: boolean;
};

const useEthBridgeTxArgs = ({ isConnected }: UseEthBridgeTxArgsProps) => {
  const fromChain = useChainStore.useFromChain();
  const token = useFormStore((state) => state.token);
  const amount = useFormStore((state) => state.amount);
  const recipient = useFormStore((state) => state.recipient);
  const minimumFees = useFormStore((state) => state.minimumFees);
  const bridgingFees = useFormStore((state) => state.bridgingFees);
  const claim = useFormStore((state) => state.claim);

  const toAddress = isConnected ? recipient : DEFAULT_ADDRESS_FOR_NON_CONNECTED_USER;

  return useMemo(() => {
    if (
      !amount ||
      !toAddress ||
      (isZero(minimumFees) && fromChain.layer === ChainLayer.L2) ||
      (isUndefinedOrNull(bridgingFees) && fromChain.layer === ChainLayer.L1) ||
      (isZero(bridgingFees) && claim === "auto") ||
      !isEth(token) ||
      token.bridgeProvider !== BridgeProvider.NATIVE
    ) {
      return;
    }

    return {
      type: "bridge",
      args: {
        to: fromChain.messageServiceAddress,
        data: encodeFunctionData({
          abi: MessageService.abi,
          functionName: "sendMessage",
          args: [toAddress, minimumFees + bridgingFees, "0x"],
        }),
        value: amount + minimumFees + bridgingFees,
        chainId: fromChain.id,
      },
    };
  }, [fromChain, token, amount, toAddress, minimumFees, bridgingFees, claim]);
};

export default useEthBridgeTxArgs;
