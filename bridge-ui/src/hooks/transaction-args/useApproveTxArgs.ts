import { useMemo } from "react";

import { encodeFunctionData, erc20Abi } from "viem";

import { useChainStore } from "@/stores/chainStore";
import { useFormStore } from "@/stores/formStoreProvider";
import { isNull, isUndefined } from "@/utils/misc";
import { isCctp, isEth } from "@/utils/tokens";

type UseERC20ApproveTxArgsProps = {
  isConnected: boolean;
  allowance?: bigint;
};

const useApproveTxArgs = ({ allowance }: UseERC20ApproveTxArgsProps) => {
  const fromChain = useChainStore.useFromChain();
  const token = useFormStore((state) => state.token);
  const amount = useFormStore((state) => state.amount);

  return useMemo(() => {
    if (isEth(token) || isNull(amount) || isUndefined(allowance) || allowance >= amount) {
      return;
    }

    const spender = !isCctp(token) ? fromChain.tokenBridgeAddress : fromChain.cctpTokenMessengerV2Address;

    return {
      type: "approve",
      args: {
        to: token[fromChain.layer],
        data: encodeFunctionData({
          abi: erc20Abi,
          functionName: "approve",
          args: [spender, amount],
        }),
        value: 0n,
        chainId: fromChain.id,
      },
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [allowance, amount, fromChain.id, token]);
};

export default useApproveTxArgs;
