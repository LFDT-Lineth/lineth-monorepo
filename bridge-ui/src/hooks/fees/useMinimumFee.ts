import { useEffect, useMemo } from "react";

import { useReadContract } from "wagmi";

import { useFormStore, useChainStore } from "@/stores";
import { ChainLayer } from "@/types";

const useMinimumFee = () => {
  const fromChain = useChainStore.useFromChain();
  const setMinimumFees = useFormStore((state) => state.setMinimumFees);

  const isL2Network = useMemo(() => fromChain.layer === ChainLayer.L2, [fromChain.layer]);

  const { data, isLoading, error, queryKey, refetch } = useReadContract({
    address: fromChain.messageServiceAddress,
    abi: [
      {
        inputs: [],
        name: "minimumFeeInWei",
        outputs: [
          {
            internalType: "uint256",
            name: "",
            type: "uint256",
          },
        ],
        stateMutability: "view",
        type: "function",
      },
    ],
    functionName: "minimumFeeInWei",
    chainId: fromChain.id,
    query: {
      enabled: !!isL2Network,
    },
  });

  useEffect(() => {
    if (!isL2Network) {
      setMinimumFees(0n);
    } else if (data) {
      setMinimumFees(data as bigint);
    }
  }, [data, isL2Network, setMinimumFees]);

  return {
    isLoading,
    minimumFee: (data as bigint | undefined) ?? 0n,
    error,
    queryKey,
    refetchMinimumFee: refetch,
  };
};

export default useMinimumFee;
