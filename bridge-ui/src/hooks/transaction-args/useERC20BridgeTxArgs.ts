import { useMemo } from "react";

import { encodeFunctionData } from "viem";

import { TOKEN_BRIDGE_ABI } from "@/abis/TokenBridge";
import { DEFAULT_ADDRESS_FOR_NON_CONNECTED_USER } from "@/constants/general";
import { useChainStore } from "@/stores/chainStore";
import { useFormStore } from "@/stores/formStoreProvider";
import { BridgeProvider, ChainLayer, ClaimType } from "@/types";
import { isNull, isUndefined, isUndefinedOrNull, isZero, isUndefinedOrEmptyString } from "@/utils/misc";
import { isEth } from "@/utils/tokens";

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
          abi: TOKEN_BRIDGE_ABI,
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
