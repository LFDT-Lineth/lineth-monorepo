import { useAccount } from "wagmi";
import { PiApproximateEqualsBold } from "react-icons/pi";
import { useChainStore } from "@/stores/chainStore";
import { formatBalance } from "@/utils/format";
import useTokenPrices from "@/hooks/useTokenPrices";
import { zeroAddress } from "viem";
import { NetworkType } from "@/config";

type ReceivedAmountProps = {
  receivedAmount?: string;
};

export function ReceivedAmount({ receivedAmount }: ReceivedAmountProps) {
  const { isConnected } = useAccount();
  const { token, fromChain, tokenAddress, networkType } = useChainStore((state) => ({
    token: state.token,
    fromChain: state.fromChain,
    tokenAddress: state.token?.[state.networkLayer] || zeroAddress,
    networkType: state.networkType,
  }));

  const { data: tokenPrices } = useTokenPrices([tokenAddress], fromChain?.id);

  return (
    <div className="flex min-h-20 flex-col gap-2 rounded-lg bg-backgroundColor p-3">
      {isConnected && (
        <>
          <span className="text-2xl font-semibold">
            {(parseFloat(receivedAmount || "0") > 0 && formatBalance(receivedAmount)) || 0} {token?.symbol}
          </span>
          {networkType === NetworkType.MAINNET && (
            <span className="label-text flex items-center">
              {receivedAmount &&
              parseFloat(receivedAmount) > 0 &&
              tokenPrices?.[tokenAddress.toLowerCase()]?.usd &&
              tokenPrices?.[tokenAddress.toLowerCase()]?.usd > 0 ? (
                <>
                  <PiApproximateEqualsBold />
                  {(Number(receivedAmount) * tokenPrices?.[tokenAddress.toLowerCase()]?.usd).toLocaleString("en-US", {
                    style: "currency",
                    currency: "USD",
                    maximumFractionDigits: 4,
                  })}
                </>
              ) : (
                ""
              )}
            </span>
          )}
        </>
      )}
    </div>
  );
}
