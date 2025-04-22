import { Address } from "viem";
import { useQuery } from "@tanstack/react-query";
import { BridgeProvider, Chain, ChainLayer, ClaimType, Token } from "@/types";
import { estimateEthBridgingGasUsed, isEth } from "@/utils";

type UseEthBridgingFeeProps = {
  account?: Address;
  recipient: Address;
  amount: bigint;
  fromChain: Chain;
  toChain: Chain;
  nextMessageNumber: bigint;
  token: Token;
  claimingType: ClaimType;
};

const useEthBridgingGasUsed = ({
  token,
  account,
  recipient,
  amount,
  fromChain,
  toChain,
  nextMessageNumber,
  claimingType,
}: UseEthBridgingFeeProps) => {
  const queryKey = [
    "ethBridgingFee",
    account,
    token.symbol,
    fromChain.id,
    toChain.id,
    nextMessageNumber?.toString(),
    amount?.toString(),
    recipient,
    claimingType,
  ];

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey,
    enabled:
      isEth(token) &&
      token.bridgeProvider === BridgeProvider.NATIVE &&
      fromChain.layer === ChainLayer.L1 &&
      !!account &&
      !!nextMessageNumber &&
      !!amount &&
      !!recipient &&
      (claimingType === ClaimType.AUTO_PAID || claimingType === ClaimType.AUTO_SPONSORED),
    queryFn: async () =>
      await estimateEthBridgingGasUsed({
        address: account!,
        recipient,
        amount,
        nextMessageNumber,
        fromChain,
        toChain,
        claimingType,
      }),
  });

  return { data, isLoading, isError, refetch };
};

export default useEthBridgingGasUsed;
