import { useEstimateFeesPerGas, useWatchBlockNumber } from "wagmi";
import { SupportedChainIds } from "@/types";

const useFeeData = (chainId: SupportedChainIds) => {
  const { data, refetch } = useEstimateFeesPerGas({ chainId, type: "eip1559" });

  useWatchBlockNumber({
    onBlockNumber: () => refetch(),
    poll: true,
    pollingInterval: 20_000,
  });

  return { feeData: data?.maxFeePerGas };
};

export default useFeeData;
