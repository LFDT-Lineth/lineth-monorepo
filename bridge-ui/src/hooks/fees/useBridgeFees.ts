import { useQuery } from "@tanstack/react-query";
import { useConnection, useConfig } from "wagmi";

import { type BridgeFees, getAdapter } from "@/adapters";
import { DEFAULT_ADDRESS_FOR_NON_CONNECTED_USER } from "@/constants/general";
import { useChainStore } from "@/stores/chainStore";
import { useFormStore } from "@/stores/formStoreProvider";
import { ClaimType } from "@/types";

const DEFAULT_FEES: BridgeFees = {
  protocolFee: null,
  bridgingFee: 0n,
  claimType: ClaimType.AUTO_SPONSORED,
};

export default function useBridgeFees() {
  const { address, isConnected } = useConnection();
  const wagmiConfig = useConfig();
  const fromChain = useChainStore.useFromChain();
  const toChain = useChainStore.useToChain();
  const token = useFormStore((state) => state.token);
  const amount = useFormStore((state) => state.amount);
  const recipient = useFormStore((state) => state.recipient);
  const selectedMode = useFormStore((state) => state.selectedMode);
  const claim = useFormStore((state) => state.claim);

  const adapter = getAdapter(token, fromChain, toChain);
  const fromAddress = isConnected ? address : DEFAULT_ADDRESS_FOR_NON_CONNECTED_USER;
  const toAddress = isConnected ? recipient : DEFAULT_ADDRESS_FOR_NON_CONNECTED_USER;
  const manualClaim = claim === ClaimType.MANUAL;
  const hasPositiveAmount = amount !== null && amount > 0n;
  const shouldFetchFees = !!adapter?.getFees && !!fromAddress && hasPositiveAmount;
  const sourceTokenAddress = token[fromChain.layer];

  const { data, isLoading, isFetching, isError } = useQuery({
    queryKey: [
      "bridgeFees",
      adapter?.id,
      fromChain.id,
      toChain.id,
      sourceTokenAddress,
      amount?.toString(),
      fromAddress,
      toAddress,
      selectedMode,
      manualClaim,
    ],
    queryFn: () =>
      adapter!.getFees!({
        amount: amount ?? 0n,
        token,
        fromChain,
        toChain,
        address: fromAddress!,
        recipient: toAddress,
        wagmiConfig,
        options: { selectedMode: selectedMode ?? undefined, manualClaim },
      }),
    enabled: shouldFetchFees,
    refetchInterval: 30_000,
  });

  const fees = data ?? DEFAULT_FEES;
  const hasValidFeeData = !hasPositiveAmount || (!!adapter?.getFees && !!data && !isError);
  const resolvedClaimType = !isFetching && data ? data.claimType : undefined;

  return { fees, isLoading, hasValidFeeData, resolvedClaimType };
}
