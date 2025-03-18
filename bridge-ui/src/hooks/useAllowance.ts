import { useAccount, useReadContract } from "wagmi";
import { erc20Abi } from "viem";
import { useFormStore, useChainStore } from "@/stores";
import { isEth } from "@/utils";
import { isCctp } from "@/utils/tokens";
import { CCTP_TOKEN_MESSENGER } from "@/utils/cctp";

const useAllowance = () => {
  const { address } = useAccount();
  const token = useFormStore((state) => state.token);
  const fromChain = useChainStore.useFromChain();
  const spender = !isCctp(token) ? fromChain.tokenBridgeAddress : CCTP_TOKEN_MESSENGER;

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
