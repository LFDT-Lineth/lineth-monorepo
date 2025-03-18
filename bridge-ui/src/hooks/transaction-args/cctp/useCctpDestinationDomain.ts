import { useChainStore } from "@/stores";

const useCctpDestinationDomain = () => {
  const toChain = useChainStore.useToChain();
  return toChain.cctpDomain;
};

export default useCctpDestinationDomain;
