import { useState, useEffect, useCallback } from "react";
import { useBlockNumber, useEstimateFeesPerGas } from "wagmi";
import { NetworkLayer, TokenInfo } from "@/config";
import { useQueryClient } from "@tanstack/react-query";
import { useChainStore } from "@/stores/chainStore";
import usePostmanFee from "./usePostmanFee";
import { useFormContext } from "react-hook-form";

type useExecutionFeeProps = {
  token: TokenInfo | null;
  claim: string | undefined;
  networkLayer: NetworkLayer;
  minimumFee: bigint;
};

const useExecutionFee = ({ token, claim, networkLayer, minimumFee }: useExecutionFeeProps) => {
  const [minFees, setMinFees] = useState<bigint>(0n);
  const toChain = useChainStore((state) => state.toChain);
  const queryClient = useQueryClient();
  const { data: blockNumber } = useBlockNumber({ watch: true });
  const { data: feeData, queryKey } = useEstimateFeesPerGas({ chainId: toChain?.id, type: "legacy" });
  const { calculatePostmanFee } = usePostmanFee({ currentLayer: networkLayer, claimingType: claim });

  const { watch } = useFormContext();

  const [amount, recipient] = watch(["amount", "recipient"]);

  const calculateFee = useCallback(
    async ({
      claim,
      networkLayer,
      minimumFee,
      gasPrice,
    }: {
      claim: string | undefined;
      networkLayer: NetworkLayer | undefined;
      minimumFee: bigint;
      gasPrice: bigint | undefined;
    }): Promise<bigint | undefined> => {
      const isL1 = networkLayer === NetworkLayer.L1;
      const isL2 = networkLayer === NetworkLayer.L2;
      const isAutoClaim = claim === "auto";
      const isManualClaim = claim === "manual";

      // postman fee
      if (isL1 && isAutoClaim && gasPrice) {
        return calculatePostmanFee(amount, recipient);
      }

      // 0
      if (isL1 && isManualClaim) {
        return BigInt(0);
      }

      // anti-DDoS fee
      if (isL2 && isManualClaim) {
        return minimumFee;
      }

      return undefined;
    },
    [amount, calculatePostmanFee, recipient],
  );

  useEffect(() => {
    if (blockNumber && blockNumber % 5n === 0n) {
      queryClient.invalidateQueries({ queryKey });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [blockNumber, queryClient]);

  useEffect(() => {
    setMinFees(0n);
    if (!token) return;

    async function calculateExecutionFee() {
      const fee = await calculateFee({
        claim,
        networkLayer,
        minimumFee,
        gasPrice: feeData?.gasPrice,
      });

      if (fee !== undefined) {
        setMinFees(fee);
      }
    }

    calculateExecutionFee();
  }, [claim, networkLayer, token, minimumFee, feeData?.gasPrice, calculateFee]);

  return minFees;
};

export default useExecutionFee;
