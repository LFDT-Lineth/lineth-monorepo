import { useMemo } from "react";

import { useQuery } from "@tanstack/react-query";
import log from "loglevel";
import { Address } from "viem";

import { fetchTokenPrices } from "@/services/tokenService";
import { useConfigStore } from "@/stores/configStore";

type UseTokenPrices = {
  data: Record<string, number>;
  isLoading: boolean;
  refetch: () => void;
  error: Error | null;
};

export default function useTokenPrices(tokenAddresses: Address[], chainId?: number): UseTokenPrices {
  const currency = useConfigStore((state) => state.currency);

  const addressesKey = tokenAddresses.join("-");
  // eslint-disable-next-line react-hooks/exhaustive-deps -- addressesKey is derived from tokenAddresses
  const memoizedTokenAddresses = useMemo(() => tokenAddresses, [addressesKey]);

  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ["tokenPrices", addressesKey, chainId, currency.value],
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
