import { useReadContract } from "wagmi";
import MessageService from "@/abis/MessageService.json";
import { Chain, ChainLayer, ClaimType } from "@/types";

type UseMessageNumberProps = {
  fromChain: Chain;
  claimingType: ClaimType;
};

const useMessageNumber = ({ fromChain, claimingType }: UseMessageNumberProps) => {
  const { data } = useReadContract({
    address: fromChain.messageServiceAddress,
    abi: MessageService.abi,
    functionName: "nextMessageNumber",
    chainId: fromChain.id,
    query: {
      enabled: fromChain.layer === ChainLayer.L1 && claimingType === ClaimType.AUTO_PAID,
    },
  });
  return data as bigint;
};

export default useMessageNumber;
