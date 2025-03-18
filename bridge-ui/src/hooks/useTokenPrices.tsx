import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { Address } from "viem";
import log from "loglevel";
import { fetchTokenPrices } from "@/services/tokenService";
import { useConfigStore } from "@/stores";

type UseTokenPrices = {
  data: Record<string, number>;
  isLoading: boolean;
  refetch: () => void;
  error: Error | null;
};

export default function useTokenPrices(tokenAddresses: Address[], chainId?: number): UseTokenPrices {
  const currency = useConfigStore((state) => state.currency);

  // eslint-disable-next-line react-hooks/exhaustive-deps
  const memoizedTokenAddresses = useMemo(() => tokenAddresses, [JSON.stringify(tokenAddresses)]);

  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ["tokenPrices", memoizedTokenAddresses.join("-"), chainId, currency.value],
    queryFn: () => fetchTokenPrices(memoizedTokenAddresses, currency.value, chainId),
    enabled: !!chainId && memoizedTokenAddresses.length > 0 && [1, 59144].includes(chainId),
    refetchOnWindowFocus: false,
    staleTime: 1000 * 60 * 5,
  });

  const result = useMemo(() => {
    if (isError) {
      log.error("Error in useTokenPrices", { error });
      return { data: {}, isLoading, refetch, error };
    }

    return {
      data: data || {},
      isLoading,
      refetch,
      error: error || null,
    };
  }, [isError, data, isLoading, refetch, error]);

  return result;
}
