import { Address } from "viem";
import { useQuery } from "@tanstack/react-query";
import { BridgeProvider, Chain, ChainLayer, ClaimType, Token } from "@/types";
import { estimateERC20BridgingGasUsed, isEth } from "@/utils";
import { useConfig } from "wagmi";

type UseERC20BridgingFeeProps = {
  account?: Address;
  recipient: Address;
  amount: bigint;
  token: Token;
  fromChain: Chain;
  toChain: Chain;
  nextMessageNumber: bigint;
  claimingType: ClaimType;
};

const useERC20BridgingGasUsed = ({
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

  const wagmiConfig = useConfig();

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
      (claimingType === ClaimType.AUTO_PAID || claimingType === ClaimType.AUTO_SPONSORED),
    queryFn: async () =>
      await estimateERC20BridgingGasUsed({
        address: account!,
        recipient,
        token,
        amount,
        nextMessageNumber,
        fromChain,
        toChain,
        claimingType,
        wagmiConfig,
      }),
  });

  return { data, isLoading, isError, refetch };
};

export default useERC20BridgingGasUsed;
