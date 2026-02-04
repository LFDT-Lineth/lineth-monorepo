import { erc20Abi } from "viem";
import { useConnection, useReadContract } from "wagmi";

import { useChainStore } from "@/stores/chainStore";
import { useFormStore } from "@/stores/formStoreProvider";
import { isCctp, isEth } from "@/utils/tokens";

const useAllowance = () => {
  const { address } = useConnection();
  const token = useFormStore((state) => state.token);
  const fromChain = useChainStore.useFromChain();
  const spender = !isCctp(token) ? fromChain.tokenBridgeAddress : fromChain.cctpTokenMessengerV2Address;

  const {
    data: allowance,
    queryKey,
    refetch,
  } = useReadContract({
    abi: erc20Abi,
    functionName: "allowance",
    args: [address ?? "0x", spender],
    address: token[fromChain.layer] ?? "0x",
    query: {
      enabled: !!token && !isEth(token) && !!address && !!fromChain,
    },
    chainId: fromChain.id,
  });

  return { allowance, queryKey, refetchAllowance: refetch };
};

export default useAllowance;
