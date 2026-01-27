import { erc20Abi } from "viem";
import { useAccount, useBalance, useReadContract } from "wagmi";

import { useChainStore } from "@/stores";
import { Token } from "@/types";
import { isEth } from "@/utils";

const useTokenBalance = (token: Token) => {
  const { address } = useAccount();
  const fromChain = useChainStore.useFromChain();

  const ethBalance = useBalance({
    chainId: fromChain.id,
    address,
    query: {
      enabled: !!address && !!isEth(token),
    },
  });

  const erc20Balance = useReadContract({
    address: token[fromChain.layer] || "0x",
    abi: erc20Abi,
    functionName: "balanceOf",
    args: [address || "0x"],
    chainId: fromChain.id,
    query: {
      enabled: !isEth(token) && !!address,
    },
  });

  const isError = ethBalance.isError || erc20Balance.isError;
  const isLoading = ethBalance.isLoading || erc20Balance.isLoading;
  const balance = isEth(token) ? ethBalance.data?.value : erc20Balance.data;
  const queryKey = ethBalance.queryKey || erc20Balance.queryKey;

  return {
    balance: balance || 0n,
    isError,
    isLoading,
    queryKey,
    refetch: () => {
      if (isEth(token)) {
        ethBalance.refetch();
      } else {
        erc20Balance.refetch();
      }
    },
  };
};

export default useTokenBalance;
