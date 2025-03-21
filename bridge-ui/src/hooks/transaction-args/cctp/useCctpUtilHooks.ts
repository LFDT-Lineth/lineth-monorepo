// Break pattern of 1 hook-1 file because TypeScript and CI were going nuts on filenames such as useCctpDestinationDomain.ts

import { useChainStore } from "@/stores";
import { getCctpFee } from "@/services/cctp";
import { useQuery } from "@tanstack/react-query";
import { CCTP_TRANSFER_MAX_FEE_FALLBACK } from "@/utils/cctp";

const useCctpSrcDomain = () => {
  const fromChain = useChainStore.useFromChain();
  return fromChain.cctpDomain;
};

export const useCctpDestinationDomain = () => {
  const toChain = useChainStore.useToChain();
  return toChain.cctpDomain;
};

export const useCctpFee = (): bigint => {
  const fromChain = useChainStore.useFromChain();
  const srcDomain = useCctpSrcDomain();
  const dstDomain = useCctpDestinationDomain();
  const { data } = useQuery({
    queryKey: ["useCctpFee", srcDomain, dstDomain],
    queryFn: async () => getCctpFee(srcDomain, dstDomain, fromChain.testnet),
  });
  if (!data) return CCTP_TRANSFER_MAX_FEE_FALLBACK;
  return BigInt(data.minimumFee);
};
