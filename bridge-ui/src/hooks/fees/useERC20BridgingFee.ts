import { Address } from "viem";
import { useQuery } from "@tanstack/react-query";
import { BridgeProvider, Chain, ChainLayer, Token } from "@/types";
import { estimateERC20GasFee, isEth } from "@/utils";

type UseERC20BridgingFeeProps = {
  account?: Address;
  recipient: Address;
  amount: bigint;
  token: Token;
  fromChain: Chain;
  toChain: Chain;
  nextMessageNumber: bigint;
  claimingType: "auto" | "manual";
};

const useERC20BridgingFee = ({
  account,
  token,
  fromChain,
  toChain,
  nextMessageNumber,
  recipient,
  amount,
  claimingType,
}: UseERC20BridgingFeeProps) => {
  const queryKey = [
    "erc20BridgingFee",
    account,
    token,
    fromChain.id,
    toChain.id,
    nextMessageNumber?.toString(),
    amount?.toString(),
    recipient,
  ];

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey,
    enabled:
      !!token &&
      !isEth(token) &&
      token.bridgeProvider === BridgeProvider.NATIVE &&
      fromChain.layer === ChainLayer.L1 &&
      !!account &&
      !!fromChain &&
      !!toChain &&
      !!nextMessageNumber &&
      !!amount &&
      !!recipient &&
      claimingType === "auto",
    queryFn: async () =>
      await estimateERC20GasFee({
        address: account!,
        recipient,
        token,
        amount,
        nextMessageNumber,
        fromChain,
        toChain,
        claimingType,
      }),
  });

  return { data, isLoading, isError, refetch };
};

export default useERC20BridgingFee;
