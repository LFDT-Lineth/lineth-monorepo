import { useMemo } from "react";
import { encodeFunctionData } from "viem";
import { useFormStore, useChainStore } from "@/stores";
import TokenBridge from "@/abis/TokenBridge.json";
import { isEth, isNull, isUndefined, isUndefinedOrNull, isZero, isUndefinedOrEmptyString } from "@/utils";
import { BridgeProvider, ChainLayer, ClaimType } from "@/types";
import { DEFAULT_ADDRESS_FOR_NON_CONNECTED_USER } from "@/constants";

type UseERC20BridgeTxArgsProps = {
  isConnected: boolean;
  allowance?: bigint;
};

const useERC20BridgeTxArgs = ({ isConnected, allowance }: UseERC20BridgeTxArgsProps) => {
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
      isEth(token) ||
      isNull(amount) ||
      (isConnected && (isUndefined(allowance) || allowance < amount)) ||
      isUndefinedOrEmptyString(toAddress) ||
      (isZero(minimumFees) && fromChain.layer === ChainLayer.L2) ||
      (isUndefinedOrNull(bridgingFees) && fromChain.layer === ChainLayer.L1) ||
      (isZero(bridgingFees) && claim === ClaimType.AUTO_PAID) ||
      token.bridgeProvider !== BridgeProvider.NATIVE
    ) {
      return;
    }

    return {
      type: "bridge",
      args: {
        to: fromChain.tokenBridgeAddress,
        data: encodeFunctionData({
          abi: TokenBridge.abi,
          functionName: "bridgeToken",
          args: [token[fromChain.layer], amount, toAddress],
        }),
        value: minimumFees + bridgingFees,
        chainId: fromChain.id,
      },
    };
  }, [allowance, amount, bridgingFees, claim, fromChain, minimumFees, toAddress, token, isConnected]);
};

export default useERC20BridgeTxArgs;
