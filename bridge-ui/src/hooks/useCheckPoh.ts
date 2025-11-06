import { checkPoh } from "@/lib/linea-poh/api";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback } from "react";

export const useCheckPoh = (address: string) => {
  const { data, isLoading, refetch } = useQuery({
    queryKey: ["isHuman", address],
    queryFn: () => checkPoh(address as string),
    staleTime: Infinity,
    enabled: Boolean(address),
  });

  return {
    data,
    isLoading,
    refetch,
  };
};

export const usePrefetchPoh = () => {
  const queryClient = useQueryClient();

  return useCallback(
    (address?: string) => {
      if (!address) return;
      queryClient
        .prefetchQuery({
          queryKey: ["isHuman", address],
          queryFn: () => checkPoh(address),
          staleTime: Infinity,
        })
        .catch(() => {});
    },
    [queryClient],
  );
};
