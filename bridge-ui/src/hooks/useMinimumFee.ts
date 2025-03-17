import { useMemo } from "react";
import { useReadContract } from "wagmi";
import { useChainStore } from "@/stores/chainStore";
import { NetworkLayer } from "@/config";
import { getChainNetworkLayer } from "@/utils/chainsUtil";

const useMinimumFee = () => {
  const { messageServiceAddress, fromChain } = useChainStore((state) => ({
    messageServiceAddress: state.messageServiceAddress,
    fromChain: state.fromChain,
  }));

  const isL2Network = useMemo(() => fromChain && getChainNetworkLayer(fromChain) === NetworkLayer.L2, [fromChain]);

  const { data, isLoading, error, queryKey, refetch } = useReadContract({
    address: messageServiceAddress ?? "0x",
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
    chainId: fromChain?.id,
    query: {
      enabled: !!messageServiceAddress && !!fromChain?.id && !!isL2Network,
    },
  });

  return {
    isLoading,
    minimumFee: (data as bigint | undefined) ?? 0n,
    error,
    queryKey,
    refetchMinimumFee: refetch,
  };
};

export default useMinimumFee;
