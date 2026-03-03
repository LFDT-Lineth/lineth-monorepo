import { erc20Abi } from "viem";
import { useConnection, useReadContract } from "wagmi";

import { getAdapter } from "@/adapters";
import { useChainStore } from "@/stores/chainStore";
import { useFormStore } from "@/stores/formStoreProvider";
import { isEth } from "@/utils/tokens";

const useAllowance = () => {
  const { address } = useConnection();
  const token = useFormStore((state) => state.token);
  const fromChain = useChainStore.useFromChain();
  const toChain = useChainStore.useToChain();

  const adapter = getAdapter(token, fromChain, toChain);
  const spender = adapter?.getApprovalTarget(token, fromChain) ?? fromChain.tokenBridgeAddress;

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
